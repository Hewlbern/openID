//go:build cssstub

// Package input provides the RequestParser interface.
package input

// RequestParser converts an incoming HttpRequest to an Operation.
type RequestParser interface {
	Handle(request interface{}) (interface{}, error) // TODO: use actual HttpRequest and Operation types
}
