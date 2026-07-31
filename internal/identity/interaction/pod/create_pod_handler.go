//go:build cssstub

package pod

type JsonInteractionHandlerInput struct {
	AccountId *string
	Json      map[string]interface{}
}

type JsonRepresentation struct {
	Json map[string]interface{}
}

type PodStore interface {
	FindPods(accountId string) ([]Pod, error)
	Get(podId string) (*Pod, error)
	GetOwners(podId string) ([]string, error)
	RemoveOwner(podId, webId string) error
	UpdateOwner(podId, webId string, visible bool) error
}

type Pod struct {
	ID        string
	BaseURL   string
	AccountId string
}

type PodCreator interface {
	HandleSafe(data map[string]interface{}) (*PodCreationResult, error)
}

type PodCreationResult struct {
	PodURL    string
	WebId     string
	PodId     string
	WebIdLink *string
}

type WebIdLinkRoute interface {
	GetPath(params map[string]string) string
}

type PodIdRoute interface {
	GetPath(params map[string]string) string
}

type CreatePodHandler struct {
	podStore       PodStore
	podCreator     PodCreator
	webIdLinkRoute WebIdLinkRoute
	podIdRoute     PodIdRoute
	allowRoot      bool
}

func NewCreatePodHandler(
	podStore PodStore,
	podCreator PodCreator,
	webIdLinkRoute WebIdLinkRoute,
	podIdRoute PodIdRoute,
	allowRoot bool,
) *CreatePodHandler {
	return &CreatePodHandler{
		podStore:       podStore,
		podCreator:     podCreator,
		webIdLinkRoute: webIdLinkRoute,
		podIdRoute:     podIdRoute,
		allowRoot:      allowRoot,
	}
}

func (h *CreatePodHandler) GetView(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }

	accountId := "placeholder-account-id"
	pods := make(map[string]string)

	podList, _ := h.podStore.FindPods(accountId)
	for _, pod := range podList {
		params := map[string]string{
			"accountId": accountId,
			"podId":     pod.ID,
		}
		pods[pod.BaseURL] = h.podIdRoute.GetPath(params)
	}

	// Placeholder for schema description
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"settings": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"webId": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	json := make(map[string]interface{})
	for k, v := range schema {
		json[k] = v
	}
	json["pods"] = pods

	return &JsonRepresentation{Json: json}, nil
}

func (h *CreatePodHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	// Placeholder for validation
	name := "placeholder-name"
	settings := map[string]interface{}{
		"webId": "placeholder-web-id",
	}
	// Placeholder for JSON access
	// if input.Json != nil {
	// 	if val, ok := input.Json["name"].(string); ok {
	// 		name = val
	// 	}
	// 	if val, ok := input.Json["settings"].(map[string]interface{}); ok {
	// 		settings = val
	// 	}
	// }

	// Placeholder for account ID assertion
	// if input.AccountId == nil {
	// 	return nil, errors.New("account ID required")
	// }
	accountId := "placeholder-account-id"

	result, _ := h.podCreator.HandleSafe(map[string]interface{}{
		"accountId": accountId,
		"webId":     settings["webId"],
		"name":      name,
		"settings":  settings,
	})

	var webIdResource *string
	if result.WebIdLink != nil {
		params := map[string]string{
			"accountId": accountId,
			"webIdLink": *result.WebIdLink,
		}
		resource := h.webIdLinkRoute.GetPath(params)
		webIdResource = &resource
	}

	podParams := map[string]string{
		"accountId": accountId,
		"podId":     result.PodId,
	}
	podResource := h.podIdRoute.GetPath(podParams)

	return &JsonRepresentation{
		Json: map[string]interface{}{
			"pod":           result.PodURL,
			"webId":         result.WebId,
			"podResource":   podResource,
			"webIdResource": webIdResource,
		},
	}, nil
}
