//go:build cssstub

// Package errorhandler implements a failsafe ErrorHandler that returns a simple text description of an error.
package errorhandler

type SafeErrorHandler struct {
	ErrorHandler   ErrorHandler
	ShowStackTrace bool
}

func NewSafeErrorHandler(errorHandler ErrorHandler, showStackTrace bool) *SafeErrorHandler {
	return &SafeErrorHandler{ErrorHandler: errorHandler, ShowStackTrace: showStackTrace}
}

func (h *SafeErrorHandler) Handle(input ErrorHandlerArgs) (interface{}, error) {
	// TODO: Implement failsafe error handling
	return nil, nil
}
