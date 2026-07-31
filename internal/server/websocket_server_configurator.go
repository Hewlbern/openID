//go:build cssstub

package server

import (
	"fmt"
	"log"
	"net/http"
)

// WebSocketServerConfigurator adds WebSocket upgrade handling to an http.Server.
type WebSocketServerConfigurator struct {
	handler WebSocketHandler
}

// NewWebSocketServerConfigurator creates a new configurator with the given handler.
func NewWebSocketServerConfigurator(handler WebSocketHandler) *WebSocketServerConfigurator {
	return &WebSocketServerConfigurator{handler: handler}
}

// HandleSafe attaches the WebSocket upgrade handler to the http.Server.
func (c *WebSocketServerConfigurator) HandleSafe(server *http.Server) error {
	origHandler := server.Handler
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Connection") == "Upgrade" && r.Header.Get("Upgrade") == "websocket" {
			log.Printf("WebSocketServerConfigurator: received WebSocket upgrade request for %s", r.URL.Path)
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
				return
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				log.Printf("WebSocketServerConfigurator: hijack error: %v", err)
				return
			}
			defer conn.Close()
			// Perform minimal WebSocket handshake (RFC 6455 not fully implemented here)
			fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			// Pass the raw connection to the handler
			input := WebSocketHandlerInput{
				WebSocket:      conn, // In a real implementation, wrap this in a WebSocket abstraction
				UpgradeRequest: r,
			}
			if err := c.handler.HandleSafe(input); err != nil {
				log.Printf("WebSocketServerConfigurator: handler error: %v", err)
			}
			return
		}
		// Fallback to original handler
		if origHandler != nil {
			origHandler.ServeHTTP(w, r)
		} else {
			http.NotFound(w, r)
		}
	})
	return nil
}
