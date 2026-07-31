//go:build cssstub

// Package body implements a BodyParser that converts incoming requests to a BasicRepresentation without further parsing.
package body

import (
	"errors"
)

// RawBodyParser converts incoming HTTP requests to a BasicRepresentation.
type RawBodyParser struct{}

// CanHandle always returns nil (no-op for raw parser).
func (p *RawBodyParser) CanHandle(args BodyParserArgs) error {
	return nil
}

// Handle converts the request to a BasicRepresentation, validating headers.
func (p *RawBodyParser) Handle(args BodyParserArgs) (interface{}, error) {
	// TODO: Implement header checks and return BasicRepresentation
	return nil, errors.New("RawBodyParser.Handle not implemented")
}
