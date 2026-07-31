//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"fmt"
	"net/http"
	"strings"
)

// TargetExtractor extracts the target URL from a request
type TargetExtractor interface {
	Extract(r *http.Request) (string, error)
}

// DPoPWebIdExtractor extracts WebID from DPoP token
type DPoPWebIdExtractor struct {
	originalURLExtractor TargetExtractor
	// TODO: Add token verifier
}

// NewDPoPWebIdExtractor creates a new DPoPWebIdExtractor
func NewDPoPWebIdExtractor(originalURLExtractor TargetExtractor) *DPoPWebIdExtractor {
	return &DPoPWebIdExtractor{
		originalURLExtractor: originalURLExtractor,
	}
}

// Extract implements CredentialsExtractor
func (e *DPoPWebIdExtractor) Extract(r *http.Request) (*Credentials, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return nil, fmt.Errorf("no DPoP-bound Authorization header specified")
	}

	// Check if it's a DPoP token
	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[0] != "DPoP" {
		return nil, fmt.Errorf("invalid DPoP token format")
	}

	dpop := r.Header.Get("DPoP")
	if dpop == "" {
		return nil, fmt.Errorf("no DPoP header specified")
	}

	// Get the original URL
	originalURL, err := e.originalURLExtractor.Extract(r)
	if err != nil {
		return nil, fmt.Errorf("failed to extract original URL: %w", err)
	}

	// TODO: Implement proper DPoP token verification
	// For now, just use the token as the WebID
	return &Credentials{
		Agent: &Agent{
			WebID: parts[1],
		},
	}, nil
}
