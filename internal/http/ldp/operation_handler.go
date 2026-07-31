//go:build cssstub

// Package ldp provides the OperationHandler interface and input struct.
package ldp

type OperationHandlerInput struct {
	Operation interface{} // TODO: use actual Operation type from http package
}

type OperationHandler interface {
	CanHandle(input OperationHandlerInput) error
	Handle(input OperationHandlerInput) (interface{}, error) // TODO: use actual ResponseDescription type
}
