package frontmatter

import (
	"strings"
	"testing"
)

func TestParseNoFrontmatter(t *testing.T) {
	raw := []byte("# Title\n\nBody text.\n")
	d, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.HadFrontmatter {
		t.Fatal("should not detect frontmatter")
	}
	if d.Body != string(raw) {
		t.Fatalf("body mismatch: %q", d.Body)
	}
}

func TestParseExistingFrontmatter(t *testing.T) {
	raw := []byte("---\ntitle: Hello\nauthor: Jo\n---\n# Body\n")
	d, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !d.HadFrontmatter {
		t.Fatal("should detect frontmatter")
	}
	if d.Body != "# Body\n" {
		t.Fatalf("body mismatch: %q", d.Body)
	}
	if len(d.Meta.Content) != 4 { // 2 key/value pairs
		t.Fatalf("expected 2 keys, got %d nodes", len(d.Meta.Content))
	}
}

func TestMergeNoOverrideKeepsExisting(t *testing.T) {
	raw := []byte("---\ntags: [existing]\n---\nbody\n")
	d, _ := Parse(raw)
	if err := Merge(&d, map[string]any{"tags": []string{"new"}}, false); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	out, _ := d.Render()
	if !strings.Contains(string(out), "existing") || strings.Contains(string(out), "new") {
		t.Fatalf("no-override should keep existing tags: %s", out)
	}
}

func TestMergeOverrideReplaces(t *testing.T) {
	raw := []byte("---\ntags: [existing]\n---\nbody\n")
	d, _ := Parse(raw)
	if err := Merge(&d, map[string]any{"tags": []string{"new"}}, true); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	out, _ := d.Render()
	if !strings.Contains(string(out), "new") || strings.Contains(string(out), "existing") {
		t.Fatalf("override should replace tags: %s", out)
	}
}

func TestMergeAddsToFileWithoutFrontmatter(t *testing.T) {
	raw := []byte("# Just a body\n")
	d, _ := Parse(raw)
	if err := Merge(&d, map[string]any{"tags": []string{"a", "b"}}, false); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	out, _ := d.Render()
	s := string(out)
	if !strings.HasPrefix(s, "---\n") || !strings.Contains(s, "tags:") || !strings.HasSuffix(s, "# Just a body\n") {
		t.Fatalf("should prepend frontmatter and keep body: %q", s)
	}
}

func TestEmptyMergeOmitsKey(t *testing.T) {
	// Simulate "no tags found": caller passes an empty add map, so no key appears.
	raw := []byte("# Body only\n")
	d, _ := Parse(raw)
	if err := Merge(&d, map[string]any{}, false); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	out, _ := d.Render()
	if strings.Contains(string(out), "---") {
		t.Fatalf("empty merge should not add a frontmatter block: %q", out)
	}
}

func TestNewlinePreservedCRLF(t *testing.T) {
	raw := []byte("---\r\ntitle: X\r\n---\r\nbody line\r\n")
	d, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.NewLine != "\r\n" {
		t.Fatalf("expected CRLF detection, got %q", d.NewLine)
	}
	_ = Merge(&d, map[string]any{"tags": []string{"z"}}, false)
	out, _ := d.Render()
	if !strings.Contains(string(out), "\r\n") || strings.Contains(string(out), "\n\n\r") {
		t.Fatalf("CRLF should be preserved on render: %q", out)
	}
	if strings.Contains(strings.ReplaceAll(string(out), "\r\n", ""), "\n") {
		t.Fatalf("render should contain no bare LF when CRLF: %q", out)
	}
}

func TestBodyPreservedByteForByte(t *testing.T) {
	body := "# Heading\n\nParagraph with `code` and a list:\n- one\n- two\n"
	raw := []byte("---\nk: v\n---\n" + body)
	d, _ := Parse(raw)
	_ = Merge(&d, map[string]any{"tags": []string{"t"}}, false)
	out, _ := d.Render()
	if !strings.HasSuffix(string(out), body) {
		t.Fatalf("body should be preserved exactly, got tail %q", string(out))
	}
}

func TestKeyOrderPreserved(t *testing.T) {
	raw := []byte("---\nzeta: 1\nalpha: 2\n---\nbody\n")
	d, _ := Parse(raw)
	out, _ := d.Render()
	zi := strings.Index(string(out), "zeta")
	ai := strings.Index(string(out), "alpha")
	if zi < 0 || ai < 0 || zi > ai {
		t.Fatalf("original key order (zeta before alpha) should be preserved: %q", out)
	}
}
