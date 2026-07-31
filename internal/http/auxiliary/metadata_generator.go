//go:build cssstub

// Package auxiliary provides the MetadataGenerator interface and base struct.
package auxiliary

// MetadataGenerator generates metadata for resources, e.g., adding RDF triples or link headers.
type MetadataGenerator interface {
	// HandleSafe generates or modifies metadata and returns an error if something goes wrong.
	HandleSafe(metadata interface{}) error
}

// BaseMetadataGenerator can be embedded to provide a default no-op implementation of MetadataGenerator.
type BaseMetadataGenerator struct{}

// HandleSafe is the default implementation that does nothing and returns nil.
func (b *BaseMetadataGenerator) HandleSafe(metadata interface{}) error {
	// Default implementation does nothing.
	return nil
}
