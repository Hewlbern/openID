package identityapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"solid-go/internal/rdf"
	"solid-go/internal/resourcestore"
)

func (s *Service) handleRecordsSPARQL(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	acc := s.accountFromRequest(r)
	if acc == nil {
		w.Header().Set("WWW-Authenticate", `Bearer realm="solid"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	query, err := readSPARQLQuery(r)
	if err != nil || strings.TrimSpace(query) == "" {
		http.Error(w, "sparql: missing query", http.StatusBadRequest)
		return
	}
	g, err := s.loadRecordsGraph(r.Context(), acc.Handle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := rdf.QuerySPARQL(g, query, rdf.DefaultSPARQLPrefixes(s.BaseURL))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/sparql-results+json")
	_ = json.NewEncoder(w).Encode(res)
}

func readSPARQLQuery(r *http.Request) (string, error) {
	if q := r.URL.Query().Get("query"); q != "" {
		return q, nil
	}
	if r.Method != http.MethodPost {
		return "", nil
	}
	ct := r.Header.Get("Content-Type")
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if strings.Contains(ct, "application/sparql-query") || strings.Contains(ct, "text/plain") {
		return string(body), nil
	}
	if strings.Contains(ct, "application/json") {
		var payload struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		return payload.Query, nil
	}
	if strings.Contains(ct, "application/x-www-form-urlencoded") {
		vals, err := parseFormQuery(string(body))
		if err != nil {
			return "", err
		}
		return vals, nil
	}
	return string(body), nil
}

func parseFormQuery(body string) (string, error) {
	vals, err := url.ParseQuery(body)
	if err != nil {
		return "", err
	}
	return vals.Get("query"), nil
}

func (s *Service) loadRecordsGraph(ctx context.Context, handle string) (*rdf.Graph, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	g := rdf.NewGraph()
	paths := []string{
		handle + "/records/cursor/transcripts.jsonld",
		handle + "/records/catalog.jsonld",
		handle + "/records/catalog.ttl",
	}
	for _, path := range paths {
		res, err := s.Store.Get(ctx, path)
		if err != nil {
			if err == resourcestore.ErrNotFound {
				continue
			}
			return nil, err
		}
		part, err := graphFromResource(res)
		if err != nil {
			return nil, err
		}
		g.Triples = append(g.Triples, part.Triples...)
	}
	return g, nil
}

func graphFromResource(res *resourcestore.Resource) (*rdf.Graph, error) {
	ct := strings.ToLower(res.ContentType)
	if strings.Contains(ct, "turtle") || strings.HasSuffix(res.Path, ".ttl") {
		return rdf.ParseTurtle(string(res.Body))
	}
	return rdf.GraphFromJSONLD(res.Body)
}
