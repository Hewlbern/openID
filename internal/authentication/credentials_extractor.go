//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"net/http"
)

// CredentialsExtractor is responsible for extracting credentials from an incoming request.
type CredentialsExtractor interface {
	// Extract extracts credentials from the given HTTP request.
	Extract(r *http.Request) (*Credentials, error)
}
