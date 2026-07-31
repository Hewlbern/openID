//go:build cssstub

package interaction

type OidcControlHandler struct {
	*ControlHandler
}

func NewOidcControlHandler(controls map[string]interface{}, source JsonInteractionHandler) *OidcControlHandler {
	return &OidcControlHandler{
		ControlHandler: NewControlHandler(controls, source),
	}
}

func (h *OidcControlHandler) GenerateControls(input JsonInteractionHandlerInput) (map[string]interface{}, error) {
	if input.OidcInteraction == nil {
		return make(map[string]interface{}), nil
	}
	return h.ControlHandler.generateControls(input)
}
