//go:build cssstub

package interaction

type StaticInteractionHandler struct {
	response map[string]interface{}
}

func NewStaticInteractionHandler(response map[string]interface{}) *StaticInteractionHandler {
	return &StaticInteractionHandler{
		response: response,
	}
}

func (h *StaticInteractionHandler) CanHandle(input JsonInteractionHandlerInput) error {
	return nil
}

func (h *StaticInteractionHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	return &JsonRepresentation{
		Json: h.response,
	}, nil
}
