package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/jo-hoe/archivist/internal/tagging"
)

// tagsFile is the YAML shape for the candidate tag list:
//
//	tags:
//	  - name: urgent
//	    description: Needs prompt attention
//	  - name: finance
//	    description: Money-related
type tagsFile struct {
	Tags []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	} `yaml:"tags"`
}

// LoadTagSet reads the candidate tags from a YAML file.
func LoadTagSet(path string) (tagging.TagSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tagging.TagSet{}, fmt.Errorf("read tags file %q: %w", path, err)
	}
	var tf tagsFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return tagging.TagSet{}, fmt.Errorf("parse tags file %q: %w", path, err)
	}
	set := tagging.TagSet{Tags: make([]tagging.TagDef, 0, len(tf.Tags))}
	for _, t := range tf.Tags {
		if t.Name == "" {
			return tagging.TagSet{}, fmt.Errorf("tags file %q has a tag with no name", path)
		}
		set.Tags = append(set.Tags, tagging.TagDef{Name: t.Name, Description: t.Description})
	}
	if len(set.Tags) == 0 {
		return tagging.TagSet{}, fmt.Errorf("tags file %q lists no tags", path)
	}
	return set, nil
}

// descriptionsFile is the YAML shape for path-keyed folder descriptions:
//
//	descriptions:
//	  invoice: Bills and receipts
//	  invoice/car: Vehicle invoices
type descriptionsFile struct {
	Descriptions map[string]string `yaml:"descriptions"`
}

// LoadDescriptions reads path-keyed folder descriptions from a YAML file. An
// empty path returns an empty map (descriptions are optional for the scan
// strategy).
func LoadDescriptions(path string) (map[string]string, error) {
	if path == "" {
		return map[string]string{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read descriptions file %q: %w", path, err)
	}
	var df descriptionsFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return nil, fmt.Errorf("parse descriptions file %q: %w", path, err)
	}
	if df.Descriptions == nil {
		df.Descriptions = map[string]string{}
	}
	return df.Descriptions, nil
}
