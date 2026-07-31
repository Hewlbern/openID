//go:build cssstub

// Package metadatawriter implements a writer that adds constant headers to the response.
package metadatawriter

type ConstantMetadataWriter struct {
	Headers map[string]string
}

func NewConstantMetadataWriter(headers map[string]string) *ConstantMetadataWriter {
	return &ConstantMetadataWriter{Headers: headers}
}

func (w *ConstantMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	for key, value := range w.Headers {
		setHeader(response, key, value)
	}
	return nil
}
