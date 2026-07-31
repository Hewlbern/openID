//go:build cssstub

package interaction

import (
	"errors"
)

type ViewInteractionHandler struct {
	source interface {
		JsonInteractionHandler
		JsonView
	}
}

func NewViewInteractionHandler(source interface {
	JsonInteractionHandler
	JsonView
}) *ViewInteractionHandler {
	return &ViewInteractionHandler{
		source: source,
	}
}

func (h *ViewInteractionHandler) CanHandle(input JsonInteractionHandlerInput) error {
	method := input.Method
	if method != "GET" && method != "POST" {
		return errors.New("only GET/POST requests are supported")
	}

	if method == "POST" {
		return h.source.CanHandle(input)
	}
	return nil
}

func (h *ViewInteractionHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	if input.Method == "GET" {
		return h.source.GetView(input)
	}
	return h.source.Handle(input)
}
