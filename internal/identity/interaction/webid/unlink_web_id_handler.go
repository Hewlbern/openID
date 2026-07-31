//go:build cssstub

package webid

import (
	"errors"
)

type UnlinkWebIdHandler struct {
	webIdStore WebIdStore
	webIdRoute WebIdLinkRoute
}

func NewUnlinkWebIdHandler(webIdStore WebIdStore, webIdRoute WebIdLinkRoute) *UnlinkWebIdHandler {
	return &UnlinkWebIdHandler{
		webIdStore: webIdStore,
		webIdRoute: webIdRoute,
	}
}

func (h *UnlinkWebIdHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for path parsing
	// match := parsePath(h.webIdRoute, input.Target.Path)
	webIdLink := "placeholder-webid-link"

	link, _ := h.webIdStore.Get(webIdLink)

	// Placeholder for account ID verification
	// verifyAccountId(input.AccountId, link?.AccountId)
	accountId := "placeholder-account-id"
	if link != nil && link.AccountId != accountId {
		return nil, errors.New("account ID mismatch")
	}

	h.webIdStore.Delete(webIdLink)

	return &JsonRepresentation{Json: make(map[string]interface{})}, nil
}
