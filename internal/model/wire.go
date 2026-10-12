package model

// This file defines the /v1/systemone wire protocol DTOs and the per-model
// hooks that absorb the two shape differences between von and laya:
//
//  1. State encoding: von accepts a bare JSON string; laya expects {"body": ...}.
//  2. Answer location: von returns answers at the top level; laya nests them
//     under an "answers" object.
//
// Keeping these DTOs unexported means the JSON quirks never leak into the typed
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

// wireResponse is the decoded response. laya nests per-question answers under
// Answers; von places them at the top level alongside Usage. decodeAnswers in
// transport.go reconciles the two shapes.
type wireResponse struct {
	Answers    map[string]wireAnswer `json:"answers"`
	Usage      wireUsage             `json:"usage"`
	Truncation *bool                 `json:"truncation,omitempty"`
}
