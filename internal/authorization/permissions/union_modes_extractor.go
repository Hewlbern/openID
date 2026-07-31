//go:build cssstub

// Package permissions provides implementations for extracting required permissions from HTTP requests.
package permissions

import "net/http"

// UnionModesExtractor combines permissions from multiple extractors.
// It extracts permissions from each extractor and combines them into a single set.
type UnionModesExtractor struct {
	extractors []ModesExtractor
}

// NewUnionModesExtractor creates a new UnionModesExtractor with the given extractors.
func NewUnionModesExtractor(extractors ...ModesExtractor) *UnionModesExtractor {
	return &UnionModesExtractor{
		extractors: extractors,
	}
}

// Extract implements ModesExtractor.
// It extracts permissions from each extractor and combines them into a single set.
func (e *UnionModesExtractor) Extract(r *http.Request) PermissionSet {
	// Create a new permission set
	perms := NewACLPermissionSet()

	// Extract permissions from each extractor
	for _, extractor := range e.extractors {
		extractorPerms := extractor.Extract(r)
		// Add all permissions from this extractor
		for _, mode := range []Permission{
			Read,
			Write,
			Append,
			Control,
		} {
			if extractorPerms.Has(mode) {
				perms.Add(mode)
			}
		}
	}

	return perms
}

// AddExtractor adds a new extractor to the union.
func (e *UnionModesExtractor) AddExtractor(extractor ModesExtractor) {
	e.extractors = append(e.extractors, extractor)
}

// RemoveExtractor removes an extractor from the union.
func (e *UnionModesExtractor) RemoveExtractor(extractor ModesExtractor) {
	for i, ex := range e.extractors {
		if ex == extractor {
			e.extractors = append(e.extractors[:i], e.extractors[i+1:]...)
			break
		}
	}
}

// GetExtractors gets all extractors in the union.
func (e *UnionModesExtractor) GetExtractors() []ModesExtractor {
	return e.extractors
}

// ClearExtractors removes all extractors from the union.
func (e *UnionModesExtractor) ClearExtractors() {
	e.extractors = nil
}

// GetRequiredPermissions gets the required permissions from all extractors.
func (e *UnionModesExtractor) GetRequiredPermissions(r *http.Request) PermissionSet {
	return e.Extract(r)
}

// HasExtractor checks if the union contains a specific extractor.
func (e *UnionModesExtractor) HasExtractor(extractor ModesExtractor) bool {
	for _, ex := range e.extractors {
		if ex == extractor {
			return true
		}
	}
	return false
}

// GetExtractorCount gets the number of extractors in the union.
func (e *UnionModesExtractor) GetExtractorCount() int {
	return len(e.extractors)
}
