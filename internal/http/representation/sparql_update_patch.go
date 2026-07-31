//go:build cssstub

// Package representation provides the SparqlUpdatePatch struct.
package representation

// Algebra is a placeholder for the SPARQL algebra type.
type Algebra interface{}

// SparqlUpdatePatch is a specific type of Patch corresponding to a SPARQL update.
type SparqlUpdatePatch interface {
	Patch
	Algebra() Algebra
}
