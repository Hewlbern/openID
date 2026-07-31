//go:build cssstub

package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// WebSocketAdvertiser advertises a WebSocket through the Updates-Via header.
type WebSocketAdvertiser struct {
	socketUrl string
}

// NewWebSocketAdvertiser creates a new WebSocketAdvertiser with the given base URL.
func NewWebSocketAdvertiser(baseUrl string) *WebSocketAdvertiser {
	u, _ := url.Parse(baseUrl)
	if hasScheme(baseUrl, "http", "ws") {
		u.Scheme = "ws"
	} else {
		u.Scheme = "wss"
	}
	return &WebSocketAdvertiser{socketUrl: u.String()}
}

// Handle sets the Updates-Via header on the response.
func (w *WebSocketAdvertiser) Handle(resp http.ResponseWriter) {
	resp.Header().Set("Updates-Via", w.socketUrl)
}

// hasScheme checks if the URL starts with any of the given schemes.
func hasScheme(u string, schemes ...string) bool {
	for _, scheme := range schemes {
		if strings.HasPrefix(u, scheme+"://") {
			return true
		}
	}
	return false
}
