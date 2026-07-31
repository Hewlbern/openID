//go:build cssstub

// Package auxiliary provides the SuffixAuxiliaryIdentifierStrategy struct and logic.
package auxiliary

import "fmt"

// SuffixAuxiliaryIdentifierStrategy is a strategy for suffix-based auxiliary identifiers.
type SuffixAuxiliaryIdentifierStrategy struct {
	Suffix string
}

// NewSuffixAuxiliaryIdentifierStrategy constructs a new SuffixAuxiliaryIdentifierStrategy.
func NewSuffixAuxiliaryIdentifierStrategy(suffix string) (*SuffixAuxiliaryIdentifierStrategy, error) {
	if len(suffix) == 0 {
		return nil, fmt.Errorf("Suffix length should be non-zero.")
	}
	return &SuffixAuxiliaryIdentifierStrategy{Suffix: suffix}, nil
}

// GetAuxiliaryIdentifier returns the identifier of the auxiliary resource corresponding to the given resource.
func (s *SuffixAuxiliaryIdentifierStrategy) GetAuxiliaryIdentifier(identifier map[string]string) map[string]string {
	return map[string]string{"path": identifier["path"] + s.Suffix}
}

// GetAuxiliaryIdentifiers returns all the identifiers of corresponding auxiliary resources.
func (s *SuffixAuxiliaryIdentifierStrategy) GetAuxiliaryIdentifiers(identifier map[string]string) []map[string]string {
	return []map[string]string{s.GetAuxiliaryIdentifier(identifier)}
}

// IsAuxiliaryIdentifier checks if the input identifier corresponds to an auxiliary resource.
func (s *SuffixAuxiliaryIdentifierStrategy) IsAuxiliaryIdentifier(identifier map[string]string) bool {
	return len(identifier["path"]) >= len(s.Suffix) && identifier["path"][len(identifier["path"])-len(s.Suffix):] == s.Suffix
}

// GetSubjectIdentifier returns the identifier of the resource which this auxiliary resource is referring to.
func (s *SuffixAuxiliaryIdentifierStrategy) GetSubjectIdentifier(identifier map[string]string) (map[string]string, error) {
	if !s.IsAuxiliaryIdentifier(identifier) {
		return nil, fmt.Errorf("%s does not end on %s so no conversion is possible.", identifier["path"], s.Suffix)
	}
	return map[string]string{"path": identifier["path"][:len(identifier["path"])-len(s.Suffix)]}, nil
}
