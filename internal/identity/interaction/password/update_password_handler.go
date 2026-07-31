//go:build cssstub

package password

import (
	"errors"
	"log"
)

type UpdatePasswordHandler struct {
	passwordStore PasswordStore
	passwordRoute PasswordIdRoute
}

func NewUpdatePasswordHandler(passwordStore PasswordStore, passwordRoute PasswordIdRoute) *UpdatePasswordHandler {
	return &UpdatePasswordHandler{
		passwordStore: passwordStore,
		passwordRoute: passwordRoute,
	}
}

func (h *UpdatePasswordHandler) GetView() (*JsonRepresentation, error) {
	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"oldPassword": map[string]interface{}{
				"type": "string",
			},
			"newPassword": map[string]interface{}{
				"type": "string",
			},
		},
	}

	return &JsonRepresentation{Json: schema}, nil
}

func (h *UpdatePasswordHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	oldPassword := "placeholder-old-password"
	newPassword := "placeholder-new-password"
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["oldPassword"].(string); ok {
	// 		oldPassword = val
	// 	}
	// 	if val, ok := input.Json["newPassword"].(string); ok {
	// 		newPassword = val
	// 	}
	// }

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

	// Make sure the old password is correct
	_, err := h.passwordStore.Authenticate(login.Email, oldPassword)
	if err != nil {
		log.Printf("Invalid password when trying to reset for email %s", login.Email)
		return nil, errors.New("old password is invalid")
	}

	h.passwordStore.Update(passwordId, newPassword)

	return &JsonRepresentation{Json: make(map[string]interface{})}, nil
}
