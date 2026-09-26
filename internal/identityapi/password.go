package identityapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	resetTokenTTL  = 30 * time.Minute
	adminResetTTL  = 2 * time.Hour
	forgotLimit    = 5
	forgotWindow   = 15 * time.Minute
	resetLimit     = 20
	resetWindow    = 15 * time.Minute
	adminLimit     = 30
	adminWindow    = time.Hour
	minPasswordLen = 8
	maxPasswordLen = 128
	auditRetain    = 500
	resetsPath     = ".openid/password-resets.json"
	auditPath      = ".openid/security-audit.json"
)

var errInvalidReset = errors.New("invalid or expired reset link")

type resetRecord struct {
	Hash      string    `json:"hash"`
	AccountID string    `json:"accountId"`
	Expires   time.Time `json:"expires"`
	Used      bool      `json:"used"`
}

type resetFile struct {
	Tokens []resetRecord `json:"tokens"`
}

type auditEvent struct {
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	Handle string    `json:"handle,omitempty"`
	IP     string    `json:"ip,omitempty"`
}

type auditFile struct {
	Events []auditEvent `json:"events"`
}

type hitLimiter struct {
	mu sync.Mutex
	m  map[string][]time.Time
}

func (l *hitLimiter) allow(key string, n int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.m == nil {
		l.m = map[string][]time.Time{}
	}
	cut := now.Add(-window)
	kept := make([]time.Time, 0, len(l.m[key])+1)
	for _, t := range l.m[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= n {
		l.m[key] = kept
		return false
	}
	l.m[key] = append(kept, now)
	return true
}

func (s *Service) mailEnabled() bool {
	return s.mail != nil && s.mail.Enabled()
}

func (s *Service) loadResets() {
	raw, err := s.Store.Get(context.Background(), resetsPath)
	if err != nil || raw == nil || len(raw.Body) == 0 {
		return
	}
	var file resetFile
	if json.Unmarshal(raw.Body, &file) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i := range file.Tokens {
		rec := file.Tokens[i]
		if rec.Hash == "" || !now.Before(rec.Expires) {
			continue
		}
		cp := rec
		s.resets[rec.Hash] = &cp
	}
}

func (s *Service) saveResetsLocked() {
	if !s.persistOK {
		return
	}
	file := resetFile{}
	now := time.Now().UTC()
	for _, rec := range s.resets {
		if rec == nil || !now.Before(rec.Expires) {
			continue
		}
		file.Tokens = append(file.Tokens, *rec)
	}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	_, _ = s.Store.Put(context.Background(), resetsPath, "application/json", raw, "", "")
}

func (s *Service) audit(action, handle, ip string) {
	if s.Logger != nil {
		s.Logger.Info("security action=%s handle=%s", action, handle)
	}
	s.appendAudit(auditEvent{
		Time:   time.Now().UTC(),
		Action: action,
		Handle: handle,
		IP:     ip,
	})
}

func (s *Service) appendAudit(ev auditEvent) {
	raw, err := s.Store.Get(context.Background(), auditPath)
	var file auditFile
	if err == nil && raw != nil && len(raw.Body) > 0 {
		_ = json.Unmarshal(raw.Body, &file)
	}
	file.Events = append(file.Events, ev)
	if len(file.Events) > auditRetain {
		file.Events = file.Events[len(file.Events)-auditRetain:]
	}
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return
	}
	_, _ = s.Store.Put(context.Background(), auditPath, "application/json", body, "", "")
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

type forgotResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func genericForgot() forgotResponse {
	return forgotResponse{
		OK:      true,
		Message: "If an account with a deliverable email matches, a reset link is on its way. It expires in 30 minutes and can be used once.",
	}
}

// deliverableEmail is an address we can actually send mail to.
// Local-only hosts (localhost, .local, .invalid) are not deliverable.
func deliverableEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	local, host := email[:at], email[at+1:]
	if local == "" || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	if host == "localhost" || host == "local" || host == "invalid" {
		return false
	}
	if strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".invalid") {
		return false
	}
	if !strings.Contains(host, ".") {
		return false
	}
	return true
}

func normalizeEmail(email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", true
	}
	if strings.ContainsAny(email, " \t\r\n") || strings.Count(email, "@") != 1 {
		return "", false
	}
	at := strings.Index(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "", false
	}
	return email, true
}

func passwordOK(pw string) bool {
	return len(pw) >= minPasswordLen && len(pw) <= maxPasswordLen
}

func secretMatch(want, got string) bool {
	if want == "" || got == "" {
		return false
	}
	ah := sha256.Sum256([]byte(want))
	bh := sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}

func adminSecretFrom(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-OpenID-Admin-Secret")); v != "" {
		return v
	}
	auth := r.Header.Get("Authorization")
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func (s *Service) publicResetLink(token string) string {
	base := strings.TrimRight(s.PublicURL, "/")
	if base == "" {
		base = strings.TrimRight(s.BaseURL, "/")
	}
	return base + "/reset#token=" + token
}

func (s *Service) issueResetLocked(acc *Account, ttl time.Duration) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	raw := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	exp := time.Now().UTC().Add(ttl)
	for h, rec := range s.resets {
		if rec != nil && rec.AccountID == acc.ID && !rec.Used {
			delete(s.resets, h)
		}
	}
	s.resets[hash] = &resetRecord{Hash: hash, AccountID: acc.ID, Expires: exp}
	s.saveResetsLocked()
	return raw, exp, nil
}

func (s *Service) consumeResetLocked(raw string) (*Account, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errInvalidReset
	}
	sum := sha256.Sum256([]byte(raw))
	rec := s.resets[hex.EncodeToString(sum[:])]
	if rec == nil || rec.Used || !time.Now().UTC().Before(rec.Expires) {
		return nil, errInvalidReset
	}
	acc := s.byID[rec.AccountID]
	if acc == nil {
		return nil, errInvalidReset
	}
	rec.Used = true
	return acc, nil
}

func (s *Service) findAccountLocked(email, handle string) *Account {
	email = strings.ToLower(strings.TrimSpace(email))
	if email != "" {
		if acc := s.accounts[email]; acc != nil {
			return acc
		}
	}
	if h := sanitizeSlug(handle); h != "" {
		if acc := s.byHandle[h]; acc != nil {
			return acc
		}
	}
	if email != "" {
		if acc := s.byHandle[sanitizeSlug(email)]; acc != nil {
			return acc
		}
	}
	return nil
}

func (s *Service) invalidateAuthLocked(acc *Account) {
	if acc == nil {
		return
	}
	acc.TokenVersion++
	if s.Tokens != nil {
		s.Tokens.SetTokenVersion(acc.WebID, acc.TokenVersion)
	}
	for sid, id := range s.sessions {
		if id == acc.ID {
			delete(s.sessions, sid)
		}
	}
	revoked := false
	for _, g := range s.spark {
		if g == nil || g.WebID != acc.WebID || g.Revoked {
			continue
		}
		g.Revoked = true
		s.Tokens.RevokeJTI(g.JTI)
		revoked = true
	}
	if revoked {
		s.saveSparkGrantsLocked()
	}
}

func (s *Service) reindexEmailLocked(acc *Account, email string) error {
	if email == acc.Email {
		return nil
	}
	if email != "" {
		if other := s.accounts[email]; other != nil && other.ID != acc.ID {
			return errors.New("email taken")
		}
	}
	if acc.Email != "" {
		delete(s.accounts, acc.Email)
	}
	acc.Email = email
	if email != "" {
		s.accounts[email] = acc
	}
	return nil
}

func (s *Service) forgotPassword(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.limiter.allow("forgot:"+ip, forgotLimit, forgotWindow) {
		writeJSON(w, http.StatusTooManyRequests, forgotResponse{
			OK:      false,
			Message: "Too many reset requests. Try again later.",
		})
		return
	}
	var req struct {
		Email  string `json:"email"`
		Handle string `json:"handle"`
		ID     string `json:"id"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<16))
	_ = dec.Decode(&req)
	id := strings.TrimSpace(req.ID)
	if id != "" {
		if strings.Contains(id, "@") && req.Email == "" {
			req.Email = id
		} else if req.Handle == "" {
			req.Handle = id
		}
	}
	var (
		sendTo string
		token  string
		handle string
	)
	s.mu.Lock()
	acc := s.findAccountLocked(req.Email, req.Handle)
	if acc != nil && deliverableEmail(acc.Email) && s.mailEnabled() {
		tok, _, err := s.issueResetLocked(acc, resetTokenTTL)
		if err == nil {
			sendTo = acc.Email
			token = tok
			handle = acc.Handle
		}
	} else {
		// Spend similar work when there is nothing to send.
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		sum := sha256.Sum256(buf)
		_ = hex.EncodeToString(sum[:])
	}
	s.mu.Unlock()
	if token != "" {
		s.audit("reset_email_queued", handle, ip)
		s.sendResetEmail(r.Context(), sendTo, token, handle)
	}
	writeJSON(w, http.StatusOK, genericForgot())
}

func (s *Service) sendResetEmail(ctx context.Context, to, token, handle string) {
	if s.mail == nil || token == "" {
		return
	}
	link := s.publicResetLink(token)
	body := "A password reset was requested for your OpenID account.\r\n\r\n" +
		"Open this link to choose a new password. It expires in 30 minutes and works once:\r\n\r\n" +
		link + "\r\n\r\n" +
		"If you did not ask for this, you can ignore this email. Your password will not change.\r\n"
	if err := s.mail.Send(ctx, to, "Reset your OpenID password", body); err != nil && s.Logger != nil {
		s.Logger.Warn("password reset email failed handle=%s", handle)
	}
}

func (s *Service) resetPassword(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !s.limiter.allow("reset:"+ip, resetLimit, resetWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !passwordOK(req.Password) {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "could not update password", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	acc, err := s.consumeResetLocked(req.Token)
	if err != nil {
		s.saveResetsLocked()
		s.mu.Unlock()
		http.Error(w, "invalid or expired reset link", http.StatusBadRequest)
		return
	}
	acc.PasswordHash = string(hash)
	s.invalidateAuthLocked(acc)
	s.saveLocked()
	s.saveResetsLocked()
	handle := acc.Handle
	s.mu.Unlock()
	s.audit("password_reset_completed", handle, ip)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Service) adminReset(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.AdminSecret) == "" {
		http.NotFound(w, r)
		return
	}
	ip := clientIP(r)
	if !s.limiter.allow("admin:"+ip, adminLimit, adminWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !secretMatch(s.AdminSecret, adminSecretFrom(r)) {
		s.audit("admin_reset_denied", "", ip)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Handle string `json:"handle"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	handle := sanitizeSlug(req.Handle)
	if handle == "" {
		http.Error(w, "handle required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	acc := s.byHandle[handle]
	if acc == nil {
		s.mu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	token, exp, err := s.issueResetLocked(acc, adminResetTTL)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, "could not issue reset", http.StatusInternalServerError)
		return
	}
	s.audit("admin_reset_issued", handle, ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"handle":   handle,
		"resetUrl": s.publicResetLink(token),
		"expires":  exp,
	})
}

func (s *Service) adminRevokeClients(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(s.AdminSecret) == "" {
		http.NotFound(w, r)
		return
	}
	ip := clientIP(r)
	if !s.limiter.allow("admin:"+ip, adminLimit, adminWindow) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !secretMatch(s.AdminSecret, adminSecretFrom(r)) {
		s.audit("admin_revoke_denied", "", ip)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Handle string `json:"handle"`
		All    bool   `json:"all"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	handle := sanitizeSlug(req.Handle)
	if handle == "" && !req.All {
		http.Error(w, "handle or all required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	var accountID string
	if handle != "" {
		acc := s.byHandle[handle]
		if acc == nil {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		accountID = acc.ID
	}
	n := 0
	for id, c := range s.clients {
		if c == nil {
			delete(s.clients, id)
			continue
		}
		if accountID != "" && c.AccountID != accountID {
			continue
		}
		delete(s.clients, id)
		n++
	}
	s.saveLocked()
	s.mu.Unlock()
	action := "admin_clients_revoked"
	if req.All {
		action = "admin_clients_revoked_all"
	}
	s.audit(action, handle, ip)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "revoked": n})
}
