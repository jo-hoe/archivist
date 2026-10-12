package taxonomy

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// DeclaredProvider serves a taxonomy defined entirely in a YAML document, rather
// than scanned from disk. Target folders are created by the mover when a file is
// placed; the tree here is purely the decision structure.
type DeclaredProvider struct {
	root *Node
}

// declaredNode is the YAML shape for a declared taxonomy.
//
//	name: invoice
//	description: Bills and receipts
//	children:
//	  - name: car
//	    description: Vehicle-related invoices
type declaredNode struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Children    []declaredNode `yaml:"children"`
}

// NewDeclaredProvider parses a declared taxonomy from YAML. The top-level
// document is the root node; its name is informational and its children are the
// first level of choices.
func NewDeclaredProvider(data []byte) (*DeclaredProvider, error) {
	var root declaredNode
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse declared taxonomy: %w", err)
	}
	return &DeclaredProvider{root: convert(root, "")}, nil
}

// convert turns the YAML shape into the internal Node tree, computing slash paths.
func convert(dn declaredNode, parentPath string) *Node {
	path := parentPath
	if parentPath == "" {
		path = "" // root keeps empty path
	}
	node := &Node{Name: dn.Name, Path: path, Description: dn.Description}
	for _, c := range dn.Children {
		childPath := joinRel(path, c.Name)
		node.Children = append(node.Children, convertChild(c, childPath))
	}
	return node
}

func convertChild(dn declaredNode, path string) *Node {
	node := &Node{Name: dn.Name, Path: path, Description: dn.Description}
	for _, c := range dn.Children {
		node.Children = append(node.Children, convertChild(c, joinRel(path, c.Name)))
	}
	return node
}

// Root returns the declared root.
func (p *DeclaredProvider) Root(ctx context.Context) (*Node, error) {
	return p.root, nil
}

// Children returns the pre-parsed children of n.
func (p *DeclaredProvider) Children(ctx context.Context, n *Node) ([]*Node, error) {
	return n.Children, nil
}
