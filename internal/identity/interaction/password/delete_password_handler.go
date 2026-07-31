//go:build cssstub

package password

import "errors"

type DeletePasswordHandler struct {
	passwordStore PasswordStore
	passwordRoute PasswordIdRoute
}

func NewDeletePasswordHandler(passwordStore PasswordStore, passwordRoute PasswordIdRoute) *DeletePasswordHandler {
	return &DeletePasswordHandler{
		passwordStore: passwordStore,
		passwordRoute: passwordRoute,
	}
}

func (h *DeletePasswordHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for path parsing
	// match := parsePath(h.passwordRoute, input.Target.Path)
	passwordId := "placeholder-password-id"

	login, _ := h.passwordStore.Get(passwordId)

	// Placeholder for account ID verification
	// verifyAccountId(input.AccountId, login?.AccountId)
	accountId := "placeholder-account-id"
	if login != nil && login.AccountId != accountId {
		return nil, errors.New("account ID mismatch")
	}

	h.passwordStore.Delete(passwordId)

	return &JsonRepresentation{Json: make(map[string]interface{})}, nil
}
