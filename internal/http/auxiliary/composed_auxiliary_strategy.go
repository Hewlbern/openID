//go:build cssstub

// Package auxiliary provides the ComposedAuxiliaryStrategy struct and related logic.
package auxiliary

// ComposedAuxiliaryStrategy provides AuxiliaryStrategy functionality by combining
// an AuxiliaryIdentifierStrategy, MetadataGenerator, and Validator.
type ComposedAuxiliaryStrategy struct {
	IdentifierStrategy AuxiliaryIdentifierStrategy
	MetadataGenerator MetadataGenerator // optional
	Validator         Validator        // optional
	OwnAuthorization  bool
	RequiredInRoot    bool
}

// NewComposedAuxiliaryStrategy constructs a new ComposedAuxiliaryStrategy.
func NewComposedAuxiliaryStrategy(
	identifierStrategy AuxiliaryIdentifierStrategy,
	metadataGenerator MetadataGenerator,
	validator Validator,
	ownAuthorization bool,
	requiredInRoot bool,
) *ComposedAuxiliaryStrategy {
	return &ComposedAuxiliaryStrategy{
		IdentifierStrategy: identifierStrategy,
		MetadataGenerator: metadataGenerator,
		Validator:         validator,
		OwnAuthorization:  ownAuthorization,
		RequiredInRoot:    requiredInRoot,
	}
}

func (c *ComposedAuxiliaryStrategy) GetAuxiliaryIdentifier(identifier interface{}) interface{} {
	return c.IdentifierStrategy.GetAuxiliaryIdentifier(identifier)
}

func (c *ComposedAuxiliaryStrategy) GetAuxiliaryIdentifiers(identifier interface{}) []interface{} {
	return c.IdentifierStrategy.GetAuxiliaryIdentifiers(identifier)
}

func (c *ComposedAuxiliaryStrategy) IsAuxiliaryIdentifier(identifier interface{}) bool {
	return c.IdentifierStrategy.IsAuxiliaryIdentifier(identifier)
}

func (c *ComposedAuxiliaryStrategy) GetSubjectIdentifier(identifier interface{}) interface{} {
	return c.IdentifierStrategy.GetSubjectIdentifier(identifier)
}

func (c *ComposedAuxiliaryStrategy) UsesOwnAuthorization() bool {
	return c.OwnAuthorization
}

func (c *ComposedAuxiliaryStrategy) IsRequiredInRoot() bool {
	return c.RequiredInRoot
}

func (c *ComposedAuxiliaryStrategy) AddMetadata(metadata interface{}) error {
	if c.MetadataGenerator != nil {
		return c.MetadataGenerator.HandleSafe(metadata)
	}
	return nil
}

func (c *ComposedAuxiliaryStrategy) Validate(representation interface{}) error {
	if c.Validator != nil {
		// Attempt to extract identifier from the representation if possible
		var identifier interface{}
		if repMap, ok := representation.(map[string]interface{}); ok {
			if meta, ok := repMap["metadata"].(map[string]interface{}); ok {
				if id, ok := meta["identifier"]; ok {
					identifier = id
				}
			}
		}
		return c.Validator.HandleSafe(ValidatorInput{
			Representation: representation,
			Identifier:     identifier,
		})
	}
	return nil
}
