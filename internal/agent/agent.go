package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"solid-go/internal/authn"
	"solid-go/internal/rdf"
	"solid-go/internal/resourcestore"
	"solid-go/internal/wac"
)

// Agent is an AI agent identity bound to a Solid WebID and Ed25519 key.
type Agent struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	WebID       string    `json:"webId"`
	PodPath     string    `json:"podPath"`
	PublicKey   string    `json:"publicKey"` // base64 raw url
	DIDKey      string    `json:"didKey"`
	Created     time.Time `json:"created"`
	privateKey  ed25519.PrivateKey
}

// Registry manages AI agent identities.
type Registry struct {
	Store   *resourcestore.Store
	Tokens  *authn.TokenService
	BaseURL string

	mu     sync.RWMutex
	agents map[string]*Agent // id -> agent
	byWeb  map[string]*Agent
}

func NewRegistry(store *resourcestore.Store, tokens *authn.TokenService, baseURL string) *Registry {
	return &Registry{
		Store:   store,
		Tokens:  tokens,
		BaseURL: strings.TrimRight(baseURL, "/"),
		agents:  map[string]*Agent{},
		byWeb:   map[string]*Agent{},
	}
}

type registerRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey,omitempty"` // optional: client-supplied pubkey
}

type registerResponse struct {
	Agent      *Agent `json:"agent"`
	PrivateKey string `json:"privateKey,omitempty"` // only if server generated
	Token      string `json:"token"`
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

func (r *Registry) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/agents", r.handleAgents)
	mux.HandleFunc("/agents/", r.handleAgentByID)
}

func (r *Registry) handleAgents(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		r.register(w, req)
	case http.MethodGet:
		r.mu.RLock()
		defer r.mu.RUnlock()
		list := make([]*Agent, 0, len(r.agents))
		for _, a := range r.agents {
			list = append(list, publicAgent(a))
		}
		writeJSON(w, list)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Registry) handleAgentByID(w http.ResponseWriter, req *http.Request) {
	id := strings.Trim(strings.TrimPrefix(req.URL.Path, "/agents/"), "/")
	if id == "" {
		r.handleAgents(w, req)
		return
	}
	r.mu.RLock()
	a := r.agents[id]
	r.mu.RUnlock()
	if a == nil {
		http.NotFound(w, req)
		return
	}
	writeJSON(w, publicAgent(a))
}

func (r *Registry) register(w http.ResponseWriter, req *http.Request) {
	var body registerRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		body.Name = "agent"
	}
	var pub ed25519.PublicKey
	var priv ed25519.PrivateKey
	var issuedPriv string
	if body.PublicKey != "" {
		raw, err := base64.RawURLEncoding.DecodeString(body.PublicKey)
		if err != nil {
			raw, err = base64.StdEncoding.DecodeString(body.PublicKey)
		}
		if err != nil || len(raw) != ed25519.PublicKeySize {
			http.Error(w, "invalid publicKey", http.StatusBadRequest)
			return
		}
		pub = ed25519.PublicKey(raw)
	} else {
		var err error
		pub, priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		issuedPriv = base64.RawURLEncoding.EncodeToString(priv)
	}

	id := uuid.NewString()
	slug := "agent-" + id[:8]
	podPath := slug + "/"
	webID := r.BaseURL + "/" + podPath + "profile/card#me"
	pubB64 := base64.RawURLEncoding.EncodeToString(pub)
	didKey := "did:key:z6Mk" + pubB64[:22]

	a := &Agent{
		ID:         id,
		Name:       body.Name,
		WebID:      webID,
		PodPath:    podPath,
		PublicKey:  pubB64,
		DIDKey:     didKey,
		Created:    time.Now().UTC(),
		privateKey: priv,
	}

	if err := r.provision(context.Background(), a); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	r.mu.Lock()
	r.agents[id] = a
	r.byWeb[webID] = a
	r.mu.Unlock()

	tok, _ := r.Tokens.Issue(webID, id, 24*time.Hour)
	writeJSON(w, registerResponse{
		Agent:      publicAgent(a),
		PrivateKey: issuedPriv,
		Token:      tok,
	})
}

func (r *Registry) provision(ctx context.Context, a *Agent) error {
	if _, err := r.Store.PutContainer(ctx, a.PodPath); err != nil {
		return err
	}
	if _, err := r.Store.PutContainer(ctx, a.PodPath+"profile/"); err != nil {
		return err
	}
	if _, err := r.Store.PutContainer(ctx, a.PodPath+"inbox/"); err != nil {
		return err
	}
	profile := agentWebIDTurtle(a)
	if _, err := r.Store.Put(ctx, a.PodPath+"profile/card", "text/turtle", []byte(profile), "", ""); err != nil {
		return err
	}
	acl := wac.DefaultPublicACL(r.BaseURL+"/"+a.PodPath, a.WebID)
	if _, err := r.Store.Put(ctx, strings.TrimSuffix(a.PodPath, "/")+"/.acl", "text/turtle", []byte(acl), "", ""); err != nil {
		return err
	}
	cardACL := wac.DefaultPublicACL(r.BaseURL+"/"+a.PodPath+"profile/card", a.WebID)
	_, err := r.Store.Put(ctx, a.PodPath+"profile/card.acl", "text/turtle", []byte(cardACL), "", "")
	return err
}

func agentWebIDTurtle(a *Agent) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"foaf":   "http://xmlns.com/foaf/0.1/",
		"solid":  "http://www.w3.org/ns/solid/terms#",
		"pim":    "http://www.w3.org/ns/pim/space#",
		"sec":    "https://w3id.org/security#",
		"schema": "http://schema.org/",
	}
	g.AddIRI(a.WebID, "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://xmlns.com/foaf/0.1/Agent")
	g.AddLiteral(a.WebID, "http://xmlns.com/foaf/0.1/name", a.Name)
	storage := strings.TrimSuffix(a.WebID, "profile/card#me")
	g.AddIRI(a.WebID, "http://www.w3.org/ns/pim/space#storage", storage)
	g.AddLiteral(a.WebID, "https://w3id.org/security#publicKeyMultibase", "z"+a.PublicKey)
	g.AddIRI(a.WebID, "http://www.w3.org/ns/solid/terms#oidcIssuer", issuerFromWebID(a.WebID))
	g.AddIRI(a.WebID, "http://schema.org/identifier", a.DIDKey)
	_ = fmt.Sprintf("%v", prefixes)
	return rdf.SerializeTurtle(g, prefixes)
}

func issuerFromWebID(webID string) string {
	if !strings.Contains(webID, "://") {
		return webID
	}
	scheme := strings.SplitN(webID, "://", 2)[0]
	rest := strings.SplitN(webID, "://", 2)[1]
	host := strings.SplitN(rest, "/", 2)[0]
	return scheme + "://" + host
}

func (r *Registry) PublicKey(webID string) (ed25519.PublicKey, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a := r.byWeb[webID]
	if a == nil {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(a.PublicKey)
	if err != nil {
		return nil, false
	}
	return ed25519.PublicKey(raw), true
}

func (r *Registry) PrivateKey(webID string) (ed25519.PrivateKey, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a := r.byWeb[webID]
	if a == nil || a.privateKey == nil {
		return nil, false
	}
	return a.privateKey, true
}

func publicAgent(a *Agent) *Agent {
	cp := *a
	cp.privateKey = nil
	return &cp
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
