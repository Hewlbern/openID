//go:build cssstub

package account

import (
	"context"
)

type JsonInteractionHandlerInput struct {
	Method          string
	AccountId       *string
	OidcInteraction *interface{}
}

type JsonRepresentation struct {
	Json     map[string]interface{}
	Metadata interface{}
}

type CreateAccountHandler struct {
	*ResolveLoginHandler
}

type ResolveLoginHandler struct {
	accountStore AccountStore
	cookieStore  CookieStore
}

type AccountStore interface {
	Create(ctx context.Context) (string, error)
}

type CookieStore interface {
	Get(cookie string) (string, error)
}

type LoginOutputType struct {
	AccountId string
}

func NewCreateAccountHandler(accountStore AccountStore, cookieStore CookieStore) *CreateAccountHandler {
	return &CreateAccountHandler{
		ResolveLoginHandler: &ResolveLoginHandler{
			accountStore: accountStore,
			cookieStore:  cookieStore,
		},
	}
}

func (h *CreateAccountHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	return &JsonRepresentation{
		Json: make(map[string]interface{}),
	}, nil
}

func (h *CreateAccountHandler) Login(ctx context.Context) (*JsonRepresentation, error) {
	accountId, err := h.accountStore.Create(ctx)
	if err != nil {
		return nil, err
	}
	return &JsonRepresentation{
		Json: map[string]interface{}{"accountId": accountId},
	}, nil
}
