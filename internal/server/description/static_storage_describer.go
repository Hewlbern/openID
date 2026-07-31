//go:build cssstub

package description

import (
	"errors"
)

// --- Stubs for RDF types ---
type RDFTerm struct {
	Value string
	Type  string // e.g., "NamedNode", "Literal"
}

type RDFQuad struct {
	Subject   RDFTerm
	Predicate RDFTerm
	Object    RDFTerm
}

// --- StaticStorageDescriber implementation ---
type StaticStorageDescriber struct {
	terms map[string][]string // predicate URI -> object URIs
}

// NewStaticStorageDescriber creates a new StaticStorageDescriber.
func NewStaticStorageDescriber(terms map[string][]string) (*StaticStorageDescriber, error) {
	if terms == nil {
		return nil, errors.New("terms map cannot be nil")
	}
	return &StaticStorageDescriber{terms: terms}, nil
}

// Handle generates RDF triples for the storage description resource.
func (s *StaticStorageDescriber) Handle(target ResourceIdentifier) ([]RDFQuad, error) {
	subject := RDFTerm{Value: target.Path, Type: "NamedNode"}
	return s.generateTriples(subject), nil
}

// generateTriples yields all triples for the subject.
func (s *StaticStorageDescriber) generateTriples(subject RDFTerm) []RDFQuad {
	var quads []RDFQuad
	for predicate, objects := range s.terms {
		predTerm := RDFTerm{Value: predicate, Type: "NamedNode"}
		for _, obj := range objects {
			objTerm := RDFTerm{Value: obj, Type: "NamedNode"} // For simplicity, treat all as NamedNode
			quads = append(quads, RDFQuad{Subject: subject, Predicate: predTerm, Object: objTerm})
		}
	}
	return quads
}
