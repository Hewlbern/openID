//go:build cssstub

package webid

import (
	"errors"
	"log"
)

type JsonInteractionHandlerInput struct {
	AccountId *string
	Json      map[string]interface{}
}

type JsonRepresentation struct {
	Json map[string]interface{}
}

type StorageLocationStrategy interface {
	GetStorageIdentifier(data map[string]interface{}) (*ResourceIdentifier, error)
}

type OwnershipValidator interface {
	HandleSafe(data map[string]interface{}) error
}

type PodStore interface {
	FindByBaseURL(baseURL string) (*Pod, error)
}

type WebIdStore interface {
	FindLinks(accountId string) ([]WebIdLink, error)
	IsLinked(webId, accountId string) (bool, error)
	Create(webId, accountId string) (string, error)
	Get(webIdLink string) (*WebIdLink, error)
	Delete(webIdLink string) error
}

type WebIdLink struct {
	ID        string
	WebId     string
	AccountId string
}

type Pod struct {
	AccountId string
}

type ResourceIdentifier struct {
	Path string
}

type LinkWebIdHandlerArgs struct {
	BaseUrl            string
	OwnershipValidator OwnershipValidator
	PodStore           PodStore
	WebIdStore         WebIdStore
	WebIdRoute         WebIdLinkRoute
	StorageStrategy    StorageLocationStrategy
}

type LinkWebIdHandler struct {
	baseUrl            string
	ownershipValidator OwnershipValidator
	podStore           PodStore
	webIdStore         WebIdStore
	webIdRoute         WebIdLinkRoute
	storageStrategy    StorageLocationStrategy
}

func NewLinkWebIdHandler(args LinkWebIdHandlerArgs) *LinkWebIdHandler {
	return &LinkWebIdHandler{
		baseUrl:            args.BaseUrl,
		ownershipValidator: args.OwnershipValidator,
		podStore:           args.PodStore,
		webIdStore:         args.WebIdStore,
		webIdRoute:         args.WebIdRoute,
		storageStrategy:    args.StorageStrategy,
	}
}

func (h *LinkWebIdHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }

	accountId := "placeholder-account-id"
	webIdLinks := make(map[string]string)

	links, _ := h.webIdStore.FindLinks(accountId)
	for _, link := range links {
		params := map[string]string{
			"accountId": accountId,
			"webIdLink": link.ID,
		}
		webIdLinks[link.WebId] = h.webIdRoute.GetPath(params)
	}

	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"webId": map[string]interface{}{
				"type": "string",
			},
		},
	}

	json := make(map[string]interface{})
	for k, v := range schema {
		json[k] = v
	}
	json["webIdLinks"] = webIdLinks

	return &JsonRepresentation{Json: json}, nil
}

func (h *LinkWebIdHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }
	accountId := "placeholder-account-id"

	// Placeholder for validation
	webId := "placeholder-web-id"
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["webId"].(string); ok {
	// 		webId = val
	// 	}
	// }

	isLinked, _ := h.webIdStore.IsLinked(webId, accountId)
	if isLinked {
		log.Printf("Trying to link WebID %s to account %s which already has this link", webId, accountId)
		return nil, errors.New(webId + " is already registered to this account")
	}

	// Only need to check ownership if the account did not create the pod
	isCreator := false
	baseUrl, err := h.storageStrategy.GetStorageIdentifier(map[string]interface{}{
		"path": webId,
	})
	if err == nil {
		pod, _ := h.podStore.FindByBaseURL(baseUrl.Path)
		if pod != nil {
			isCreator = accountId == pod.AccountId
		}
	}

	if !isCreator {
		h.ownershipValidator.HandleSafe(map[string]interface{}{
			"webId": webId,
		})
	}

	webIdLink, _ := h.webIdStore.Create(webId, accountId)
	resource := h.webIdRoute.GetPath(map[string]string{
		"accountId": accountId,
		"webIdLink": webIdLink,
	})

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"resource":   resource,
			"webId":      webId,
			"oidcIssuer": h.baseUrl,
		},
	}, nil
}
