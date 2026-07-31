//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"net/http"
)

// UnionCredentialsExtractor combines multiple CredentialsExtractors.
type UnionCredentialsExtractor struct {
	extractors []CredentialsExtractor
}

// NewUnionCredentialsExtractor creates a new UnionCredentialsExtractor
func NewUnionCredentialsExtractor(extractors ...CredentialsExtractor) *UnionCredentialsExtractor {
	return &UnionCredentialsExtractor{
		extractors: extractors,
	}
}

// Extract implements CredentialsExtractor. Combines results from all extractors.
func (u *UnionCredentialsExtractor) Extract(r *http.Request) (*Credentials, error) {
	combined := &Credentials{}
	for _, extractor := range u.extractors {
		creds, err := extractor.Extract(r)
		if err != nil {
			return nil, err
		}
		if creds == nil {
			continue
		}
		if creds.Agent != nil {
			combined.Agent = creds.Agent
		}
		if creds.Client != nil {
			combined.Client = creds.Client
		}
		if creds.Issuer != nil {
			combined.Issuer = creds.Issuer
		}
	}
	return combined, nil
}
