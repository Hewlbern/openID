//go:build cssstub

// Package metadata provides a parser for the content-length header.
package metadata

import (
	"fmt"
	"strconv"
)

type ContentLengthParser struct{}

func (p *ContentLengthParser) Handle(input map[string]interface{}) error {
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
	contentLength, _ := headers["content-length"].(string)
	if contentLength != "" {
		length, err := strconv.Atoi(contentLength)
		if err == nil {
			metadata["contentLength"] = length
		} else {
			// Log warning: invalid content-length header
		}
	}
	return nil
}
