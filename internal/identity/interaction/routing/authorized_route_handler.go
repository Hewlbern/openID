//go:build cssstub

package routing

import (
	"errors"
	"log"
)

type AccountIdRoute interface {
	GetPath(parameters map[string]string) string
	MatchPath(path string) map[string]string
}

type AuthorizedRouteHandler struct {
	*InteractionRouteHandler
	route AccountIdRoute
}

func NewAuthorizedRouteHandler(route AccountIdRoute, source JsonInteractionHandler) *AuthorizedRouteHandler {
	return &AuthorizedRouteHandler{
		InteractionRouteHandler: NewInteractionRouteHandler(route, source),
		route:                   route,
	}
}

func (h *AuthorizedRouteHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for account ID access
	// accountId := input.AccountId
	accountId := "placeholder-account-id"

	if accountId == "" {
		log.Printf("Trying to access %s without authorization", input.Target.Path)
		return nil, errors.New("unauthorized")
	}

	match := h.route.MatchPath(input.Target.Path)
	if match == nil {
		return nil, errors.New("route not found")
	}

	if match["accountId"] != accountId {
		log.Printf("Trying to access %s with wrong authorization: %s", input.Target.Path, accountId)
		return nil, errors.New("forbidden")
	}

	return h.source.Handle(input)
}
