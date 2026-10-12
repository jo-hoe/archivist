package model

import (
	"errors"
	"fmt"
)

// Sentinel errors for conditions callers may want to branch on with errors.Is.
var (
	// ErrStateTooLong is returned when input exceeds the model's token limit and
	// the overflow policy is to error rather than crop.
	ErrStateTooLong = errors.New("state exceeds model token limit")
	// ErrStateTruncated is returned when the server reports it truncated input.
	ErrStateTruncated = errors.New("state was truncated by the model server")
	// ErrTooManyOptions is returned when a Choice has more options than the
	// model allows and the option policy is to error rather than adapt.
	ErrTooManyOptions = errors.New("choice has more options than the model allows")
	// ErrOverflowRefused is returned when the server refuses an oversized
	// request (HTTP 422).
	ErrOverflowRefused = errors.New("model server refused oversized request")
	// ErrModelUnavailable is returned when the model server cannot be reached
	// or returns an unexpected status.
	ErrModelUnavailable = errors.New("model server unavailable")
	// ErrEmptyChoice is returned when a Choice question has no options.
	ErrEmptyChoice = errors.New("choice question has no options")
)

// LimitError wraps a sentinel with the offending and allowed sizes for context.
type LimitError struct {
	Err     error // one of the sentinels above
	Kind    string
	Got     int
	Allowed int
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%s: %s %d exceeds limit %d", e.Err, e.Kind, e.Got, e.Allowed)
}

func (e *LimitError) Unwrap() error { return e.Err }
