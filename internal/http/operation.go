//go:build cssstub

// Package http provides the Operation type and related interfaces for REST operations.
package http

import (
	"solid-go-main/internal/http/conditions"
	"solid-go-main/internal/http/representation"
)

// Operation represents a single REST operation.
type Operation struct {
	Method      string                                 // The HTTP method (GET/POST/PUT/PATCH/DELETE/etc.)
	Target      representation.ResourceIdentifier      // Identifier of the target
	Preferences *representation.RepresentationPreferences // Representation preferences of the response
	Conditions  conditions.Conditions                  // Conditions the resource must fulfill for a valid operation (optional)
	Body        representation.Representation          // Representation of the body and metadata headers
}
