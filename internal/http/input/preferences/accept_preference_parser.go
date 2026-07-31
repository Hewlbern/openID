//go:build cssstub

// Package preferences provides AcceptPreferenceParser for extracting preferences from Accept-* headers.
package preferences

import (
	"strings"
	"solid-go-main/internal/http/representation"
)

// AcceptHeader represents a parsed Accept-* header value.
type AcceptHeader struct {
	Range  string
	Weight float64
}

// AcceptParserFunc parses a header value into AcceptHeader slices.
type AcceptParserFunc func(string) []AcceptHeader

// AcceptPreferenceParser extracts preferences from Accept-* headers.
type AcceptPreferenceParser struct{}

// NewAcceptPreferenceParser creates a new AcceptPreferenceParser.
func NewAcceptPreferenceParser() *AcceptPreferenceParser {
	return &AcceptPreferenceParser{}
}

// Handle extracts preferences from Accept-* headers in the request.
func (p *AcceptPreferenceParser) Handle(headers map[string]string) *representation.RepresentationPreferences {
	preferences := &representation.RepresentationPreferences{}
	parsers := []struct {
		Name   string
		Header string
		Parse  AcceptParserFunc
	}{
		{"type", "accept", parseAccept},
		{"charset", "accept-charset", parseAcceptCharset},
		{"encoding", "accept-encoding", parseAcceptEncoding},
		{"language", "accept-language", parseAcceptLanguage},
		{"datetime", "accept-datetime", parseAcceptDateTime},
	}
	for _, parser := range parsers {
		if value, ok := headers[parser.Header]; ok && value != "" {
			result := map[string]float64{}
			for _, h := range parser.Parse(value) {
				result[h.Range] = h.Weight
			}
			if len(result) > 0 {
				switch parser.Name {
				case "type": preferences.Type = result
				case "charset": preferences.Charset = result
				case "encoding": preferences.Encoding = result
				case "language": preferences.Language = result
				case "datetime": preferences.Datetime = result
				}
			}
		}
	}
	return preferences
}

// The following are stub parser functions. Replace with real implementations as needed.
func parseAccept(value string) []AcceptHeader          { return parseAcceptGeneric(value) }
func parseAcceptCharset(value string) []AcceptHeader   { return parseAcceptGeneric(value) }
func parseAcceptEncoding(value string) []AcceptHeader  { return parseAcceptGeneric(value) }
func parseAcceptLanguage(value string) []AcceptHeader  { return parseAcceptGeneric(value) }
func parseAcceptDateTime(value string) []AcceptHeader  { return parseAcceptGeneric(value) }

// parseAcceptGeneric is a stub that splits comma-separated values and assigns weight 1.0.
func parseAcceptGeneric(value string) []AcceptHeader {
	parts := strings.Split(value, ",")
	headers := make([]AcceptHeader, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			headers = append(headers, AcceptHeader{Range: p, Weight: 1.0})
		}
	}
	return headers
}
