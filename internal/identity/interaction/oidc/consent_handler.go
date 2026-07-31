//go:build cssstub

package oidc

import (
	"errors"
)

type Grant interface {
	RejectOIDCScope(scope string)
	AddOIDCScope(scopes string)
	AddOIDCClaims(claims []string)
	AddResourceScope(indicator, scopes string)
	Save() (string, error)
}

type InteractionDetails struct {
	MissingOIDCScope      []string
	MissingOIDCClaims     []string
	MissingResourceScopes map[string][]string
}

type InteractionResults struct {
	Consent map[string]interface{}
}

type ConsentHandler struct {
	providerFactory ProviderFactory
}

func NewConsentHandler(providerFactory ProviderFactory) *ConsentHandler {
	return &ConsentHandler{
		providerFactory: providerFactory,
	}
}

func (h *ConsentHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for OIDC interaction assertion
	if input.OidcInteraction == nil {
		return nil, errors.New("OIDC interaction required")
	}

	// Placeholder for validation
	remember := false
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["remember"].(bool); ok {
	// 		remember = val
	// 	}
	// }

	grant, _ := h.getGrant(input.OidcInteraction)
	h.updateGrant(grant, input.OidcInteraction, remember)

	location, _ := h.updateInteraction(input.OidcInteraction, grant)

	// Return redirect error
	return nil, &RedirectError{Location: location}
}

func (h *ConsentHandler) getGrant(oidcInteraction *interface{}) (Grant, error) {
	// Placeholder for session validation
	if oidcInteraction == nil {
		return nil, errors.New("only interactions with a valid session are supported")
	}

	// Placeholder for grant retrieval/creation
	// provider, _ := h.providerFactory.GetProvider(context.Background())

	// Placeholder for grant creation
	grant := &PlaceholderGrant{}
	return grant, nil
}

func (h *ConsentHandler) updateGrant(grant Grant, oidcInteraction *interface{}, remember bool) {
	// Reject the offline_access scope if the user does not want to be remembered
	if !remember {
		grant.RejectOIDCScope("offline_access")
	}

	// Placeholder for details
	details := &InteractionDetails{
		MissingOIDCScope:      []string{"openid", "profile"},
		MissingOIDCClaims:     []string{"sub", "name"},
		MissingResourceScopes: map[string][]string{"https://example.com": {"read", "write"}},
	}

	// Grant all the requested scopes and claims
	if len(details.MissingOIDCScope) > 0 {
		scopes := ""
		for i, scope := range details.MissingOIDCScope {
			if i > 0 {
				scopes += " "
			}
			scopes += scope
		}
		grant.AddOIDCScope(scopes)
	}

	if len(details.MissingOIDCClaims) > 0 {
		grant.AddOIDCClaims(details.MissingOIDCClaims)
	}

	for indicator, scopes := range details.MissingResourceScopes {
		scopeStr := ""
		for i, scope := range scopes {
			if i > 0 {
				scopeStr += " "
			}
			scopeStr += scope
		}
		grant.AddResourceScope(indicator, scopeStr)
	}
}

func (h *ConsentHandler) updateInteraction(oidcInteraction *interface{}, grant Grant) (string, error) {
	// grantId, _ := grant.Save()

	// Placeholder for consent results
	// consent := make(map[string]interface{})
	// Only need to update the grantId if it is new
	// if !oidcInteraction.grantId {
	// 	consent["grantId"] = grantId
	// }

	// Placeholder for finish interaction
	location := "placeholder-location"
	return location, nil
}

type PlaceholderGrant struct{}

func (g *PlaceholderGrant) RejectOIDCScope(scope string)              {}
func (g *PlaceholderGrant) AddOIDCScope(scopes string)                {}
func (g *PlaceholderGrant) AddOIDCClaims(claims []string)             {}
func (g *PlaceholderGrant) AddResourceScope(indicator, scopes string) {}
func (g *PlaceholderGrant) Save() (string, error) {
	return "placeholder-grant-id", nil
}
