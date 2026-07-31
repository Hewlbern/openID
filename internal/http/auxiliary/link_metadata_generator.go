//go:build cssstub

// Package auxiliary provides the LinkMetadataGenerator struct and logic.
package auxiliary

// LinkMetadataGenerator adds a link to the auxiliary resource when called on the subject resource.
type LinkMetadataGenerator struct {
	Link               string // Using string for NamedNode IRI
	IdentifierStrategy AuxiliaryIdentifierStrategy
}

// NewLinkMetadataGenerator constructs a new LinkMetadataGenerator.
func NewLinkMetadataGenerator(link string, identifierStrategy AuxiliaryIdentifierStrategy) *LinkMetadataGenerator {
	return &LinkMetadataGenerator{
		Link:               link,
		IdentifierStrategy: identifierStrategy,
	}
}

// HandleSafe adds a link to the auxiliary resource if the input is not an auxiliary resource.
// Expects metadata to be a map with keys "identifier" (map with key "path") and an "add" method.
func (l *LinkMetadataGenerator) HandleSafe(metadata interface{}) error {
	meta, ok := metadata.(map[string]interface{})
	if !ok {
		return nil // or return an error if you want strict type checking
	}
	identifierObj, ok := meta["identifier"].(map[string]interface{})
	if !ok {
		return nil
	}
	identifier := map[string]interface{}{"path": identifierObj["path"]}
	if !l.IdentifierStrategy.IsAuxiliaryIdentifier(identifier) {
		if addFunc, ok := meta["add"].(func(link string, auxPath string, metaType string)); ok {
			auxID := l.IdentifierStrategy.GetAuxiliaryIdentifier(identifier).(map[string]interface{})
			addFunc(l.Link, auxID["path"].(string), "ResponseMetadata")
		}
	}
	return nil
}
