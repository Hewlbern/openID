//go:build cssstub

// Package metadatawriter implements a writer that adds auxiliary Link headers.
package metadatawriter

type AuxiliaryLinkMetadataWriter struct {
	AuxiliaryStrategy interface{}
	SpecificStrategy  interface{}
	RelationType      string
}

func NewAuxiliaryLinkMetadataWriter(auxStrategy, specStrategy interface{}, relationType string) *AuxiliaryLinkMetadataWriter {
	return &AuxiliaryLinkMetadataWriter{AuxiliaryStrategy: auxStrategy, SpecificStrategy: specStrategy, RelationType: relationType}
}

func (w *AuxiliaryLinkMetadataWriter) Handle(input MetadataWriterInput) error {
	// Implements logic to add a Link header for an auxiliary resource if appropriate.
	response := input.Response.(map[string]interface{}) // TODO: Replace with actual HttpResponse type
	metadata := input.Metadata.(map[string]interface{}) // TODO: Replace with actual RepresentationMetadata type

	var identifier string
	if types, ok := metadata["types"].([]string); ok {
		for _, t := range types {
			if t == "ldp:Resource" {
				identifier = metadata["identifier"].(string)
				break
			}
		}
	}
	if identifier == "" {
		target, ok := metadata["target"].(string)
		if ok {
			identifier = target
		}
	}

	if identifier != "" {
		auxStrategy, ok := w.AuxiliaryStrategy.(interface{ IsAuxiliaryIdentifier(string) bool })
		specStrategy, ok2 := w.SpecificStrategy.(interface{ GetAuxiliaryIdentifier(string) string })
		if ok && ok2 && !auxStrategy.IsAuxiliaryIdentifier(identifier) {
			auxID := specStrategy.GetAuxiliaryIdentifier(identifier)
			setHeader(response, "Link", "<"+auxID+">; rel=\""+w.RelationType+"\"")
		}
	}
	return nil
}
