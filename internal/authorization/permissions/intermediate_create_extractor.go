//go:build cssstub

// Package permissions provides implementations for extracting required permissions from HTTP requests.
package permissions

import (
	"net/http"
	"path"
	"strings"
)

// IntermediateCreateExtractor extracts the required permissions for intermediate resource creation.
// It determines what permissions are needed when creating intermediate resources in a path.
type IntermediateCreateExtractor struct {
	storage Storage
}

// NewIntermediateCreateExtractor creates a new IntermediateCreateExtractor.
func NewIntermediateCreateExtractor(storage Storage) *IntermediateCreateExtractor {
	return &IntermediateCreateExtractor{
		storage: storage,
	}
}

// Extract implements ModesExtractor.
// It extracts the required permissions for intermediate resource creation based on the HTTP request.
func (e *IntermediateCreateExtractor) Extract(r *http.Request) PermissionSet {
	perms := &AclPermissionSet{
		PermissionSet: NewPermissionSet(),
		Control:       false,
	}

	// Check if this is an intermediate resource creation request
	if e.isIntermediateCreateRequest(r) {
		// Add Write permission for creation
		perms.Add(Write)

		// Add Append permission if needed
		if e.requiresAppend(r) {
			perms.Add(Append)
		}

		// Add Control permission if needed
		if e.requiresControl(r) {
			perms.Control = true
		}
	}

	return perms.PermissionSet
}

// isIntermediateCreateRequest checks if the request is for intermediate resource creation.
func (e *IntermediateCreateExtractor) isIntermediateCreateRequest(r *http.Request) bool {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		return false
	}

	// Get all path components
	components := strings.Split(r.URL.Path, "/")
	if len(components) <= 2 {
		return false
	}

	// Check each intermediate path
	currentPath := ""
	for i := 1; i < len(components)-1; i++ {
		currentPath = path.Join(currentPath, components[i])
		if !e.storage.Exists(currentPath) {
			return true
		}
	}

	return false
}

// requiresAppend checks if the request requires Append permission.
func (e *IntermediateCreateExtractor) requiresAppend(r *http.Request) bool {
	// POST requests typically require Append permission
	if r.Method == http.MethodPost {
		return true
	}

	// Check content type for specific formats that require Append
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/n3") || strings.Contains(contentType, "application/sparql-update") {
		return true
	}

	return false
}

// requiresControl checks if the request requires Control permission.
func (e *IntermediateCreateExtractor) requiresControl(r *http.Request) bool {
	// Check for Link header with type
	if link := r.Header.Get("Link"); link != "" {
		if strings.Contains(link, "type") {
			return true
		}
	}

	// Check for specific content types that might need Control
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/n3") || strings.Contains(contentType, "application/sparql-update") {
		return true
	}

	return false
}

// GetRequiredPermissions gets the required permissions for intermediate resource creation.
func (e *IntermediateCreateExtractor) GetRequiredPermissions(r *http.Request) PermissionSet {
	return e.Extract(r)
}

// IsCreateRequest checks if the request is a resource creation request.
func (e *IntermediateCreateExtractor) IsCreateRequest(r *http.Request) bool {
	return r.Method == http.MethodPut || r.Method == http.MethodPost
}

// IsIntermediateCreateRequest checks if the request is for intermediate resource creation.
func (e *IntermediateCreateExtractor) IsIntermediateCreateRequest(r *http.Request) bool {
	return e.isIntermediateCreateRequest(r)
}

// GetIntermediatePaths gets all intermediate paths that need to be created.
func (e *IntermediateCreateExtractor) GetIntermediatePaths(resourcePath string) []string {
	var paths []string
	components := strings.Split(resourcePath, "/")
	currentPath := ""

	for i := 1; i < len(components)-1; i++ {
		currentPath = path.Join(currentPath, components[i])
		if !e.storage.Exists(currentPath) {
			paths = append(paths, currentPath)
		}
	}

	return paths
}
