//go:build cssstub

// Package authorization provides implementations for authorization and access control.
package authorization

import (
	"solid-go/internal/authentication"
	"solid-go/internal/authorization/permissions"
)

// PermissionReaderInput represents the input for reading permissions
type PermissionReaderInput struct {
	// Credentials of the entity requesting access to resources
	Credentials *authentication.Credentials
	// For each credential, the reader will check which of the given per-resource access modes are available.
	// However, non-exhaustive information about other access modes and resources can still be returned.
	RequestedModes map[string]permissions.PermissionSet
}

// PermissionReader defines the interface for reading permissions.
// It is responsible for determining what permissions are required for a given request.
type PermissionReader interface {
	// Read reads the permissions required for the given request.
	// It returns a map of resource identifiers to their permission sets.
	Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error)
}

// NewPermissionReader creates a new PermissionReader.
func NewPermissionReader() PermissionReader {
	return &defaultPermissionReader{}
}

// defaultPermissionReader is the default implementation of PermissionReader.
type defaultPermissionReader struct{}

// Read implements PermissionReader.
func (r *defaultPermissionReader) Read(input PermissionReaderInput) (map[string]permissions.PermissionSet, error) {
	result := make(map[string]permissions.PermissionSet)

	// For each requested resource and its modes
	for resource, requestedModes := range input.RequestedModes {
		// Create a new permission set for this resource
		perms := permissions.NewPermissionSet()

		// Add permissions based on requested modes
		for mode := range requestedModes {
			perms.Add(mode)
		}

		result[resource] = perms
	}

	return result, nil
}
