//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"net/http"
)

// UnsecureConstantCredentialsExtractor always returns a constant agent (webId).
type UnsecureConstantCredentialsExtractor struct {
	credentials *Credentials
}

// NewUnsecureConstantCredentialsExtractor creates a new constant extractor with the given webId.
func NewUnsecureConstantCredentialsExtractor(webId string) *UnsecureConstantCredentialsExtractor {
	return &UnsecureConstantCredentialsExtractor{
		credentials: &Credentials{
			Agent: &Agent{WebID: webId},
		},
	}
}

// Extract implements CredentialsExtractor
func (e *UnsecureConstantCredentialsExtractor) Extract(r *http.Request) (*Credentials, error) {
	return e.credentials, nil
}
