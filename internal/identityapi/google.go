package identityapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// GoogleIdentity is the verified Google profile used to mint a Solid session.
type GoogleIdentity struct {
	Sub   string
	Email string
	Name  string
}

// VerifyGoogleIDToken checks a Google ID token. Tests replace this.
var VerifyGoogleIDToken = defaultVerifyGoogleIDToken

func googleClientID() string {
	for _, k := range []string{"GOOGLE_CLIENT_ID", "OPENID_GOOGLE_CLIENT_ID"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func defaultVerifyGoogleIDToken(ctx context.Context, raw string) (*GoogleIdentity, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("id token required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://oauth2.googleapis.com/tokeninfo?id_token="+raw, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var doc struct {
		Iss           string `json:"iss"`
		Aud           string `json:"aud"`
		Azp           string `json:"azp"`
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Name          string `json:"name"`
		Error         string `json:"error"`
		ErrorDesc     string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("google tokeninfo: %s", strings.TrimSpace(string(body)))
	}
	if res.StatusCode >= 400 {
		if doc.ErrorDesc != "" {
			return nil, fmt.Errorf("%s", doc.ErrorDesc)
		}
		if doc.Error != "" {
			return nil, fmt.Errorf("%s", doc.Error)
		}
		return nil, fmt.Errorf("invalid google id token")
	}
	if doc.Iss != "https://accounts.google.com" && doc.Iss != "accounts.google.com" {
		return nil, fmt.Errorf("google id token issuer mismatch")
	}
	expect := googleClientID()
	if expect != "" && doc.Aud != expect && doc.Azp != expect {
		return nil, fmt.Errorf("google id token audience mismatch")
	}
	if doc.EmailVerified == "false" {
		return nil, fmt.Errorf("google email is not verified")
	}
	if doc.Sub == "" {
		return nil, fmt.Errorf("google id token missing sub")
	}
	return &GoogleIdentity{
		Sub:   doc.Sub,
		Email: strings.ToLower(strings.TrimSpace(doc.Email)),
		Name:  strings.TrimSpace(doc.Name),
	}, nil
}

type googleLoginReq struct {
	IDToken string `json:"idToken"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Sub     string `json:"sub"`
}

func (s *Service) handleGoogle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req googleLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ident, err := VerifyGoogleIDToken(r.Context(), req.IDToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if ident.Email == "" {
		ident.Email = strings.ToLower(strings.TrimSpace(req.Email))
	}
	if ident.Name == "" {
		ident.Name = strings.TrimSpace(req.Name)
	}
	acc, err := s.loginOrCreateGoogle(r.Context(), ident)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	token, _ := s.Tokens.Issue(acc.WebID, "", passwordSessionTTL)
	sid := uuid.NewString()
	s.mu.Lock()
	s.sessions[sid] = acc.ID
	s.mu.Unlock()
	http.SetCookie(w, sessionCookie(r, sid, int(passwordSessionTTL.Seconds())))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token":   token,
		"webId":   acc.WebID,
		"account": acc,
		"via":     "google",
	})
}

func (s *Service) loginOrCreateGoogle(ctx context.Context, ident *GoogleIdentity) (*Account, error) {
	if ident == nil || ident.Sub == "" {
		return nil, fmt.Errorf("google identity required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	acc := s.accounts["google:"+ident.Sub]
	if acc == nil && ident.Email != "" {
		acc = s.accounts[ident.Email]
	}
	if acc != nil {
		if acc.GoogleSub == "" {
			acc.GoogleSub = ident.Sub
			s.accounts["google:"+ident.Sub] = acc
		}
		if acc.Email == "" && ident.Email != "" {
			acc.Email = ident.Email
			s.accounts[ident.Email] = acc
		}
		if acc.Name == "" && ident.Name != "" {
			acc.Name = ident.Name
		}
		s.saveLocked()
		return acc, nil
	}
	handle := s.uniqueHandleLocked(ident.Email, ident.Name, ident.Sub)
	if handle == "" {
		return nil, fmt.Errorf("could not allocate a handle")
	}
	pwd := make([]byte, 32)
	_, _ = rand.Read(pwd)
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(pwd)), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	name := ident.Name
	if name == "" {
		name = handle
	}
	id := uuid.NewString()
	podPath := handle + "/"
	webID := s.BaseURL + "/" + podPath + "profile/card#me"
	acc = &Account{
		ID:           id,
		Handle:       handle,
		Email:        ident.Email,
		Name:         name,
		PasswordHash: string(hash),
		GoogleSub:    ident.Sub,
		WebID:        webID,
		PodPath:      podPath,
		PublicURL:    s.BaseURL + "/i/" + handle,
		Created:      time.Now().UTC(),
	}
	s.indexAccount(acc)
	s.saveLocked()
	s.mu.Unlock()
	err = s.provisionPod(ctx, acc, name)
	s.mu.Lock()
	if err != nil {
		s.dropAccountLocked(acc)
		s.saveLocked()
		return nil, err
	}
	return acc, nil
}

func (s *Service) uniqueHandleLocked(email, name, sub string) string {
	base := sanitizeSlug(strings.Split(email, "@")[0])
	if base == "" {
		base = sanitizeSlug(name)
	}
	if len(base) < 2 {
		if len(sub) >= 8 {
			base = "g" + sanitizeSlug(sub)[:8]
		} else {
			base = "guser"
		}
	}
	if len(base) > 24 {
		base = base[:24]
	}
	for i := 0; i < 20; i++ {
		h := base
		if i > 0 {
			cut := 20
			if len(base) < cut {
				cut = len(base)
			}
			h = fmt.Sprintf("%s-%d", strings.TrimRight(base[:cut], "-"), i+1)
		}
		if reservedHandles[h] || s.byHandle[h] != nil {
			continue
		}
		return h
	}
	return ""
}
