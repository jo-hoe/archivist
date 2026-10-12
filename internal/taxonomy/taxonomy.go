// Package taxonomy provides the target directory structure the sorter descends
// through. A Provider yields a tree of Nodes, each carrying a human-readable
// description used as the Choice criteria when the model picks a subfolder.
package taxonomy

import "context"

// Node is one folder in the taxonomy tree.
type Node struct {
	// Name is the folder's base name (the last path segment).
	Name string
	// Path is the folder path relative to the taxonomy root ("" for the root).
	Path string
	// Description is the Choice criteria text; defaults to Name when unset.
	Description string
	// Children are the immediate subfolders.
	Children []*Node
}

// Provider yields the taxonomy tree.
type Provider interface {
	// Root returns the top of the tree.
	Root(ctx context.Context) (*Node, error)
	// Children returns the immediate children of n, lazily when applicable.
	Children(ctx context.Context, n *Node) ([]*Node, error)
}

// DescribeOr returns the node's description, falling back to its name.
func (n *Node) DescribeOr() string {
	if n.Description != "" {
		return n.Description
	}
	return n.Name
}
