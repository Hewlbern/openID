//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"net/http"
)

// PublicCredentialsExtractor always returns public credentials
type PublicCredentialsExtractor struct{}

// NewPublicCredentialsExtractor creates a new PublicCredentialsExtractor
func NewPublicCredentialsExtractor() *PublicCredentialsExtractor {
	return &PublicCredentialsExtractor{}
}

// Extract implements CredentialsExtractor
func (e *PublicCredentialsExtractor) Extract(r *http.Request) (*Credentials, error) {
	return &Credentials{
		Agent: &Agent{
			WebID: "https://example.org/public",
		},
	}, nil
}
