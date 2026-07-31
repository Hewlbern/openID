//go:build cssstub

package oidc

import (
	"context"
	"errors"
)

type AllClientMetadata map[string]interface{}

type ProviderFactory interface {
	GetProvider(ctx context.Context) (Provider, error)
}

type Provider interface {
	FindClient(clientId string) (Client, error)
}

type Client interface {
	Metadata() AllClientMetadata
}

type OutType struct {
	Client map[string]interface{} `json:"client"`
	WebId  *string                `json:"webId,omitempty"`
}

// Only extract specific fields to prevent leaking information
// Based on https://www.w3.org/ns/solid/oidc-context.jsonld
var CLIENT_KEYS = []string{
	"client_id",
	"client_uri",
	"logo_uri",
	"policy_uri",
	"client_name",
	"contacts",
	"grant_types",
	"scope",
}

type ClientInfoHandler struct {
	providerFactory ProviderFactory
}

func NewClientInfoHandler(providerFactory ProviderFactory) *ClientInfoHandler {
	return &ClientInfoHandler{
		providerFactory: providerFactory,
	}
}

func (h *ClientInfoHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for OIDC interaction assertion
	if input.OidcInteraction == nil {
		return nil, errors.New("OIDC interaction required")
	}

	provider, _ := h.providerFactory.GetProvider(context.Background())
	clientId := "placeholder-client-id" // Placeholder for getting client_id from interaction
	client, _ := provider.FindClient(clientId)

	metadata := make(AllClientMetadata)
	if client != nil {
		metadata = client.Metadata()
	}

	// Filter metadata to only include allowed keys
	jsonLd := make(map[string]interface{})
	for _, key := range CLIENT_KEYS {
		if value, exists := metadata[key]; exists {
			jsonLd[key] = value
		}
	}
	jsonLd["@context"] = "https://www.w3.org/ns/solid/oidc-context.jsonld"

	// Note: this is the `accountId` from the OIDC library, in which we store the WebID
	webId := "placeholder-web-id" // Placeholder for getting webId from interaction

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"client": jsonLd,
			"webId":  webId,
		},
	}, nil
}
