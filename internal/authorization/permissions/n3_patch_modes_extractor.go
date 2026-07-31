//go:build cssstub

// Package permissions provides types and utilities for handling authorization permissions.
package permissions

import (
	"net/http"
)

// N3Patch represents an N3 Patch document.
type N3Patch struct {
	Deletes    []string
	Inserts    []string
	Conditions []string
}

// N3PatchModesExtractor extracts required access modes from an N3 Patch.
type N3PatchModesExtractor struct {
	resourceSet ResourceSet
}

// NewN3PatchModesExtractor creates a new N3PatchModesExtractor.
func NewN3PatchModesExtractor(resourceSet ResourceSet) *N3PatchModesExtractor {
	return &N3PatchModesExtractor{
		resourceSet: resourceSet,
	}
}

// Extract implements ModesExtractor.
func (e *N3PatchModesExtractor) Extract(r *http.Request) (AccessMap, error) {
	// TODO: Parse N3Patch from request body
	var patch N3Patch
	// For now, assume patch is parsed from r.Body

	requiredModes := make(AccessMap)
	target := r.URL.Path

	// When conditions are non-empty, treat as a Read operation
	if len(patch.Conditions) > 0 {
		if _, ok := requiredModes[target]; !ok {
			requiredModes[target] = make(map[AccessMode]struct{})
		}
		requiredModes[target][Read] = struct{}{}
	}

	// When insertions are non-empty, treat as an Append operation
	if len(patch.Inserts) > 0 {
		if _, ok := requiredModes[target]; !ok {
			requiredModes[target] = make(map[AccessMode]struct{})
		}
		requiredModes[target][Append] = struct{}{}
		exists, err := e.resourceSet.HasResource(target)
		if err != nil {
			return nil, err
		}
		if !exists {
			requiredModes[target][Create] = struct{}{}
		}
	}

	// When deletions are non-empty, treat as a Read and Write operation
	if len(patch.Deletes) > 0 {
		if _, ok := requiredModes[target]; !ok {
			requiredModes[target] = make(map[AccessMode]struct{})
		}
		requiredModes[target][Read] = struct{}{}
		requiredModes[target][Write] = struct{}{}
	}

	return requiredModes, nil
}
