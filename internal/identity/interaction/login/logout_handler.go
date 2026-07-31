//go:build cssstub

package login

import (
	"errors"
	"time"
)

type JsonInteractionHandlerInput struct {
	AccountId *string
	Metadata  interface{}
}

type JsonRepresentation struct {
	Json     map[string]interface{}
	Metadata interface{}
}

type CookieStore interface {
	Get(cookie string) (string, error)
	Delete(cookie string) error
}

type RepresentationMetadata interface {
	Set(key, value string)
}

type LogoutHandler struct {
	cookieStore CookieStore
}

func NewLogoutHandler(cookieStore CookieStore) *LogoutHandler {
	return &LogoutHandler{
		cookieStore: cookieStore,
	}
}

func (h *LogoutHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for metadata extraction
	cookie := "placeholder-cookie"

	if cookie != "" {
		// Make sure the cookie belongs to the logged-in user
		foundId, _ := h.cookieStore.Get(cookie)
		if input.AccountId == nil || foundId != *input.AccountId {
			return nil, errors.New("invalid cookie")
		}

		h.cookieStore.Delete(cookie)

		// Setting the expiration time of a cookie to somewhere in the past causes browsers to delete that cookie
		outputMetadata := make(map[string]interface{})
		outputMetadata["accountCookie"] = cookie
		outputMetadata["accountCookieExpiration"] = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)

		return &JsonRepresentation{
			Json:     make(map[string]interface{}),
			Metadata: outputMetadata,
		}, nil
	}

	return &JsonRepresentation{
		Json: make(map[string]interface{}),
	}, nil
}
