//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

import (
	"net/http"
	"regexp"
	"strings"
)

// UnsecureWebIdExtractor extracts WebID from Authorization header
type UnsecureWebIdExtractor struct{}

// NewUnsecureWebIdExtractor creates a new UnsecureWebIdExtractor
func NewUnsecureWebIdExtractor() *UnsecureWebIdExtractor {
	return &UnsecureWebIdExtractor{}
}

// Extract implements CredentialsExtractor
func (e *UnsecureWebIdExtractor) Extract(r *http.Request) (*Credentials, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "WebID ") {
		return nil, ErrNotImplemented
	}

	// Extract WebID from Authorization header
	re := regexp.MustCompile(`^WebID\s+(.*)$`)
	matches := re.FindStringSubmatch(auth)
	if len(matches) < 2 {
		return nil, ErrNotImplemented
	}

	webID := matches[1]
	return &Credentials{
		Agent: &Agent{
			WebID: webID,
		},
	}, nil
}
