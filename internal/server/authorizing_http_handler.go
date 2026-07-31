//go:build cssstub

package server

import (
	"context"
	"encoding/json"
	"log"
)

// --- Types for credentials, access, and errors ---
type Credentials struct {
	// Add fields as needed
}

type AccessMode string

type Identifier struct {
	Path string
}

type AccessMap map[Identifier][]AccessMode

type PermissionReaderInput struct {
	Credentials    Credentials
	RequestedModes AccessMap
}

type AuthorizerInput struct {
	Credentials          Credentials
	RequestedModes       AccessMap
	AvailablePermissions map[Identifier][]AccessMode
}

type OperationHttpHandlerInput struct {
	Request   interface{}
	Operation interface{}
}

type ResponseDescription struct{}

// --- Custom error type for attaching metadata ---
type Metadata map[string]interface{}

type HttpError struct {
	Msg      string
	Metadata Metadata
}

func (e *HttpError) Error() string {
	return e.Msg
}

func NewHttpError(msg string) *HttpError {
	return &HttpError{Msg: msg, Metadata: make(Metadata)}
}

// --- Interfaces for dependencies ---
type CredentialsExtractor interface {
	HandleSafe(ctx context.Context, request interface{}) (Credentials, error)
}
type ModesExtractor interface {
	HandleSafe(ctx context.Context, operation interface{}) (AccessMap, error)
}
type PermissionReader interface {
	HandleSafe(ctx context.Context, input PermissionReaderInput) (map[Identifier][]AccessMode, error)
}
type Authorizer interface {
	HandleSafe(ctx context.Context, input AuthorizerInput) error
}
type OperationHttpHandler interface {
	HandleSafe(ctx context.Context, input OperationHttpHandlerInput) (ResponseDescription, error)
}

// --- AuthorizingHttpHandler implementation ---
type AuthorizingHttpHandler struct {
	credentialsExtractor CredentialsExtractor
	modesExtractor       ModesExtractor
	permissionReader     PermissionReader
	authorizer           Authorizer
	operationHandler     OperationHttpHandler
}

func NewAuthorizingHttpHandler(
	credentialsExtractor CredentialsExtractor,
	modesExtractor ModesExtractor,
	permissionReader PermissionReader,
	authorizer Authorizer,
	operationHandler OperationHttpHandler,
) *AuthorizingHttpHandler {
	return &AuthorizingHttpHandler{
		credentialsExtractor: credentialsExtractor,
		modesExtractor:       modesExtractor,
		permissionReader:     permissionReader,
		authorizer:           authorizer,
		operationHandler:     operationHandler,
	}
}

func (h *AuthorizingHttpHandler) Handle(ctx context.Context, input OperationHttpHandlerInput) (ResponseDescription, error) {
	credentials, err := h.credentialsExtractor.HandleSafe(ctx, input.Request)
	if err != nil {
		log.Printf("Failed to extract credentials: %v", err)
		return ResponseDescription{}, err
	}
	credJson, _ := json.Marshal(credentials)
	log.Printf("Extracted credentials: %s", credJson)

	requestedModes, err := h.modesExtractor.HandleSafe(ctx, input.Operation)
	if err != nil {
		log.Printf("Failed to extract required modes: %v", err)
		return ResponseDescription{}, err
	}
	log.Printf("Retrieved required modes: %v", requestedModes)

	availablePermissions, err := h.permissionReader.HandleSafe(ctx, PermissionReaderInput{
		Credentials:    credentials,
		RequestedModes: requestedModes,
	})
	if err != nil {
		log.Printf("Failed to read available permissions: %v", err)
		return ResponseDescription{}, err
	}
	log.Printf("Available permissions: %v", availablePermissions)

	authInput := AuthorizerInput{
		Credentials:          credentials,
		RequestedModes:       requestedModes,
		AvailablePermissions: availablePermissions,
	}
	err = h.authorizer.HandleSafe(ctx, authInput)
	if err != nil {
		log.Printf("Authorization failed: %v", err)
		if httpErr, ok := err.(*HttpError); ok {
			h.addAccessModesToError(httpErr, requestedModes)
		}
		return ResponseDescription{}, err
	}

	log.Printf("Authorization succeeded, calling source handler")
	return h.operationHandler.HandleSafe(ctx, input)
}

// addAccessModesToError attaches requested access info to the error's metadata
func (h *AuthorizingHttpHandler) addAccessModesToError(err *HttpError, requestedModes AccessMap) {
	var requested []map[string]interface{}
	for identifier, modes := range requestedModes {
		entry := map[string]interface{}{
			"accessTarget": identifier.Path,
			"accessModes":  modes,
		}
		requested = append(requested, entry)
	}
	err.Metadata["requestedAccess"] = requested
}
