//go:build cssstub

package password

import (
	"context"
	"log"
)

// Import the login package for ResolveLoginHandler
// import "../login"

type AccountStore interface {
	UpdateSetting(ctx context.Context, accountId, setting string, value interface{}) error
}

type CookieStore interface {
	Generate(accountId string) (string, error)
	Delete(cookie string) error
}

type PasswordLoginHandlerArgs struct {
	AccountStore  AccountStore
	PasswordStore PasswordStore
	CookieStore   CookieStore
}

type PasswordLoginHandler struct {
	// *login.ResolveLoginHandler
	passwordStore PasswordStore
}

func NewPasswordLoginHandler(args PasswordLoginHandlerArgs) *PasswordLoginHandler {
	return &PasswordLoginHandler{
		// ResolveLoginHandler: login.NewResolveLoginHandler(args.AccountStore, args.CookieStore),
		passwordStore: args.PasswordStore,
	}
}

func (h *PasswordLoginHandler) GetView() (*JsonRepresentation, error) {
	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type": "string",
			},
			"remember": map[string]interface{}{
				"type": "boolean",
			},
		},
	}

	return &JsonRepresentation{Json: schema}, nil
}

func (h *PasswordLoginHandler) Login(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	email := "placeholder-email"
	// password := "placeholder-password"
	remember := false
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["email"].(string); ok {
	// 		email = val
	// 	}
	// 	if val, ok := input.Json["password"].(string); ok {
	// 		password = val
	// 	}
	// 	if val, ok := input.Json["remember"].(bool); ok {
	// 		remember = val
	// 	}
	// }

	// Try to log in, will error if email/password combination is invalid
	// result, _ := h.passwordStore.Authenticate(email, password)
	accountId := "placeholder-account-id"
	log.Printf("Logging in user %s", email)

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"accountId": accountId,
			"remember":  remember,
		},
	}, nil
}
