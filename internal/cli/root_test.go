package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jo-hoe/archivist/internal/config"
	"github.com/jo-hoe/archivist/internal/fileops"
	"github.com/jo-hoe/archivist/internal/pipeline"
)

// writeConfig writes a config file in a temp dir and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestConfigFileLoads(t *testing.T) {
	path := writeConfig(t, "source: ./from-file\ntarget: ./out\nmodel: laya\ntagging-threshold: 0.42\n")

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", path, "--source", "./from-flag", "--dry-run"})

	// Capture the resolved config by stubbing the run through PersistentPreRunE +
	// configFromViper via a tiny reimplementation: execute only pre-run wiring.
	// Instead we drive the command far enough to validate precedence using a
	// known-missing source so Build fails fast after config resolution.
	err := root.Execute()
	if err == nil {
		t.Fatal("expected failure resolving a missing source directory")
	}
	// The flag source (./from-flag) must win over the file source (./from-file).
	if !strings.Contains(err.Error(), "from-flag") {
		t.Fatalf("flag should override config file; got error: %v", err)
	}
}

func TestConfigFileValuesApplied(t *testing.T) {
	// With no overriding flag, values come from the file. A missing source dir
	// surfaces the file's source path in the error.
	path := writeConfig(t, "source: ./only-in-file\ntarget: ./out\n")

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", path})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "only-in-file") {
		t.Fatalf("config-file source should be used; got: %v", err)
	}
}

func TestMissingExplicitConfigErrors(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", "does-not-exist.yaml"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("expected read-config error, got: %v", err)
	}
}

func TestEnvOverridesConfigFile(t *testing.T) {
	path := writeConfig(t, "source: ./file-src\ntarget: ./out\n")
	t.Setenv("ARCHIVIST_SOURCE", "./env-src")

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--config", path})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "env-src") {
		t.Fatalf("env var should override config file; got: %v", err)
	}
}

func TestTaggingLabel(t *testing.T) {
	c := config.Defaults()
	if got := taggingLabel(c); got != "off" {
		t.Fatalf("no tags file => off, got %q", got)
	}
	c.TagsFile = "tags.yaml"
	c.Tagging.Strategy = config.TaggingChoice
	if got := taggingLabel(c); got != "choice" {
		t.Fatalf("expected choice, got %q", got)
	}
}

func TestResultLineAndJoin(t *testing.T) {
	if got := join([]string{"a", "b", "c"}); got != "a, b, c" {
		t.Fatalf("join: %q", got)
	}
	ok := resultLine(pipeline.Result{Source: "x.md", Dest: "out/x.md", Tags: []string{"t"}})
	if !strings.Contains(ok, "x.md") || !strings.Contains(ok, "tags: t") {
		t.Fatalf("result line missing parts: %q", ok)
	}
	bad := resultLine(pipeline.Result{Source: "y.md", Err: os.ErrPermission})
	if !strings.Contains(bad, "y.md") || !strings.Contains(bad, "permission") {
		t.Fatalf("error line missing parts: %q", bad)
	}
}

func TestTailGeneric(t *testing.T) {
	in := []fileops.Entry{{Rel: "1"}, {Rel: "2"}, {Rel: "3"}}
	if got := tail(in, 2); len(got) != 2 || got[0].Rel != "2" {
		t.Fatalf("tail(2): %+v", got)
	}
	if got := tail(in, 5); len(got) != 3 {
		t.Fatalf("tail(5) should return all: %+v", got)
	}
}
