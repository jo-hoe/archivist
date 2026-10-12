// Package modeltest provides a scriptable fake System1 implementation for use
// in unit tests across packages.
package modeltest

import (
	"context"

	"github.com/jo-hoe/archivist/internal/model"
)

// Fake is a scriptable System1. Set the handler fields to control responses.
type Fake struct {
	ModelName string
	Caps      model.Capabilities
	ChoiceFn  func(state model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error)
	NoulFn    func(state model.State, q model.NoulQuestion) (model.NoulAnswer, error)
	ScoreFn   func(state model.State, q model.ScoreQuestion) (model.ScoreAnswer, error)
}

func (f *Fake) Name() string { return f.ModelName }

func (f *Fake) Capabilities() model.Capabilities { return f.Caps }

func (f *Fake) Choice(_ context.Context, state model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error) {
	return f.ChoiceFn(state, q)
}

func (f *Fake) Noul(_ context.Context, state model.State, q model.NoulQuestion) (model.NoulAnswer, error) {
	return f.NoulFn(state, q)
}

func (f *Fake) Score(_ context.Context, state model.State, q model.ScoreQuestion) (model.ScoreAnswer, error) {
	return f.ScoreFn(state, q)
}

// ChoosePick returns a ChoiceFn that always selects the given key at the given
// confidence, with a trivial probability distribution.
func ChoosePick(key string, confidence float64) func(model.State, model.ChoiceQuestion) (model.ChoiceAnswer, error) {
	return func(_ model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error) {
		probs := map[string]float64{}
		for _, o := range q.Options {
			probs[o] = 0
		}
		probs[key] = confidence
		return model.ChoiceAnswer{Choice: key, Confidence: confidence, Probabilities: probs}, nil
	}
}
