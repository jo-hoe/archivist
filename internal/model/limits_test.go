package model

import (
	"errors"
	"strings"
	"testing"
)

func TestEnforceStateUnderLimit(t *testing.T) {
	s := State{Body: "short"}
	got, err := EnforceState(s, 100, OverflowError, HeuristicTokenizer{})
	if err != nil || got.Body != "short" {
		t.Fatalf("under-limit should pass unchanged: %v %q", err, got.Body)
	}
}

func TestEnforceStateErrorPolicy(t *testing.T) {
	s := State{Body: strings.Repeat("a", 1000)} // ~250 tokens
	_, err := EnforceState(s, 10, OverflowError, HeuristicTokenizer{})
	if !errors.Is(err, ErrStateTooLong) {
		t.Fatalf("expected ErrStateTooLong, got %v", err)
	}
}

func TestEnforceStateCropPolicy(t *testing.T) {
	s := State{Body: strings.Repeat("a", 1000)}
	got, err := EnforceState(s, 10, OverflowCrop, HeuristicTokenizer{})
	if err != nil {
		t.Fatalf("crop should not error: %v", err)
	}
	if len([]rune(got.Body)) >= 1000 {
		t.Fatalf("crop should shorten body, len=%d", len([]rune(got.Body)))
	}
	if !strings.Contains(got.Body, "…") {
		t.Fatal("cropped body should contain the ellipsis marker")
	}
}

func TestEnforceStateDisabled(t *testing.T) {
	s := State{Body: strings.Repeat("a", 1000)}
	got, err := EnforceState(s, 0, OverflowError, HeuristicTokenizer{})
	if err != nil || got.Body != s.Body {
		t.Fatalf("maxTokens<=0 should disable the check")
	}
}

func TestEnforceOptionsUnderLimit(t *testing.T) {
	q := ChoiceQuestion{Criteria: map[string]string{"a": "A", "b": "B"}, Options: []string{"a", "b"}}
	got, err := EnforceOptions(q, 10, OptionError)
	if err != nil || len(got.Options) != 2 {
		t.Fatalf("under-limit should pass: %v", err)
	}
}

func TestEnforceOptionsErrorPolicy(t *testing.T) {
	q := ChoiceQuestion{Criteria: map[string]string{"a": "A", "b": "B", "c": "C"}, Options: []string{"a", "b", "c"}}
	_, err := EnforceOptions(q, 2, OptionError)
	if !errors.Is(err, ErrTooManyOptions) {
		t.Fatalf("expected ErrTooManyOptions, got %v", err)
	}
}

func TestEnforceOptionsAdaptPolicy(t *testing.T) {
	q := ChoiceQuestion{Criteria: map[string]string{"a": "A", "b": "B", "c": "C"}, Options: []string{"a", "b", "c"}}
	got, err := EnforceOptions(q, 2, OptionAdapt)
	if err != nil {
		t.Fatalf("adapt should not error: %v", err)
	}
	if len(got.Options) != 2 || len(got.Criteria) != 2 {
		t.Fatalf("adapt should trim to 2, got %d options %d criteria", len(got.Options), len(got.Criteria))
	}
	if got.Options[0] != "a" || got.Options[1] != "b" {
		t.Fatalf("adapt should keep the first N in order, got %v", got.Options)
	}
}

func TestEnforceOptionsEmpty(t *testing.T) {
	_, err := EnforceOptions(ChoiceQuestion{}, 10, OptionError)
	if !errors.Is(err, ErrEmptyChoice) {
		t.Fatalf("expected ErrEmptyChoice, got %v", err)
	}
}

func TestHeuristicTokenizer(t *testing.T) {
	tok := HeuristicTokenizer{}
	if tok.Count("") != 0 {
		t.Fatal("empty string should be 0 tokens")
	}
	if tok.Count("abcd") != 1 || tok.Count("abcde") != 2 {
		t.Fatalf("unexpected token counts: %d %d", tok.Count("abcd"), tok.Count("abcde"))
	}
}
