//go:build cssstub

// Package output provides the ResponseWriter interface.
package output

type ResponseWriter interface {
	Write(response interface{}) error // TODO: use actual response type
}
