package fileops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIndexerNonRecursive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "a")
	writeFile(t, filepath.Join(root, "b.markdown"), "b")
	writeFile(t, filepath.Join(root, "ignore.txt"), "x")
	writeFile(t, filepath.Join(root, "sub", "c.md"), "c")

	entries, err := Indexer{Recursive: false}.Snapshot(context.Background(), root)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("non-recursive should find 2 md files, got %d: %v", len(entries), entries)
	}
}

func TestIndexerRecursive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.md"), "a")
	writeFile(t, filepath.Join(root, "sub", "c.md"), "c")

	entries, err := Indexer{Recursive: true}.Snapshot(context.Background(), root)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("recursive should find 2 md files, got %d", len(entries))
	}
}

func TestIndexerListFile(t *testing.T) {
	root := t.TempDir()
	f1 := filepath.Join(root, "x.md")
	f2 := filepath.Join(root, "y.md")
	writeFile(t, f1, "x")
	writeFile(t, f2, "y")
	list := filepath.Join(root, "list.txt")
	writeFile(t, list, "# comment\n"+f1+"\n\n"+f2+"\n"+f1+"\n")

	entries, err := Indexer{}.Snapshot(context.Background(), list)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("list should de-dup to 2 entries, got %d", len(entries))
	}
}

func TestMoverMovesAndRewrites(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in", "doc.md")
	writeFile(t, src, "original")
	destDir := filepath.Join(root, "out", "invoice")

	m := NewMover()
	dest, err := m.Move(src, destDir, []byte("rewritten"))
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if filepath.Base(dest) != "doc.md" {
		t.Fatalf("dest base should be doc.md, got %q", dest)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be removed after move")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "rewritten" {
		t.Fatalf("dest content should be rewritten, got %q", got)
	}
}

func TestMoverInPlaceUpdate(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "invoice")
	src := filepath.Join(dir, "doc.md")
	writeFile(t, src, "old")

	m := NewMover()
	// Destination dir == source dir → in-place update, source not removed.
	dest, err := m.Move(src, dir, []byte("new"))
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if dest != src {
		t.Fatalf("in-place dest should equal src")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "new" {
		t.Fatalf("in-place content should update, got %q", got)
	}
}

func TestRenameOrCopyFallsBackOnEXDEV(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "a.md")
	dst := filepath.Join(root, "b.md")
	writeFile(t, src, "data")

	m := NewMover()
	m.renameFn = func(_, _ string) error { return &os.LinkError{Op: "rename", Err: exdevErr()} }

	if err := m.renameOrCopy(src, dst); err != nil {
		t.Fatalf("renameOrCopy fallback: %v", err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "data" {
		t.Fatalf("copied content mismatch: %q", got)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be removed after copy fallback")
	}
}

// TestMoverNeverOverwritesExistingFile is the core no-data-loss guarantee:
// moving a file into a folder that already holds a file of the same name must
// not clobber the existing file.
func TestMoverNeverOverwritesExistingFile(t *testing.T) {
	root := t.TempDir()
	destDir := filepath.Join(root, "invoice")
	existing := filepath.Join(destDir, "doc.md")
	writeFile(t, existing, "EXISTING - must survive")

	src := filepath.Join(root, "in", "doc.md")
	writeFile(t, src, "incoming")

	m := NewMover()
	dest, err := m.Move(src, destDir, []byte("incoming"))
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	// The existing file must be untouched.
	got, _ := os.ReadFile(existing)
	if string(got) != "EXISTING - must survive" {
		t.Fatalf("existing file was clobbered: %q", got)
	}
	// The incoming file must have landed under a different, non-colliding name.
	if dest == existing {
		t.Fatal("dest must differ from the existing file")
	}
	if filepath.Base(dest) != "doc-1.md" {
		t.Fatalf("expected collision-free name doc-1.md, got %q", filepath.Base(dest))
	}
	gotNew, _ := os.ReadFile(dest)
	if string(gotNew) != "incoming" {
		t.Fatalf("incoming content mismatch: %q", gotNew)
	}
	// Both files exist → nothing lost.
	if _, err := os.Stat(existing); err != nil {
		t.Fatal("existing file disappeared")
	}
}

// TestMoverMultipleCollisions increments the suffix for repeated collisions.
func TestMoverMultipleCollisions(t *testing.T) {
	root := t.TempDir()
	destDir := filepath.Join(root, "out")
	writeFile(t, filepath.Join(destDir, "x.md"), "0")
	writeFile(t, filepath.Join(destDir, "x-1.md"), "1")

	src := filepath.Join(root, "in", "x.md")
	writeFile(t, src, "2")

	m := NewMover()
	dest, err := m.Move(src, destDir, []byte("2"))
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if filepath.Base(dest) != "x-2.md" {
		t.Fatalf("expected x-2.md, got %q", filepath.Base(dest))
	}
}

// TestMoverSourcePreservedOnDestWriteFailure ensures a failure to write the
// destination never removes the source.
func TestMoverSourcePreservedOnDestWriteFailure(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "in", "doc.md")
	writeFile(t, src, "precious")

	// Point destDir at a path blocked by an existing regular file so MkdirAll fails.
	blocker := filepath.Join(root, "blocker")
	writeFile(t, blocker, "x")
	destDir := filepath.Join(blocker, "sub")

	m := NewMover()
	if _, err := m.Move(src, destDir, []byte("new")); err == nil {
		t.Fatal("expected Move to fail when dest dir cannot be created")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must be preserved on failure: %v", err)
	}
	got, _ := os.ReadFile(src)
	if string(got) != "precious" {
		t.Fatalf("source content changed: %q", got)
	}
}
