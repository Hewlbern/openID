//go:build cssstub

// Package representation provides the N3Patch struct.
package representation

// Quad is a placeholder for RDF quads.
type Quad interface{}

// N3Patch is a representation of an N3 Patch, including deletes, inserts, and conditions.
type N3Patch interface {
	Patch
	GetDeletes() []Quad
	GetInserts() []Quad
	GetConditions() []Quad
}

// IsN3Patch checks if the given value is an N3Patch.
func IsN3Patch(patch interface{}) bool {
	if p, ok := patch.(N3Patch); ok {
		return p.GetDeletes() != nil && p.GetInserts() != nil && p.GetConditions() != nil
	}
	return false
}
