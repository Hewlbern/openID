package identityapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"solid-go/internal/authn"
	"solid-go/internal/rdf"
	"solid-go/internal/resourcestore"
	"solid-go/internal/wac"
)

// Service provides account, pod, WebID, and client-credentials identity APIs.
type Service struct {
	Store   *resourcestore.Store
	Tokens  *authn.TokenService
	BaseURL string

	mu       sync.RWMutex
	accounts map[string]*Account // email -> account
	clients  map[string]*ClientCredentials
	sessions map[string]string // cookie -> accountID
}

type Account struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	WebID        string `json:"webId"`
	PodPath      string `json:"podPath"`
	Created      time.Time `json:"created"`
}

type ClientCredentials struct {
	ID       string `json:"id"`
	Secret   string `json:"secret,omitempty"`
	WebID    string `json:"webId"`
	AccountID string `json:"accountId"`
	Name     string `json:"name"`
}

func New(store *resourcestore.Store, tokens *authn.TokenService, baseURL string) *Service {
	return &Service{
		Store:    store,
		Tokens:   tokens,
		BaseURL:  strings.TrimRight(baseURL, "/"),
		accounts: map[string]*Account{},
		clients:  map[string]*ClientCredentials{},
		sessions: map[string]string{},
	}
}

func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/idp/", s.handleIDP)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleOIDCConfig)
	mux.HandleFunc("/.well-known/solid", s.handleSolidDescription)
	mux.HandleFunc("/oauth/token", s.handleToken)
}

func (s *Service) handleSolidDescription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/ld+json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"@context": "https://www.w3.org/ns/solid/oidc-context.jsonld",
		"@id":      s.BaseURL + "/",
		"provider": s.BaseURL + "/idp/",
	})
}

func (s *Service) handleOIDCConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"issuer":                                s.BaseURL,
		"authorization_endpoint":                s.BaseURL + "/idp/auth",
		"token_endpoint":                        s.BaseURL + "/oauth/token",
		"registration_endpoint":                 s.BaseURL + "/idp/register",
		"jwks_uri":                              s.BaseURL + "/idp/jwks",
		"scopes_supported":                      []string{"openid", "profile", "offline_access", "webid"},
		"response_types_supported":              []string{"code", "id_token", "token"},
		"grant_types_supported":                 []string{"authorization_code", "client_credentials", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"dpop_signing_alg_values_supported":     []string{"ES256", "EdDSA"},
		"solid_oidc_supported":                  "https://solidproject.org/TR/oidc",
	})
}

func (s *Service) handleIDP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/idp/")
	switch {
	case path == "register" && r.Method == http.MethodPost:
		s.register(w, r)
	case path == "login" && r.Method == http.MethodPost:
		s.login(w, r)
	case path == "logout" && r.Method == http.MethodPost:
		s.logout(w, r)
	case path == "accounts/me" && r.Method == http.MethodGet:
		s.me(w, r)
	case path == "pods" && r.Method == http.MethodPost:
		s.createPod(w, r)
	case path == "client-credentials" && r.Method == http.MethodPost:
		s.createClientCredentials(w, r)
	case path == "auth" && r.Method == http.MethodGet:
		// simplified authorize: redirect with code
		s.authorize(w, r)
	case path == "jwks":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"controls": map[string]string{
				"register":           s.BaseURL + "/idp/register",
				"login":              s.BaseURL + "/idp/login",
				"logout":             s.BaseURL + "/idp/logout",
				"account":            s.BaseURL + "/idp/accounts/me",
				"createPod":          s.BaseURL + "/idp/pods",
				"clientCredentials":  s.BaseURL + "/idp/client-credentials",
			},
			"version": "solid-go/1.0",
		})
	}
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	CreatePod bool  `json:"createPod"`
}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[req.Email]; ok {
		http.Error(w, "account exists", http.StatusConflict)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id := uuid.NewString()
	slug := sanitizeSlug(strings.Split(req.Email, "@")[0])
	podPath := slug + "/"
	webID := s.BaseURL + "/" + podPath + "profile/card#me"
	acc := &Account{
		ID:           id,
		Email:        req.Email,
		PasswordHash: string(hash),
		WebID:        webID,
		PodPath:      podPath,
		Created:      time.Now().UTC(),
	}
	s.accounts[req.Email] = acc
	if req.CreatePod || true {
		if err := s.provisionPod(context.Background(), acc, req.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	token, _ := s.Tokens.Issue(acc.WebID, "", 24*time.Hour)
	sid := uuid.NewString()
	s.sessions[sid] = acc.ID
	http.SetCookie(w, &http.Cookie{Name: "solid-session", Value: sid, Path: "/", HttpOnly: true})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"account": acc,
		"token":   token,
		"webId":   acc.WebID,
		"pod":     s.BaseURL + "/" + acc.PodPath,
	})
}

func (s *Service) provisionPod(ctx context.Context, acc *Account, name string) error {
	if _, err := s.Store.PutContainer(ctx, acc.PodPath); err != nil {
		return err
	}
	if _, err := s.Store.PutContainer(ctx, acc.PodPath+"profile/"); err != nil {
		return err
	}
	if name == "" {
		name = "Solid User"
	}
	profile := webIDProfileTurtle(acc.WebID, name, s.BaseURL+"/"+acc.PodPath)
	if _, err := s.Store.Put(ctx, acc.PodPath+"profile/card", "text/turtle", []byte(profile), "", "*"); err != nil {
		// allow overwrite
		_, err = s.Store.Put(ctx, acc.PodPath+"profile/card", "text/turtle", []byte(profile), "", "")
		if err != nil {
			return err
		}
	}
	acl := wac.DefaultPublicACL(s.BaseURL+"/"+acc.PodPath, acc.WebID)
	_, err := s.Store.Put(ctx, strings.TrimSuffix(acc.PodPath, "/")+"/.acl", "text/turtle", []byte(acl), "", "")
	if err != nil {
		return err
	}
	_, err = s.Store.Put(ctx, acc.PodPath+"profile/card.acl", "text/turtle", []byte(wac.DefaultPublicACL(s.BaseURL+"/"+acc.PodPath+"profile/card", acc.WebID)), "", "")
	return err
}

func webIDProfileTurtle(webID, name, storage string) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"foaf":   "http://xmlns.com/foaf/0.1/",
		"solid":  "http://www.w3.org/ns/solid/terms#",
		"pim":    "http://www.w3.org/ns/pim/space#",
		"schema": "http://schema.org/",
	}
	g.AddIRI(webID, "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://xmlns.com/foaf/0.1/Person")
	g.AddLiteral(webID, "http://xmlns.com/foaf/0.1/name", name)
	g.AddIRI(webID, "http://www.w3.org/ns/pim/space#storage", storage)
	issuer := storage
	if i := strings.Index(strings.TrimPrefix(storage, "https://"), "/"); i >= 0 {
		// storage is baseURL/pod/ — issuer is baseURL
	}
	if idx := strings.LastIndex(strings.TrimSuffix(storage, "/"), "/"); idx > 0 {
		// trim pod segment
		rest := strings.TrimSuffix(storage, "/")
		if j := strings.LastIndex(rest, "/"); j > len("http://x") {
			issuer = rest[:j]
		}
	}
	_ = issuer
	// Prefer scheme+host from webID
	if u := webID; strings.Contains(u, "://") {
		without := strings.SplitN(u, "://", 2)[1]
		hostpath := strings.SplitN(without, "/", 2)[0]
		scheme := strings.SplitN(u, "://", 2)[0]
		g.AddIRI(webID, "http://www.w3.org/ns/solid/terms#oidcIssuer", scheme+"://"+hostpath)
	}
	return rdf.SerializeTurtle(g, prefixes)
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	acc, ok := s.accounts[strings.ToLower(req.Email)]
	s.mu.RUnlock()
	if !ok || bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, _ := s.Tokens.Issue(acc.WebID, "", 24*time.Hour)
	sid := uuid.NewString()
	s.mu.Lock()
	s.sessions[sid] = acc.ID
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "solid-session", Value: sid, Path: "/", HttpOnly: true})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"token": token, "webId": acc.WebID, "account": acc})
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("solid-session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "solid-session", Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) accountFromRequest(r *http.Request) *Account {
	c, err := r.Cookie("solid-session")
	if err != nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.sessions[c.Value]
	if !ok {
		return nil
	}
	for _, a := range s.accounts {
		if a.ID == id {
			return a
		}
	}
	return nil
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	acc := s.accountFromRequest(r)
	if acc == nil {
		// try bearer
		creds, err := s.Tokens.Extract(r)
		if err != nil || creds.WebID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.mu.RLock()
		for _, a := range s.accounts {
			if a.WebID == creds.WebID {
				acc = a
				break
			}
		}
		s.mu.RUnlock()
	}
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(acc)
}

func (s *Service) createPod(w http.ResponseWriter, r *http.Request) {
	acc := s.accountFromRequest(r)
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.provisionPod(r.Context(), acc, body.Name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"pod":   s.BaseURL + "/" + acc.PodPath,
		"webId": acc.WebID,
	})
}

func (s *Service) createClientCredentials(w http.ResponseWriter, r *http.Request) {
	acc := s.accountFromRequest(r)
	if acc == nil {
		creds, err := s.Tokens.Extract(r)
		if err != nil || creds.WebID == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.mu.RLock()
		for _, a := range s.accounts {
			if a.WebID == creds.WebID {
				acc = a
				break
			}
		}
		s.mu.RUnlock()
	}
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "agent"
	}
	id := "cc_" + uuid.NewString()
	secret := randomHex(24)
	cc := &ClientCredentials{ID: id, Secret: secret, WebID: acc.WebID, AccountID: acc.ID, Name: body.Name}
	s.mu.Lock()
	s.clients[id] = cc
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cc)
}

func (s *Service) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = r.ParseForm()
	grant := r.FormValue("grant_type")
	switch grant {
	case "client_credentials":
		id, secret := r.FormValue("client_id"), r.FormValue("client_secret")
		if id == "" {
			// basic auth
			u, p, ok := r.BasicAuth()
			if ok {
				id, secret = u, p
			}
		}
		s.mu.RLock()
		cc, ok := s.clients[id]
		s.mu.RUnlock()
		if !ok || cc.Secret != secret {
			http.Error(w, "invalid_client", http.StatusUnauthorized)
			return
		}
		tok, err := s.Tokens.Issue(cc.WebID, cc.ID, time.Hour)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": tok,
			"token_type":   "Bearer",
			"expires_in":   3600,
			"webid":        cc.WebID,
		})
	case "authorization_code", "":
		// simplified: exchange any code for token if session present
		acc := s.accountFromRequest(r)
		webID := r.FormValue("webid")
		if acc != nil {
			webID = acc.WebID
		}
		if webID == "" {
			http.Error(w, "invalid_grant", http.StatusBadRequest)
			return
		}
		tok, _ := s.Tokens.Issue(webID, "", time.Hour)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": tok,
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     tok,
			"webid":        webID,
		})
	default:
		http.Error(w, "unsupported_grant_type", http.StatusBadRequest)
	}
}

func (s *Service) authorize(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	if redirect == "" {
		http.Error(w, "redirect_uri required", http.StatusBadRequest)
		return
	}
	code := randomHex(16)
	sep := "?"
	if strings.Contains(redirect, "?") {
		sep = "&"
	}
	http.Redirect(w, r, fmt.Sprintf("%s%scode=%s&state=%s", redirect, sep, code, state), http.StatusFound)
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		out = "pod-" + randomHex(4)
	}
	return out
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashEmail utility
func HashEmail(email string) string {
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:8])
}
