//go:build cssstub

package authentication

import (
	"net/http"
	"testing"
)

func TestBearerWebIdExtractor(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		expectedWebID string
		expectError   bool
		errorMessage  string
	}{
		{
			name:          "Valid Bearer Token",
			authHeader:    "Bearer https://example.org/user",
			expectedWebID: "https://example.org/user",
			expectError:   false,
		},
		{
			name:          "Valid Bearer Token Lowercase",
			authHeader:    "bearer https://example.org/user",
			expectedWebID: "https://example.org/user",
			expectError:   false,
		},
		{
			name:          "Missing Authorization Header",
			authHeader:    "",
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "no Bearer Authorization header specified",
		},
		{
			name:          "Invalid Bearer Format",
			authHeader:    "Basic https://example.org/user",
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "invalid Bearer token format",
		},
		{
			name:          "Malformed Bearer Token",
			authHeader:    "Bearer",
			expectedWebID: "",
			expectError:   true,
			errorMessage:  "invalid Bearer token format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewBearerWebIdExtractor()
			req := &http.Request{
				Header: make(http.Header),
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
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
