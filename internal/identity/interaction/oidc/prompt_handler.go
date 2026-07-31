//go:build cssstub

package oidc

import (
	"errors"
	"log"
)

type InteractionRoute interface {
	GetPath() string
}

// OutType is already defined in client_info_handler.go

type PromptHandler struct {
	promptRoutes map[string]InteractionRoute
}

func NewPromptHandler(promptRoutes map[string]InteractionRoute) *PromptHandler {
	return &PromptHandler{
		promptRoutes: promptRoutes,
	}
}

func (h *PromptHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for getting prompt from OIDC interaction
	prompt := "placeholder-prompt"

	if prompt != "" && h.promptRoutes[prompt] != nil {
		location := h.promptRoutes[prompt].GetPath()
		log.Printf("Current prompt is %s with URL %s", prompt, location)
		// Not throwing redirect error since we also want to the prompt to the output json.
		return &JsonRepresentation{
			Json: map[string]interface{}{
				"location": location,
				"prompt":   prompt,
			},
		}, nil
	}

	log.Printf("Received unsupported prompt %s", prompt)
	return nil, errors.New("unsupported prompt: " + prompt)
}
