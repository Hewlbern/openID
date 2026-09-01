package identityapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"solid-go/internal/authn"
)

type sparkGrant struct {
	JTI     string    `json:"jti"`
	WebID   string    `json:"webId"`
	Issued  time.Time `json:"issued"`
	Expires time.Time `json:"expires"`
	Revoked bool      `json:"revoked"`
}

type sparkGrantFile struct {
	Grants []sparkGrant `json:"grants"`
}

func (s *Service) requireFullAccount(r *http.Request) *Account {
	acc := s.accountFromRequest(r)
	if acc == nil {
		return nil
	}
	creds, err := s.Tokens.Extract(r)
	if err == nil && creds != nil && creds.IsSpark() {
		if c, cerr := r.Cookie("solid-session"); cerr == nil {
			s.mu.RLock()
			id, ok := s.sessions[c.Value]
			s.mu.RUnlock()
			if ok && id == acc.ID {
				return acc
			}
		}
		return nil
	}
	return acc
}

func (s *Service) handleSparkTokenMint(w http.ResponseWriter, r *http.Request) {
	acc := s.requireFullAccount(r)
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ttl := authn.SparkTokenTTL()
	var body struct {
		TTL string `json:"ttl"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if d, err := time.ParseDuration(strings.TrimSpace(body.TTL)); err == nil && d > 0 && d <= 90*24*time.Hour {
		ttl = d
	}
	tok, jti, exp, err := s.Tokens.IssueSpark(acc.WebID, ttl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.spark[jti] = &sparkGrant{
		JTI: jti, WebID: acc.WebID, Issued: time.Now().UTC(), Expires: exp,
	}
	s.saveSparkGrantsLocked()
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":      tok,
		"tokenType":  "Bearer",
		"jti":        jti,
		"aud":        authn.AudienceSparkMCP,
		"scope":      authn.ScopeSpark,
		"webId":      acc.WebID,
		"expires":    exp,
		"expiresIn":  int(time.Until(exp).Seconds()),
		"mcpUrl":     s.BaseURL + "/mcp",
		"ttl":        ttl.String(),
	})
}

func (s *Service) handleSparkTokenGet(w http.ResponseWriter, r *http.Request) {
	acc := s.requireFullAccount(r)
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	s.mu.RLock()
	var active []map[string]any
	for _, g := range s.spark {
		if g == nil || g.WebID != acc.WebID || g.Revoked || g.Expires.Before(now) {
			continue
		}
		active = append(active, map[string]any{
			"jti":     g.JTI,
			"expires": g.Expires,
			"issued":  g.Issued,
			"aud":     authn.AudienceSparkMCP,
			"scope":   authn.ScopeSpark,
		})
	}
	s.mu.RUnlock()
	if active == nil {
		active = []map[string]any{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tokens": active,
		"mcpUrl": s.BaseURL + "/mcp",
		"webId":  acc.WebID,
		"ttl":    authn.SparkTokenTTL().String(),
	})
}

func (s *Service) handleSparkTokenRevoke(w http.ResponseWriter, r *http.Request) {
	acc := s.requireFullAccount(r)
	if acc == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	jti := strings.TrimSpace(r.URL.Query().Get("jti"))
	if jti == "" {
		var body struct {
			JTI string `json:"jti"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		jti = strings.TrimSpace(body.JTI)
	}
	n := 0
	s.mu.Lock()
	for _, g := range s.spark {
		if g == nil || g.WebID != acc.WebID || g.Revoked {
			continue
		}
		if jti != "" && g.JTI != jti {
			continue
		}
		g.Revoked = true
		s.Tokens.RevokeJTI(g.JTI)
		n++
	}
	s.saveSparkGrantsLocked()
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "revoked": n})
}

func (s *Service) loadSparkGrants() {
	raw, err := s.Store.Get(context.Background(), ".openid/spark-grants.json")
	if err != nil || raw == nil || len(raw.Body) == 0 {
		return
	}
	var file sparkGrantFile
	if json.Unmarshal(raw.Body, &file) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range file.Grants {
		g := file.Grants[i]
		cp := g
		s.spark[g.JTI] = &cp
		if g.Revoked {
			s.Tokens.RevokeJTI(g.JTI)
		}
	}
}

func (s *Service) saveSparkGrantsLocked() {
	if !s.persistOK {
		return
	}
	file := sparkGrantFile{}
	for _, g := range s.spark {
		if g != nil {
			file.Grants = append(file.Grants, *g)
		}
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	_, _ = s.Store.Put(context.Background(), ".openid/spark-grants.json", "application/json", raw, "", "")
}
