//go:build cssstub

package variables

// VariableType is the type of a variable
type VariableType string

const (
	// StringType is a string variable
	StringType VariableType = "string"
	// NumberType is a number variable
	NumberType VariableType = "number"
	// BooleanType is a boolean variable
	BooleanType VariableType = "boolean"
	// ObjectType is an object variable
	ObjectType VariableType = "object"
	// ArrayType is an array variable
	ArrayType VariableType = "array"
)

// Variable is a variable
type Variable struct {
	// Name is the name of the variable
	Name string
	// Type is the type of the variable
	Type VariableType
	// Value is the value of the variable
	Value interface{}
}

// ShorthandResolver resolves shorthands
type ShorthandResolver interface {
	// Resolve resolves a shorthand
	Resolve(shorthand string) (string, error)
}

// CombinedShorthandResolver combines multiple shorthand resolvers
type CombinedShorthandResolver struct {
	resolvers []ShorthandResolver
}

// NewCombinedShorthandResolver creates a new CombinedShorthandResolver
func NewCombinedShorthandResolver(resolvers ...ShorthandResolver) *CombinedShorthandResolver {
	return &CombinedShorthandResolver{
		resolvers: resolvers,
	}
}

// Resolve implements ShorthandResolver.Resolve
func (r *CombinedShorthandResolver) Resolve(shorthand string) (string, error) {
	// Try each resolver
	for _, resolver := range r.resolvers {
		if value, err := resolver.Resolve(shorthand); err == nil {
			return value, nil
		}
	}
	return "", nil
}

// AddResolver adds a resolver to the combined resolver
func (r *CombinedShorthandResolver) AddResolver(resolver ShorthandResolver) {
	r.resolvers = append(r.resolvers, resolver)
}
