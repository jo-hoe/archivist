package tagging

import (
	"context"
	"fmt"

	"github.com/jo-hoe/archivist/internal/model"
)

// NoulStrategy asks a yes/no (Noul) question per candidate tag and applies every
// tag whose raw probability meets or exceeds Threshold. The result may contain
// zero, one, or many tags. This is the default tagging strategy.
type NoulStrategy struct {
	Set       TagSet
	Threshold float64
}

// NewNoulStrategy constructs a NoulStrategy.
func NewNoulStrategy(set TagSet, threshold float64) *NoulStrategy {
	return &NoulStrategy{Set: set, Threshold: threshold}
}

func (*NoulStrategy) Name() string { return "noul" }

// Tags evaluates each tag independently and keeps those above the threshold.
func (s *NoulStrategy) Tags(ctx context.Context, m model.System1, state model.State) ([]string, error) {
	var applied []string
	for _, tag := range s.Set.Tags {
		// Include the tag description in Instructions so it reaches both von
		// and laya without needing model-specific criteria key conventions.
		instructions := fmt.Sprintf("Does the tag %q apply to this document?", tag.Name)
		if desc := tag.describeOr(); desc != tag.Name {
			instructions = fmt.Sprintf("Does the tag %q (%s) apply to this document?", tag.Name, desc)
		}
		q := model.NoulQuestion{Instructions: instructions}
		ans, err := m.Noul(ctx, state, q)
		if err != nil {
			return nil, fmt.Errorf("noul for tag %q: %w", tag.Name, err)
		}
		if ans.NoulRaw >= s.Threshold {
			applied = append(applied, tag.Name)
		}
	}
	return applied, nil
}
