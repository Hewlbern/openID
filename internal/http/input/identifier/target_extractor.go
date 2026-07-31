//go:build cssstub

// Package identifier provides the TargetExtractor interface for extracting resource targets from HTTP requests.
package identifier

import "solid-go-main/internal/http/representation"

// TargetExtractor extracts a ResourceIdentifier from an incoming HTTP request.
type TargetExtractor interface {
	Handle(method string, headers map[string]string, urlPath string, connectionIsTLS bool) (*representation.ResourceIdentifier, error)
}
