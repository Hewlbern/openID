//go:build cssstub

// Package authorization provides implementations for permission-based authorization.
package authorization

import (
	"context"
	"net/http"

	"solid-go/internal/authorization/access"
	"solid-go/internal/authorization/permissions"
)

// PermissionBasedAuthorizer implements authorization based on permissions.
// It checks if an agent has the required permissions for a resource.
type PermissionBasedAuthorizer struct {
	accessChecker access.AccessChecker
	modeExtractor permissions.ModesExtractor
	reader        PermissionReader
}

// NewPermissionBasedAuthorizer creates a new PermissionBasedAuthorizer.
func NewPermissionBasedAuthorizer(
	checker access.AccessChecker,
	extractor permissions.ModesExtractor,
	reader PermissionReader,
) *PermissionBasedAuthorizer {
	return &PermissionBasedAuthorizer{
		accessChecker: checker,
		modeExtractor: extractor,
		reader:        reader,
	}
}

// Authorize implements Authorizer.
// It checks if the agent has the required permissions for the resource.
func (a *PermissionBasedAuthorizer) Authorize(ctx context.Context, r *http.Request) error {
	// Extract WebID from request
	webID := r.Header.Get("X-WebID")
	if webID == "" {
		return ErrUnauthorized
	}

	// Get required permissions
	modes := a.modeExtractor.Extract(r)

	// Get current permissions
	perms, err := a.reader.Read(ctx, r.URL.Path)
	if err != nil {
		return err
	}

	// Check each required permission
	for _, mode := range []permissions.Permission{
		permissions.Read,
		permissions.Write,
		permissions.Append,
		permissions.Control,
	} {
		if modes.Has(mode) && !perms.Has(mode) {
			return ErrForbidden
		}
	}

	return nil
}
