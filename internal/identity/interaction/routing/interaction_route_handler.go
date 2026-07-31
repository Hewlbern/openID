//go:build cssstub

package routing

import (
	"errors"
)

type JsonInteractionHandler interface {
	CanHandle(input JsonInteractionHandlerInput) error
	Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error)
}

type JsonInteractionHandlerInput struct {
	Target *ResourceIdentifier
}

type JsonRepresentation struct {
	Json map[string]interface{}
}

type ResourceIdentifier struct {
	Path string
}

type InteractionRouteHandler struct {
	route  InteractionRoute
	source JsonInteractionHandler
}

func NewInteractionRouteHandler(route InteractionRoute, source JsonInteractionHandler) *InteractionRouteHandler {
	return &InteractionRouteHandler{
		route:  route,
		source: source,
	}
}

func (h *InteractionRouteHandler) CanHandle(input JsonInteractionHandlerInput) error {
	if input.Target == nil {
		return errors.New("target is required")
	}

	if h.route.MatchPath(input.Target.Path) == nil {
		return errors.New("route not found")
	}

	return h.source.CanHandle(input)
}

func (h *InteractionRouteHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	return h.source.Handle(input)
}
