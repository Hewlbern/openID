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

// passwordSessionTTL is long-lived so local sign-in stays up; the password is mainly for cloud sync.
const passwordSessionTTL = 10 * 365 * 24 * time.Hour

// Service provides account, pod, WebID, and client-credentials identity APIs.
type Service struct {
	Store   *resourcestore.Store
	Tokens  *authn.TokenService
	BaseURL string

	mu        sync.RWMutex
	accounts  map[string]*Account // email or "handle:"+handle -> account (emails kept for login)
	byHandle  map[string]*Account
	byID      map[string]*Account
	clients   map[string]*ClientCredentials
	sessions  map[string]string // cookie -> accountID
	spark     map[string]*sparkGrant // jti -> grant
	persistOK bool
}

type Account struct {
	ID           string    `json:"id"`
	Handle       string    `json:"handle"`
	Email        string    `json:"email,omitempty"`
	Name         string    `json:"name,omitempty"`
	Bio          string    `json:"bio,omitempty"`
	PasswordHash string    `json:"-"`
	WebID        string    `json:"webId"`
	PodPath      string    `json:"podPath"`
	PublicURL    string    `json:"publicUrl,omitempty"`
	Created      time.Time `json:"created"`
}

type ClientCredentials struct {
	ID        string `json:"id"`
	Secret    string `json:"secret,omitempty"`
	WebID     string `json:"webId"`
	AccountID string `json:"accountId"`
	Name      string `json:"name"`
}

func New(store *resourcestore.Store, tokens *authn.TokenService, baseURL string) *Service {
	s := &Service{
		Store:     store,
		Tokens:    tokens,
		BaseURL:   strings.TrimRight(baseURL, "/"),
		accounts:  map[string]*Account{},
		byHandle:  map[string]*Account{},
		byID:      map[string]*Account{},
		clients:   map[string]*ClientCredentials{},
		sessions:  map[string]string{},
		spark:     map[string]*sparkGrant{},
		persistOK: true,
	}
	s.load()
	s.loadSparkGrants()
	return s
}

func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("/idp/", s.handleIDP)
	mux.HandleFunc("/.well-known/openid-configuration", s.handleOIDCConfig)
	mux.HandleFunc("/.well-known/solid", s.handleSolidDescription)
	mux.HandleFunc("/oauth/token", s.handleToken)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/records/sparql", s.handleRecordsSPARQL)
}

func (s *Service) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"name":         "OpenID",
		"baseUrl":      s.BaseURL,
		"handlePrefix": "/i/",
		"protocol":     "Solid Protocol",
	})
}

func (s *Service) AccountCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}

func (s *Service) FindByHandle(handle string) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byHandle[strings.ToLower(handle)]
}

// AccountFromRequest resolves a session cookie or Bearer token to an account.
func (s *Service) AccountFromRequest(r *http.Request) *Account {
	return s.accountFromRequest(r)
}

// FindByWebID returns the account bound to a WebID, if any.
func (s *Service) FindByWebID(webID string) *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accountByWebID(webID)
}

func (s *Service) PublicAccount(handle string) map[string]interface{} {
	acc := s.FindByHandle(handle)
	if acc == nil {
		return nil
	}
	return map[string]interface{}{
		"handle":    acc.Handle,
		"name":      acc.Name,
		"bio":       acc.Bio,
		"webId":     acc.WebID,
		"pod":       s.BaseURL + "/" + acc.PodPath,
		"publicUrl": s.BaseURL + "/i/" + acc.Handle,
		"created":   acc.Created,
	}
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
	case strings.HasPrefix(path, "handles/") && r.Method == http.MethodGet:
		s.handleAvailability(w, strings.TrimPrefix(path, "handles/"))
	case path == "register" && r.Method == http.MethodPost:
		s.register(w, r)
	case path == "login" && r.Method == http.MethodPost:
		s.login(w, r)
	case path == "profile" && r.Method == http.MethodPatch:
		s.updateProfile(w, r)
	case path == "logout" && r.Method == http.MethodPost:
		s.logout(w, r)
	case path == "accounts/me" && r.Method == http.MethodGet:
		s.me(w, r)
	case path == "pods" && r.Method == http.MethodPost:
		s.createPod(w, r)
	case path == "spark-token" && r.Method == http.MethodGet:
		s.handleSparkTokenGet(w, r)
	case path == "spark-token" && r.Method == http.MethodPost:
		s.handleSparkTokenMint(w, r)
	case path == "spark-token" && r.Method == http.MethodDelete:
		s.handleSparkTokenRevoke(w, r)
	case path == "client-credentials" && r.Method == http.MethodPost:
		s.createClientCredentials(w, r)
	case path == "replica/adopt" && r.Method == http.MethodPost:
		s.adoptReplica(w, r)
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
				"register":          s.BaseURL + "/idp/register",
				"login":             s.BaseURL + "/idp/login",
				"logout":            s.BaseURL + "/idp/logout",
				"account":           s.BaseURL + "/idp/accounts/me",
				"createPod":         s.BaseURL + "/idp/pods",
				"clientCredentials": s.BaseURL + "/idp/client-credentials",
				"sparkToken":        s.BaseURL + "/idp/spark-token",
			},
			"version": "solid-go/1.0",
		})
	}
}

type registerReq struct {
	Handle    string `json:"handle"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Name      string `json:"name"`
	Bio       string `json:"bio"`
	CreatePod bool   `json:"createPod"`
}

var reservedHandles = map[string]bool{
	"idp": true, "agents": true, "audit": true, "notifications": true,
	"oauth": true, "health": true, "app": true, "i": true, "static": true,
	"api": true, "admin": true, "www": true, "well-known": true,
	"welcome": true, "dashboard": true, "login": true, "mcp": true, "records": true,
	"share": true, "conversations": true,
}

func (s *Service) handleAvailability(w http.ResponseWriter, handle string) {
	handle = sanitizeSlug(strings.Trim(handle, "/"))
	available := handle != "" && len(handle) >= 2 && !reservedHandles[handle] && s.FindByHandle(handle) == nil
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"handle":    handle,
		"available": available,
		"publicUrl": s.BaseURL + "/i/" + handle,
	})
}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Handle = sanitizeSlug(req.Handle)
	if req.Handle == "" && req.Email != "" {
		req.Handle = sanitizeSlug(strings.Split(req.Email, "@")[0])
	}
	if req.Handle == "" || req.Password == "" {
		http.Error(w, "handle and password required", http.StatusBadRequest)
		return
	}
	if reservedHandles[req.Handle] {
		http.Error(w, "handle reserved", http.StatusConflict)
		return
	}
	if req.Name == "" {
		req.Name = req.Handle
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.Email != "" {
		if _, ok := s.accounts[req.Email]; ok {
			http.Error(w, "account exists", http.StatusConflict)
			return
		}
	}
	if _, ok := s.byHandle[req.Handle]; ok {
		http.Error(w, "handle taken", http.StatusConflict)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id := uuid.NewString()
	podPath := req.Handle + "/"
	webID := s.BaseURL + "/" + podPath + "profile/card#me"
	acc := &Account{
		ID:           id,
		Handle:       req.Handle,
		Email:        req.Email,
		Name:         req.Name,
		Bio:          req.Bio,
		PasswordHash: string(hash),
		WebID:        webID,
		PodPath:      podPath,
		PublicURL:    s.BaseURL + "/i/" + req.Handle,
		Created:      time.Now().UTC(),
	}
	s.indexAccount(acc)
	if req.CreatePod || true {
		if err := s.provisionPod(context.Background(), acc, req.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	s.saveLocked()
	token, _ := s.Tokens.Issue(acc.WebID, "", passwordSessionTTL)
	sid := uuid.NewString()
	s.sessions[sid] = acc.ID
	http.SetCookie(w, sessionCookie(r, sid, int(passwordSessionTTL.Seconds())))
	s.saveLocalAuth(acc.Handle, req.Password)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"account": acc,
		"token":   token,
		"webId":   acc.WebID,
		"pod":     s.BaseURL + "/" + acc.PodPath,
	})
}

// sessionCookie is usable from the Vercel site (same-origin proxy) or a
// cross-site OPENID_API origin. Bearer tokens in localStorage remain the
// primary /app auth; the cookie is a same-site fallback.
func sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name:     "solid-session",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   maxAge,
		SameSite: http.SameSiteLaxMode,
	}
	https := r != nil && (r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"))
	origin := ""
	if r != nil {
		origin = r.Header.Get("Origin")
	}
	crossSite := origin != "" && r.Host != "" && !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://"), r.Host)
	if https {
		c.Secure = true
	}
	if crossSite && https {
		// Cross-origin Vercel → Railway without a proxy. Lax cookies are
		// not sent on fetch(); Bearer is required. None lets the cookie
		// ride if the browser allows it.
		c.SameSite = http.SameSiteNoneMode
		c.Secure = true
	}
	if maxAge < 0 {
		c.MaxAge = -1
		c.Value = ""
	}
	return c
}

func (s *Service) provisionPod(ctx context.Context, acc *Account, name string) error {
	if _, err := s.Store.PutContainer(ctx, acc.PodPath); err != nil {
		return err
	}
	if _, err := s.Store.PutContainer(ctx, acc.PodPath+"profile/"); err != nil {
		return err
	}
	if _, err := s.Store.PutContainer(ctx, acc.PodPath+"inbox/"); err != nil {
		return err
	}
	if _, err := s.Store.PutContainer(ctx, acc.PodPath+"public/"); err != nil {
		return err
	}
	if name == "" {
		name = "Solid User"
	}
	profile := webIDProfileTurtle(acc.WebID, name, s.BaseURL+"/"+acc.PodPath, acc.Handle)
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

func webIDProfileTurtle(webID, name, storage, handle string) string {
	g := rdf.NewGraph()
	prefixes := map[string]string{
		"foaf":   "http://xmlns.com/foaf/0.1/",
		"solid":  "http://www.w3.org/ns/solid/terms#",
		"pim":    "http://www.w3.org/ns/pim/space#",
		"schema": "http://schema.org/",
	}
	g.AddIRI(webID, "http://www.w3.org/1999/02/22-rdf-syntax-ns#type", "http://xmlns.com/foaf/0.1/Person")
	g.AddLiteral(webID, "http://xmlns.com/foaf/0.1/name", name)
	if handle != "" {
		g.AddLiteral(webID, "http://xmlns.com/foaf/0.1/nick", handle)
	}
	g.AddIRI(webID, "http://www.w3.org/ns/pim/space#storage", storage)
	g.AddIRI(webID, "http://www.w3.org/ns/ldp#inbox", strings.TrimSuffix(storage, "/")+"/inbox/")
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
	var acc *Account
	if req.Email != "" {
		acc = s.accounts[strings.ToLower(req.Email)]
	}
	if acc == nil && req.Handle != "" {
		acc = s.byHandle[sanitizeSlug(req.Handle)]
	}
	if acc == nil && req.Email != "" {
		acc = s.byHandle[sanitizeSlug(req.Email)]
	}
	s.mu.RUnlock()
	if acc == nil || bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, _ := s.Tokens.Issue(acc.WebID, "", passwordSessionTTL)
	sid := uuid.NewString()
	s.mu.Lock()
	s.sessions[sid] = acc.ID
	s.mu.Unlock()
	http.SetCookie(w, sessionCookie(r, sid, int(passwordSessionTTL.Seconds())))
	s.saveLocalAuth(acc.Handle, req.Password)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"token": token, "webId": acc.WebID, "account": acc})
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("solid-session"); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, sessionCookie(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) accountFromRequest(r *http.Request) *Account {
	if c, err := r.Cookie("solid-session"); err == nil {
		s.mu.RLock()
		id, ok := s.sessions[c.Value]
		acc := s.byID[id]
		s.mu.RUnlock()
		if ok && acc != nil {
			return acc
		}
	}
	creds, err := s.Tokens.Extract(r)
	if err != nil || creds.WebID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accountByWebID(creds.WebID)
}

func (s *Service) indexAccount(acc *Account) {
	if acc.Email != "" {
		s.accounts[acc.Email] = acc
	}
	if acc.Handle != "" {
		s.byHandle[acc.Handle] = acc
		s.byHandle[strings.ToLower(acc.Handle)] = acc
	}
	s.byID[acc.ID] = acc
}

func (s *Service) dropAccountLocked(acc *Account) {
	if acc == nil {
		return
	}
	if acc.Email != "" {
		delete(s.accounts, acc.Email)
	}
	if acc.Handle != "" {
		delete(s.byHandle, acc.Handle)
		delete(s.byHandle, strings.ToLower(acc.Handle))
	}
	delete(s.byID, acc.ID)
}

type adoptReplicaReq struct {
	Password string               `json:"password"`
	Account  persistedAccount     `json:"account"`
	Clients  []*ClientCredentials `json:"clients"`
}

// adoptReplica merges a replica of an existing account onto this origin.
// The caller must prove the password for the handle (local hash or the hash already on this server).
func (s *Service) adoptReplica(w http.ResponseWriter, r *http.Request) {
	var req adoptReplicaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Account.Handle = sanitizeSlug(req.Account.Handle)
	if req.Account.Handle == "" || req.Password == "" || req.Account.PasswordHash == "" {
		http.Error(w, "handle, password, and account hash required", http.StatusBadRequest)
		return
	}
	if reservedHandles[req.Account.Handle] {
		http.Error(w, "reserved handle", http.StatusBadRequest)
		return
	}
	incomingOK := bcrypt.CompareHashAndPassword([]byte(req.Account.PasswordHash), []byte(req.Password)) == nil
	s.mu.Lock()
	existing := s.byHandle[req.Account.Handle]
	existingOK := existing != nil && bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(req.Password)) == nil
	if !incomingOK && !existingOK {
		s.mu.Unlock()
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if existing != nil && !existingOK && incomingOK {
		// password matches the incoming replica only — do not clobber a different live account
		s.mu.Unlock()
		http.Error(w, "handle taken", http.StatusConflict)
		return
	}
	hadPod := existing != nil
	hash := req.Account.PasswordHash
	if !incomingOK && existing != nil {
		hash = existing.PasswordHash
	}
	if existing != nil {
		s.dropAccountLocked(existing)
		for id, c := range s.clients {
			if c != nil && (c.AccountID == existing.ID || c.AccountID == req.Account.ID) {
				delete(s.clients, id)
			}
		}
	}
	webID := s.BaseURL + "/" + req.Account.Handle + "/profile/card#me"
	acc := &Account{
		ID:           req.Account.ID,
		Handle:       req.Account.Handle,
		Email:        req.Account.Email,
		Name:         req.Account.Name,
		Bio:          req.Account.Bio,
		PasswordHash: hash,
		WebID:        webID,
		PodPath:      req.Account.Handle + "/",
		PublicURL:    s.BaseURL + "/i/" + req.Account.Handle,
		Created:      req.Account.Created,
	}
	if acc.ID == "" {
		acc.ID = uuid.NewString()
	}
	if acc.Created.IsZero() {
		acc.Created = time.Now().UTC()
	}
	s.indexAccount(acc)
	for _, c := range req.Clients {
		if c == nil || c.ID == "" {
			continue
		}
		c.AccountID = acc.ID
		c.WebID = webID
		s.clients[c.ID] = c
	}
	s.saveLocked()
	s.mu.Unlock()
	if !hadPod {
		_ = s.provisionPod(r.Context(), acc, acc.Name)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(acc)
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
		acc = s.accountByWebID(creds.WebID)
		s.mu.RUnlock()
	}
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(acc)
}

func (s *Service) accountByWebID(webID string) *Account {
	for _, a := range s.byID {
		if a.WebID == webID {
			return a
		}
	}
	return nil
}

func (s *Service) updateProfile(w http.ResponseWriter, r *http.Request) {
	acc := s.accountFromRequest(r)
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Name     string `json:"name"`
		Bio      string `json:"bio"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Password != "" && len(body.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if body.Name != "" {
		acc.Name = body.Name
	}
	if body.Bio != "" {
		acc.Bio = body.Bio
	}
	if body.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		acc.PasswordHash = string(hash)
	}
	s.saveLocked()
	s.mu.Unlock()
	_ = s.provisionPod(r.Context(), acc, acc.Name)
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
		acc = s.accountByWebID(creds.WebID)
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
	s.saveLocked()
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
	return b.String()
}

type persistedState struct {
	Accounts []persistedAccount   `json:"accounts"`
	Clients  []*ClientCredentials `json:"clients"`
}

type persistedAccount struct {
	ID           string    `json:"id"`
	Handle       string    `json:"handle"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Bio          string    `json:"bio"`
	PasswordHash string    `json:"passwordHash"`
	WebID        string    `json:"webId"`
	PodPath      string    `json:"podPath"`
	PublicURL    string    `json:"publicUrl"`
	Created      time.Time `json:"created"`
}

func (s *Service) load() {
	raw, err := s.Store.Get(context.Background(), ".openid/accounts.json")
	if err != nil || raw == nil || len(raw.Body) == 0 {
		return
	}
	var st persistedState
	if err := json.Unmarshal(raw.Body, &st); err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range st.Accounts {
		acc := &Account{
			ID: p.ID, Handle: p.Handle, Email: p.Email, Name: p.Name, Bio: p.Bio,
			PasswordHash: p.PasswordHash, WebID: p.WebID, PodPath: p.PodPath,
			PublicURL: p.PublicURL, Created: p.Created,
		}
		s.indexAccount(acc)
	}
	for _, c := range st.Clients {
		if c != nil && c.ID != "" {
			s.clients[c.ID] = c
		}
	}
}

func (s *Service) saveLocked() {
	if !s.persistOK {
		return
	}
	st := persistedState{}
	seen := map[string]bool{}
	for _, a := range s.byID {
		if seen[a.ID] {
			continue
		}
		seen[a.ID] = true
		st.Accounts = append(st.Accounts, persistedAccount{
			ID: a.ID, Handle: a.Handle, Email: a.Email, Name: a.Name, Bio: a.Bio,
			PasswordHash: a.PasswordHash, WebID: a.WebID, PodPath: a.PodPath,
			PublicURL: a.PublicURL, Created: a.Created,
		})
	}
	for _, c := range s.clients {
		st.Clients = append(st.Clients, c)
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_, _ = s.Store.Put(context.Background(), ".openid/accounts.json", "application/json", raw, "", "")
}

func (s *Service) saveLocalAuth(handle, password string) {
	if handle == "" || password == "" {
		return
	}
	peer := s.BaseURL
	if !strings.Contains(peer, "railway.app") {
		peer = "https://pod-production-ebe1.up.railway.app"
	}
	raw, err := json.MarshalIndent(map[string]string{
		"handle":   handle,
		"password": password,
		"peer":     peer,
	}, "", "  ")
	if err != nil {
		return
	}
	_, _ = s.Store.Put(context.Background(), ".openid/local-auth.json", "application/json", raw, "", "")
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
