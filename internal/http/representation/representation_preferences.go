//go:build cssstub

// Package representation provides the RepresentationPreferences struct and related types.
package representation

// ValuePreferences represents preferred values along a single content negotiation dimension.
type ValuePreferences map[string]float64

// ValuePreference is a single entry of a ValuePreferences object.
type ValuePreference struct {
	Value  string
	Weight float64
}

// RangePart represents a part of a range (e.g., bytes 0-499).
type RangePart struct {
	Start int
	End   *int // optional
}

// Range represents a range preference (e.g., bytes=0-499).
type Range struct {
	Unit  string
	Parts []RangePart
}

// RepresentationPreferences contains preferences along multiple content negotiation dimensions.
type RepresentationPreferences struct {
	Type     ValuePreferences `json:"type,omitempty"`
	Charset  ValuePreferences `json:"charset,omitempty"`
	Datetime ValuePreferences `json:"datetime,omitempty"`
	Encoding ValuePreferences `json:"encoding,omitempty"`
	Language ValuePreferences `json:"language,omitempty"`
	Range    *Range           `json:"range,omitempty"`
}

// NewRepresentationPreferences creates a new RepresentationPreferences with all fields optional.
func NewRepresentationPreferences() *RepresentationPreferences {
	return &RepresentationPreferences{}
}
