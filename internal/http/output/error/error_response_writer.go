//go:build cssstub

// Package error provides error handling interfaces for HTTP output.
package error

type ErrorResponseWriter interface {
	WriteError(err error) error
}
