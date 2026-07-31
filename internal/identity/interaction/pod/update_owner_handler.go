//go:build cssstub

package pod

import (
	"errors"
)

type ResourceIdentifier struct {
	Path string
}

type UpdateOwnerHandler struct {
	podStore PodStore
	podRoute PodIdRoute
}

func NewUpdateOwnerHandler(podStore PodStore, podRoute PodIdRoute) *UpdateOwnerHandler {
	return &UpdateOwnerHandler{
		podStore: podStore,
		podRoute: podRoute,
	}
}

func (h *UpdateOwnerHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for target and account ID
	target := &ResourceIdentifier{Path: "placeholder-path"}
	accountId := "placeholder-account-id"

	pod, _ := h.findVerifiedPod(target, accountId)
	owners, _ := h.podStore.GetOwners(pod.ID)

	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"webId": map[string]interface{}{
				"type": "string",
			},
			"visible": map[string]interface{}{
				"type": "boolean",
			},
			"remove": map[string]interface{}{
				"type": "boolean",
			},
		},
	}

	json := make(map[string]interface{})
	for k, v := range schema {
		json[k] = v
	}
	json["baseUrl"] = pod.BaseURL
	json["owners"] = owners

	return &JsonRepresentation{Json: json}, nil
}

func (h *UpdateOwnerHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	webId := "placeholder-web-id"
	visible := false
	remove := false
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["webId"].(string); ok {
	// 		webId = val
	// 	}
	// 	if val, ok := input.Json["visible"].(bool); ok {
	// 		visible = val
	// 	}
	// 	if val, ok := input.Json["remove"].(bool); ok {
	// 		remove = val
	// 	}
	// }

	// Placeholder for target and account ID
	target := &ResourceIdentifier{Path: "placeholder-path"}
	accountId := "placeholder-account-id"

	pod, _ := h.findVerifiedPod(target, accountId)

	if remove {
		h.podStore.RemoveOwner(pod.ID, webId)
	} else {
		h.podStore.UpdateOwner(pod.ID, webId, visible)
	}

	return &JsonRepresentation{Json: make(map[string]interface{})}, nil
}

func (h *UpdateOwnerHandler) findVerifiedPod(target *ResourceIdentifier, accountId string) (*Pod, error) {
	// Placeholder for path parsing
	// podId := parsePath(h.podRoute, target.Path)
	podId := "placeholder-pod-id"

	pod, _ := h.podStore.Get(podId)

	// Placeholder for account ID verification
	// verifyAccountId(accountId, pod?.AccountId)
	if pod != nil && pod.AccountId != accountId {
		return nil, errors.New("account ID mismatch")
	}

	return &Pod{
		ID:        podId,
		BaseURL:   pod.BaseURL,
		AccountId: pod.AccountId,
	}, nil
}
