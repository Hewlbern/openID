//go:build cssstub

// Package output provides the BasicResponseWriter interface.
package output

type BasicResponseWriter interface {
	WriteResponse(response interface{}) error // TODO: use actual response type
}
