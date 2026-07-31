//go:build cssstub

// Package preferences provides the PreferenceParser interface for extracting preferences from HTTP headers.
package preferences

import "solid-go-main/internal/http/representation"

// PreferenceParser creates RepresentationPreferences based on HTTP headers.
type PreferenceParser interface {
	Handle(headers map[string]string) *representation.RepresentationPreferences
}
