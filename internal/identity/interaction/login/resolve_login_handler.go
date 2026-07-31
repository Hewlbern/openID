//go:build cssstub

package login

import (
	"context"
	"log"
)

type LoginOutputType struct {
	AccountId string
	Remember  *bool
}

type AccountStore interface {
	UpdateSetting(ctx context.Context, accountId, setting string, value interface{}) error
}

// CookieStore is already defined in logout_handler.go

type ResolveLoginHandler struct {
	accountStore AccountStore
	cookieStore  CookieStore
}

func NewResolveLoginHandler(accountStore AccountStore, cookieStore CookieStore) *ResolveLoginHandler {
	return &ResolveLoginHandler{
		accountStore: accountStore,
		cookieStore:  cookieStore,
	}
}

func (h *ResolveLoginHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	result, err := h.Login(input)
	if err != nil {
		return nil, err
	}

	accountId := result.Json["accountId"].(string)
	remember, _ := result.Json["remember"].(*bool)

	json := make(map[string]interface{})
	for k, v := range result.Json {
		if k != "accountId" && k != "remember" {
			json[k] = v
		}
	}

	// Generate cookie
	authorization := "placeholder-authorization" // Placeholder for cookie generation
	json["authorization"] = authorization

	// Placeholder for metadata handling
	metadata := make(map[string]interface{})
	metadata["accountCookie"] = authorization

	// Delete old cookie if there was one
	oldCookie := "placeholder-old-cookie"
	if oldCookie != "" {
		log.Printf("Replacing old cookie %s with %s", oldCookie, authorization)
		h.cookieStore.Delete(oldCookie)
	}

	// Update the account settings
	h.UpdateRememberSetting(context.Background(), accountId, remember)

	// Placeholder for OIDC interaction handling
	// if input.OidcInteraction != nil {
	// 	json["location"] = "placeholder-location"
	// }

	return &JsonRepresentation{
		Json:     json,
		Metadata: metadata,
	}, nil
}

func (h *ResolveLoginHandler) UpdateRememberSetting(ctx context.Context, accountId string, remember *bool) {
	if remember != nil {
		h.accountStore.UpdateSetting(ctx, accountId, "rememberLogin", *remember)
		log.Printf("Updating account remember setting to %v", *remember)
	}
}

func (h *ResolveLoginHandler) Login(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// This should be implemented by concrete handlers
	return nil, nil
}
