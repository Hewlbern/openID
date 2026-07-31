//go:build cssstub

package identity

import (
	"context"
	"log"
)

type ProviderFactory interface {
	GetProvider(ctx context.Context) (OidcProvider, error)
}

type CookieStore interface {
	Get(ctx context.Context, cookie string) (string, error)
}

type InteractionHandler interface {
	HandleSafe(ctx context.Context, input InteractionHandlerInput) (Representation, error)
}

type OidcProvider interface {
	InteractionDetails(ctx context.Context, request interface{}, response interface{}) (Interaction, error)
}

type Interaction interface{}

type OperationHttpHandlerInput struct {
	Operation interface{}
	Request   interface{}
	Response  interface{}
}

type InteractionHandlerInput struct {
	Operation       interface{}
	OidcInteraction Interaction
	AccountId       string
}

type Representation struct {
	Metadata interface{}
	Data     interface{}
}

type IdentityProviderHttpHandler struct {
	providerFactory ProviderFactory
	cookieStore     CookieStore
	handler         InteractionHandler
}

func NewIdentityProviderHttpHandler(args struct {
	ProviderFactory ProviderFactory
	CookieStore     CookieStore
	Handler         InteractionHandler
}) *IdentityProviderHttpHandler {
	return &IdentityProviderHttpHandler{
		providerFactory: args.ProviderFactory,
		cookieStore:     args.CookieStore,
		handler:         args.Handler,
	}
}

func (h *IdentityProviderHttpHandler) Handle(ctx context.Context, input OperationHttpHandlerInput) (Representation, error) {
	var oidcInteraction Interaction
	provider, err := h.providerFactory.GetProvider(ctx)
	if err == nil {
		if interaction, err := provider.InteractionDetails(ctx, input.Request, input.Response); err == nil {
			log.Println("Found an active OIDC interaction.")
			oidcInteraction = interaction
		} else {
			log.Printf("No active OIDC interaction found: %v", err)
		}
	} else {
		log.Printf("Provider error: %v", err)
	}

	// Determine account
	var accountId string
	// Here you would extract the cookie from input.Operation and get the accountId
	// accountId, _ = h.cookieStore.Get(ctx, cookie)

	representation, err := h.handler.HandleSafe(ctx, InteractionHandlerInput{
		Operation:       input.Operation,
		OidcInteraction: oidcInteraction,
		AccountId:       accountId,
	})
	if err != nil {
		return Representation{}, err
	}
	return representation, nil
}
