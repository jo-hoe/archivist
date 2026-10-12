package frontmatter

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Merge adds key/value pairs into the document's frontmatter.
//
// When override is false (the default) a key already present is left untouched;
// when true, existing keys are replaced. Keys are applied in sorted order for
// deterministic output. A nil or empty add map is a no-op.
func Merge(d *Document, add map[string]any, override bool) error {
	if len(add) == 0 {
		return nil
	}
	if d.Meta == nil || d.Meta.Kind != yaml.MappingNode {
		d.Meta = &yaml.Node{Kind: yaml.MappingNode}
	}

	for _, key := range sortedKeys(add) {
		valNode, err := toNode(add[key])
		if err != nil {
			return fmt.Errorf("encode frontmatter key %q: %w", key, err)
		}
		setKey(d.Meta, key, valNode, override)
	}
	return nil
}

// setKey inserts or updates key in a mapping node. Existing keys are only
// replaced when override is true.
func setKey(mapping *yaml.Node, key string, val *yaml.Node, override bool) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			if override {
				mapping.Content[i+1] = val
			}
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	mapping.Content = append(mapping.Content, keyNode, val)
}

// toNode marshals an arbitrary Go value into a yaml.Node.
func toNode(v any) (*yaml.Node, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return &n, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
