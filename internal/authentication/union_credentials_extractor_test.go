//go:build cssstub

package authentication

import (
	"fmt"
	"net/http"
	"testing"
)

type mockExtractor struct {
	creds *Credentials
	err   error
}

func (m *mockExtractor) Extract(r *http.Request) (*Credentials, error) {
	return m.creds, m.err
}

func TestUnionCredentialsExtractor(t *testing.T) {
	tests := []struct {
		name          string
		extractors    []CredentialsExtractor
		expectedCreds *Credentials
		expectError   bool
	}{
		{
			name: "Combine Results",
			extractors: []CredentialsExtractor{
				&mockExtractor{
					creds: &Credentials{
						Agent: &Agent{WebID: "http://user.example.com/#me"},
					},
					err: nil,
				},
				&mockExtractor{
					creds: &Credentials{
						Client: &Client{ClientID: "http://client.example.com/#me"},
					},
					err: nil,
				},
			},
			expectedCreds: &Credentials{
				Agent:  &Agent{WebID: "http://user.example.com/#me"},
				Client: &Client{ClientID: "http://client.example.com/#me"},
			},
			expectError: false,
		},
		{
			name: "Ignore Undefined Values",
			extractors: []CredentialsExtractor{
				&mockExtractor{
					creds: &Credentials{
						Agent: &Agent{WebID: "http://user.example.com/#me"},
					},
					err: nil,
				},
				&mockExtractor{
					creds: &Credentials{
						Client: &Client{ClientID: "http://client.example.com/#me"},
						Agent:  nil,
					},
					err: nil,
				},
			},
			expectedCreds: &Credentials{
				Agent:  &Agent{WebID: "http://user.example.com/#me"},
				Client: &Client{ClientID: "http://client.example.com/#me"},
			},
			expectError: false,
		},
		{
			name: "Skip Erroring Handlers",
			extractors: []CredentialsExtractor{
				&mockExtractor{
					creds: nil,
					err:   fmt.Errorf("error"),
				},
				&mockExtractor{
					creds: &Credentials{
						Client: &Client{ClientID: "http://client.example.com/#me"},
					},
					err: nil,
				},
			},
			expectedCreds: &Credentials{
				Client: &Client{ClientID: "http://client.example.com/#me"},
			},
			expectError: false,
		},
		{
			name: "All Handlers Error",
			extractors: []CredentialsExtractor{
				&mockExtractor{
					creds: nil,
					err:   fmt.Errorf("error 1"),
				},
				&mockExtractor{
					creds: nil,
					err:   fmt.Errorf("error 2"),
				},
			},
			expectedCreds: nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewUnionCredentialsExtractor(tt.extractors...)
			req := &http.Request{}

			creds, err := extractor.Extract(req)
			if tt.expectError {
				if err == nil {
					t.Error("Extract() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Extract() error = %v", err)
				return
			}

			// Compare Agent
			if tt.expectedCreds.Agent != nil {
				if creds.Agent == nil {
					t.Error("Extract() Agent is nil")
				} else if creds.Agent.WebID != tt.expectedCreds.Agent.WebID {
					t.Errorf("Extract() Agent.WebID = %v, want %v", creds.Agent.WebID, tt.expectedCreds.Agent.WebID)
				}
			} else if creds.Agent != nil {
				t.Error("Extract() Agent should be nil")
			}

			// Compare Client
			if tt.expectedCreds.Client != nil {
				if creds.Client == nil {
					t.Error("Extract() Client is nil")
				} else if creds.Client.ClientID != tt.expectedCreds.Client.ClientID {
					t.Errorf("Extract() Client.ClientID = %v, want %v", creds.Client.ClientID, tt.expectedCreds.Client.ClientID)
				}
			} else if creds.Client != nil {
				t.Error("Extract() Client should be nil")
			}

			// Compare Issuer
			if tt.expectedCreds.Issuer != nil {
				if creds.Issuer == nil {
					t.Error("Extract() Issuer is nil")
				} else if creds.Issuer.URL != tt.expectedCreds.Issuer.URL {
					t.Errorf("Extract() Issuer.URL = %v, want %v", creds.Issuer.URL, tt.expectedCreds.Issuer.URL)
				}
			} else if creds.Issuer != nil {
				t.Error("Extract() Issuer should be nil")
			}
		})
	}
}
