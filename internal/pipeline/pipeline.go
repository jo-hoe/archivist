// Package pipeline orchestrates processing one markdown file: parse frontmatter,
// descend the taxonomy to pick a destination, decide tags, merge frontmatter,
// and move the file — collecting per-file results without aborting the batch.
package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/jo-hoe/archivist/internal/descent"
	"github.com/jo-hoe/archivist/internal/fileops"
	"github.com/jo-hoe/archivist/internal/frontmatter"
	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/tagging"
)

// Result records the outcome of processing one file.
type Result struct {
	Source       string
	Dest         string
	DestFolder   string
	Tags         []string
	StoppedEarly bool
	Truncated    bool
	Err          error
}

// Config holds the pipeline's behavior knobs.
type Config struct {
	// TagsKey is the frontmatter key under which tags are written.
	TagsKey string
	// Override replaces pre-existing frontmatter values when true.
	Override bool
	// Overflow and Options control limit enforcement.
	Overflow model.OverflowPolicy
	Options  model.OptionPolicy
	// DryRun computes destinations and tags but performs no moves or writes.
	DryRun bool
	// ModelSeesFrontmatter sends the full raw file content (frontmatter + body)
	// to the model. By default (false) only the content body is sent, so
	// existing metadata does not influence classification.
	ModelSeesFrontmatter bool
}

// Pipeline ties the components together.
type Pipeline struct {
	Model  model.System1
	Walker *descent.Walker
	Tagger tagging.Strategy // nil => skip tagging
	Mover  *fileops.Mover
	Tok    model.Tokenizer
	Cfg    Config
	// TargetRoot is the absolute target directory the taxonomy is rooted at.
	TargetRoot string
}

const defaultTagsKey = "tags"

// ProgressHook observes per-file progress. Any field may be nil.
type ProgressHook struct {
	OnStart func(total int)
	OnFile  func(index int, r Result)
	OnDone  func(results []Result)
}

// Run processes every entry in order and returns the per-file results. A file
// error is recorded in its Result and does not stop the batch; only a nil model
// or walker is a hard precondition failure.
func (p *Pipeline) Run(ctx context.Context, entries []fileops.Entry, hook ProgressHook) ([]Result, error) {
	if p.Model == nil || p.Walker == nil || p.Mover == nil {
		return nil, fmt.Errorf("pipeline requires a model, walker, and mover")
	}
	if hook.OnStart != nil {
		hook.OnStart(len(entries))
	}

	results := make([]Result, 0, len(entries))
	for i, e := range entries {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		r := p.processOne(ctx, e)
		results = append(results, r)
		if hook.OnFile != nil {
			hook.OnFile(i, r)
		}
	}
	if hook.OnDone != nil {
		hook.OnDone(results)
	}
	return results, nil
}

// processOne runs the full flow for a single file. Each step is a small helper
// to keep cyclomatic complexity low.
func (p *Pipeline) processOne(ctx context.Context, e fileops.Entry) Result {
	res := Result{Source: e.AbsPath}

	raw, err := os.ReadFile(e.AbsPath)
	if err != nil {
		res.Err = fmt.Errorf("read %q: %w", e.AbsPath, err)
		return res
	}

	doc, err := frontmatter.Parse(raw)
	if err != nil {
		res.Err = err
		return res
	}

	state := model.State{Body: doc.Body}
	if p.Cfg.ModelSeesFrontmatter {
		state = model.State{Body: string(raw)}
	}
	state, err = model.EnforceState(state, p.Model.Capabilities().MaxStateTokens, p.Cfg.Overflow, p.tokenizer())
	if err != nil {
		res.Err = err
		return res
	}

	dest, err := p.Walker.Descend(ctx, p.Model, state)
	if err != nil {
		res.Err = err
		return res
	}
	res.StoppedEarly = dest.StoppedEarly
	destFolder := p.destFolder(dest.Node.Path)
	res.DestFolder = destFolder

	tags, err := p.decideTags(ctx, state)
	if err != nil {
		res.Err = err
		return res
	}
	res.Tags = tags

	if err := p.applyFrontmatter(&doc, tags); err != nil {
		res.Err = err
		return res
	}

	out, err := doc.Render()
	if err != nil {
		res.Err = err
		return res
	}

	if p.Cfg.DryRun {
		res.Dest = p.previewDest(destFolder, e.AbsPath)
		return res
	}

	finalDest, err := p.Mover.Move(e.AbsPath, destFolder, out)
	res.Dest = finalDest
	if err != nil {
		res.Err = err
	}
	return res
}
