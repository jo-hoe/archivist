package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jo-hoe/archivist/internal/descent"
	"github.com/jo-hoe/archivist/internal/fileops"
	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/model/modeltest"
	"github.com/jo-hoe/archivist/internal/tagging"
	"github.com/jo-hoe/archivist/internal/taxonomy"
)

func setupTarget(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"invoice/car", "invoice/house", "insurance"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil { //nolint:gosec // G301: test temp dir
			t.Fatal(err)
		}
	}
	return root
}

func newPipeline(t *testing.T, target string, fake *modeltest.Fake, tagger tagging.Strategy, dry bool) *Pipeline {
	t.Helper()
	prov := taxonomy.NewScanProvider(target, map[string]string{
		"invoice":       "Bills and receipts",
		"invoice/car":   "Vehicle invoices",
		"invoice/house": "Home invoices",
		"insurance":     "Insurance policies",
	})
	walker := descent.NewWalker(prov, descent.ThresholdStop{Threshold: 0.6}, "")
	return &Pipeline{
		Model:      fake,
		Walker:     walker,
		Tagger:     tagger,
		Mover:      fileops.NewMover(),
		Cfg:        Config{TagsKey: "tags", DryRun: dry},
		TargetRoot: target,
	}
}

func TestPipelineSortsAndTags(t *testing.T) {
	target := setupTarget(t)
	// Place a source file in an input dir.
	inDir := t.TempDir()
	src := filepath.Join(inDir, "tires.md")
	writeFile(t, src, "---\ntitle: Tire invoice\n---\nInvoice for new tires on the car.\n")

	// Model: invoice (0.95) -> car (0.9).
	fake := &modeltest.Fake{
		ChoiceFn: func(_ model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error) {
			if _, ok := q.Criteria["invoice"]; ok {
				return model.ChoiceAnswer{Choice: "invoice", Confidence: 0.95}, nil
			}
			return model.ChoiceAnswer{Choice: "car", Confidence: 0.9}, nil
		},
	}
	tagger := tagging.NewNoulStrategy(tagging.TagSet{Tags: []tagging.TagDef{
		{Name: "vehicle", Description: "Car-related"},
		{Name: "urgent", Description: "Needs attention"},
	}}, 0.7)
	// vehicle applies (0.9), urgent does not (0.2).
	fake.NoulFn = func(_ model.State, q model.NoulQuestion) (model.NoulAnswer, error) {
		if strings.Contains(q.Instructions, `"vehicle"`) {
			return model.NoulAnswer{Noul: 0.9, NoulRaw: 0.9}, nil
		}
		return model.NoulAnswer{Noul: 0.2, NoulRaw: 0.2}, nil
	}

	p := newPipeline(t, target, fake, tagger, false)
	results, err := p.Run(context.Background(), []fileops.Entry{{AbsPath: src, Rel: "tires.md"}}, ProgressHook{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("file error: %v", r.Err)
	}

	wantDir := filepath.Join(target, "invoice", "car")
	if filepath.Dir(r.Dest) != wantDir {
		t.Fatalf("expected dest in %q, got %q", wantDir, r.Dest)
	}
	if len(r.Tags) != 1 || r.Tags[0] != "vehicle" {
		t.Fatalf("expected [vehicle], got %v", r.Tags)
	}

	// Source gone, destination has merged frontmatter + preserved body.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source should be moved")
	}
	out, _ := os.ReadFile(r.Dest)
	s := string(out)
	if !strings.Contains(s, "title: Tire invoice") || !strings.Contains(s, "vehicle") {
		t.Fatalf("frontmatter not merged correctly: %s", s)
	}
	if !strings.Contains(s, "Invoice for new tires on the car.") {
		t.Fatalf("body not preserved: %s", s)
	}
}

func TestPipelineDryRunMovesNothing(t *testing.T) {
	target := setupTarget(t)
	inDir := t.TempDir()
	src := filepath.Join(inDir, "doc.md")
	writeFile(t, src, "Insurance policy document.\n")

	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("insurance", 0.95)}
	p := newPipeline(t, target, fake, nil, true)
	results, err := p.Run(context.Background(), []fileops.Entry{{AbsPath: src}}, ProgressHook{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if results[0].Err != nil {
		t.Fatalf("dry run error: %v", results[0].Err)
	}
	// Source must still exist in a dry run.
	if _, err := os.Stat(src); err != nil {
		t.Fatal("dry run must not move the source")
	}
	if !strings.Contains(results[0].Dest, "insurance") {
		t.Fatalf("dry run should still report planned dest, got %q", results[0].Dest)
	}
}

func TestPipelineNoTagsOmitsKey(t *testing.T) {
	target := setupTarget(t)
	inDir := t.TempDir()
	src := filepath.Join(inDir, "doc.md")
	writeFile(t, src, "Some generic note.\n")

	fake := &modeltest.Fake{
		ChoiceFn: modeltest.ChoosePick("insurance", 0.95),
		NoulFn: func(_ model.State, _ model.NoulQuestion) (model.NoulAnswer, error) {
			return model.NoulAnswer{Noul: 0.1, NoulRaw: 0.1}, nil // nothing applies
		},
	}
	tagger := tagging.NewNoulStrategy(tagging.TagSet{Tags: []tagging.TagDef{{Name: "x", Description: "X"}}}, 0.7)
	p := newPipeline(t, target, fake, tagger, false)
	results, _ := p.Run(context.Background(), []fileops.Entry{{AbsPath: src}}, ProgressHook{})
	out, _ := os.ReadFile(results[0].Dest)
	if strings.Contains(string(out), "tags:") {
		t.Fatalf("no tags should mean no tags key, got: %s", out)
	}
}

func TestPipelineContinuesOnFileError(t *testing.T) {
	target := setupTarget(t)
	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("insurance", 0.95)}
	p := newPipeline(t, target, fake, nil, false)

	entries := []fileops.Entry{
		{AbsPath: filepath.Join(target, "does-not-exist.md")},
	}
	results, err := p.Run(context.Background(), entries, ProgressHook{})
	if err != nil {
		t.Fatalf("Run should not hard-fail on a per-file error: %v", err)
	}
	if results[0].Err == nil {
		t.Fatal("expected a per-file read error")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: test temp dir
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // G306: test fixture
		t.Fatal(err)
	}
}
