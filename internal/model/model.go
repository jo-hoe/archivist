// Package model defines the System 1 Decision Model abstraction and its
// concrete implementations (von, laya). A System 1 model returns typed answers
// with calibrated probabilities rather than generated text.
//
// All implementations speak the same /v1/systemone wire protocol, so the HTTP
// transport is shared and per-model differences are expressed as configuration
// and Capabilities.
package model

import "context"

// System1 is a System 1 Decision Model. Each question type has its own method
// so the public surface stays strongly typed — no untyped maps leak out.
type System1 interface {
	// Name reports the model implementation name, e.g. "von" or "laya".
	Name() string
	// Capabilities reports the model's limits and features.
	Capabilities() Capabilities
	// Choice selects exactly one option and returns per-option probabilities.
	Choice(ctx context.Context, state State, q ChoiceQuestion) (ChoiceAnswer, error)
	// Noul answers a yes/no question with a calibrated probability.
	Noul(ctx context.Context, state State, q NoulQuestion) (NoulAnswer, error)
	// Score rates the state on an ordinal scale.
	Score(ctx context.Context, state State, q ScoreQuestion) (ScoreAnswer, error)
}

// BatchChoicer is an optional capability for models that can answer the same
// Choice question over many states in a single request (e.g. laya /batch).
// Callers must type-assert and consult Capabilities.SupportsBatch first.
type BatchChoicer interface {
	ChoiceBatch(ctx context.Context, states []State, q ChoiceQuestion) ([]ChoiceAnswer, error)
}

// Capabilities carries a model's limits and feature flags. Values are the
// effective limits after applying any user overrides.
type Capabilities struct {
	// MaxStateTokens is the largest input the model accepts, in tokens.
	MaxStateTokens int
	// MaxChoiceOptions is the largest number of options a Choice may contain.
	MaxChoiceOptions int
	// MaxBatch is the largest batch size for BatchChoicer; 0 when unsupported.
	MaxBatch int
	// SupportsBatch reports whether the model implements BatchChoicer.
	SupportsBatch bool
	// Language is "en" for English-only models or "multilingual".
	Language string
}

// State is the document content presented to the model.
type State struct {
	Body string
}

// ChoiceQuestion asks the model to pick one option.
type ChoiceQuestion struct {
	// Instructions is the natural-language framing of the decision.
	Instructions string
	// Criteria maps each option key to a human description used by the model.
	Criteria map[string]string
	// Options is the stable, ordered list of option keys. It fixes iteration
	// order for determinism and limit checks and must match Criteria's keys.
	Options []string
}

// ChoiceAnswer is the result of a ChoiceQuestion.
type ChoiceAnswer struct {
	// Choice is the selected option key.
	Choice string
	// Confidence is the model's calibrated confidence in the selection.
	Confidence float64
	// Probabilities is the full distribution over option keys.
	Probabilities map[string]float64
}

// NoulQuestion asks a yes/no question. Criteria is optional but recommended —
// some models (von) warn that a bare yes/no without criteria is weakly grounded.
type NoulQuestion struct {
	Instructions string
	Criteria     string
}

// NoulAnswer is the result of a NoulQuestion.
type NoulAnswer struct {
	// Noul is the model-reported probability, which may be band-mapped.
	Noul float64
	// NoulRaw is the pre-band probability used for threshold gating. When the
	// model does not expose a raw value it equals Noul.
	NoulRaw float64
	// Confidence is the model's calibrated confidence, when provided.
	Confidence float64
}

// ScoreQuestion rates the state on an ordinal scale defined by Criteria,
// ordered from lowest to highest.
type ScoreQuestion struct {
	Instructions string
	Criteria     []string
}

// ScoreAnswer is the result of a ScoreQuestion.
type ScoreAnswer struct {
	// Score is the calibrated position on the ordinal scale.
	Score float64
	// Confidence is the model's calibrated confidence, when provided.
	Confidence float64
}
