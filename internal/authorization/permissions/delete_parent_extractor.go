//go:build cssstub

// Package permissions provides types and utilities for handling authorization permissions.
package permissions

import (
	"net/http"
)

// DeleteParentExtractor adds read access on the parent container if the target resource does not exist during a delete operation.
type DeleteParentExtractor struct {
	source             ModesExtractor
	resourceSet        ResourceSet
	identifierStrategy IdentifierStrategy
}

// NewDeleteParentExtractor creates a new DeleteParentExtractor.
func NewDeleteParentExtractor(source ModesExtractor, resourceSet ResourceSet, identifierStrategy IdentifierStrategy) *DeleteParentExtractor {
	return &DeleteParentExtractor{
		source:             source,
		resourceSet:        resourceSet,
		identifierStrategy: identifierStrategy,
	}
}

// Extract implements ModesExtractor.
func (e *DeleteParentExtractor) Extract(r *http.Request) (AccessMap, error) {
	accessMap, err := e.source.Extract(r)
	if err != nil {
		return nil, err
	}

	target := r.URL.Path
	if _, ok := accessMap[target]; ok {
		if _, ok := accessMap[target][Delete]; ok {
			if !e.identifierStrategy.IsRootContainer(target) {
				exists, err := e.resourceSet.HasResource(target)
				if err != nil {
					return nil, err
				}
				if !exists {
					parent := e.identifierStrategy.GetParentContainer(target)
					if _, ok := accessMap[parent]; !ok {
						accessMap[parent] = make(map[AccessMode]struct{})
					}
					accessMap[parent][Read] = struct{}{}
				}
			}
		}
	}

	return accessMap, nil
}

// IdentifierStrategy is an interface for handling resource identifiers.
type IdentifierStrategy interface {
	IsRootContainer(path string) bool
	GetParentContainer(path string) string
}
