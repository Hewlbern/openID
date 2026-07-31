//go:build cssstub

// Package metadata provides the MetadataParser interface for parsing metadata from HTTP requests.
package metadata

type MetadataParser interface {
	HandleSafe(input map[string]interface{}) (interface{}, error)
}
