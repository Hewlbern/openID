//go:build cssstub

package interaction

import (
	"reflect"
)

type JsonInteractionHandlerInput struct {
	Method          string
	AccountId       *string
	OidcInteraction *Interaction
	// Add other fields as needed
}

type JsonInteractionHandler interface {
	CanHandle(input JsonInteractionHandlerInput) error
	Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error)
}

type InteractionRoute interface {
	GetPath(params map[string]interface{}) (string, error)
}

type ControlHandler struct {
	controls map[string]interface{} // InteractionRoute | JsonInteractionHandler
	source   JsonInteractionHandler
}

func NewControlHandler(controls map[string]interface{}, source JsonInteractionHandler) *ControlHandler {
	return &ControlHandler{
		controls: controls,
		source:   source,
	}
}

func (h *ControlHandler) CanHandle(input JsonInteractionHandlerInput) error {
	if h.source != nil {
		return h.source.CanHandle(input)
	}
	return nil
}

func (h *ControlHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	var result *JsonRepresentation
	if h.source != nil {
		result, _ = h.source.Handle(input)
	}
	controls, _ := h.generateControls(input)
	json := h.mergeControls(result.Json, controls)
	return &JsonRepresentation{
		Json:     json,
		Metadata: result.Metadata,
	}, nil
}

func (h *ControlHandler) isRoute(value interface{}) bool {
	_, ok := value.(InteractionRoute)
	return ok
}

func (h *ControlHandler) generateControls(input JsonInteractionHandlerInput) (map[string]interface{}, error) {
	controls := make(map[string]interface{})
	for key, value := range h.controls {
		controlSet, _ := h.generateControlSet(input, value)
		if controlSet != nil {
			controls[key] = controlSet
		}
	}
	return controls, nil
}

func (h *ControlHandler) generateControlSet(input JsonInteractionHandlerInput, value interface{}) (interface{}, error) {
	if h.isRoute(value) {
		route := value.(InteractionRoute)
		path, err := route.GetPath(map[string]interface{}{"accountId": input.AccountId})
		if err != nil {
			return nil, nil
		}
		return path, nil
	}
	handler := value.(JsonInteractionHandler)
	result, _ := handler.Handle(input)
	if result == nil {
		return nil, nil
	}
	// Check if json is empty
	if reflect.ValueOf(result.Json).Len() == 0 {
		return nil, nil
	}
	return result.Json, nil
}

func (h *ControlHandler) mergeControls(original, controls interface{}) map[string]interface{} {
	if original == nil {
		return controls.(map[string]interface{})
	}
	if controls == nil {
		return original.(map[string]interface{})
	}
	// Placeholder for merge logic
	return original.(map[string]interface{})
}
