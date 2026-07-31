//go:build cssstub

// Package metadata provides a parser for specific authorization schemes and stores their value as metadata.
package metadata

import (
	"fmt"
)

// AuthorizationParser parses specific authorization schemes and stores their value as metadata.
type AuthorizationParser struct {
	AuthMap map[string]string // scheme -> predicate URI
}

func NewAuthorizationParser(authMap map[string]string) *AuthorizationParser {
	return &AuthorizationParser{AuthMap: authMap}
}

func (p *AuthorizationParser) Handle(input map[string]interface{}) error {
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
	authHeader, _ := headers["authorization"].(string)
	if authHeader == "" {
		return nil
	}
	for scheme, uri := range p.AuthMap {
		if matchesAuthorizationScheme(scheme, authHeader) {
			// This metadata should not be stored permanently
			addMetadata(metadata, uri, authHeader[len(scheme)+1:])
			return nil
		}
	}
	return nil
}

// matchesAuthorizationScheme checks if the authHeader starts with the scheme (case-insensitive).
func matchesAuthorizationScheme(scheme, authHeader string) bool {
	if len(authHeader) < len(scheme)+1 {
		return false
	}
	return authHeader[:len(scheme)] == scheme && authHeader[len(scheme)] == ' '
}

// addMetadata is a placeholder for adding metadata.
func addMetadata(metadata map[string]interface{}, uri, value string) {
	// TODO: Implement actual metadata storage logic
	metadata[uri] = value
}
