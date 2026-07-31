//go:build cssstub

// Package metadata provides metadata writing interfaces for HTTP output.
package metadata

type MetadataWriter interface {
	WriteMetadata(metadata interface{}) error // TODO: use actual metadata type
}
