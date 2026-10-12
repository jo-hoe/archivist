package tagging

import (
	"context"
	"fmt"

	"github.com/jo-hoe/archivist/internal/model"
)

// ChoiceStrategy asks the model to pick exactly one tag from the candidate set.
// It always returns a single tag (unless the set is empty). An optional
// confidence threshold can suppress a low-confidence pick, yielding no tag.
type ChoiceStrategy struct {
	Set       TagSet
	Threshold float64
}

// NewChoiceStrategy constructs a ChoiceStrategy. A threshold of 0 applies the
// pick unconditionally.
func NewChoiceStrategy(set TagSet, threshold float64) *ChoiceStrategy {
	return &ChoiceStrategy{Set: set, Threshold: threshold}
}

func (*ChoiceStrategy) Name() string { return "choice" }

// Tags returns the single chosen tag, or nothing if the set is empty or the
// pick is below the threshold.
func (s *ChoiceStrategy) Tags(ctx context.Context, m model.System1, state model.State) ([]string, error) {
	if len(s.Set.Tags) == 0 {
		return nil, nil
	}

	criteria := make(map[string]string, len(s.Set.Tags))
	options := make([]string, 0, len(s.Set.Tags))
	for _, tag := range s.Set.Tags {
		criteria[tag.Name] = tag.describeOr()
		options = append(options, tag.Name)
	}

	ans, err := m.Choice(ctx, state, model.ChoiceQuestion{
		Instructions: "Pick the single most appropriate tag for this document.",
		Criteria:     criteria,
		Options:      options,
	})
	if err != nil {
		return nil, fmt.Errorf("choice tagging: %w", err)
	}
	if s.Threshold > 0 && ans.Confidence < s.Threshold {
		return nil, nil
	}
	return []string{ans.Choice}, nil
}
