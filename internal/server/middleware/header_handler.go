//go:build cssstub

package middleware

import "net/http"

// HeaderHandler sets custom headers on the response.
type HeaderHandler struct {
	headers map[string]string
}

// NewHeaderHandler creates a new HeaderHandler with the given headers.
func NewHeaderHandler(headers map[string]string) *HeaderHandler {
	return &HeaderHandler{headers: headers}
}

// Handle sets the custom headers on the response.
func (h *HeaderHandler) Handle(resp http.ResponseWriter) {
	for k, v := range h.headers {
		resp.Header().Set(k, v)
	}
}
