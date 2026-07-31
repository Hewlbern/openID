//go:build cssstub

package interaction

type Interaction interface{}

type Operation interface{}

type Representation struct {
	Metadata interface{}
	Data     interface{}
}

type InteractionHandlerInput struct {
	Operation       Operation
	OidcInteraction *Interaction
	AccountId       *string
}

type InteractionHandler interface {
	Handle(input InteractionHandlerInput) (*Representation, error)
}
