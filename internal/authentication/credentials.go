//go:build cssstub

// Package authentication provides implementations for authentication and credential management.
package authentication

// Credentials represents authentication credentials identifying an entity accessing or owning data.
type Credentials struct {
	// Agent represents the agent making the request
	Agent *Agent `json:"agent,omitempty"`
	// Client represents the client making the request
	Client *Client `json:"client,omitempty"`
	// Issuer represents the issuer of the credentials
	Issuer *Issuer `json:"issuer,omitempty"`
	// Additional fields can be added here
}

// Agent represents an agent making a request
type Agent struct {
	WebID string `json:"webId"`
}

// Client represents a client making a request
type Client struct {
	ClientID string `json:"clientId"`
}

// Issuer represents an issuer of credentials
type Issuer struct {
	URL string `json:"url"`
}
