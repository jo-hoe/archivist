// Package frontmatter parses and rewrites YAML frontmatter in markdown files.
//
// It parses the leading "---" fenced block manually (rather than via a
// parse-only library) so it can merge new keys into existing frontmatter and
// re-serialize while preserving key order, the document body byte-for-byte, and
// the file's newline style across platforms.
package frontmatter

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const fence = "---"

// Document is a parsed markdown file split into its frontmatter and body.
type Document struct {
	// Meta is the frontmatter as a YAML mapping node, preserving key order.
	// It is nil when the file had no frontmatter and none has been added.
	Meta *yaml.Node
	// Body is the content after the frontmatter block, unmodified.
	Body string
	// HadFrontmatter reports whether the source file already had a block.
	HadFrontmatter bool
	// NewLine is the detected newline style, "\n" or "\r\n".
	NewLine string
}

// Parse splits raw markdown into frontmatter and body. Files without a leading
// "---" fence parse into a Document with no Meta and the full content as Body.
func Parse(raw []byte) (Document, error) {
	nl := detectNewline(raw)
	// Normalize to \n for parsing; re-apply nl on render.
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")

	if !strings.HasPrefix(text, fence+"\n") && text != fence {
		return Document{Body: string(raw), NewLine: nl}, nil
	}

	// Find the closing fence on its own line after the opening one.
	rest := text[len(fence)+1:]
	end := findClosingFence(rest)
	if end < 0 {
		// Unterminated fence: treat the whole file as body (not frontmatter).
		return Document{Body: string(raw), NewLine: nl}, nil
	}

	fmText := rest[:end]
	body := rest[end:]
	body = strings.TrimPrefix(body, fence)
	body = strings.TrimPrefix(body, "\n")

	meta := &yaml.Node{}
	if strings.TrimSpace(fmText) != "" {
		if err := yaml.Unmarshal([]byte(fmText), meta); err != nil {
			return Document{}, fmt.Errorf("parse frontmatter yaml: %w", err)
		}
	}
	meta = mappingRoot(meta)

	return Document{
		Meta:           meta,
		Body:           restoreNewline(body, nl),
		HadFrontmatter: true,
		NewLine:        nl,
	}, nil
}

// findClosingFence returns the index in s where a line containing exactly "---"
// begins, or -1 if none is found.
func findClosingFence(s string) int {
	offset := 0
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.TrimRight(line, "\n") == fence {
			return offset
		}
		offset += len(line)
	}
	return -1
}

// mappingRoot normalizes a decoded node to a mapping node, unwrapping the
// document node yaml.Unmarshal produces.
func mappingRoot(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return &yaml.Node{Kind: yaml.MappingNode}
	}
	return n
}

// Render re-emits the document: frontmatter fence block (when Meta is non-empty)
// followed by the body, using the document's newline style.
func (d Document) Render() ([]byte, error) {
	nl := d.NewLine
	if nl == "" {
		nl = "\n"
	}

	if d.Meta == nil || len(d.Meta.Content) == 0 {
		return []byte(d.Body), nil
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(d.Meta); err != nil {
		return nil, fmt.Errorf("encode frontmatter: %w", err)
	}
	_ = enc.Close()

	fmYAML := strings.TrimRight(buf.String(), "\n")

	var out strings.Builder
	out.WriteString(fence + "\n")
	out.WriteString(fmYAML + "\n")
	out.WriteString(fence + "\n")
	out.WriteString(d.Body)

	return []byte(restoreNewline(out.String(), nl)), nil
}

func detectNewline(raw []byte) string {
	if bytes.Contains(raw, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func restoreNewline(s, nl string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if nl == "\r\n" {
		return strings.ReplaceAll(s, "\n", "\r\n")
	}
	return s
}
