//go:build cssstub

// Package body provides the BodyParser interface for parsing HTTP request bodies.
package body

// BodyParserArgs contains the arguments for the BodyParser.
type BodyParserArgs struct {
	Request  map[string]interface{}
	Metadata map[string]interface{}
}

type BodyParser interface {
	HandleSafe(input map[string]interface{}) (interface{}, error)
}
