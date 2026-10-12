package cli

import (
	"fmt"
	"io"

	"github.com/jo-hoe/archivist/internal/config"
	"github.com/jo-hoe/archivist/internal/fileops"
	"github.com/jo-hoe/archivist/internal/pipeline"
)

// printPreviewList shows the files that will be processed and the key settings.
func printPreviewList(out io.Writer, entries []fileops.Entry, c config.Config) {
	fmt.Fprintf(out, "Preview — %d file(s) to process\n", len(entries))
	fmt.Fprintf(out, "  model=%s url=%s  tagging=%s  descent=%s  taxonomy=%s\n",
		c.Model.Name, c.Model.BaseURL, taggingLabel(c), c.Descent.Strategy, c.Taxonomy.Strategy)
	if c.DryRun {
		fmt.Fprintln(out, "  (dry run — no files will be moved)")
	}
	for _, e := range entries {
		fmt.Fprintf(out, "  - %s\n", e.Rel)
	}
}

func taggingLabel(c config.Config) string {
	if c.TagsFile == "" {
		return "off"
	}
	return c.Tagging.Strategy
}

// printResult prints one file's outcome.
func printResult(out io.Writer, r pipeline.Result) {
	if r.Err != nil {
		fmt.Fprintf(out, "  ✗ %s: %v\n", relOrBase(r.Source), r.Err)
		return
	}
	tags := "no tags"
	if len(r.Tags) > 0 {
		tags = "tags: " + join(r.Tags)
	}
	marker := "→"
	if r.StoppedEarly {
		marker = "⇢" // stopped above a leaf
	}
	fmt.Fprintf(out, "  ✓ %s %s %s (%s)\n", relOrBase(r.Source), marker, r.Dest, tags)
}

// printSummary prints the final counts.
func printSummary(out io.Writer, results []pipeline.Result, dry bool) {
	var ok, failed int
	for _, r := range results {
		if r.Err != nil {
			failed++
		} else {
			ok++
		}
	}
	verb := "processed"
	if dry {
		verb = "previewed (dry run)"
	}
	fmt.Fprintf(out, "\nDone: %d %s, %d failed.\n", ok, verb, failed)
}

func join(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
