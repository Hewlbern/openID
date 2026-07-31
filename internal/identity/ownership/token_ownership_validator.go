//go:build cssstub

package ownership

import (
	"errors"
	"fmt"
	"log"
	"net/url"
)

type ExpiringStorage interface {
	Get(key string) (string, error)
	Set(key, value string, expirationMs int) error
	Delete(key string) error
}

type TokenOwnershipValidator struct {
	storage    ExpiringStorage
	expiration int // milliseconds
}

func NewTokenOwnershipValidator(storage ExpiringStorage, expirationMinutes int) *TokenOwnershipValidator {
	return &TokenOwnershipValidator{
		storage:    storage,
		expiration: expirationMinutes * 60 * 1000,
	}
}

func (v *TokenOwnershipValidator) Handle(input OwnershipValidatorInput) error {
	webId := input.WebId
	key := v.getTokenKey(webId)
	token, _ := v.storage.Get(key)
	if token == "" {
		token = v.generateToken()
		v.storage.Set(key, token, v.expiration)
		return v.throwError(webId, token)
	}
	ok, _ := v.hasToken(webId, token)
	if !ok {
		return v.throwError(webId, token)
	}
	log.Printf("Verified ownership of %s", webId)
	v.storage.Delete(key)
	return nil
}

func (v *TokenOwnershipValidator) getTokenKey(webId string) string {
	return url.QueryEscape(webId)
}

func (v *TokenOwnershipValidator) generateToken() string {
	// Use a UUID generator in real code
	return "random-token-placeholder"
}

func (v *TokenOwnershipValidator) hasToken(webId, token string) (bool, error) {
	// Placeholder: fetch RDF and check for triple
	return false, nil
}

func (v *TokenOwnershipValidator) throwError(webId, token string) error {
	log.Printf("No verification token found for %s", webId)
	details := fmt.Sprintf("<%s> <oidcIssuerRegistrationToken> \"%s\".", webId, token)
	errMsg := fmt.Sprintf("Verification token not found. Please add the RDF triple %s to the WebID document at %s to prove it belongs to you. You can remove this triple again after validation.", details, webId)
	return errors.New(errMsg)
}
