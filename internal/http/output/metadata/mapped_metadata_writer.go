//go:build cssstub

// Package metadatawriter implements a writer that maps metadata predicates to headers.
package metadatawriter

type MappedMetadataWriter struct {
	HeaderMap map[string]string // predicate URI -> header name
}

func NewMappedMetadataWriter(headerMap map[string]string) *MappedMetadataWriter {
	return &MappedMetadataWriter{HeaderMap: headerMap}
}

func (w *MappedMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	for predicate, header := range w.HeaderMap {
		if values, ok := metadata[predicate].([]string); ok && len(values) > 0 {
			setHeader(response, header, join(values, ","))
		}
	}
	return nil
}
