//go:build cssstub

package password

import (
	"log"
)

type TemplateEngine interface {
	HandleSafe(data map[string]interface{}) (string, error)
}

type EmailSender interface {
	HandleSafe(data map[string]interface{}) error
}

type ForgotPasswordStore interface {
	Generate(passwordId string) (string, error)
	Get(recordId string) (string, error)
	Delete(recordId string) error
}

type InteractionRoute interface {
	GetPath() string
}

type ForgotPasswordHandlerArgs struct {
	PasswordStore       PasswordStore
	ForgotPasswordStore ForgotPasswordStore
	TemplateEngine      TemplateEngine
	EmailSender         EmailSender
	ResetRoute          InteractionRoute
}

type ForgotPasswordHandler struct {
	passwordStore       PasswordStore
	forgotPasswordStore ForgotPasswordStore
	templateEngine      TemplateEngine
	emailSender         EmailSender
	resetRoute          InteractionRoute
}

func NewForgotPasswordHandler(args ForgotPasswordHandlerArgs) *ForgotPasswordHandler {
	return &ForgotPasswordHandler{
		passwordStore:       args.PasswordStore,
		forgotPasswordStore: args.ForgotPasswordStore,
		templateEngine:      args.TemplateEngine,
		emailSender:         args.EmailSender,
		resetRoute:          args.ResetRoute,
	}
}

func (h *ForgotPasswordHandler) GetView() (*JsonRepresentation, error) {
	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}

	return &JsonRepresentation{Json: schema}, nil
}

func (h *ForgotPasswordHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	email := "placeholder-email"
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["email"].(string); ok {
	// 		email = val
	// 	}
	// }

	// Placeholder for finding password by email
	// payload, _ := h.passwordStore.FindByEmail(email)
	payload := &PasswordLogin{ID: "placeholder-id"}

	if payload != nil && payload.ID != "" {
		recordId, err := h.forgotPasswordStore.Generate(payload.ID)
		if err != nil {
			// This error can not be thrown for privacy reasons.
			// If there always is an error, because there is a problem with the mail server for example,
			// errors would only be thrown for registered accounts.
			// Although we do also leak this information when an account tries to register an email address,
			// so this might be removed in the future.
			log.Printf("Problem sending a recovery mail: %v", err)
		} else {
			h.sendResetMail(recordId, email)
		}
	} else {
		// Don't emit an error for privacy reasons
		log.Printf("Password reset request for unknown email %s", email)
	}

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"email": email,
		},
	}, nil
}

func (h *ForgotPasswordHandler) sendResetMail(recordId, email string) {
	log.Printf("Sending password reset to %s", email)
	resetLink := h.resetRoute.GetPath() + "?rid=" + recordId
	renderedEmail, _ := h.templateEngine.HandleSafe(map[string]interface{}{
		"contents": map[string]interface{}{
			"resetLink": resetLink,
		},
	})
	h.emailSender.HandleSafe(map[string]interface{}{
		"recipient": email,
		"subject":   "Reset your password",
		"text":      "To reset your password, go to this link: " + resetLink,
		"html":      renderedEmail,
	})
}
