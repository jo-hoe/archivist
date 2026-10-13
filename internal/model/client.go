package model

import (
	"context"
	"fmt"
)

// client is the shared concrete System1 implementation. von and laya differ
// only in Config (capabilities, encoder, request extras), so the typed<->wire
// mapping lives here once.
type client struct {
	name   string
	caps   Capabilities
	trans  *Transport
	extras requestExtras
}

func (c *client) Name() string               { return c.name }
func (c *client) Capabilities() Capabilities { return c.caps }

func (c *client) Choice(ctx context.Context, state State, q ChoiceQuestion) (ChoiceAnswer, error) {
	if len(q.Options) == 0 {
		return ChoiceAnswer{}, ErrEmptyChoice
	}
	wq := wireQuestion{Type: wireTypeChoice, Instructions: q.Instructions, Criteria: q.Criteria}
	ans, err := c.trans.ask(ctx, state, wq, c.extras)
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
	wq := wireQuestion{Type: wireTypeNoul, Instructions: q.Instructions}
	if q.Criteria != "" {
		// The /v1/systemone protocol requires criteria to be a dictionary for
		// noul questions; wrap a plain description string into one.
		wq.Criteria = map[string]string{"description": q.Criteria}
	}
	ans, err := c.trans.ask(ctx, state, wq, c.extras)
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
	wq := wireQuestion{Type: wireTypeScore, Instructions: q.Instructions, Criteria: q.Criteria}
	ans, err := c.trans.ask(ctx, state, wq, c.extras)
	if err != nil {
		return ScoreAnswer{}, err
	}
	if ans.Score == nil {
		return ScoreAnswer{}, fmt.Errorf("%w: score answer missing 'score' field", ErrModelUnavailable)
	}
	return ScoreAnswer{Score: *ans.Score, Confidence: ans.Confidence}, nil
}
