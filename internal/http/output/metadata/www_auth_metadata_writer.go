//go:build cssstub

// Package metadatawriter implements a writer that adds the WWW-Authenticate header for 401 responses.
package metadatawriter

type WwwAuthMetadataWriter struct {
	Auth string
}

func NewWwwAuthMetadataWriter(auth string) *WwwAuthMetadataWriter {
	return &WwwAuthMetadataWriter{Auth: auth}
}

func (w *WwwAuthMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	if status, ok := metadata["statusCodeNumber"].(string); ok && status == "401" {
		setHeader(response, "WWW-Authenticate", w.Auth)
	}
	return nil
}
