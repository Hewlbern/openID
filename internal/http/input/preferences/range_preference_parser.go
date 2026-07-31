//go:build cssstub

// Package preferences provides RangePreferenceParser for parsing Range headers.
package preferences

import (
	"fmt"
	"strconv"
	"strings"
	"solid-go-main/internal/http/representation"
)

// RangePreferenceParser parses the Range header into range preferences.
type RangePreferenceParser struct{}

// NewRangePreferenceParser creates a new RangePreferenceParser.
func NewRangePreferenceParser() *RangePreferenceParser {
	return &RangePreferenceParser{}
}

// Handle parses the Range header and returns RepresentationPreferences.
func (p *RangePreferenceParser) Handle(headers map[string]string) (*representation.RepresentationPreferences, error) {
	rangeHeader, ok := headers["range"]
	if !ok || rangeHeader == "" {
		return &representation.RepresentationPreferences{}, nil
	}
	parts := strings.SplitN(rangeHeader, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range header format: %s", rangeHeader)
	}
	unit := strings.TrimSpace(parts[0])
	rangeTail := strings.TrimSpace(parts[1])
	if unit == "" {
		return nil, fmt.Errorf("missing unit value from range header: %s", rangeHeader)
	}
	ranges := strings.Split(rangeTail, ",")
	rangeParts := []representation.RangePart{}
	for _, entry := range ranges {
		entry = strings.TrimSpace(entry)
		se := strings.SplitN(entry, "-", 2)
		if len(se) != 2 {
			return nil, fmt.Errorf("invalid range header format: %s", rangeHeader)
		}
		start := strings.TrimSpace(se[0])
		end := strings.TrimSpace(se[1])
		if start == "" {
			if end == "" {
				return nil, fmt.Errorf("invalid range header format: %s", rangeHeader)
			}
			endVal, err := strconv.Atoi(end)
			if err != nil {
				return nil, fmt.Errorf("invalid end value in range header: %s", rangeHeader)
			}
			rangeParts = append(rangeParts, representation.RangePart{Start: -endVal})
		} else {
			startVal, err := strconv.Atoi(start)
			if err != nil {
				return nil, fmt.Errorf("invalid start value in range header: %s", rangeHeader)
			}
			var endVal *int
			if end != "" {
				v, err := strconv.Atoi(end)
				if err != nil {
					return nil, fmt.Errorf("invalid end value in range header: %s", rangeHeader)
				}
				endVal = &v
			}
			rangeParts = append(rangeParts, representation.RangePart{Start: startVal, End: endVal})
		}
	}
	return &representation.RepresentationPreferences{
		Range: &representation.Range{
			Unit:  unit,
			Parts: rangeParts,
		},
	}, nil
}
