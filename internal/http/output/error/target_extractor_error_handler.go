//go:build cssstub

// Package errorhandler implements an ErrorHandler that adds metadata to an error to indicate the targeted resource identifier.
package errorhandler

type TargetExtractorErrorHandler struct {
	ErrorHandler    ErrorHandler
	TargetExtractor interface{} // TODO: Replace with concrete TargetExtractor type
}

func NewTargetExtractorErrorHandler(errorHandler ErrorHandler, targetExtractor interface{}) *TargetExtractorErrorHandler {
	return &TargetExtractorErrorHandler{ErrorHandler: errorHandler, TargetExtractor: targetExtractor}
}

func (h *TargetExtractorErrorHandler) CanHandle(input ErrorHandlerArgs) error {
	// TODO: Delegate to wrapped error handler
	return nil
}

func (h *TargetExtractorErrorHandler) Handle(input ErrorHandlerArgs) (interface{}, error) {
	// TODO: Implement logic to add target metadata and delegate to wrapped handler
	return nil, nil
}
