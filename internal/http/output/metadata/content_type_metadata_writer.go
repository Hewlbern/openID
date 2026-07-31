//go:build cssstub

// Package metadatawriter implements a writer that adds the Content-Type header.
package metadatawriter

type ContentTypeMetadataWriter struct{}

func (w *ContentTypeMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	if contentType, ok := metadata["contentTypeObject"].(string); ok && contentType != "" {
		setHeader(response, "Content-Type", contentType)
	}
	return nil
}
