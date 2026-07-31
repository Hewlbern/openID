//go:build cssstub

// Package permissions provides types and utilities for handling authorization permissions.
package permissions

import (
	"net/http"
)

// CreateModesExtractor adds the 'create' access mode if the target resource does not exist.
type CreateModesExtractor struct {
	source      ModesExtractor
	resourceSet ResourceSet
}

// NewCreateModesExtractor creates a new CreateModesExtractor.
func NewCreateModesExtractor(source ModesExtractor, resourceSet ResourceSet) *CreateModesExtractor {
	return &CreateModesExtractor{
		source:      source,
		resourceSet: resourceSet,
	}
}

// Extract implements ModesExtractor.
// It extracts the required permissions for resource creation based on the HTTP request.
func (e *CreateModesExtractor) Extract(r *http.Request) (AccessMap, error) {
	accessMap, err := e.source.Extract(r)
	if err != nil {
		return nil, err
	}

	target := r.URL.Path
	exists, err := e.resourceSet.HasResource(target)
	if err != nil {
		return nil, err
	}

	if !exists {
		if _, ok := accessMap[target]; !ok {
			accessMap[target] = make(map[AccessMode]struct{})
		}
		accessMap[target][Create] = struct{}{}
	}

	return accessMap, nil
}

// IsCreateRequest checks if the request is for resource creation.
func (e *CreateModesExtractor) IsCreateRequest(r *http.Request) bool {
	// POST requests with a Slug header are creation requests
	if r.Method == http.MethodPost && r.Header.Get("Slug") != "" {
		return true
	}

	// PUT requests to non-existent resources are creation requests
	if r.Method == http.MethodPut {
		// This would typically check if the resource exists
		// For now, we'll assume all PUT requests are creation requests
		return true
	}

	return false
}

// IsPostRequest checks if the request is a POST request.
func (e *CreateModesExtractor) IsPostRequest(r *http.Request) bool {
	return r.Method == http.MethodPost
}

// IsPutRequest checks if the request is a PUT request.
func (e *CreateModesExtractor) IsPutRequest(r *http.Request) bool {
	return r.Method == http.MethodPut
}
