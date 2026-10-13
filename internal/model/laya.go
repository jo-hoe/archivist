package model

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// laya is an encoder-based System 1 model serving the same /v1/systemone
// protocol. It wraps state in {"body": ...}, nests answers under "answers",
// supports English and multilingual checkpoints, and its HTTP server caps
// Choice at 100 options. The English checkpoint defaults to a 512-token budget.
//
// See github.com/NandhaKishorM/laya.

const (
	layaDefaultMaxStateTokens   = 512
	layaDefaultMaxChoiceOptions = 100
	layaLanguage                = "en"
)

func newLaya(cfg Config) (System1, error) {
	trans := newTransport(transportOptions{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.ModelID,
		encoder:    bodyStateEncoder{},
		httpClient: cfg.HTTPClient,
	})
	maxTokens := pick(cfg.MaxStateTokens, layaDefaultMaxStateTokens)
	lang := pickStr(cfg.Language, layaLanguage)
	return &client{
		name:  "laya",
		trans: trans,
		// laya honors max_len/lang/device request fields; forward the effective values.
		extras:  requestExtras{lang: lang, maxLen: maxTokens, device: deviceHint(cfg.Device)},
		builder: layaBuilder{},
		decoder: layaDecoder{},
		caps: Capabilities{
			MaxStateTokens:   maxTokens,
			MaxChoiceOptions: pick(cfg.MaxChoiceOptions, layaDefaultMaxChoiceOptions),
			Language:         lang,
		},
	}, nil
}

// layaBuilder encodes typed questions into laya wire questions.
// laya uses the same question encoding as von (same /v1/systemone protocol).
type layaBuilder struct{}

func (layaBuilder) buildNoul(q NoulQuestion) wireQuestion {
	return wireQuestion{Type: wireTypeNoul, Instructions: q.Instructions}
}

func (layaBuilder) buildChoice(q ChoiceQuestion) wireQuestion {
	return wireQuestion{Type: wireTypeChoice, Instructions: q.Instructions, Criteria: q.Criteria}
}

func (layaBuilder) buildScore(q ScoreQuestion) wireQuestion {
	return wireQuestion{Type: wireTypeScore, Instructions: q.Instructions, Criteria: q.Criteria}
}

// layaDecoder decodes laya's response shape: answers are nested under the
// "answers" key in the response JSON.
type layaDecoder struct{}

func (layaDecoder) decode(body []byte, headers http.Header) (map[string]wireAnswer, wireUsage, *bool, error) {
	var resp struct {
		Answers    map[string]wireAnswer `json:"answers"`
		Usage      wireUsage             `json:"usage"`
		Truncation *bool                 `json:"truncation,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, wireUsage{}, nil, fmt.Errorf("decode laya response: %w", err)
	}
	if resp.Answers == nil {
		resp.Answers = map[string]wireAnswer{}
	}
	return resp.Answers, resp.Usage, resp.Truncation, nil
}
