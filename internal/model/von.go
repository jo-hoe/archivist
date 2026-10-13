package model

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// von is a non-autoregressive System 1 model and a drop-in for TypeSafe's Jev
// /v1/systemone protocol. It sends state as a bare JSON string and defaults to
// an 8192-token input budget.
//
// Language selection is server-side: the von checkpoint is chosen at server
// start (e.g. VON_MODEL=multilingual). archivist does not forward a lang field
// on the wire for von.
//
// See github.com/wfzyx/von.

const (
	vonDefaultMaxStateTokens   = 8192
	vonDefaultMaxChoiceOptions = 255
)

func newVon(cfg Config) (System1, error) {
	trans := newTransport(transportOptions{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.ModelID,
		encoder:    stringStateEncoder{},
		httpClient: cfg.HTTPClient,
	})
	return &client{
		name:  "von",
		trans: trans,
		caps: Capabilities{
			MaxStateTokens:   pick(cfg.MaxStateTokens, vonDefaultMaxStateTokens),
			MaxChoiceOptions: pick(cfg.MaxChoiceOptions, vonDefaultMaxChoiceOptions),
		},
		builder: vonBuilder{},
		decoder: vonDecoder{},
	}, nil
}

// vonBuilder encodes typed questions into von wire questions.
// Von uses the same question encoding for all question types.
type vonBuilder struct{}

func (vonBuilder) buildNoul(q NoulQuestion) wireQuestion {
	return wireQuestion{Type: wireTypeNoul, Instructions: q.Instructions}
}

func (vonBuilder) buildChoice(q ChoiceQuestion) wireQuestion {
	return wireQuestion{Type: wireTypeChoice, Instructions: q.Instructions, Criteria: q.Criteria}
}

func (vonBuilder) buildScore(q ScoreQuestion) wireQuestion {
	return wireQuestion{Type: wireTypeScore, Instructions: q.Instructions, Criteria: q.Criteria}
}

// vonDecoder decodes von's response shape: answers live at the top level of
// the JSON object alongside usage and truncation.
type vonDecoder struct{}

func (vonDecoder) decode(body []byte, headers http.Header) (map[string]wireAnswer, wireUsage, *bool, error) {
	// Unmarshal usage and truncation from the known fields first.
	var partial struct {
		Usage      wireUsage `json:"usage"`
		Truncation *bool     `json:"truncation,omitempty"`
	}
	if err := json.Unmarshal(body, &partial); err != nil {
		return nil, wireUsage{}, nil, fmt.Errorf("decode von response: %w", err)
	}

	// von places answers at the top level; iterate all keys and decode those
	// that look like wireAnswer objects (skip reserved keys).
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, wireUsage{}, nil, fmt.Errorf("decode von top-level: %w", err)
	}
	answers := make(map[string]wireAnswer, len(top))
	for key, val := range top {
		if key == "usage" || key == "truncation" || key == "model" {
			continue
		}
		var a wireAnswer
		if err := json.Unmarshal(val, &a); err != nil {
			continue // not an answer object (scalar metadata); skip
		}
		answers[key] = a
	}

	// Trust the truncation header even when the JSON field is absent.
	truncated := partial.Truncation
	if headers.Get("X-Von-Truncated") != "" {
		t := true
		truncated = &t
	}

	return answers, partial.Usage, truncated, nil
}
