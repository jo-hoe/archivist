// Package fileops handles discovering the input markdown files up front and
// moving processed files into their destination folders in a cross-platform,
// cross-volume-safe way.
package fileops

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// markdownExts are the file extensions treated as markdown.
var markdownExts = map[string]bool{".md": true, ".markdown": true}

// Entry is one input file in the processing snapshot.
type Entry struct {
	// AbsPath is the absolute path to the source file at snapshot time.
	AbsPath string
	// Rel is the path relative to the source root, for display.
	Rel string
}

// Indexer discovers the input set. Source may be a directory (walked) or a text
// file listing one path per line.
type Indexer struct {
	// Recursive walks subdirectories when Source is a directory.
	Recursive bool
}

// Snapshot resolves the full input set once, before any file is moved, so a
// file relocated during the run is never re-processed. Paths are resolved to
// absolute form and de-duplicated.
func (ix Indexer) Snapshot(ctx context.Context, source string) ([]Entry, error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("stat source %q: %w", source, err)
	}
	if info.IsDir() {
		return ix.walkDir(ctx, source)
	}
	return ix.readList(source)
}

// walkDir collects markdown files under a directory.
func (ix Indexer) walkDir(ctx context.Context, root string) ([]Entry, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var entries []Entry
	walk := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !ix.Recursive && path != absRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(path) {
			return nil
		}
		rel, _ := filepath.Rel(absRoot, path)
		entries = append(entries, Entry{AbsPath: path, Rel: filepath.ToSlash(rel)})
		return nil
	}
	if err := filepath.WalkDir(absRoot, walk); err != nil {
		return nil, fmt.Errorf("walk %q: %w", absRoot, err)
	}
	return entries, nil
}

// readList reads newline-separated paths from a list file, skipping blanks and
// lines starting with '#'.
func (ix Indexer) readList(listPath string) ([]Entry, error) {
	f, err := os.Open(listPath)
	if err != nil {
		return nil, fmt.Errorf("open list %q: %w", listPath, err)
	}
	defer func() { _ = f.Close() }()

	seen := map[string]bool{}
	var entries []Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		abs, err := filepath.Abs(line)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", line, err)
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		entries = append(entries, Entry{AbsPath: abs, Rel: filepath.Base(abs)})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read list %q: %w", listPath, err)
	}
	return entries, nil
}

func isMarkdown(path string) bool {
	return markdownExts[strings.ToLower(filepath.Ext(path))]
}
