//go:build cssstub

package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

type RequestParser interface {
	HandleSafe(ctx context.Context, r *http.Request) (Operation, error)
}

type ErrorHandler interface {
	HandleSafe(ctx context.Context, err error, r *http.Request) (ResponseDescription, error)
}

type ResponseWriter interface {
	HandleSafe(ctx context.Context, w http.ResponseWriter, result ResponseDescription) error
}

type Operation struct {
	Method string
	Target struct{ Path string }
	// Add fields as needed to represent the parsed operation
}

type InternalServerError struct {
	Msg   string
	Cause error
}

func (e *InternalServerError) Error() string {
	return fmt.Sprintf("InternalServerError: %s (cause: %v)", e.Msg, e.Cause)
}

type ParsingHttpHandler struct {
	requestParser    RequestParser
	errorHandler     ErrorHandler
	responseWriter   ResponseWriter
	operationHandler OperationHttpHandler
}

func NewParsingHttpHandler(
	requestParser RequestParser,
	errorHandler ErrorHandler,
	responseWriter ResponseWriter,
	operationHandler OperationHttpHandler,
) *ParsingHttpHandler {
	return &ParsingHttpHandler{
		requestParser:    requestParser,
		errorHandler:     errorHandler,
		responseWriter:   responseWriter,
		operationHandler: operationHandler,
	}
}

func (h *ParsingHttpHandler) HandleSafe(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	var result ResponseDescription

	log.Printf("ParsingHttpHandler: received %s request for %s", r.Method, r.URL.Path)

	operation, err := h.handleRequest(ctx, r)
	if err != nil {
		result, _ = h.handleError(ctx, err, r)
	} else {
		result, err = h.operationHandler.HandleSafe(ctx, OperationHttpHandlerInput{
			Request:   r,
			Operation: operation,
		})
		if err != nil {
			result, _ = h.handleError(ctx, err, r)
		}
	}

	if (result != ResponseDescription{}) {
		return h.responseWriter.HandleSafe(ctx, w, result)
	}
	return nil
}

// handleRequest parses the request and returns the operation.
func (h *ParsingHttpHandler) handleRequest(ctx context.Context, r *http.Request) (Operation, error) {
	operation, err := h.requestParser.HandleSafe(ctx, r)
	if err != nil {
		return Operation{}, err
	}
	log.Printf("ParsingHttpHandler: parsed %s operation on %s", operation.Method, operation.Target.Path)
	return operation, nil
}

// handleError normalizes and handles errors.
func (h *ParsingHttpHandler) handleError(ctx context.Context, err error, r *http.Request) (ResponseDescription, error) {
	// If not an HttpError, wrap in InternalServerError
	_, isHttpErr := err.(*HttpError)
	if !isHttpErr {
		err = &InternalServerError{
			Msg:   fmt.Sprintf("Received unexpected non-HttpError: %v", err),
			Cause: err,
		}
	}
	log.Printf("ParsingHttpHandler: handling error: %v", err)
	return h.errorHandler.HandleSafe(ctx, err, r)
}
