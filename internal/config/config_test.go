package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
}

func TestValidateRequiresSourceAndTarget(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err == nil {
		t.Fatal("expected error without source")
	}
	c.Source = "x"
	if err := c.Validate(); err == nil {
		t.Fatal("expected error without target")
	}
	c.Target = "y"
	if err := c.Validate(); err != nil {
		t.Fatalf("defaults with source+target should validate: %v", err)
	}
}

func TestValidateEnums(t *testing.T) {
	c := Defaults()
	c.Source, c.Target = "s", "t"
	c.Tagging.Strategy = "bogus"
	if err := c.Validate(); err == nil {
		t.Fatal("expected invalid tagging strategy error")
	}
}

func TestValidateThresholdRange(t *testing.T) {
	c := Defaults()
	c.Source, c.Target = "s", "t"
	c.Descent.Threshold = 1.5
	if err := c.Validate(); err == nil {
		t.Fatal("expected threshold range error")
	}
}

func TestLoadTagSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tags.yml")
	mustWriteFile(t, path, []byte("tags:\n  - name: urgent\n    description: Needs attention\n  - name: finance\n"))

	set, err := LoadTagSet(path)
	if err != nil {
		t.Fatalf("LoadTagSet: %v", err)
	}
	if len(set.Tags) != 2 || set.Tags[0].Name != "urgent" || set.Tags[1].Name != "finance" {
		t.Fatalf("unexpected tag set: %+v", set.Tags)
	}
}

func TestLoadTagSetRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yml")
	mustWriteFile(t, path, []byte("tags: []\n"))
	if _, err := LoadTagSet(path); err == nil {
		t.Fatal("expected error for empty tag list")
	}
}

func TestLoadDescriptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desc.yml")
	mustWriteFile(t, path, []byte("descriptions:\n  invoice: Bills\n  invoice/car: Cars\n"))

	d, err := LoadDescriptions(path)
	if err != nil {
		t.Fatalf("LoadDescriptions: %v", err)
	}
	if d["invoice"] != "Bills" || d["invoice/car"] != "Cars" {
		t.Fatalf("unexpected descriptions: %+v", d)
	}
}

func TestLoadDescriptionsEmptyPath(t *testing.T) {
	d, err := LoadDescriptions("")
	if err != nil || len(d) != 0 {
		t.Fatalf("empty path should yield empty map: %v %v", d, err)
	}
}

func TestBuildWiresPipeline(t *testing.T) {
	target := t.TempDir()
	mustMkdirAll(t, filepath.Join(target, "invoice"))
	inDir := t.TempDir()
	mustWriteFile(t, filepath.Join(inDir, "a.md"), []byte("body"))

	c := Defaults()
	c.Source = inDir
	c.Target = target

	built, err := Build(context.Background(), c)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if built.Pipeline == nil || built.Model == nil {
		t.Fatal("build should produce a pipeline and model")
	}
	if len(built.Entries) != 1 {
		t.Fatalf("expected 1 indexed entry, got %d", len(built.Entries))
	}
	if built.Model.Name() != "von" {
		t.Fatalf("default model should be von, got %q", built.Model.Name())
	}
}

func TestBuildTaggingDisabledWithoutFile(t *testing.T) {
	target := t.TempDir()
	inDir := t.TempDir()
	mustWriteFile(t, filepath.Join(inDir, "a.md"), []byte("body"))

	c := Defaults()
	c.Source = inDir
	c.Target = target
	// No TagsFile → tagger should be nil.
	built, err := Build(context.Background(), c)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if built.Pipeline.Tagger != nil {
		t.Fatal("tagging should be disabled without a tags file")
	}
}
