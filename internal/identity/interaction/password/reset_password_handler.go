//go:build cssstub

package password

import (
	"errors"
	"log"
)

type ResetPasswordHandler struct {
	passwordStore       PasswordStore
	forgotPasswordStore ForgotPasswordStore
}

func NewResetPasswordHandler(passwordStore PasswordStore, forgotPasswordStore ForgotPasswordStore) *ResetPasswordHandler {
	return &ResetPasswordHandler{
		passwordStore:       passwordStore,
		forgotPasswordStore: forgotPasswordStore,
	}
}

func (h *ResetPasswordHandler) GetView() (*JsonRepresentation, error) {
	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"recordId": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type": "string",
			},
		},
	}

	return &JsonRepresentation{Json: schema}, nil
}

func (h *ResetPasswordHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	password := "placeholder-password"
	recordId := "placeholder-record-id"
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["password"].(string); ok {
	// 		password = val
	// 	}
	// 	if val, ok := input.Json["recordId"].(string); ok {
	// 		recordId = val
	// 	}
	// }

	h.resetPassword(recordId, password)
	return &JsonRepresentation{Json: make(map[string]interface{})}, nil
}

func (h *ResetPasswordHandler) resetPassword(recordId, newPassword string) error {
	id, _ := h.forgotPasswordStore.Get(recordId)

	if id == "" {
		log.Printf("Trying to use invalid reset URL with record ID %s", recordId)
		return errors.New("this reset password link is no longer valid")
	}

	h.passwordStore.Update(id, newPassword)
	h.forgotPasswordStore.Delete(recordId)

	log.Printf("Resetting password for login %s", id)
	return nil
}
