//go:build cssstub

// Package http provides UnsecureWebSocketsProtocol for Solid WebSockets API Spec solid-0.1.
package http

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"github.com/gorilla/websocket"
	"solid-go-main/internal/http/representation"
)

const WebSocketsVersion = "solid-0.1"

// WebSocketListener handles a single WebSocket connection for live updates.
type WebSocketListener struct {
	conn            *websocket.Conn
	host            string
	protocol        string
	subscribedPaths map[string]struct{}
	mu              sync.Mutex
}

func NewWebSocketListener(conn *websocket.Conn) *WebSocketListener {
	return &WebSocketListener{
		conn:            conn,
		subscribedPaths: make(map[string]struct{}),
	}
}

func (l *WebSocketListener) Start(r *http.Request) {
	l.sendMessage("protocol", WebSocketsVersion)
	protocolHeader := r.Header.Get("Sec-WebSocket-Protocol")
	if protocolHeader != "" {
		if protocolHeader != WebSocketsVersion {
			l.sendMessage("error", fmt.Sprintf("Client does not support protocol %s", WebSocketsVersion))
			l.stop()
			return
		}
	} else {
		l.sendMessage("warning", fmt.Sprintf("Missing Sec-WebSocket-Protocol header, expected value '%s'", WebSocketsVersion))
	}
	l.host = r.Host
	if r.TLS != nil {
		l.protocol = "https:"
	} else {
		l.protocol = "http:"
	}
}

func (l *WebSocketListener) stop() {
	_ = l.conn.Close()
	l.mu.Lock()
	l.subscribedPaths = make(map[string]struct{})
	l.mu.Unlock()
}

func (l *WebSocketListener) onResourceChanged(changed *representation.ResourceIdentifier) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.subscribedPaths[changed.Path]; ok {
		l.sendMessage("pub", changed.Path)
	}
}

func (l *WebSocketListener) onMessage(message string) {
	match := regexp.MustCompile(`^(\w+)\s+(\S.+)$`).FindStringSubmatch(message)
	if len(match) != 3 {
		l.sendMessage("warning", fmt.Sprintf("Unrecognized message format: %s", message))
		return
	}
	typeStr, value := match[1], match[2]
	switch typeStr {
	case "sub":
		l.subscribe(value)
	default:
		l.sendMessage("warning", fmt.Sprintf("Unrecognized message type: %s", typeStr))
	}
}

func (l *WebSocketListener) subscribe(path string) {
	resolved, err := url.Parse(path)
	if err != nil {
		l.sendMessage("error", fmt.Sprintf("Invalid URL: %s", path))
		return
	}
	if resolved.Host != "" && resolved.Host != l.host {
		l.sendMessage("error", fmt.Sprintf("Mismatched host: expected %s but got %s", l.host, resolved.Host))
		return
	}
	if resolved.Scheme != "" && resolved.Scheme+":" != l.protocol {
		l.sendMessage("error", fmt.Sprintf("Mismatched protocol: expected %s but got %s", l.protocol, resolved.Scheme))
		return
	}
	urlStr := resolved.String()
	l.mu.Lock()
	l.subscribedPaths[urlStr] = struct{}{}
	l.mu.Unlock()
	l.sendMessage("ack", urlStr)
}

func (l *WebSocketListener) sendMessage(msgType, value string) {
	_ = l.conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("%s %s", msgType, value)))
}

// UnsecureWebSocketsProtocol provides live update functionality following the Solid WebSockets API Spec solid-0.1.
type UnsecureWebSocketsProtocol struct {
	listeners map[*WebSocketListener]struct{}
	mu        sync.Mutex
	path      string
}

func NewUnsecureWebSocketsProtocol(baseUrl string) *UnsecureWebSocketsProtocol {
	path := "/"
	if u, err := url.Parse(baseUrl); err == nil {
		path = u.Path
	}
	return &UnsecureWebSocketsProtocol{
		listeners: make(map[*WebSocketListener]struct{}),
		path:      path,
	}
}

func (u *UnsecureWebSocketsProtocol) CanHandle(r *http.Request) error {
	if r.URL.Path != u.path {
		return fmt.Errorf("Only WebSocket requests to %s are supported", u.path)
	}
	return nil
}

func (u *UnsecureWebSocketsProtocol) Handle(conn *websocket.Conn, r *http.Request) {
	listener := NewWebSocketListener(conn)
	u.mu.Lock()
	u.listeners[listener] = struct{}{}
	u.mu.Unlock()
	log.Printf("New WebSocket added, %d in total", len(u.listeners))
	listener.Start(r)
	// Listen for close
	go func() {
		defer func() {
			u.mu.Lock()
			delete(u.listeners, listener)
			u.mu.Unlock()
			log.Printf("WebSocket closed, %d remaining", len(u.listeners))
		}()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			listener.onMessage(string(msg))
		}
	}()
}

func (u *UnsecureWebSocketsProtocol) OnResourceChanged(changed *representation.ResourceIdentifier) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for listener := range u.listeners {
		listener.onResourceChanged(changed)
	}
}
