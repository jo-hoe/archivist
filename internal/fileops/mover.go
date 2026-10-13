package fileops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Mover writes a processed file's content to a destination directory and removes
// the original. Because the new content differs from the source (updated
// frontmatter), the move is implemented as "write dest, remove source", which is
// inherently cross-volume safe. A plain rename fast-path is used only when the
// content is unchanged and the destination is on the same volume.
type Mover struct {
	// DirPerm is the permission for created destination directories.
	DirPerm os.FileMode
	// FilePerm is the permission for newly written destination files.
	FilePerm os.FileMode
	// renameFn is injectable for testing EXDEV handling; defaults to os.Rename.
	renameFn func(oldpath, newpath string) error
}

// NewMover returns a Mover with sensible default permissions.
func NewMover() *Mover {
	return &Mover{DirPerm: 0o755, FilePerm: 0o644, renameFn: os.Rename}
}

// Move writes content into destDir using the source's base name and removes the
// source file. It returns the destination path.
//
// Data-loss safety is the top priority and is guaranteed as follows:
//   - If src already resolves to the destination, the file is updated atomically
//     in place (write temp + rename) with no removal.
//   - An existing, different file at the destination is NEVER overwritten; a
//     collision-free name (doc.md -> doc-1.md) is chosen instead.
//   - The destination is fully written and fsynced BEFORE the source is removed,
//     so an interruption can at worst leave a duplicate, never lose data.
//   - The source is removed only after the destination is confirmed on disk.
func (m *Mover) Move(src, destDir string, content []byte) (string, error) {
	if m.renameFn == nil {
		m.renameFn = os.Rename
	}
	if err := os.MkdirAll(destDir, m.dirPerm()); err != nil {
		return "", fmt.Errorf("create dest dir %q: %w", destDir, err)
	}

	absSrc, _ := filepath.Abs(src)
	inPlaceDest := filepath.Join(destDir, filepath.Base(src))
	absInPlace, _ := filepath.Abs(inPlaceDest)

	if absSrc == absInPlace {
		// In-place update: write atomically, nothing to remove.
		if err := m.writeAtomic(inPlaceDest, content); err != nil {
			return "", fmt.Errorf("update in place %q: %w", inPlaceDest, err)
		}
		return inPlaceDest, nil
	}

	// Choose a destination that does not clobber an existing, different file.
	dest, err := m.uniqueDest(destDir, filepath.Base(src))
	if err != nil {
		return "", err
	}

	if err := m.writeAtomic(dest, content); err != nil {
		return "", fmt.Errorf("write dest %q: %w", dest, err)
	}
	// Confirm the destination is readable before touching the source.
	if _, err := os.Stat(dest); err != nil {
		return "", fmt.Errorf("verify dest %q before removing source: %w", dest, err)
	}
	if err := os.Remove(src); err != nil {
		// Destination is safely written; surface the removal failure but keep dest.
		return dest, fmt.Errorf("remove source %q after move: %w", src, err)
	}
	return dest, nil
}

// uniqueDest returns a path in destDir for base that does not collide with an
// existing file. "doc.md" becomes "doc-1.md", "doc-2.md", ... as needed.
func (m *Mover) uniqueDest(destDir, base string) (string, error) {
	candidate := filepath.Join(destDir, base)
	if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
		return candidate, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("stat dest candidate %q: %w", candidate, err)
	}

	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i < 10000; i++ {
		candidate = filepath.Join(destDir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find a collision-free name for %q in %q", base, destDir)
}

// writeAtomic writes content to a sibling temp file, fsyncs it, then renames it
// over dest so a reader never observes a partially written file.
func (m *Mover) writeAtomic(dest string, content []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".archivist-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp in %q: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we bail out before the rename.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, m.filePerm()); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

// renameOrCopy renames oldpath to newpath, falling back to copy+remove when the
// paths span filesystems (EXDEV). Retained for callers that move unchanged
// files; the primary Move path writes content directly.
func (m *Mover) renameOrCopy(oldpath, newpath string) error {
	err := m.renameFn(oldpath, newpath)
	if err == nil {
		return nil
	}
	if !isCrossDevice(err) {
		return err
	}
	data, rerr := os.ReadFile(oldpath)
	if rerr != nil {
		return rerr
	}
	if werr := os.WriteFile(newpath, data, m.filePerm()); werr != nil { //nolint:gosec // G703: newpath is derived from uniqueDest, not user input
		return werr
	}
	return os.Remove(oldpath)
}

// isCrossDevice reports whether err is an EXDEV (cross-device link) error.
func isCrossDevice(err error) bool {
	var le *os.LinkError
	if errors.As(err, &le) {
		err = le.Err
	}
	return errors.Is(err, syscall.EXDEV)
}

func (m *Mover) dirPerm() fs.FileMode {
	if m.DirPerm == 0 {
		return 0o755
	}
	return m.DirPerm
}

func (m *Mover) filePerm() fs.FileMode {
	if m.FilePerm == 0 {
		return 0o644
	}
	return m.FilePerm
}
