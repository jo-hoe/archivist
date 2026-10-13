// Package config defines archivist's runtime configuration and assembles the
// processing pipeline from it. It also loads the user-supplied tags and
// taxonomy-description YAML files.
package config

import (
	"fmt"
	"strings"

	"github.com/jo-hoe/archivist/internal/model"
)

// Strategy name constants used for the string-valued enum fields.
const (
	TaggingNoul   = "noul"
	TaggingChoice = "choice"

	DescentThreshold = "threshold"
	DescentStayHere  = "stayhere"

	TaxonomyScan     = "scan"
	TaxonomyDeclared = "declared"

	OverflowError = "error"
	OverflowCrop  = "crop"

	OptionsError = "error"
	OptionsAdapt = "adapt"
)

// Config is the full, validated runtime configuration. All fields have sensible
// defaults applied by Defaults.
type Config struct {
	// Source is a directory to walk or a text file listing input paths.
	Source string
	// Recursive walks subdirectories when Source is a directory.
	Recursive bool
	// Target is the root directory files are sorted into (required).
	Target string
	// TagsFile is the YAML listing candidate tags; empty disables tagging.
	TagsFile string

	Model    ModelConfig
	Tagging  TaggingConfig
	Descent  DescentConfig
	Taxonomy TaxonomyConfig

	// OverrideMeta replaces pre-existing frontmatter values when true.
	OverrideMeta bool
	// NoPreview skips the preview list (always true in Docker).
	NoPreview bool
	// DryRun computes decisions without moving files.
	DryRun bool
	// OnOverflow is "error" (default) or "crop".
	OnOverflow string
	// OnOptions is "error" (default) or "adapt".
	OnOptions string
}

// ModelConfig configures the System 1 model client.
type ModelConfig struct {
	Name             string
	BaseURL          string
	ModelID          string
	APIKey           string
	Language         string
	MaxStateTokens   int
	MaxChoiceOptions int
	// Device hints the model server's compute device: "auto" (default),
	// "cuda", "mps", "cpu", etc. archivist forwards it; the server honors it.
	Device string
}

// TaggingConfig configures the tagging strategy.
type TaggingConfig struct {
	Strategy  string
	Threshold float64
}

// DescentConfig configures the directory-descent stop strategy.
type DescentConfig struct {
	Strategy  string
	Threshold float64
}

// TaxonomyConfig configures how the target structure is provided.
type TaxonomyConfig struct {
	Strategy string
	// DescriptionsFile is the path-keyed descriptions YAML for the scan strategy.
	DescriptionsFile string
	// DeclaredFile is the full-tree YAML for the declared strategy.
	DeclaredFile string
}

// Defaults returns a Config populated with sensible defaults.
func Defaults() Config {
	return Config{
		Recursive: false,
		Model: ModelConfig{
			Name:    model.DefaultModelName,
			BaseURL: "http://localhost:8000",
			Device:  "auto",
		},
		Tagging:    TaggingConfig{Strategy: TaggingNoul, Threshold: 0.7},
		Descent:    DescentConfig{Strategy: DescentThreshold, Threshold: 0.6},
		Taxonomy:   TaxonomyConfig{Strategy: TaxonomyScan},
		OnOverflow: OverflowError,
		OnOptions:  OptionsError,
	}
}

// Validate checks required fields and enum values, returning a single error
// describing the first problem found.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Source) == "" {
		return fmt.Errorf("source is required")
	}
	if strings.TrimSpace(c.Target) == "" {
		return fmt.Errorf("target is required")
	}
	if err := c.validateEnums(); err != nil {
		return err
	}
	if c.Taxonomy.Strategy == TaxonomyDeclared && c.Taxonomy.DeclaredFile == "" {
		return fmt.Errorf("taxonomy strategy %q requires a declared taxonomy file", TaxonomyDeclared)
	}
	if err := checkThreshold("tagging threshold", c.Tagging.Threshold); err != nil {
		return err
	}
	return checkThreshold("descent threshold", c.Descent.Threshold)
}

func (c Config) validateEnums() error {
	checks := []struct {
		name    string
		got     string
		allowed [2]string
	}{
		{"tagging strategy", c.Tagging.Strategy, [2]string{TaggingNoul, TaggingChoice}},
		{"descent strategy", c.Descent.Strategy, [2]string{DescentThreshold, DescentStayHere}},
		{"taxonomy strategy", c.Taxonomy.Strategy, [2]string{TaxonomyScan, TaxonomyDeclared}},
		{"on-overflow", c.OnOverflow, [2]string{OverflowError, OverflowCrop}},
		{"on-options", c.OnOptions, [2]string{OptionsError, OptionsAdapt}},
	}
	for _, ch := range checks {
		if err := checkEnum(ch.name, ch.got, ch.allowed[0], ch.allowed[1]); err != nil {
			return err
		}
	}
	return nil
}

func checkEnum(name, got string, allowed ...string) error {
	for _, a := range allowed {
		if got == a {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q (allowed: %s)", name, got, strings.Join(allowed, ", "))
}

func checkThreshold(name string, v float64) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("%s must be between 0 and 1, got %v", name, v)
	}
	return nil
}
