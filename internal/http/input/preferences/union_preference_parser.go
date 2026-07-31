//go:build cssstub

// Package preferences provides UnionPreferenceParser for combining multiple PreferenceParsers.
package preferences

import (
	"errors"
	"solid-go-main/internal/http/representation"
)

// PreferenceParser interface for all preference parsers.
type PreferenceParser interface {
	Handle(headers map[string]string) *representation.RepresentationPreferences
}

// UnionPreferenceParser combines the results of multiple PreferenceParsers.
type UnionPreferenceParser struct {
	Parsers []PreferenceParser
}

// NewUnionPreferenceParser creates a new UnionPreferenceParser.
func NewUnionPreferenceParser(parsers []PreferenceParser) *UnionPreferenceParser {
	return &UnionPreferenceParser{Parsers: parsers}
}

// Handle combines the results of all parsers.
func (u *UnionPreferenceParser) Handle(headers map[string]string) (*representation.RepresentationPreferences, error) {
	results := make([]*representation.RepresentationPreferences, 0, len(u.Parsers))
	for _, parser := range u.Parsers {
		if parser == nil {
			continue
		}
		prefs := parser.Handle(headers)
		results = append(results, prefs)
	}
	rangeCount := 0
	for _, result := range results {
		if result.Range != nil {
			rangeCount++
		}
	}
	if rangeCount > 1 {
		return nil, errors.New("found multiple range values; this implies a misconfiguration")
	}
	preferences := &representation.RepresentationPreferences{}
	for _, result := range results {
		if result == nil {
			continue
		}
		if result.Range != nil {
			preferences.Range = result.Range
		}
		if result.Type != nil {
			if preferences.Type == nil {
				preferences.Type = representation.ValuePreferences{}
			}
			for k, v := range result.Type {
				preferences.Type[k] = v
			}
		}
		if result.Charset != nil {
			if preferences.Charset == nil {
				preferences.Charset = representation.ValuePreferences{}
			}
			for k, v := range result.Charset {
				preferences.Charset[k] = v
			}
		}
		if result.Encoding != nil {
			if preferences.Encoding == nil {
				preferences.Encoding = representation.ValuePreferences{}
			}
			for k, v := range result.Encoding {
				preferences.Encoding[k] = v
			}
		}
		if result.Language != nil {
			if preferences.Language == nil {
				preferences.Language = representation.ValuePreferences{}
			}
			for k, v := range result.Language {
				preferences.Language[k] = v
			}
		}
		if result.Datetime != nil {
			if preferences.Datetime == nil {
				preferences.Datetime = representation.ValuePreferences{}
			}
			for k, v := range result.Datetime {
				preferences.Datetime[k] = v
			}
		}
	}
	return preferences, nil
}
