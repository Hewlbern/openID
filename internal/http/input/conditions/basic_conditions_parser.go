//go:build cssstub

// Package conditions provides BasicConditionsParser for parsing HTTP precondition headers.
package conditions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ETagHandler is a placeholder for ETag handling logic.
type ETagHandler interface{}

// BasicConditionsOptions holds parsed precondition options.
type BasicConditionsOptions struct {
	MatchesETag     []string
	NotMatchesETag  []string
	ModifiedSince   *time.Time
	UnmodifiedSince *time.Time
}

// BasicConditions is a placeholder for a Conditions implementation.
type BasicConditions struct {
	ETagHandler ETagHandler
	Options     BasicConditionsOptions
}

// NewBasicConditions creates a new BasicConditions.
func NewBasicConditions(handler ETagHandler, opts BasicConditionsOptions) *BasicConditions {
	return &BasicConditions{ETagHandler: handler, Options: opts}
}

// BasicConditionsParser parses HTTP precondition headers into a Conditions object.
type BasicConditionsParser struct {
	ETagHandler ETagHandler
}

// NewBasicConditionsParser creates a new BasicConditionsParser.
func NewBasicConditionsParser(handler ETagHandler) *BasicConditionsParser {
	return &BasicConditionsParser{ETagHandler: handler}
}

// Handle parses the relevant headers and returns a Conditions object if any are present.
func (p *BasicConditionsParser) Handle(method string, headers map[string]string) (*BasicConditions, error) {
	options := BasicConditionsOptions{
		MatchesETag:    parseTagHeader(headers, "if-match"),
		NotMatchesETag: parseTagHeader(headers, "if-none-match"),
	}
	if len(options.NotMatchesETag) == 0 && (strings.ToUpper(method) == "GET" || strings.ToUpper(method) == "HEAD") {
		if t := parseDateHeader(headers, "if-modified-since"); t != nil {
			options.ModifiedSince = t
		}
	}
	if len(options.MatchesETag) == 0 {
		if t := parseDateHeader(headers, "if-unmodified-since"); t != nil {
			options.UnmodifiedSince = t
		}
	}
	// Only return if at least one condition is set
	if !isOptionsEmpty(options) {
		return NewBasicConditions(p.ETagHandler, options), nil
	}
	return nil, nil
}

// parseDateHeader parses a date header into a time.Time pointer.
func parseDateHeader(headers map[string]string, header string) *time.Time {
	if val, ok := headers[header]; ok && val != "" {
		t, err := httpDateParse(val)
		if err == nil {
			return &t
		}
	}
	return nil
}

// parseTagHeader parses a comma-separated ETag header into a slice.
func parseTagHeader(headers map[string]string, header string) []string {
	if val, ok := headers[header]; ok && val != "" {
		return splitCommaSeparated(val)
	}
	return nil
}

// splitCommaSeparated splits a comma-separated string into a slice.
func splitCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// httpDateParse parses an HTTP date string.
func httpDateParse(val string) (time.Time, error) {
	// Try RFC1123 first
	if t, err := time.Parse(time.RFC1123, val); err == nil {
		return t, nil
	}
	// Try RFC1123Z
	if t, err := time.Parse(time.RFC1123Z, val); err == nil {
		return t, nil
	}
	// Try RFC850
	if t, err := time.Parse(time.RFC850, val); err == nil {
		return t, nil
	}
	// Try ANSI C asctime
	if t, err := time.Parse(time.ANSIC, val); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid HTTP date: %s", val)
}

// isOptionsEmpty returns true if all fields in BasicConditionsOptions are empty.
func isOptionsEmpty(opts BasicConditionsOptions) bool {
	return len(opts.MatchesETag) == 0 && len(opts.NotMatchesETag) == 0 && opts.ModifiedSince == nil && opts.UnmodifiedSince == nil
}

// DebugString returns a JSON debug string for BasicConditionsOptions.
func (opts BasicConditionsOptions) DebugString() string {
	b, _ := json.Marshal(opts)
	return string(b)
}
