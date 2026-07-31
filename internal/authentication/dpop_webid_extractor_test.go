//go:build cssstub

package authentication

import (
	"fmt"
	"net/http"
	"testing"
)

type mockTargetExtractor struct {
	url string
	err error
}

func (m *mockTargetExtractor) Extract(r *http.Request) (string, error) {
	return m.url, m.err
}

func TestDPoPWebIdExtractor(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		dpopHeader    string
		originalURL   string
		urlError      error
		expectedWebID string
		expectError   bool
		errorMessage  string
	}{
		{
			name:          "Valid DPoP Token",
			authHeader:    "DPoP https://example.org/user",
			dpopHeader:    "valid-dpop-token",
			originalURL:   "https://example.org/resource",
			expectedWebID: "https://example.org/user",
			expectError:   false,
		},
		{
			name:          "Valid DPoP Token Lowercase",
			authHeader:    "dpop https://example.org/user",
			dpopHeader:    "valid-dpop-token",
			originalURL:   "https://example.org/resource",
			expectedWebID: "https://example.org/user",
			expectError:   false,
		},
		{
			name:          "Missing Authorization Header",
			authHeader:    "",
			dpopHeader:    "valid-dpop-token",
			originalURL:   "https://example.org/resource",
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "no DPoP-bound Authorization header specified",
		},
		{
			name:          "Missing DPoP Header",
			authHeader:    "DPoP https://example.org/user",
			dpopHeader:    "",
			originalURL:   "https://example.org/resource",
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "no DPoP header specified",
		},
		{
			name:          "Invalid DPoP Format",
			authHeader:    "Bearer https://example.org/user",
			dpopHeader:    "valid-dpop-token",
			originalURL:   "https://example.org/resource",
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "invalid DPoP token format",
		},
		{
			name:          "URL Extraction Error",
			authHeader:    "DPoP https://example.org/user",
			dpopHeader:    "valid-dpop-token",
			originalURL:   "",
			urlError:      fmt.Errorf("failed to extract URL"),
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "failed to extract original URL: failed to extract URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewDPoPWebIdExtractor(&mockTargetExtractor{
				url: tt.originalURL,
				err: tt.urlError,
			})
			req := &http.Request{
				Header: make(http.Header),
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if tt.dpopHeader != "" {
				req.Header.Set("DPoP", tt.dpopHeader)
			}

			creds, err := extractor.Extract(req)
			if tt.expectError {
				if err == nil {
					t.Error("Extract() expected error, got nil")
					return
				}
				if err.Error() != tt.errorMessage {
					t.Errorf("Extract() error = %v, want %v", err, tt.errorMessage)
				}
				return
			}
			if err != nil {
				t.Errorf("Extract() error = %v", err)
				return
			}
			if creds.Agent.WebID != tt.expectedWebID {
				t.Errorf("Extract() WebID = %v, want %v", creds.Agent.WebID, tt.expectedWebID)
			}

			// Test that other fields are nil
			if creds.Client != nil {
				t.Error("Extract() Client should be nil")
			}
			if creds.Issuer != nil {
				t.Error("Extract() Issuer should be nil")
			}
		})
	}
}
