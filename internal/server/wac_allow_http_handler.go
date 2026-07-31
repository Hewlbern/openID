//go:build cssstub

package server

import (
	"context"
	"log"
)

var validMethods = map[string]bool{"GET": true, "HEAD": true}
var validAclModes = map[AccessMode]bool{
	"read":    true,
	"write":   true,
	"append":  true,
	"control": true,
}

type AclPermissionSet map[AccessMode]bool

type RepresentationMetadata struct {
	Headers map[string]string
}

func (m *RepresentationMetadata) Add(header, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[header] = value
}

type NotModifiedHttpError struct {
	Msg string
}

func (e *NotModifiedHttpError) Error() string { return e.Msg }

// --- WacAllowHttpHandler implementation ---
type WacAllowHttpHandler struct {
	credentialsExtractor CredentialsExtractor
	modesExtractor       ModesExtractor
	permissionReader     PermissionReader
	operationHandler     OperationHttpHandler
}

func NewWacAllowHttpHandler(
	credentialsExtractor CredentialsExtractor,
	modesExtractor ModesExtractor,
	permissionReader PermissionReader,
	operationHandler OperationHttpHandler,
) *WacAllowHttpHandler {
	return &WacAllowHttpHandler{
		credentialsExtractor: credentialsExtractor,
		modesExtractor:       modesExtractor,
		permissionReader:     permissionReader,
		operationHandler:     operationHandler,
	}
}

func (h *WacAllowHttpHandler) Handle(ctx context.Context, input OperationHttpHandlerInput) (ResponseDescription, error) {
	// Call the wrapped handler first
	resp, err := h.operationHandler.HandleSafe(ctx, input)
	if err != nil {
		if _, ok := err.(*NotModifiedHttpError); ok {
			// Continue to add WAC-Allow headers to 304 responses
			log.Printf("WacAllowHttpHandler: NotModifiedHttpError, will add WAC-Allow headers")
		} else {
			log.Printf("WacAllowHttpHandler: operation handler error: %v", err)
			return resp, err
		}
	}

	// Only add WAC-Allow headers for GET/HEAD requests
	operation, ok := input.Operation.(map[string]interface{})
	if !ok {
		return resp, nil
	}
	method, _ := operation["method"].(string)
	if !validMethods[method] {
		return resp, nil
	}

	metadata := &RepresentationMetadata{}
	log.Printf("WacAllowHttpHandler: Determining available permissions.")
	credentials, err := h.credentialsExtractor.HandleSafe(ctx, input.Request)
	if err != nil {
		log.Printf("WacAllowHttpHandler: failed to extract credentials: %v", err)
		return resp, err
	}
	requestedModes, err := h.modesExtractor.HandleSafe(ctx, input.Operation)
	if err != nil {
		log.Printf("WacAllowHttpHandler: failed to extract required modes: %v", err)
		return resp, err
	}
	_, err = h.permissionReader.HandleSafe(ctx, PermissionReaderInput{
		Credentials:    credentials,
		RequestedModes: requestedModes,
	})
	if err != nil {
		log.Printf("WacAllowHttpHandler: failed to read available permissions: %v", err)
		return resp, err
	}

	// Simulate permission set extraction
	permissionSet := AclPermissionSet{"read": true, "write": false} // Placeholder
	var everyone AclPermissionSet
	// For demonstration, just use permissionSet for both
	everyone = permissionSet

	log.Printf("WacAllowHttpHandler: Adding WAC-Allow metadata")
	h.addWacAllowMetadata(metadata, everyone, permissionSet)

	return resp, nil
}

// addWacAllowMetadata adds WAC-Allow headers to the metadata
func (h *WacAllowHttpHandler) addWacAllowMetadata(metadata *RepresentationMetadata, everyone, user AclPermissionSet) {
	modes := make(map[AccessMode]bool)
	for mode := range user {
		modes[mode] = true
	}
	for mode := range everyone {
		modes[mode] = true
	}
	for mode := range modes {
		if validAclModes[mode] {
			capitalizedMode := string(mode)
			if len(capitalizedMode) > 0 {
				capitalizedMode = string(capitalizedMode[0]-32) + capitalizedMode[1:]
			}
			if everyone[mode] {
				metadata.Add("WAC-Allow-Public", capitalizedMode)
			}
			if user[mode] {
				metadata.Add("WAC-Allow-User", capitalizedMode)
			}
		}
	}
}
