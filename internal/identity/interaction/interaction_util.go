//go:build cssstub

package interaction

import (
	"errors"
	"log"
)

type InteractionResults map[string]interface{}

type Provider interface {
	SessionProvider
	GrantProvider
}

type SessionProvider interface {
	FindSession(cookie string) (Session, error)
}

type GrantProvider interface {
	FindGrant(grantId string) (Grant, error)
}

type Session interface {
	Find(cookie string) (Session, error)
}

type Grant interface {
	Find(grantId string) (Grant, error)
}

type JsonRepresentation struct {
	Json     map[string]interface{}
	Metadata interface{}
}

const ACCOUNT_PROMPT = "account"

type AccountInteractionResults struct {
	AccountPrompt *string
	InteractionResults
}

func AssertOidcInteraction(oidcInteraction *Interaction) error {
	if oidcInteraction == nil {
		log.Printf("Trying to perform OIDC operation without being in an OIDC authentication flow")
		return errors.New("This action can only be performed as part of an OIDC authentication flow")
	}
	return nil
}

func FinishInteraction(oidcInteraction Interaction, result AccountInteractionResults, mergeWithLastSubmission bool) (string, error) {
	// Placeholder for finish interaction logic
	return "", nil
}

func ForgetWebId(provider Provider, oidcInteraction Interaction) error {
	// Placeholder for forget WebID logic
	return nil
}
