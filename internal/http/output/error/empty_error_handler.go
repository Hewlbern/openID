//go:build cssstub

// Package errorhandler implements an ErrorHandler that returns an error response without a body for certain status codes.
package errorhandler

type EmptyErrorHandler struct {
	StatusCodes []int
	Always      bool
}

func NewEmptyErrorHandler(statusCodes []int, always bool) *EmptyErrorHandler {
	return &EmptyErrorHandler{StatusCodes: statusCodes, Always: always}
}

func (h *EmptyErrorHandler) CanHandle(input ErrorHandlerArgs) error {
	// TODO: Implement logic to check if handler can handle the error
	return nil
}

func (h *EmptyErrorHandler) Handle(input ErrorHandlerArgs) (interface{}, error) {
	// TODO: Implement error response creation without body
	return nil, nil
}
