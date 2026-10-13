package model

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Config describes how to construct a System1 client. Zero-valued limit fields
// fall back to the model's built-in defaults.
type Config struct {
	// Name selects the implementation: "von" (default) or "laya".
	Name string
	// BaseURL is the model server root, e.g. "http://localhost:8000".
	BaseURL string
	// ModelID is the optional model identifier sent in the request body.
	ModelID string
	// APIKey is an optional bearer token.
	APIKey string
	// Language overrides the model's default language ("en"/"multilingual").
	Language string
	// MaxStateTokens overrides the model's token limit; 0 keeps the default.
	MaxStateTokens int
	// MaxChoiceOptions overrides the model's option limit; 0 keeps the default.
	MaxChoiceOptions int
	// Device is a compute-device hint ("auto", "cuda", "mps", "cpu"). It is a
	// request hint for models that accept one (laya); for von the device is
	// selected server-side via its own environment and this value is ignored.
	Device string
	// HTTPClient is optional; a sensible default is used when nil.
	HTTPClient *http.Client
}

// factory builds a System1 from a Config.
type factory func(Config) (System1, error)

// registry maps lowercase model names to their factory functions.
// To add a new model:
//
//  1. Create internal/model/<name>.go implementing newName(Config) (System1, error).
//  2. Add an entry here: "name": newName.
//  3. Add compose/<name>.env and two profile services (CPU + GPU) in docker-compose.yml.
//
// The CLI, config validation, and wire transport require no further changes.
var registry = map[string]factory{
	"von":  newVon,
	"laya": newLaya,
}

// DefaultModelName is used when Config.Name is empty.
const DefaultModelName = "von"

// New constructs a System1 from cfg, dispatching on cfg.Name.
func New(cfg Config) (System1, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Name))
	if name == "" {
		name = DefaultModelName
	}
	build, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown model %q (available: %s)", name, strings.Join(Available(), ", "))
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("model %q requires a base URL", name)
	}
	return build(cfg)
}

// Available returns the registered model names in sorted order.
func Available() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// pick returns override when non-zero, else fallback.
func pick(override, fallback int) int {
	if override > 0 {
		return override
	}
	return fallback
}

func pickStr(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

// deviceHint normalizes a device string into a request hint. "auto" and empty
// mean "let the server choose" and are sent as an empty hint (omitted on the
// wire via omitempty).
func deviceHint(device string) string {
	d := strings.ToLower(strings.TrimSpace(device))
	if d == "" || d == "auto" {
		return ""
	}
	return d
}
