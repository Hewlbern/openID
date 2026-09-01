package conversations

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/conversations", s.handleRoot)
	mux.HandleFunc("/conversations/", s.handleItem)
	mux.HandleFunc("/share/c/", s.handleShare)
}

func (s *Service) handleRoot(w http.ResponseWriter, r *http.Request) {
	actor, err := s.actorFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := s.List(r.Context(), actor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conversations": list})
	case http.MethodPost:
		var in SaveInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		c, err := s.Save(r.Context(), actor, in)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, ErrUnauthorized) {
				status = http.StatusUnauthorized
			}
			http.Error(w, err.Error(), status)
			return
		}
		writeJSON(w, http.StatusCreated, s.ResultOf(c))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) handleItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/conversations/"), "/")
	if rest == "" {
		s.handleRoot(w, r)
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	actor, err := s.actorFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		c, err := s.Get(r.Context(), actor, id)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	case action == "share" && r.Method == http.MethodPost:
		var body struct {
			Public bool `json:"public"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		c, err := s.Share(r.Context(), actor, id, body.Public)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, c)
	case action == "unshare" && r.Method == http.MethodPost:
		if err := s.Unshare(r.Context(), actor, id); err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (s *Service) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, SharePrefix), "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	c, err := s.PublicGet(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") || strings.Contains(accept, "ld+json") {
		writeJSON(w, http.StatusOK, c)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(renderShareHTML(c)))
}

func writeStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, ErrUnauthorized):
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case errors.Is(err, ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
