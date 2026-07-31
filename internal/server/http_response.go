//go:build cssstub

package server

import "net/http"

// HttpResponse is a wrapper for http.ResponseWriter in Go.
type HttpResponse struct {
	Writer http.ResponseWriter
}
