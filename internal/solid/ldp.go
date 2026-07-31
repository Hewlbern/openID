package solid

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"solid-go/internal/authn"
	"solid-go/internal/logging"
	"solid-go/internal/rdf"
	"solid-go/internal/resourcestore"
	"solid-go/internal/wac"
)

// AuditHook is called after successful mutating operations.
type AuditHook func(ctx context.Context, agentWebID, method, path string, body []byte)

// NotifyHook is called after resource changes.
type NotifyHook func(path, activity string)

// LDPHandler serves Solid LDP HTTP operations.
type LDPHandler struct {
	Store   *resourcestore.Store
	WAC     *wac.Checker
	Tokens  *authn.TokenService
	BaseURL string
	Logger  logging.Logger
	OnAudit AuditHook
	OnNotify NotifyHook
}

func (h *LDPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	creds, err := h.Tokens.Extract(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	agent := ""
	if creds != nil {
		agent = creds.WebID
	}

	switch r.Method {
	case http.MethodOptions:
		h.writeOptions(w, path, agent)
		return
	case http.MethodGet:
		h.handleGet(w, r, path, agent, false)
	case http.MethodHead:
		h.handleGet(w, r, path, agent, true)
	case http.MethodPut:
		h.handlePut(w, r, path, agent)
	case http.MethodPost:
		h.handlePost(w, r, path, agent)
	case http.MethodPatch:
		h.handlePatch(w, r, path, agent)
	case http.MethodDelete:
		h.handleDelete(w, r, path, agent)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *LDPHandler) authorize(w http.ResponseWriter, path, agent string, required wac.Mode) bool {
	modes, ok, err := h.WAC.Allowed(rContext(), path, agent, required)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	h.writeWACAllow(w, modes)
	if !ok {
		if agent == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="solid", DPoP realm="solid"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return false
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
		return false
	}
	return true
}

func rContext() context.Context { return context.Background() }

func (h *LDPHandler) writeWACAllow(w http.ResponseWriter, m wac.Mode) {
	var parts []string
	user := []string{}
	pub := []string{}
	if m.Read {
		user = append(user, "read")
		pub = append(pub, "read")
	}
	if m.Write {
		user = append(user, "write")
	}
	if m.Append {
		user = append(user, "append")
	}
	if m.Control {
		user = append(user, "control")
	}
	if len(user) > 0 {
		parts = append(parts, `user="`+strings.Join(user, " ")+`"`)
	}
	if len(pub) > 0 {
		parts = append(parts, `public="`+strings.Join(pub, " ")+`"`)
	}
	if len(parts) > 0 {
		w.Header().Set("WAC-Allow", strings.Join(parts, ","))
	}
}

func (h *LDPHandler) writeOptions(w http.ResponseWriter, path, agent string) {
	modes, _, _ := h.WAC.Allowed(rContext(), path, agent, wac.Mode{Read: true})
	h.writeWACAllow(w, modes)
	w.Header().Set("Allow", "OPTIONS, HEAD, GET, PUT, POST, PATCH, DELETE")
	w.Header().Set("Accept-Patch", "text/n3, application/sparql-update")
	w.Header().Set("Accept-Post", "text/turtle, application/ld+json, application/octet-stream")
	w.WriteHeader(http.StatusOK)
}

func (h *LDPHandler) handleGet(w http.ResponseWriter, r *http.Request, path, agent string, head bool) {
	if !h.authorize(w, path, agent, wac.Mode{Read: true}) {
		return
	}
	ctx := r.Context()
	res, err := h.Store.Get(ctx, path)
	if err != nil {
		if errors.Is(err, resourcestore.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body := res.Body
	ct := res.ContentType
	if res.IsContainer {
		ct = "text/turtle"
		body = h.containerTurtle(path, res)
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("ETag", res.ETag)
	w.Header().Set("Last-Modified", res.Modified.UTC().Format(http.TimeFormat))
	h.writeLinks(w, path, res.IsContainer)
	ifMatch := r.Header.Get("If-None-Match")
	if ifMatch != "" && ifMatch == res.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	if !head {
		_, _ = w.Write(body)
	}
}

func (h *LDPHandler) containerTurtle(path string, res *resourcestore.Resource) []byte {
	ctx := context.Background()
	children, _ := h.Store.List(ctx, path)
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"ldp":  "http://www.w3.org/ns/ldp#",
		"rdfs": "http://www.w3.org/2000/01/rdf-schema#",
		"xsd":  "http://www.w3.org/2001/XMLSchema#",
		"dct":  "http://purl.org/dc/terms/",
	}
	subj := h.resourceURL(path)
	g.AddIRI(subj, "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://www.w3.org/ns/ldp#Container")
	g.AddIRI(subj, "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://www.w3.org/ns/ldp#BasicContainer")
	g.AddLiteral(subj, "http://purl.org/dc/terms/modified", res.Modified.Format(time.RFC3339))
	for _, c := range children {
		g.AddIRI(subj, "http://www.w3.org/ns/ldp#contains", h.resourceURL(c))
	}
	return []byte(rdf.SerializeTurtle(g, prefixes))
}

func (h *LDPHandler) resourceURL(path string) string {
	base := strings.TrimRight(h.BaseURL, "/")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return base + "/"
	}
	return base + "/" + path
}

func (h *LDPHandler) writeLinks(w http.ResponseWriter, path string, isContainer bool) {
	acl := resourcestore.AuxPath(path, ".acl")
	meta := resourcestore.AuxPath(path, ".meta")
	w.Header().Add("Link", `<`+h.resourceURL(acl)+`>; rel="acl"`)
	w.Header().Add("Link", `<`+h.resourceURL(meta)+`>; rel="describedby"`)
	if isContainer {
		w.Header().Add("Link", `<http://www.w3.org/ns/ldp#BasicContainer>; rel="type"`)
		w.Header().Add("Link", `<http://www.w3.org/ns/ldp#Container>; rel="type"`)
	} else {
		w.Header().Add("Link", `<http://www.w3.org/ns/ldp#Resource>; rel="type"`)
	}
	w.Header().Add("Link", `<`+h.BaseURL+`/.well-known/solid>; rel="http://www.w3.org/ns/solid/terms#storageDescription"`)
}

func (h *LDPHandler) handlePut(w http.ResponseWriter, r *http.Request, path, agent string) {
	link := r.Header.Get("Link")
	isContainer := strings.Contains(link, "BasicContainer") || strings.Contains(link, "Container") || strings.HasSuffix(path, "/")
	required := wac.Mode{Write: true}
	if !h.authorize(w, path, agent, required) {
		return
	}
	ctx := r.Context()
	if isContainer {
		res, err := h.Store.PutContainer(ctx, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Location", h.resourceURL(res.Path))
		w.Header().Set("ETag", res.ETag)
		h.writeLinks(w, res.Path, true)
		w.WriteHeader(http.StatusCreated)
		h.fire(ctx, agent, "PUT", res.Path, nil)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	// strip charset
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	res, err := h.Store.Put(ctx, path, ct, body, r.Header.Get("If-Match"), r.Header.Get("If-None-Match"))
	if err != nil {
		if errors.Is(err, resourcestore.ErrPrecondition) {
			http.Error(w, "Precondition Failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", h.resourceURL(res.Path))
	w.Header().Set("ETag", res.ETag)
	h.writeLinks(w, res.Path, false)
	// Created vs OK — we don't track prior existence cheaply after put; use 201
	w.WriteHeader(http.StatusCreated)
	h.fire(ctx, agent, "PUT", res.Path, body)
}

func (h *LDPHandler) handlePost(w http.ResponseWriter, r *http.Request, path, agent string) {
	if !h.authorize(w, path, agent, wac.Mode{Append: true}) {
		return
	}
	if path != "" && !strings.HasSuffix(path, "/") {
		// posting to document not allowed
		http.Error(w, "Can only POST to containers", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slug := r.Header.Get("Slug")
	ct := r.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	link := r.Header.Get("Link")
	isContainer := strings.Contains(link, "BasicContainer") || strings.Contains(link, "Container")
	res, err := h.Store.Post(r.Context(), path, slug, ct, body, isContainer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", h.resourceURL(res.Path))
	w.Header().Set("ETag", res.ETag)
	h.writeLinks(w, res.Path, res.IsContainer)
	w.WriteHeader(http.StatusCreated)
	h.fire(r.Context(), agent, "POST", res.Path, body)
}

func (h *LDPHandler) handlePatch(w http.ResponseWriter, r *http.Request, path, agent string) {
	ct := r.Header.Get("Content-Type")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	isAppend := false
	required := wac.Mode{Write: true}
	if strings.Contains(ct, "sparql-update") || strings.Contains(ct, "n3") {
		// could be append-only inserts; require write for simplicity unless INSERT-only
	}
	_ = isAppend
	if !h.authorize(w, path, agent, required) {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	res, err := h.Store.Get(ctx, path)
	if err != nil {
		if errors.Is(err, resourcestore.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if res.IsContainer {
		http.Error(w, "Cannot PATCH container body directly", http.StatusBadRequest)
		return
	}
	g, err := rdf.ParseTurtle(string(res.Body))
	if err != nil {
		g = rdf.NewGraph()
	}
	var out *rdf.Graph
	switch {
	case strings.Contains(ct, "sparql-update"):
		out, err = rdf.ApplySPARQLUpdate(g, string(body))
	case strings.Contains(ct, "n3") || ct == "text/turtle":
		out, err = rdf.ApplyN3Patch(g, string(body))
	default:
		http.Error(w, "Unsupported Accept-Patch type", http.StatusUnsupportedMediaType)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	serialized := []byte(rdf.SerializeTurtle(out, map[string]string{}))
	updated, err := h.Store.Put(ctx, path, "text/turtle", serialized, r.Header.Get("If-Match"), "")
	if err != nil {
		if errors.Is(err, resourcestore.ErrPrecondition) {
			http.Error(w, "Precondition Failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", updated.ETag)
	w.WriteHeader(http.StatusOK)
	h.fire(ctx, agent, "PATCH", path, serialized)
}

func (h *LDPHandler) handleDelete(w http.ResponseWriter, r *http.Request, path, agent string) {
	if !h.authorize(w, path, agent, wac.Mode{Write: true}) {
		return
	}
	err := h.Store.Delete(r.Context(), path, r.Header.Get("If-Match"))
	if err != nil {
		if errors.Is(err, resourcestore.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if errors.Is(err, resourcestore.ErrNotEmpty) {
			http.Error(w, "Container not empty", http.StatusConflict)
			return
		}
		if errors.Is(err, resourcestore.ErrPrecondition) {
			http.Error(w, "Precondition Failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	h.fire(r.Context(), agent, "DELETE", path, nil)
}

func (h *LDPHandler) fire(ctx context.Context, agent, method, path string, body []byte) {
	if h.OnAudit != nil {
		h.OnAudit(ctx, agent, method, path, body)
	}
	if h.OnNotify != nil {
		activity := "Update"
		switch method {
		case "POST", "PUT":
			activity = "Create"
		case "DELETE":
			activity = "Delete"
		}
		h.OnNotify(path, activity)
	}
	_ = filepath.Base(path)
}
