//go:build cssstub

// Package metadata provides a filter that errors on JSON-LD with a plain application/json content-type.
package metadata

import (
	"fmt"
)

type PlainJsonLdFilter struct{}

func (p *PlainJsonLdFilter) Handle(input map[string]interface{}) error {
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
	contentType, _ := headers["content-type"].(string)
	if contentType == "application/json" && linkHasContextRelation(headers["link"]) {
		return fmt.Errorf("JSON-LD is only supported with the application/ld+json content type.")
	}
	return nil
}

// linkHasContextRelation is a placeholder for checking Link header for JSON-LD context.
func linkHasContextRelation(link interface{}) bool {
	// TODO: Implement actual check for JSON-LD context relation
	return false
}
