//go:build cssstub

// Package metadata provides types and interfaces translated from the TypeScript input/metadata folder.
package metadata

// MetadataParser parses a specific part of an HttpRequest and converts it into metadata.
type MetadataParser interface {
	Handle(args interface{}) (interface{}, error) // TODO: use actual argument and RepresentationMetadata types
}
