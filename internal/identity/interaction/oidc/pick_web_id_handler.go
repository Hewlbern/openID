//go:build cssstub

package oidc

import (
	"context"
	"errors"
	"log"
)

type WebIdStore interface {
	FindLinks(accountId string) ([]WebIdLink, error)
	IsLinked(webId, accountId string) (bool, error)
}

type WebIdLink struct {
	WebId string
}

type PickWebIdHandler struct {
	webIdStore      WebIdStore
	providerFactory ProviderFactory
}

func NewPickWebIdHandler(webIdStore WebIdStore, providerFactory ProviderFactory) *PickWebIdHandler {
	return &PickWebIdHandler{
		webIdStore:      webIdStore,
		providerFactory: providerFactory,
	}
}

func (h *PickWebIdHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }

	// Placeholder for schema description
	description := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"webId": map[string]interface{}{
				"type": "string",
			},
			"remember": map[string]interface{}{
				"type": "boolean",
			},
		},
	}

	// Get WebIDs for the account
	accountId := "placeholder-account-id"
	links, _ := h.webIdStore.FindLinks(accountId)
	webIds := make([]string, len(links))
	for i, link := range links {
		webIds[i] = link.WebId
	}

	json := make(map[string]interface{})
	for k, v := range description {
		json[k] = v
	}
	json["webIds"] = webIds

	return &JsonRepresentation{Json: json}, nil
}

func (h *PickWebIdHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for OIDC interaction assertion
	if input.OidcInteraction == nil {
		return nil, errors.New("OIDC interaction required")
	}

	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }

	// Placeholder for validation
	webId := "placeholder-web-id"
	// remember := false
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["webId"].(string); ok {
	// 		webId = val
	// 	}
	// 	if val, ok := input.Json["remember"].(bool); ok {
	// 		remember = val
	// 	}
	// }

	// Check if WebID is linked to account
	accountId := "placeholder-account-id"
	isLinked, _ := h.webIdStore.IsLinked(webId, accountId)
	if !isLinked {
		log.Printf("Trying to pick WebID %s which does not belong to account %s", webId, accountId)
		return nil, errors.New("WebID does not belong to this account")
	}

	// We need to explicitly forget the WebID from the session or the library won't allow us to update the value
	provider, _ := h.providerFactory.GetProvider(context.Background())
	h.forgetWebId(provider, input.OidcInteraction)

	// Update the interaction to get the redirect URL
	// login := map[string]interface{}{
	// 	// Note that `accountId` here is unrelated to our user accounts but is part of the OIDC library
	// 	"accountId": webId,
	// 	"remember":  remember,
	// }

	// Placeholder for finish interaction
	location := "placeholder-location"

	// Return redirect error
	return nil, &RedirectError{Location: location}
}

func (h *PickWebIdHandler) forgetWebId(provider Provider, oidcInteraction *interface{}) {
	// Placeholder for forgetting WebID logic
	// This would typically clear the WebID from the interaction session
}
