// Package tagging decides which tags to apply to a document using a System 1
// model. Two strategies are provided: Noul (ask yes/no per tag, keep those above
// a confidence threshold) and Choice (pick exactly one tag).
package tagging

import (
	"context"

	"github.com/jo-hoe/archivist/internal/model"
)

// TagDef is one candidate tag with a human description used by the model.
type TagDef struct {
	Name        string
	Description string
}

// describeOr returns the description, falling back to the tag name.
func (t TagDef) describeOr() string {
	if t.Description != "" {
		return t.Description
	}
	return t.Name
}

// TagSet is the ordered set of candidate tags, loaded from a YAML file.
type TagSet struct {
	Tags []TagDef
}

// Strategy decides which tags to apply to a given document state.
type Strategy interface {
	// Tags returns the applied tags (possibly empty). An empty result means the
	// caller must not add a tags key to the frontmatter at all.
	Tags(ctx context.Context, m model.System1, state model.State) ([]string, error)
	// Name reports the strategy name.
	Name() string
}
