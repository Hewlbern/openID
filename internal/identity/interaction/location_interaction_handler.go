//go:build cssstub

package interaction

import (
	"log"
)

type RedirectHttpError struct {
	Location string
}

func (e *RedirectHttpError) Error() string {
	return "redirect error"
}

func IsRedirectHttpError(err error) bool {
	_, ok := err.(*RedirectHttpError)
	return ok
}

type LocationInteractionHandler struct {
	source JsonInteractionHandler
}

func NewLocationInteractionHandler(source JsonInteractionHandler) *LocationInteractionHandler {
	return &LocationInteractionHandler{
		source: source,
	}
}

func (h *LocationInteractionHandler) CanHandle(input JsonInteractionHandlerInput) error {
	return h.source.CanHandle(input)
}

func (h *LocationInteractionHandler) Handle(input JsonInteractionHandlerInput) (*JsonRepresentation, error) {
	result, err := h.source.Handle(input)
	if err != nil {
		if IsRedirectHttpError(err) {
			redirectErr := err.(*RedirectHttpError)
			log.Printf("Converting redirect error to location field in JSON body with location %s", redirectErr.Location)
			return &JsonRepresentation{
				Json:     map[string]interface{}{"location": redirectErr.Location},
				Metadata: map[string]interface{}{"location": redirectErr.Location},
			}, nil
		}
		return nil, err
	}
	return result, nil
}
