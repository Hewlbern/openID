//go:build cssstub

package server

import "net/http"

// HttpHandlerInput represents the input for an HTTP handler.
type HttpHandlerInput struct {
	Request  *http.Request
	Response http.ResponseWriter
}

// The HttpHandler interface is already defined elsewhere in the codebase.
