//go:build cssstub

package server

import "net/http"

// WebSocketHandlerInput represents the input for a WebSocket handler.
type WebSocketHandlerInput struct {
	WebSocket      interface{} // Replace with actual WebSocket type if using a library
	UpgradeRequest *http.Request
}

// WebSocketHandler is an interface for handling WebSocket connections.
type WebSocketHandler interface {
	HandleSafe(input WebSocketHandlerInput) error
}
