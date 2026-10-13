package tagging

import (
	"context"
	"strings"
	"testing"

	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/model/modeltest"
)

func sampleSet() TagSet {
	return TagSet{Tags: []TagDef{
		{Name: "urgent", Description: "Needs prompt attention"},
		{Name: "finance", Description: "Money-related"},
		{Name: "personal", Description: "Private matters"},
	}}
}

// tagFromInstructions extracts the tag name from the Instructions string built
// by NoulStrategy — format: `Does the tag "<name>" (<description>) apply...?`
func tagFromInstructions(instructions string) string {
	start := strings.Index(instructions, `"`)
	end := strings.LastIndex(instructions, `"`)
	if start < 0 || end <= start {
		return ""
	}
	return instructions[start+1 : end]
}

func TestNoulStrategyAppliesAboveThreshold(t *testing.T) {
	// urgent raw=0.9 (apply), finance raw=0.5 (skip at 0.7), personal raw=0.8 (apply).
	raws := map[string]float64{"urgent": 0.9, "finance": 0.5, "personal": 0.8}
	fake := &modeltest.Fake{
		NoulFn: func(_ model.State, q model.NoulQuestion) (model.NoulAnswer, error) {
			name := tagFromInstructions(q.Instructions)
			raw := raws[name]
			return model.NoulAnswer{Noul: raw, NoulRaw: raw}, nil
		},
	}
	s := NewNoulStrategy(sampleSet(), 0.7)
	got, err := s.Tags(context.Background(), fake, model.State{Body: "x"})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(got) != 2 || got[0] != "urgent" || got[1] != "personal" {
		t.Fatalf("expected [urgent personal], got %v", got)
	}
}

func TestNoulStrategyGatesOnRawNotBanded(t *testing.T) {
	// Banded Noul=0.85 but raw=0.4 → should NOT apply at threshold 0.7.
	fake := &modeltest.Fake{
		NoulFn: func(_ model.State, _ model.NoulQuestion) (model.NoulAnswer, error) {
			return model.NoulAnswer{Noul: 0.85, NoulRaw: 0.4}, nil
		},
	}
	s := NewNoulStrategy(sampleSet(), 0.7)
	got, _ := s.Tags(context.Background(), fake, model.State{Body: "x"})
	if len(got) != 0 {
		t.Fatalf("gating should use raw (0.4), expected no tags, got %v", got)
	}
}

func TestNoulStrategyZeroTags(t *testing.T) {
	fake := &modeltest.Fake{
		NoulFn: func(_ model.State, _ model.NoulQuestion) (model.NoulAnswer, error) {
			return model.NoulAnswer{Noul: 0.1, NoulRaw: 0.1}, nil
		},
	}
	s := NewNoulStrategy(sampleSet(), 0.7)
	got, _ := s.Tags(context.Background(), fake, model.State{Body: "x"})
	if got != nil {
		t.Fatalf("expected nil (no tags), got %v", got)
	}
}

func TestChoiceStrategyPicksOne(t *testing.T) {
	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("finance", 0.8)}
	s := NewChoiceStrategy(sampleSet(), 0)
	got, err := s.Tags(context.Background(), fake, model.State{Body: "x"})
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(got) != 1 || got[0] != "finance" {
		t.Fatalf("expected [finance], got %v", got)
	}
}

func TestChoiceStrategyThresholdSuppresses(t *testing.T) {
	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("finance", 0.3)}
	s := NewChoiceStrategy(sampleSet(), 0.6)
	got, _ := s.Tags(context.Background(), fake, model.State{Body: "x"})
	if got != nil {
		t.Fatalf("low-confidence pick should yield no tag, got %v", got)
	}
}

func TestChoiceStrategyEmptySet(t *testing.T) {
	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("x", 1)}
	s := NewChoiceStrategy(TagSet{}, 0)
	got, _ := s.Tags(context.Background(), fake, model.State{Body: "x"})
	if got != nil {
		t.Fatalf("empty set should yield no tags, got %v", got)
	}
}
