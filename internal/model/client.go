package model

import (
	"context"
	"fmt"
)

// client is the shared concrete System1 implementation. It is model-agnostic:
// all wire-format differences are handled by the injected requestBuilder and
// responseDecoder. Adding a new model never requires editing this file.
type client struct {
	name    string
	caps    Capabilities
	trans   *Transport
	extras  requestExtras
	builder requestBuilder
	decoder responseDecoder
}

func (c *client) Name() string               { return c.name }
func (c *client) Capabilities() Capabilities { return c.caps }

func (c *client) Choice(ctx context.Context, state State, q ChoiceQuestion) (ChoiceAnswer, error) {
	if len(q.Options) == 0 {
		return ChoiceAnswer{}, ErrEmptyChoice
	}
	wq := c.builder.buildChoice(q)
	ans, err := c.trans.ask(ctx, state, wq, c.extras, c.decoder)
	if err != nil {
		return ChoiceAnswer{}, err
	}
	if ans.Choice == nil {
		return ChoiceAnswer{}, fmt.Errorf("%w: choice answer missing 'choice' field", ErrModelUnavailable)
	}
	return ChoiceAnswer{
		Choice:        *ans.Choice,
		Confidence:    ans.Confidence,
		Probabilities: ans.Probabilities,
	}, nil
}

func (c *client) Noul(ctx context.Context, state State, q NoulQuestion) (NoulAnswer, error) {
	wq := c.builder.buildNoul(q)
	ans, err := c.trans.ask(ctx, state, wq, c.extras, c.decoder)
	if err != nil {
		return NoulAnswer{}, err
	}
	if ans.Noul == nil {
		return NoulAnswer{}, fmt.Errorf("%w: noul answer missing 'noul' field", ErrModelUnavailable)
	}
	// Gate on the raw (pre-band) probability when the model exposes it; fall
	// back to the reported value otherwise.
	raw := *ans.Noul
	if ans.NoulRaw != nil {
		raw = *ans.NoulRaw
	}
	return NoulAnswer{Noul: *ans.Noul, NoulRaw: raw, Confidence: ans.Confidence}, nil
}

func (c *client) Score(ctx context.Context, state State, q ScoreQuestion) (ScoreAnswer, error) {
	wq := c.builder.buildScore(q)
	ans, err := c.trans.ask(ctx, state, wq, c.extras, c.decoder)
	if err != nil {
		return ScoreAnswer{}, err
	}
	if ans.Score == nil {
		return ScoreAnswer{}, fmt.Errorf("%w: score answer missing 'score' field", ErrModelUnavailable)
	}
	return ScoreAnswer{Score: *ans.Score, Confidence: ans.Confidence}, nil
}
