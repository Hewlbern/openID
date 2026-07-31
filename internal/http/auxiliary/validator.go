//go:build cssstub

// Package auxiliary provides the Validator interface and base struct.
package auxiliary

// ValidatorInput represents the input for a Validator, containing a Representation and its ResourceIdentifier.
type ValidatorInput struct {
	Representation map[string]interface{} // Use a map for flexible representation
	Identifier     map[string]interface{} // Use a map for flexible identifier
}

// Validator validates Representations in some way, e.g., for shape or content.
type Validator interface {
	HandleSafe(input ValidatorInput) error
}

// BaseValidator can be embedded for default implementations.
type BaseValidator struct{}

func (b *BaseValidator) HandleSafe(input ValidatorInput) error {
	// Default implementation does nothing.
	return nil
}
