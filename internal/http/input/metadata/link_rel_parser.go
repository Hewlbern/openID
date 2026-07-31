//go:build cssstub

// Package metadata provides a parser for Link headers with a specific rel value and adds them as metadata.
package metadata

import (
	"fmt"
)

type LinkRelParser struct {
	LinkRelMap map[string]LinkRelObject
}

type LinkRelObject struct {
	Value     string
	Ephemeral bool
	AllowList []string
}

func (p *LinkRelParser) Handle(input map[string]interface{}) error {
	req, ok := input["request"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid request type")
	}
	metadata, ok := input["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid metadata type")
	}
	headers, ok := req["headers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid headers type")
	}
	linkHeader, _ := headers["link"].(string)
	links := parseLinkHeader(linkHeader)
	for _, link := range links {
		if obj, ok := p.LinkRelMap[link.Rel]; ok {
			obj.AddToMetadata(link.Target, metadata)
		}
	}
	return nil
}

type LinkHeader struct {
	Target     string
	Parameters map[string]string
	Rel        string
}

// parseLinkHeader is a placeholder for parsing Link headers.
func parseLinkHeader(header string) []LinkHeader {
	// TODO: Implement actual Link header parsing
	return nil
}

func (o *LinkRelObject) AddToMetadata(object string, metadata map[string]interface{}) {
	if o.objectAllowed(object) {
		// TODO: Add to metadata, handle ephemeral and allowList
		metadata[o.Value] = object
	}
}

func (o *LinkRelObject) objectAllowed(object string) bool {
	if o.AllowList == nil {
		return true
	}
	for _, allowed := range o.AllowList {
		if allowed == object {
			return true
		}
	}
	return false
}
