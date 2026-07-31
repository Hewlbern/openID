//go:build cssstub

package authentication

import (
	"net/http"
	"testing"
)

func TestPublicCredentialsExtractor(t *testing.T) {
	extractor := &PublicCredentialsExtractor{}
	req := &http.Request{}

	creds, err := extractor.Extract(req)
	if err != nil {
		t.Errorf("Extract() error = %v", err)
	}
	if creds.Agent.WebID != "https://example.org/public" {
		t.Errorf("Extract() WebID = %v, want %v", creds.Agent.WebID, "https://example.org/public")
	}

	// Test with empty credentials
	creds, err = extractor.Extract(req)
	if err != nil {
		t.Errorf("Extract() error = %v", err)
	}
	if creds.Agent == nil {
		t.Error("Extract() Agent is nil")
	}
	if creds.Client != nil {
		t.Error("Extract() Client should be nil")
	}
	if creds.Issuer != nil {
		t.Error("Extract() Issuer should be nil")
	}
}
