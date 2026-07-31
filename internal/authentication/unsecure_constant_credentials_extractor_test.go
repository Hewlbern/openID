//go:build cssstub

package authentication

import (
	"net/http"
	"testing"
)

func TestUnsecureConstantCredentialsExtractor(t *testing.T) {
	tests := []struct {
		name          string
		webID         string
		expectedWebID string
	}{
		{
			name:          "Simple WebID",
			webID:         "http://alice.example/card#me",
			expectedWebID: "http://alice.example/card#me",
		},
		{
			name:          "Complex WebID",
			webID:         "http://example.com/#me",
			expectedWebID: "http://example.com/#me",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewUnsecureConstantCredentialsExtractor(tt.webID)
			req := &http.Request{}

			creds, err := extractor.Extract(req)
			if err != nil {
				t.Errorf("Extract() error = %v", err)
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
