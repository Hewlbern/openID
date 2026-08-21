package web

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"solid-go/internal/identityapi"
)

//go:embed static/*
var staticFS embed.FS

type Handler struct {
	IDP     *identityapi.Service
	BaseURL string
	files   http.Handler
}

func New(idp *identityapi.Service, baseURL string) *Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		sub = staticFS
	}
	return &Handler{
		IDP:     idp,
		BaseURL: strings.TrimRight(baseURL, "/"),
		files:   http.FileServer(http.FS(sub)),
	}
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.Handle("/static/", http.StripPrefix("/static/", h.files))
	mux.HandleFunc("/app", h.serveApp)
	mux.HandleFunc("/app/", h.serveApp)
	mux.HandleFunc("/i/", h.servePublic)
	mux.HandleFunc("/welcome", h.serveLanding)
	mux.HandleFunc("/welcome/", h.serveLanding)
	mux.HandleFunc("/dashboard", h.ServeDashboard)
	mux.HandleFunc("/dashboard/", h.ServeDashboard)
	mux.HandleFunc("/login", h.ServeDashboard)
	mux.HandleFunc("/records", h.serveRecords)
	mux.HandleFunc("/records/", h.serveRecords)
}

func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "dash.html")
}

func (h *Handler) ServeLanding(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "index.html")
}

func (h *Handler) serveLanding(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "index.html")
}

func (h *Handler) serveApp(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "app.html")
}

func (h *Handler) serveRecords(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, r, "records.html")
}

func (h *Handler) servePublic(w http.ResponseWriter, r *http.Request) {
	handle := strings.Trim(strings.TrimPrefix(r.URL.Path, "/i/"), "/")
	if handle == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if r.Header.Get("Accept") != "" && !strings.Contains(r.Header.Get("Accept"), "text/html") &&
		(strings.Contains(r.Header.Get("Accept"), "application/json") || strings.Contains(r.Header.Get("Accept"), "ld+json")) {
		pub := h.IDP.PublicAccount(handle)
		if pub == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pub)
		return
	}
	h.serveFile(w, r, "profile.html")
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name string) {
	f, err := staticFS.Open("static/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, name, info.ModTime(), rs)
}

func WantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	html := strings.Index(accept, "text/html")
	json := strings.Index(accept, "application/json")
	if html < 0 {
		return false
	}
	if json < 0 {
		return true
	}
	return html < json
}
