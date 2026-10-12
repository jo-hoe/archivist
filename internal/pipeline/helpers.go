package pipeline

import (
	"context"
	"path/filepath"

	"github.com/jo-hoe/archivist/internal/frontmatter"
	"github.com/jo-hoe/archivist/internal/model"
)

// tokenizer returns the configured tokenizer or a heuristic default.
func (p *Pipeline) tokenizer() model.Tokenizer {
	if p.Tok != nil {
		return p.Tok
	}
	return model.HeuristicTokenizer{}
}

// destFolder resolves a taxonomy node's relative path against the target root.
func (p *Pipeline) destFolder(relPath string) string {
	if relPath == "" {
		return p.TargetRoot
	}
	return filepath.Join(p.TargetRoot, filepath.FromSlash(relPath))
}

// decideTags runs the tagging strategy when configured; nil tagger => no tags.
func (p *Pipeline) decideTags(ctx context.Context, state model.State) ([]string, error) {
	if p.Tagger == nil {
		return nil, nil
	}
	return p.Tagger.Tags(ctx, p.Model, state)
}

// applyFrontmatter merges the tags into the document. When there are no tags the
// tags key is deliberately omitted (no empty list is written).
func (p *Pipeline) applyFrontmatter(doc *frontmatter.Document, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	key := p.Cfg.TagsKey
	if key == "" {
		key = defaultTagsKey
	}
	return frontmatter.Merge(doc, map[string]any{key: tags}, p.Cfg.Override)
}

// previewDest computes the destination path a dry run would use, mirroring the
// mover's base-name behavior without touching disk.
func (p *Pipeline) previewDest(destFolder, src string) string {
	return filepath.Join(destFolder, filepath.Base(src))
}
