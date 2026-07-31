//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"fmt"
	"net/http"
	"strings"
)

// BearerWebIdExtractor extracts WebID from Bearer token
type BearerWebIdExtractor struct {
	// TODO: Add token verifier
}

// NewBearerWebIdExtractor creates a new BearerWebIdExtractor
func NewBearerWebIdExtractor() *BearerWebIdExtractor {
	return &BearerWebIdExtractor{}
}

// Extract implements CredentialsExtractor
func (e *BearerWebIdExtractor) Extract(r *http.Request) (*Credentials, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil, fmt.Errorf("no Bearer Authorization header specified")
	}

	// Check if it's a Bearer token
	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("invalid Bearer token format")
	}

	// TODO: Implement proper Bearer token verification
	// For now, just use the token as the WebID
	return &Credentials{
		Agent: &Agent{
			WebID: parts[1],
		},
	}, nil
}
