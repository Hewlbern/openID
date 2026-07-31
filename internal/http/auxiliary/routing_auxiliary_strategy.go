//go:build cssstub

// Package auxiliary provides the RoutingAuxiliaryStrategy struct and logic.
package auxiliary

// RoutingAuxiliaryStrategy is a strategy for routing auxiliary resources among multiple strategies.
type RoutingAuxiliaryStrategy struct {
	RoutingAuxiliaryIdentifierStrategy
	Sources []AuxiliaryStrategy
}

// NewRoutingAuxiliaryStrategy constructs a new RoutingAuxiliaryStrategy.
func NewRoutingAuxiliaryStrategy(sources []AuxiliaryStrategy) *RoutingAuxiliaryStrategy {
	r := &RoutingAuxiliaryStrategy{
		RoutingAuxiliaryIdentifierStrategy: *NewRoutingAuxiliaryIdentifierStrategy(nil),
		Sources: sources,
	}
	r.RoutingAuxiliaryIdentifierStrategy.Sources = make([]AuxiliaryIdentifierStrategy, len(sources))
	for i, s := range sources {
		r.RoutingAuxiliaryIdentifierStrategy.Sources[i] = s.(AuxiliaryIdentifierStrategy)
	}
	return r
}

// UsesOwnAuthorization returns whether the matching strategy uses its own authorization.
func (r *RoutingAuxiliaryStrategy) UsesOwnAuthorization(identifier map[string]string) bool {
	source := r.getMatchingSource(identifier)
	if s, ok := source.(interface{ UsesOwnAuthorization(map[string]string) bool }); ok {
		return s.UsesOwnAuthorization(identifier)
	}
	return false
}

// IsRequiredInRoot returns whether the matching strategy is required in root.
func (r *RoutingAuxiliaryStrategy) IsRequiredInRoot(identifier map[string]string) bool {
	source := r.getMatchingSource(identifier)
	if s, ok := source.(interface{ IsRequiredInRoot(map[string]string) bool }); ok {
		return s.IsRequiredInRoot(identifier)
	}
	return false
}

// AddMetadata calls addMetadata on the matching or all strategies as appropriate.
func (r *RoutingAuxiliaryStrategy) AddMetadata(metadata interface{}) error {
	identifier := map[string]string{"path": "TODO: extract from metadata"}
	match := false
	for _, source := range r.Sources {
		if source.IsAuxiliaryIdentifier(identifier) {
			if err := source.AddMetadata(metadata); err != nil {
				return err
			}
			match = true
			break
		}
	}
	if !match {
		for _, source := range r.Sources {
			if err := source.AddMetadata(metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

// Validate calls validate on the matching strategy.
func (r *RoutingAuxiliaryStrategy) Validate(representation interface{}) error {
	identifier := map[string]string{"path": "TODO: extract from representation"}
	source := r.getMatchingSource(identifier)
	if s, ok := source.(interface{ Validate(interface{}) error }); ok {
		return s.Validate(representation)
	}
	return nil
}
