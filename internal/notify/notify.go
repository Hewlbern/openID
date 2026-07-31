package notify

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub broadcasts Solid notification activities (WebSocketChannel2023-style + SSE).
type Hub struct {
	mu      sync.RWMutex
	subs    map[string]map[chan []byte]bool // topic path -> subscribers
	wsSubs  map[*websocket.Conn]string
	upgrader websocket.Upgrader
}

func NewHub() *Hub {
	return &Hub{
		subs:   map[string]map[chan []byte]bool{},
		wsSubs: map[*websocket.Conn]string{},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

type Activity struct {
	Type      string    `json:"type"`
	Object    string    `json:"object"`
	Published time.Time `json:"published"`
	ID        string    `json:"id,omitempty"`
}

func (h *Hub) Publish(path, activityType string) {
	act := Activity{
		Type:      activityType,
		Object:    path,
		Published: time.Now().UTC(),
		ID:        "urn:activity:" + time.Now().Format("20060102150405.000000000"),
	}
	raw, _ := json.Marshal(act)
	h.mu.RLock()
	defer h.mu.RUnlock()
	// notify exact + parent topics
	topics := []string{path, "", "*"}
	for _, t := range topics {
		for ch := range h.subs[t] {
			select {
			case ch <- raw:
			default:
			}
		}
	}
	for conn, topic := range h.wsSubs {
		if topic == "*" || topic == path || topic == "" {
			_ = conn.WriteMessage(websocket.TextMessage, raw)
		}
	}
}

func (h *Hub) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/notifications/websocket", h.handleWS)
	mux.HandleFunc("/notifications/stream", h.handleSSE)
	mux.HandleFunc("/notifications/", h.handleSubscribeDescription)
}

func (h *Hub) handleSubscribeDescription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/ld+json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"@context": "https://www.w3.org/ns/solid/notification/v1",
		"type":     []string{"NotificationChannel2023", "WebSocketChannel2023"},
		"receiveFrom": map[string]string{
			"websocket": "/notifications/websocket",
			"streaming": "/notifications/stream",
		},
	})
}

func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) {
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		topic = "*"
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.wsSubs[conn] = topic
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.wsSubs, conn)
		h.mu.Unlock()
		_ = conn.Close()
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	topic := r.URL.Query().Get("topic")
	if topic == "" {
		topic = "*"
	}
	ch := make(chan []byte, 16)
	h.mu.Lock()
	if h.subs[topic] == nil {
		h.subs[topic] = map[chan []byte]bool{}
	}
	h.subs[topic][ch] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.subs[topic], ch)
		h.mu.Unlock()
		close(ch)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-ch:
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(msg)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}
