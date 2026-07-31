//go:build cssstub

// Package errorhandler provides interfaces and implementations for error handling in HTTP output.
package errorhandler

type ErrorHandlerArgs struct {
	Error   interface{} // TODO: Replace with concrete HttpError type
	Request interface{} // TODO: Replace with concrete HttpRequest type
}

// ErrorHandler converts an error into a ResponseDescription based on the request preferences.
type ErrorHandler interface {
	CanHandle(input ErrorHandlerArgs) error
	Handle(input ErrorHandlerArgs) (interface{}, error) // TODO: use actual ResponseDescription type
	HandleSafe(input ErrorHandlerArgs) (interface{}, error)
}
