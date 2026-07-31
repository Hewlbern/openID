//go:build cssstub

package server

import (
	"net/http"
)

// HttpServerFactory is an interface for creating HTTP servers.
type HttpServerFactory interface {
	CreateServer() (*http.Server, error)
}

// IsHttpsServer returns true if the server is configured for HTTPS (has a TLSConfig).
func IsHttpsServer(server *http.Server) bool {
	return server.TLSConfig != nil && len(server.TLSConfig.Certificates) > 0
}
