package model

import "net/http"

// This file defines the /v1/systemone wire protocol DTOs and the three
// per-model Strategy interfaces that absorb per-model wire-format differences:
//
//  1. stateEncoder   — how State is JSON-encoded (string vs {"body":...})
//  2. requestBuilder — how typed questions are encoded into wireQuestion
//  3. responseDecoder — how raw JSON + headers are decoded into answers
//
// Each model provides its own implementations; shared code (client, transport)
// depends only on these interfaces, so adding a new model never requires
// touching those files.
//
// Keeping all DTOs unexported means the JSON quirks never leak into the typed
// public surface in model.go.

// stateEncoder renders a State into the JSON value a given model expects.
type stateEncoder interface {
	encode(State) any
}

// stringStateEncoder sends the body as a bare JSON string (von).
type stringStateEncoder struct{}

func (stringStateEncoder) encode(s State) any { return s.Body }

// bodyStateEncoder wraps the body in {"body": ...} (laya).
type bodyStateEncoder struct{}

func (bodyStateEncoder) encode(s State) any { return map[string]string{"body": s.Body} }

// requestBuilder encodes typed public questions into wire questions.
// Each model provides its own implementation to handle question-format
// differences without touching shared code.
type requestBuilder interface {
	buildNoul(NoulQuestion) wireQuestion
	buildChoice(ChoiceQuestion) wireQuestion
	buildScore(ScoreQuestion) wireQuestion
}

// responseDecoder extracts answers from the server's raw response body and
// HTTP headers. Each model provides its own implementation to handle
// answer-shape differences (von: top-level keys; laya: nested "answers").
type responseDecoder interface {
	decode(body []byte, headers http.Header) (map[string]wireAnswer, wireUsage, *bool, error)
}

// wireRequest is the POST body for /v1/systemone. laya-only fields use omitempty
// so von requests stay clean.
type wireRequest struct {
	Model      string                  `json:"model,omitempty"`
	State      any                     `json:"state"`
	Questions  map[string]wireQuestion `json:"questions"`
	Lang       string                  `json:"lang,omitempty"`
	MaxLen     int                     `json:"max_len,omitempty"`
	HeadMaxLen int                     `json:"head_max_len,omitempty"`
	Device     string                  `json:"device,omitempty"`
}

// wireQuestion is one question in a request. Criteria is a map for choice, a
// []string for score, a string for noul, or omitted.
type wireQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Question type discriminators used on the wire.
const (
	wireTypeChoice = "choice"
	wireTypeNoul   = "noul"
	wireTypeScore  = "score"
)

// wireAnswer is one answer. Pointer fields distinguish "absent" from "zero".
type wireAnswer struct {
	Choice        *string            `json:"choice,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
	NoulRaw       *float64           `json:"noul_raw,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// wireUsage carries token accounting returned by the server.
type wireUsage struct {
	InputTokens int `json:"input_tokens"`
}
