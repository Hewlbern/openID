//go:build cssstub

// Package metadata provides a parser for the Slug header and converts its contents to metadata.
package metadata

import (
	"fmt"
)

type SlugParser struct{}

func (p *SlugParser) Handle(input map[string]interface{}) error {
	req, ok := input["request"].(map[string]interface{})
	if !ok {
		return nil
	}
	metadata, ok := input["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	headers, ok := req["headers"].(map[string]interface{})
	if !ok {
		return nil
	}
	slug, _ := headers["slug"].(string)
	if slug != "" {
		metadata["slug"] = slug
	}
	return nil
}
