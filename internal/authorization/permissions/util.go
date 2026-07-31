//go:build cssstub

// Package permissions provides utility functions for permission handling.
package permissions

import (
	"net/http"
	"strings"
)

// PermissionUtil provides utility functions for working with permissions.
type PermissionUtil struct{}

// NewPermissionUtil creates a new PermissionUtil.
func NewPermissionUtil() *PermissionUtil {
	return &PermissionUtil{}
}

// GetRequiredPermissions gets the required permissions for a request.
func (u *PermissionUtil) GetRequiredPermissions(r *http.Request) PermissionSet {
	perms := NewACLPermissionSet()

	// Add permissions based on HTTP method
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		perms.Add(Read)
	case http.MethodPut, http.MethodPost:
		perms.Add(Write)
		perms.Add(Append)
	case http.MethodDelete:
		perms.Add(Write)
	case http.MethodPatch:
		perms.Add(Write)
		if u.isN3Patch(r) {
			perms.Add(Append)
		}
		if u.isSparqlUpdate(r) {
			perms.Add(Append)
		}
	}

	// Add Control permission if needed
	if u.requiresControl(r) {
		perms.Add(Control)
	}

	return perms
}

// isN3Patch checks if the request is an N3 patch operation.
func (u *PermissionUtil) isN3Patch(r *http.Request) bool {
	return r.Method == http.MethodPatch &&
		strings.Contains(r.Header.Get("Content-Type"), "text/n3")
}

// isSparqlUpdate checks if the request is a SPARQL update operation.
func (u *PermissionUtil) isSparqlUpdate(r *http.Request) bool {
	return r.Method == http.MethodPatch &&
		strings.Contains(r.Header.Get("Content-Type"), "application/sparql-update")
}

// requiresControl checks if the request requires Control permission.
func (u *PermissionUtil) requiresControl(r *http.Request) bool {
	// Check for Link header with type
	if link := r.Header.Get("Link"); link != "" {
		if strings.Contains(link, "type") {
			return true
		}
	}

	// Check for specific HTTP methods that might need Control
	switch r.Method {
	case http.MethodDelete:
		return true
	case http.MethodPatch:
		return true
	}

	return false
}

// HasRequiredPermissions checks if a permission set has all required permissions.
func (u *PermissionUtil) HasRequiredPermissions(perms PermissionSet, required PermissionSet) bool {
	for _, mode := range []Permission{
		Read,
		Write,
		Append,
		Control,
	} {
		if required.Has(mode) && !perms.Has(mode) {
			return false
		}
	}
	return true
}

// GetMissingPermissions gets the permissions that are missing from a permission set.
func (u *PermissionUtil) GetMissingPermissions(perms PermissionSet, required PermissionSet) PermissionSet {
	missing := NewACLPermissionSet()
	for _, mode := range []Permission{
		Read,
		Write,
		Append,
		Control,
	} {
		if required.Has(mode) && !perms.Has(mode) {
			missing.Add(mode)
		}
	}
	return missing
}

// IntersectPermissions gets the intersection of two permission sets.
func (u *PermissionUtil) IntersectPermissions(a, b PermissionSet) PermissionSet {
	result := NewACLPermissionSet()
	for _, mode := range []Permission{
		Read,
		Write,
		Append,
		Control,
	} {
		if a.Has(mode) && b.Has(mode) {
			result.Add(mode)
		}
	}
	return result
}

// UnionPermissions gets the union of two permission sets.
func (u *PermissionUtil) UnionPermissions(a, b PermissionSet) PermissionSet {
	result := NewACLPermissionSet()
	for _, mode := range []Permission{
		Read,
		Write,
		Append,
		Control,
	} {
		if a.Has(mode) || b.Has(mode) {
			result.Add(mode)
		}
	}
	return result
}
