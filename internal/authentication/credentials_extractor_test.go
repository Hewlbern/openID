//go:build cssstub

package authentication

import (
	"net/http"
	"testing"
)

func TestCredentialsExtractor(t *testing.T) {
	// Test cases for different credential extractors
	tests := []struct {
		name          string
		extractor     CredentialsExtractor
		request       *http.Request
		expectedCreds *Credentials
		expectedError error
	}{
		{
			name:      "PublicCredentialsExtractor",
			extractor: NewPublicCredentialsExtractor(),
			request:   &http.Request{},
			expectedCreds: &Credentials{
				Agent: &Agent{
					WebID: "https://example.org/public",
				},
			},
			expectedError: nil,
		},
		{
			name:      "UnsecureConstantCredentialsExtractor",
			extractor: NewUnsecureConstantCredentialsExtractor("https://example.org/constant"),
			request:   &http.Request{},
			expectedCreds: &Credentials{
				Agent: &Agent{
					WebID: "https://example.org/constant",
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := tt.extractor.Extract(tt.request)
			if err != tt.expectedError {
				t.Errorf("Extract() error = %v, want %v", err, tt.expectedError)
			}
			if creds.Agent.WebID != tt.expectedCreds.Agent.WebID {
				t.Errorf("Extract() WebID = %v, want %v", creds.Agent.WebID, tt.expectedCreds.Agent.WebID)
			}
		})
	}
}
