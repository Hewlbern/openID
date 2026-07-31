//go:build cssstub

package oidc

import (
	"errors"
)

type JsonInteractionHandlerInput struct {
	OidcInteraction *interface{}
}

type JsonRepresentation struct {
	Json map[string]interface{}
}

type CancelOidcHandler struct{}

func NewCancelOidcHandler() *CancelOidcHandler {
	return &CancelOidcHandler{}
}

func (h *CancelOidcHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for OIDC interaction assertion
	if input.OidcInteraction == nil {
		return nil, errors.New("OIDC interaction required")
	}

	// Placeholder for finish interaction with error
	location := "placeholder-location"

	// Return redirect error
	return nil, &RedirectError{Location: location}
}

type RedirectError struct {
	Location string
}

func (e *RedirectError) Error() string {
	return "redirect to " + e.Location
}
