//go:build cssstub

// Package authorization provides implementations for reading authentication-related auxiliary resource permissions.
package authorization

import (
	"context"
	"path/filepath"

	"solid-go/internal/authorization/permissions"
)

// AuthAuxiliaryReader reads permissions from authentication-related auxiliary resources.
// It handles reading permissions from resources that are auxiliary to the main resource
// and are specifically related to authentication.
type AuthAuxiliaryReader struct {
	reader PermissionReader
}

// NewAuthAuxiliaryReader creates a new AuthAuxiliaryReader with the given reader.
func NewAuthAuxiliaryReader(reader PermissionReader) *AuthAuxiliaryReader {
	return &AuthAuxiliaryReader{
		reader: reader,
	}
}

// Read implements PermissionReader.
// It reads permissions from authentication-related auxiliary resources associated with the given resource.
func (r *AuthAuxiliaryReader) Read(ctx context.Context, resource string) (permissions.PermissionSet, error) {
	// Get authentication-related auxiliary resource paths
	auxPaths := r.getAuthAuxiliaryPaths(resource)

	// Create a new permission set
	perms := permissions.NewACLPermissionSet()

	// Read permissions from each auxiliary resource
	for _, path := range auxPaths {
		auxPerms, err := r.reader.Read(ctx, path)
		if err != nil {
			continue // Skip this auxiliary resource if it returns an error
		}

		// Add all permissions from this auxiliary resource
		for _, mode := range []permissions.Permission{
			permissions.Read,
			permissions.Write,
			permissions.Append,
			permissions.Control,
		} {
			if auxPerms.Has(mode) {
				perms.Add(mode)
			}
		}
	}

	return perms, nil
}

// getAuthAuxiliaryPaths returns the paths of authentication-related auxiliary resources for the given resource.
func (r *AuthAuxiliaryReader) getAuthAuxiliaryPaths(resource string) []string {
	dir := filepath.Dir(resource)
	base := filepath.Base(resource)
	return []string{
		filepath.Join(dir, "."+base+".auth"),
		filepath.Join(dir, "."+base+".webid"),
	}
}

// GetAuthAuxiliaryPath gets the auth auxiliary resource path
func (r *AuthAuxiliaryReader) GetAuthAuxiliaryPath(path string) string {
	return filepath.Join(filepath.Dir(path), ".auth")
}

// IsAuthAuxiliary checks if a path is an auth auxiliary resource
func (r *AuthAuxiliaryReader) IsAuthAuxiliary(path string) bool {
	return filepath.Base(path) == ".auth"
}

// GetAuthResource gets the auth resource for a path
func (r *AuthAuxiliaryReader) GetAuthResource(path string) string {
	return filepath.Join(filepath.Dir(path), "auth")
}
