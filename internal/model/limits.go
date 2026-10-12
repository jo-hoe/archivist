package model

// This file enforces per-model token and option limits before a request is
// sent. The server remains authoritative, but a cheap local pre-check lets the
// tool fail fast or adapt according to user policy.

// OverflowPolicy decides what happens when a State exceeds the token limit.
type OverflowPolicy int

const (
	// OverflowError rejects oversized input (default).
	OverflowError OverflowPolicy = iota
	// OverflowCrop middle-truncates oversized input to fit.
	OverflowCrop
)

// OptionPolicy decides what happens when a Choice has too many options.
type OptionPolicy int

const (
	// OptionError rejects over-limit option sets (default).
	OptionError OptionPolicy = iota
	// OptionAdapt deterministically trims option sets to the limit.
	OptionAdapt
)

// Tokenizer estimates the token count of a string. The default implementation
// is a coarse heuristic; the server performs the authoritative count.
type Tokenizer interface {
	Count(string) int
}

// HeuristicTokenizer approximates tokens as roughly four characters each.
type HeuristicTokenizer struct{}

const charsPerToken = 4

func (HeuristicTokenizer) Count(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + charsPerToken - 1) / charsPerToken
}

// cropFraction mirrors von's middle-truncation: keep 60% head, 40% tail.
const cropHeadFraction = 0.6

// EnforceState checks a State against maxTokens. When over the limit it either
// errors (OverflowError) or middle-truncates (OverflowCrop). maxTokens <= 0
// disables the check.
func EnforceState(s State, maxTokens int, policy OverflowPolicy, tok Tokenizer) (State, error) {
	if maxTokens <= 0 {
		return s, nil
	}
	got := tok.Count(s.Body)
	if got <= maxTokens {
		return s, nil
	}
	if policy == OverflowError {
		return State{}, &LimitError{Err: ErrStateTooLong, Kind: "tokens", Got: got, Allowed: maxTokens}
	}
	return State{Body: cropMiddle(s.Body, maxTokens)}, nil
}

// cropMiddle keeps the head and tail of body, dropping the middle, so the result
// is approximately maxTokens long. It operates on runes to stay valid UTF-8.
func cropMiddle(body string, maxTokens int) string {
	runes := []rune(body)
	keep := maxTokens * charsPerToken
	if keep >= len(runes) {
		return body
	}
	head := int(float64(keep) * cropHeadFraction)
	tail := keep - head
	return string(runes[:head]) + "\n…\n" + string(runes[len(runes)-tail:])
}

// EnforceOptions checks a ChoiceQuestion against maxOptions. When over the limit
// it either errors (OptionError) or trims to the first maxOptions in Options
// order (OptionAdapt). maxOptions <= 0 disables the check.
func EnforceOptions(q ChoiceQuestion, maxOptions int, policy OptionPolicy) (ChoiceQuestion, error) {
	if len(q.Options) == 0 {
		return ChoiceQuestion{}, ErrEmptyChoice
	}
	if maxOptions <= 0 || len(q.Options) <= maxOptions {
		return q, nil
	}
	if policy == OptionError {
		return ChoiceQuestion{}, &LimitError{Err: ErrTooManyOptions, Kind: "options", Got: len(q.Options), Allowed: maxOptions}
	}

	trimmed := make([]string, maxOptions)
	copy(trimmed, q.Options[:maxOptions])
	criteria := make(map[string]string, maxOptions)
	for _, opt := range trimmed {
		criteria[opt] = q.Criteria[opt]
	}
	return ChoiceQuestion{Instructions: q.Instructions, Criteria: criteria, Options: trimmed}, nil
}
