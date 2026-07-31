//go:build cssstub

package server

import "net/http"

// HttpRequest is a wrapper for http.Request in Go.
type HttpRequest struct {
	Request *http.Request
}
