//go:build cssstub

package interaction

import (
	"time"
)

type AccountStore interface {
	GetSetting(accountId, setting string) (interface{}, error)
}

type CookieStore interface {
	Get(cookie string) (string, error)
	Refresh(cookie string) (*time.Time, error)
}

type RepresentationMetadata interface {
	Get(key string) interface{}
	Set(key string, value interface{})
	Has(key string) bool
}

type CookieInteractionHandler struct {
	source       JsonInteractionHandler
	accountStore AccountStore
	cookieStore  CookieStore
}

func NewCookieInteractionHandler(source JsonInteractionHandler, accountStore AccountStore, cookieStore CookieStore) *CookieInteractionHandler {
	return &CookieInteractionHandler{
		source:       source,
		accountStore: accountStore,
		cookieStore:  cookieStore,
	}
}

func (h *CookieInteractionHandler) CanHandle(input JsonInteractionHandlerInput) error {
	return h.source.CanHandle(input)
}

func (h *CookieInteractionHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	output, _ := h.source.Handle(input)
	// Placeholder for cookie handling logic
	return output, nil
}
