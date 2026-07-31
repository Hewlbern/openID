//go:build cssstub

package interaction

const InternalApiVersion = "0.5"

type VersionHandler struct {
	source JsonInteractionHandler
}

func NewVersionHandler(source JsonInteractionHandler) *VersionHandler {
	return &VersionHandler{
		source: source,
	}
}

func (h *VersionHandler) CanHandle(input JsonInteractionHandlerInput) error {
	return h.source.CanHandle(input)
}

func (h *VersionHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	result, err := h.source.Handle(input)
	if err != nil {
		return nil, err
	}
	if result.Json == nil {
		result.Json = make(map[string]interface{})
	}
	result.Json["version"] = InternalApiVersion
	return result, nil
}
