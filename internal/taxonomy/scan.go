package taxonomy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ScanProvider builds the taxonomy by scanning the target directory tree. Folder
// descriptions come from a path-keyed map (loaded from a YAML by the caller);
// paths use forward slashes relative to the root, "" being the root itself.
type ScanProvider struct {
	// RootDir is the absolute or relative target directory to scan.
	RootDir string
	// Descriptions maps a slash-separated relative path to its description.
	Descriptions map[string]string
}

// NewScanProvider constructs a ScanProvider. descriptions may be nil.
func NewScanProvider(rootDir string, descriptions map[string]string) *ScanProvider {
	if descriptions == nil {
		descriptions = map[string]string{}
	}
	return &ScanProvider{RootDir: rootDir, Descriptions: descriptions}
}

// Root returns the taxonomy root node.
func (p *ScanProvider) Root(ctx context.Context) (*Node, error) {
	info, err := os.Stat(p.RootDir)
	if err != nil {
		return nil, fmt.Errorf("stat target root %q: %w", p.RootDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("target root %q is not a directory", p.RootDir)
	}
	return &Node{Name: filepath.Base(p.RootDir), Path: "", Description: p.Descriptions[""]}, nil
}

// Children lists n's immediate subdirectories, attaching descriptions by path.
func (p *ScanProvider) Children(ctx context.Context, n *Node) ([]*Node, error) {
	dir := p.absPath(n.Path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}

	var children []*Node
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		relPath := joinRel(n.Path, e.Name())
		children = append(children, &Node{
			Name:        e.Name(),
			Path:        relPath,
			Description: p.Descriptions[relPath],
		})
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
	return children, nil
}

// absPath resolves a slash-separated relative node path against RootDir.
func (p *ScanProvider) absPath(rel string) string {
	if rel == "" {
		return p.RootDir
	}
	return filepath.Join(p.RootDir, filepath.FromSlash(rel))
}

// joinRel joins a parent slash-path with a child name, keeping forward slashes.
func joinRel(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "/" + child
}
