//go:build cssstub

// Package auxiliary provides the AuxiliaryIdentifierStrategy interface.
//
// AuxiliaryIdentifierStrategy defines a strategy for handling auxiliary-related ResourceIdentifiers.
//
// This interface allows implementations to:
//   - Retrieve the identifier of the auxiliary resource corresponding to a given resource.
//   - Retrieve all auxiliary resource identifiers for a given resource.
//   - Check if an identifier is for an auxiliary resource.
//   - Retrieve the subject resource identifier for a given auxiliary resource.
package auxiliary

// AuxiliaryIdentifierStrategy is the interface that wraps the methods for
// handling auxiliary-related ResourceIdentifiers.
type AuxiliaryIdentifierStrategy interface {
	// GetAuxiliaryIdentifier returns the identifier of the auxiliary resource corresponding to the given resource.
	// This does not guarantee that the auxiliary resource exists. Should error if there are multiple results.
	//
	// identifier: The ResourceIdentifier for which to find the auxiliary resource.
	// Returns: The ResourceIdentifier of the corresponding auxiliary resource.
	GetAuxiliaryIdentifier(identifier interface{}) interface{}

	// GetAuxiliaryIdentifiers returns all identifiers of corresponding auxiliary resources.
	// This can be used when there are potentially multiple results. In the case of a single result,
	// this should be a slice containing the result of GetAuxiliaryIdentifier.
	//
	// identifier: The ResourceIdentifier for which to find auxiliary resources.
	// Returns: A slice of ResourceIdentifiers for the corresponding auxiliary resources.
	GetAuxiliaryIdentifiers(identifier interface{}) []interface{}

	// IsAuxiliaryIdentifier checks if the input identifier corresponds to an auxiliary resource.
	// This does not check if that auxiliary resource exists, only if the identifier indicates
	// that there could be an auxiliary resource there.
	//
	// identifier: Identifier to check.
	// Returns: true if the input identifier points to an auxiliary resource.
	IsAuxiliaryIdentifier(identifier interface{}) bool

	// GetSubjectIdentifier returns the identifier of the resource which this auxiliary resource is referring to.
	// This does not guarantee that this resource exists.
	//
	// identifier: Identifier of the auxiliary resource.
	// Returns: The ResourceIdentifier of the subject resource.
	GetSubjectIdentifier(identifier interface{}) interface{}
}
