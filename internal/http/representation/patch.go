//go:build cssstub

// Package representation provides the Patch struct.
package representation

// Patch represents the changes needed for a PATCH request.
type Patch interface {
	Representation
}
