//go:build cssstub

// Package conditions provides the ConditionsParser interface for parsing HTTP request conditions.
package conditions

// Conditions represents the result of parsing HTTP precondition headers.
type Conditions interface{}

// ConditionsParser creates a Conditions object based on the input request headers.
type ConditionsParser interface {
	Handle(method string, headers map[string]string) (Conditions, error)
}
