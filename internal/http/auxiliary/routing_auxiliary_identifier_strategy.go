//go:build cssstub

// Package auxiliary provides the RoutingAuxiliaryIdentifierStrategy struct and logic.
package auxiliary

// RoutingAuxiliaryIdentifierStrategy is a strategy for routing auxiliary identifiers among multiple strategies.
type RoutingAuxiliaryIdentifierStrategy struct {
	Sources []AuxiliaryIdentifierStrategy
}

// NewRoutingAuxiliaryIdentifierStrategy constructs a new RoutingAuxiliaryIdentifierStrategy.
func NewRoutingAuxiliaryIdentifierStrategy(sources []AuxiliaryIdentifierStrategy) *RoutingAuxiliaryIdentifierStrategy {
	return &RoutingAuxiliaryIdentifierStrategy{Sources: sources}
}

// GetAuxiliaryIdentifier routes to the matching strategy.
func (r *RoutingAuxiliaryIdentifierStrategy) GetAuxiliaryIdentifier(identifier map[string]string) map[string]string {
	return r.getMatchingSource(identifier).GetAuxiliaryIdentifier(identifier)
}

// GetAuxiliaryIdentifiers routes to the matching strategy.
func (r *RoutingAuxiliaryIdentifierStrategy) GetAuxiliaryIdentifiers(identifier map[string]string) []map[string]string {
	return r.getMatchingSource(identifier).GetAuxiliaryIdentifiers(identifier)
}

// IsAuxiliaryIdentifier checks if any strategy matches.
func (r *RoutingAuxiliaryIdentifierStrategy) IsAuxiliaryIdentifier(identifier map[string]string) bool {
	return r.getMatchingSource(identifier) != nil
}

// GetSubjectIdentifier routes to the matching strategy.
func (r *RoutingAuxiliaryIdentifierStrategy) GetSubjectIdentifier(identifier map[string]string) map[string]string {
	return r.getMatchingSource(identifier).GetSubjectIdentifier(identifier)
}

// getMatchingSource returns the first matching strategy for the identifier.
func (r *RoutingAuxiliaryIdentifierStrategy) getMatchingSource(identifier map[string]string) AuxiliaryIdentifierStrategy {
	for _, source := range r.Sources {
		if source.IsAuxiliaryIdentifier(identifier) {
			return source
		}
	}
	return nil
}
