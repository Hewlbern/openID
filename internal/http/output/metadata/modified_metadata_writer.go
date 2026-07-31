//go:build cssstub

// Package metadatawriter implements a writer that generates Last-Modified and ETag headers.
package metadatawriter

type ModifiedMetadataWriter struct{}

func (w *ModifiedMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	if modified, ok := metadata["modified"].(string); ok && modified != "" {
		setHeader(response, "Last-Modified", modified) // Should be formatted as RFC1123 if needed
	}
	if etag, ok := metadata["etag"].(string); ok && etag != "" {
		setHeader(response, "ETag", etag)
	}
	return nil
}
