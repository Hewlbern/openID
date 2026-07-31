//go:build cssstub

package client_credentials

import (
	"errors"
	"log"
	"strings"
)

type JsonInteractionHandlerInput struct {
	AccountId *string
	Json      interface{}
}

type JsonRepresentation struct {
	Json map[string]interface{}
}

type JsonInteractionHandler interface {
	Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error)
}

type JsonView interface {
	GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error)
}

type WebIdStore interface {
	IsLinked(webId, accountId string) (bool, error)
}

type ClientCredentialsStore interface {
	FindByLabel(label string) (*ClientCredentials, error)
	Delete(id string) error
	FindByAccount(accountId string) ([]ClientCredentialInfo, error)
	Create(label, webId, accountId string) (*ClientCredentialResult, error)
}

type ClientCredentialInfo struct {
	Id    string
	Label string
}

type ClientCredentialResult struct {
	Secret string
	Id     string
}

type ClientCredentialsIdRoute interface {
	GetPath(params map[string]interface{}) string
}

type CreateClientCredentialsHandler struct {
	webIdStore             WebIdStore
	clientCredentialsStore ClientCredentialsStore
	clientCredentialsRoute ClientCredentialsIdRoute
}

func NewCreateClientCredentialsHandler(webIdStore WebIdStore, clientCredentialsStore ClientCredentialsStore, clientCredentialsRoute ClientCredentialsIdRoute) *CreateClientCredentialsHandler {
	return &CreateClientCredentialsHandler{
		webIdStore:             webIdStore,
		clientCredentialsStore: clientCredentialsStore,
		clientCredentialsRoute: clientCredentialsRoute,
	}
}

func (h *CreateClientCredentialsHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	if input.AccountId == nil || *input.AccountId == "" {
		return nil, errors.New("account ID not found")
	}

	clientCredentials := make(map[string]string)
	credentials, _ := h.clientCredentialsStore.FindByAccount(*input.AccountId)
	for _, cred := range credentials {
		clientCredentials[cred.Label] = h.clientCredentialsRoute.GetPath(map[string]interface{}{
			"accountId":           *input.AccountId,
			"clientCredentialsId": cred.Id,
		})
	}

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"clientCredentials": clientCredentials,
		},
	}, nil
}

func (h *CreateClientCredentialsHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	if input.AccountId == nil || *input.AccountId == "" {
		return nil, errors.New("account ID not found")
	}

	// Placeholder for validation logic
	jsonData := input.Json.(map[string]interface{})
	name, _ := jsonData["name"].(string)
	webId, _ := jsonData["webId"].(string)

	linked, _ := h.webIdStore.IsLinked(webId, *input.AccountId)
	if !linked {
		log.Printf("Trying to create token for %s which does not belong to account %s", webId, *input.AccountId)
		return nil, errors.New("WebID does not belong to this account")
	}

	cleanedName := sanitizeUrlPart(strings.TrimSpace(name))
	label := cleanedName + "_" + generateUUID()

	result, _ := h.clientCredentialsStore.Create(label, webId, *input.AccountId)
	resource := h.clientCredentialsRoute.GetPath(map[string]interface{}{
		"accountId":           *input.AccountId,
		"clientCredentialsId": result.Id,
	})

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"id":       label,
			"secret":   result.Secret,
			"resource": resource,
		},
	}, nil
}

func sanitizeUrlPart(part string) string {
	// Placeholder for URL sanitization
	return part
}

func generateUUID() string {
	// Placeholder for UUID generation
	return "uuid-placeholder"
}
