package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jo-hoe/archivist/internal/descent"
	"github.com/jo-hoe/archivist/internal/fileops"
	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/pipeline"
	"github.com/jo-hoe/archivist/internal/tagging"
	"github.com/jo-hoe/archivist/internal/taxonomy"
)

// Build assembles a ready-to-run pipeline and the input snapshot from a
// validated Config. The caller should call Validate first.
type Built struct {
	Pipeline *pipeline.Pipeline
	Entries  []fileops.Entry
	Model    model.System1
}

// Build constructs all components. It resolves the target to an absolute path,
// loads YAML inputs, indexes the source files up front, and wires the pipeline.
func Build(ctx context.Context, c Config) (*Built, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	target, err := filepath.Abs(c.Target)
	if err != nil {
		return nil, fmt.Errorf("resolve target: %w", err)
	}
	if err := os.MkdirAll(target, 0o750); err != nil {
		return nil, fmt.Errorf("create target %q: %w", target, err)
	}

	m, err := model.New(c.modelConfig())
	if err != nil {
		return nil, err
	}

	prov, err := c.taxonomyProvider(target)
	if err != nil {
		return nil, err
	}

	tagger, err := c.tagger()
	if err != nil {
		return nil, err
	}

	entries, err := fileops.Indexer{Recursive: c.Recursive}.Snapshot(ctx, c.Source)
	if err != nil {
		return nil, err
	}

	p := &pipeline.Pipeline{
		Model:  m,
		Walker: descent.NewWalker(prov, c.stopStrategy(), ""),
		Tagger: tagger,
		Mover:  fileops.NewMover(),
		Cfg: pipeline.Config{
			Override: c.OverrideMeta,
			Overflow: c.overflowPolicy(),
			Options:  c.optionPolicy(),
			DryRun:   c.DryRun,
		},
		TargetRoot: target,
	}
	return &Built{Pipeline: p, Entries: entries, Model: m}, nil
}

func (c Config) modelConfig() model.Config {
	return model.Config{
		Name:             c.Model.Name,
		BaseURL:          c.Model.BaseURL,
		ModelID:          c.Model.ModelID,
		APIKey:           c.Model.APIKey,
		Language:         c.Model.Language,
		MaxStateTokens:   c.Model.MaxStateTokens,
		MaxChoiceOptions: c.Model.MaxChoiceOptions,
		Device:           c.Model.Device,
	}
}

func (c Config) taxonomyProvider(target string) (taxonomy.Provider, error) {
	if c.Taxonomy.Strategy == TaxonomyDeclared {
		data, err := os.ReadFile(c.Taxonomy.DeclaredFile)
		if err != nil {
			return nil, fmt.Errorf("read declared taxonomy %q: %w", c.Taxonomy.DeclaredFile, err)
		}
		return taxonomy.NewDeclaredProvider(data)
	}
	descriptions, err := LoadDescriptions(c.Taxonomy.DescriptionsFile)
	if err != nil {
		return nil, err
	}
	return taxonomy.NewScanProvider(target, descriptions), nil
}

func (c Config) tagger() (tagging.Strategy, error) {
	if c.TagsFile == "" {
		return nil, nil // tagging disabled
	}
	set, err := LoadTagSet(c.TagsFile)
	if err != nil {
		return nil, err
	}
	if c.Tagging.Strategy == TaggingChoice {
		return tagging.NewChoiceStrategy(set, c.Tagging.Threshold), nil
	}
	return tagging.NewNoulStrategy(set, c.Tagging.Threshold), nil
}

func (c Config) stopStrategy() descent.StopStrategy {
	if c.Descent.Strategy == DescentStayHere {
		return descent.StayHereStop{Threshold: c.Descent.Threshold}
	}
	return descent.ThresholdStop{Threshold: c.Descent.Threshold}
}

func (c Config) overflowPolicy() model.OverflowPolicy {
	if c.OnOverflow == OverflowCrop {
		return model.OverflowCrop
	}
	return model.OverflowError
}

func (c Config) optionPolicy() model.OptionPolicy {
	if c.OnOptions == OptionsAdapt {
		return model.OptionAdapt
	}
	return model.OptionError
}
