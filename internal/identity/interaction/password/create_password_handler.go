//go:build cssstub

package password

type JsonInteractionHandlerInput struct {
	AccountId *string
	Json      map[string]interface{}
}

type JsonRepresentation struct {
	Json map[string]interface{}
}

type PasswordStore interface {
	FindByAccount(accountId string) ([]PasswordLogin, error)
	Create(email, accountId, password string) (string, error)
	ConfirmVerification(passwordId string) error
	Get(passwordId string) (*PasswordLogin, error)
	Delete(passwordId string) error
	Update(passwordId, newPassword string) error
	Authenticate(email, password string) (map[string]string, error)
	FindByEmail(email string) (*PasswordLogin, error)
}

type PasswordLogin struct {
	ID        string
	Email     string
	AccountId string
}

type PasswordIdRoute interface {
	GetPath(params map[string]string) string
}

type CreatePasswordHandler struct {
	passwordStore PasswordStore
	passwordRoute PasswordIdRoute
}

func NewCreatePasswordHandler(passwordStore PasswordStore, passwordRoute PasswordIdRoute) *CreatePasswordHandler {
	return &CreatePasswordHandler{
		passwordStore: passwordStore,
		passwordRoute: passwordRoute,
	}
}

func (h *CreatePasswordHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }

	accountId := "placeholder-account-id"
	passwordLogins := make(map[string]string)

	logins, _ := h.passwordStore.FindByAccount(accountId)
	for _, login := range logins {
		params := map[string]string{
			"accountId":  accountId,
			"passwordId": login.ID,
		}
		passwordLogins[login.Email] = h.passwordRoute.GetPath(params)
	}

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
		},
	}

	json := make(map[string]interface{})
	for k, v := range schema {
		json[k] = v
	}
	json["passwordLogins"] = passwordLogins

	return &JsonRepresentation{Json: json}, nil
}

func (h *CreatePasswordHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	email := "placeholder-email"
	password := "placeholder-password"
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["email"].(string); ok {
	// 		email = val
	// 	}
	// 	if val, ok := input.Json["password"].(string); ok {
	// 		password = val
	// 	}
	// }

	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }
	accountId := "placeholder-account-id"

	passwordId, _ := h.passwordStore.Create(email, accountId, password)
	params := map[string]string{
		"accountId":  accountId,
		"passwordId": passwordId,
	}
	resource := h.passwordRoute.GetPath(params)

	// If we ever want to add email verification this would have to be checked separately
	h.passwordStore.ConfirmVerification(passwordId)

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"resource": resource,
		},
	}, nil
}
