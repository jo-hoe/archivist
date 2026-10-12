// Package descent walks the taxonomy tree by asking a System 1 model to choose
// a subfolder at each level, stopping according to a pluggable StopStrategy.
package descent

import (
	"context"
	"fmt"

	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/taxonomy"
)

// Result reports where a descent landed.
type Result struct {
	// Node is the folder the file should be placed in.
	Node *taxonomy.Node
	// Depth is the number of levels descended from the root (0 = stayed at root).
	Depth int
	// StoppedEarly is true when the walk stopped above a leaf.
	StoppedEarly bool
}

// StopStrategy decides, given the current node and the model's Choice over its
// children, whether to descend (and into which child) or stop here.
type StopStrategy interface {
	// Prepare optionally augments the Choice question before it is asked (e.g.
	// to inject a synthetic "stay here" option). It returns the question to ask
	// and the set of real child keys.
	Prepare(current *taxonomy.Node, children []*taxonomy.Node) model.ChoiceQuestion
	// Decide inspects the answer and returns the chosen child (nil to stop).
	Decide(children []*taxonomy.Node, ans model.ChoiceAnswer) (next *taxonomy.Node, stop bool)
}

// Walker performs the iterative Choice descent.
type Walker struct {
	Provider taxonomy.Provider
	Stop     StopStrategy
	// Instructions is the natural-language framing used for each Choice.
	Instructions string
}

// NewWalker constructs a Walker with a default instruction when none is given.
func NewWalker(p taxonomy.Provider, stop StopStrategy, instructions string) *Walker {
	if instructions == "" {
		instructions = "Select the folder that best fits this document."
	}
	return &Walker{Provider: p, Stop: stop, Instructions: instructions}
}

// Descend walks from the taxonomy root downward, returning the destination node.
// It stops when the current node has no children or the stop strategy says so.
func (w *Walker) Descend(ctx context.Context, m model.System1, state model.State) (Result, error) {
	current, err := w.Provider.Root(ctx)
	if err != nil {
		return Result{}, err
	}

	depth := 0
	for {
		children, err := w.Provider.Children(ctx, current)
		if err != nil {
			return Result{}, err
		}
		if len(children) == 0 {
			return Result{Node: current, Depth: depth, StoppedEarly: false}, nil
		}

		next, stop, err := w.step(ctx, m, state, current, children)
		if err != nil {
			return Result{}, err
		}
		if stop || next == nil {
			return Result{Node: current, Depth: depth, StoppedEarly: true}, nil
		}
		current = next
		depth++
	}
}

// step asks one Choice question and applies the stop strategy.
func (w *Walker) step(ctx context.Context, m model.System1, state model.State, current *taxonomy.Node, children []*taxonomy.Node) (*taxonomy.Node, bool, error) {
	q := w.Stop.Prepare(current, children)
	if q.Instructions == "" {
		q.Instructions = w.Instructions
	}
	ans, err := m.Choice(ctx, state, q)
	if err != nil {
		return nil, false, fmt.Errorf("choice at %q: %w", pathOf(current), err)
	}
	next, stop := w.Stop.Decide(children, ans)
	return next, stop, nil
}

func pathOf(n *taxonomy.Node) string {
	if n.Path == "" {
		return "<root>"
	}
	return n.Path
}

// buildChoice constructs a ChoiceQuestion from children, one option per child.
func buildChoice(children []*taxonomy.Node) model.ChoiceQuestion {
	criteria := make(map[string]string, len(children))
	options := make([]string, 0, len(children))
	for _, c := range children {
		criteria[c.Name] = c.DescribeOr()
		options = append(options, c.Name)
	}
	return model.ChoiceQuestion{Criteria: criteria, Options: options}
}

// childByName finds a child node by its option key.
func childByName(children []*taxonomy.Node, name string) *taxonomy.Node {
	for _, c := range children {
		if c.Name == name {
			return c
		}
	}
	return nil
}
