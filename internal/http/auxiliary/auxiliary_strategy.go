//go:build cssstub

// Package auxiliary provides the AuxiliaryStrategy interface.
package auxiliary

type AuxiliaryStrategy interface {
	// Returns true if the given identifier is an auxiliary resource.
	IsAuxiliary(identifier interface{}) bool
	// Returns the subject identifier for the given auxiliary identifier.
	GetSubjectIdentifier(auxiliaryIdentifier interface{}) (interface{}, error)
	// Returns the auxiliary identifier for the given subject identifier.
	GetAuxiliaryIdentifier(subjectIdentifier interface{}) (interface{}, error)
	// Returns the types of auxiliary resources handled by this strategy.
	GetAuxiliaryTypes() []string
}
