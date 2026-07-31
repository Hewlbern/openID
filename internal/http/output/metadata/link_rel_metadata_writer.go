//go:build cssstub

// Package metadatawriter implements a writer that adds Link headers based on metadata predicates.
package metadatawriter

type LinkRelMetadataWriter struct {
	LinkRelMap map[string]string // predicate URI -> rel value
}

func NewLinkRelMetadataWriter(linkRelMap map[string]string) *LinkRelMetadataWriter {
	return &LinkRelMetadataWriter{LinkRelMap: linkRelMap}
}

func (w *LinkRelMetadataWriter) Handle(input MetadataWriterInput) error {
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type
	for predicate, relValue := range w.LinkRelMap {
		if values, ok := metadata[predicate].([]string); ok && len(values) > 0 {
			linkHeaders := []string{}
			for _, v := range values {
				linkHeaders = append(linkHeaders, "<"+v+">; rel=\""+relValue+"\"")
			}
			setHeader(response, "Link", join(linkHeaders, ","))
		}
	}
	return nil
}
