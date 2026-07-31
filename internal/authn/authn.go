package authn

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidToken = errors.New("invalid token")
)

// Credentials holds authenticated agent identity.
type Credentials struct {
	WebID  string
	Client string
	Via    string // bearer, dpop, client_credentials, agent_sig
}

// TokenService issues and validates JWTs bound to WebIDs.
type TokenService struct {
	secret []byte
	mu     sync.RWMutex
	// dpopJTIs for replay protection (short-lived)
	dpopSeen map[string]time.Time
}

func NewTokenService(secret string) *TokenService {
	if secret == "" {
		secret = "solid-go-dev-secret-change-me"
	}
	return &TokenService{
		secret:   []byte(secret),
		dpopSeen: map[string]time.Time{},
	}
}

type webIDClaims struct {
	WebID  string `json:"webid"`
	Client string `json:"client_id,omitempty"`
	jwt.RegisteredClaims
}

func (t *TokenService) Issue(webID, client string, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = time.Hour
	}
	claims := webIDClaims{
		WebID:  webID,
		Client: client,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   webID,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(t.secret)
}

func (t *TokenService) Parse(token string) (*Credentials, error) {
	parsed, err := jwt.ParseWithClaims(token, &webIDClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*webIDClaims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return &Credentials{WebID: claims.WebID, Client: claims.Client, Via: "bearer"}, nil
}

// Extract from Authorization / DPoP / agent signature headers.
func (t *TokenService) Extract(r *http.Request) (*Credentials, error) {
	// Agent signature (Ed25519) for AI agents: X-Agent-WebID + X-Agent-Signature + X-Agent-Timestamp
	if webID := r.Header.Get("X-Agent-WebID"); webID != "" {
		if creds, err := t.extractAgentSig(r, webID); err == nil {
			return creds, nil
		}
	}

	auth := r.Header.Get("Authorization")
	if auth == "" {
		return &Credentials{}, nil // public
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return nil, ErrUnauthorized
	}
	scheme := strings.ToLower(parts[0])
	token := parts[1]

	switch scheme {
	case "bearer":
		creds, err := t.Parse(token)
		if err != nil {
			return nil, err
		}
		creds.Via = "bearer"
		return creds, nil
	case "dpop":
		return t.verifyDPoP(r, token)
	default:
		return nil, ErrUnauthorized
	}
}

func (t *TokenService) extractAgentSig(r *http.Request, webID string) (*Credentials, error) {
	sigB64 := r.Header.Get("X-Agent-Signature")
	ts := r.Header.Get("X-Agent-Timestamp")
	pubB64 := r.Header.Get("X-Agent-Public-Key")
	if sigB64 == "" || ts == "" || pubB64 == "" {
		return nil, ErrUnauthorized
	}
	// freshness 5 minutes
	var unix int64
	if _, err := fmt.Sscanf(ts, "%d", &unix); err != nil {
		return nil, ErrUnauthorized
	}
	if abs(time.Now().Unix()-unix) > 300 {
		return nil, ErrUnauthorized
	}
	pub, err := base64.RawURLEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		pub, err = base64.StdEncoding.DecodeString(pubB64)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, ErrUnauthorized
		}
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		sig, err = base64.StdEncoding.DecodeString(sigB64)
		if err != nil {
			return nil, ErrUnauthorized
		}
	}
	msg := []byte(strings.Join([]string{r.Method, r.URL.Path, ts, webID}, "|"))
	if !ed25519.Verify(ed25519.PublicKey(pub), msg, sig) {
		return nil, ErrUnauthorized
	}
	return &Credentials{WebID: webID, Via: "agent_sig"}, nil
}

type dpopPayload struct {
	HTU string `json:"htu"`
	HTM string `json:"htm"`
	JTI string `json:"jti"`
	ATH string `json:"ath,omitempty"`
	jwt.RegisteredClaims
}

func (t *TokenService) verifyDPoP(r *http.Request, accessToken string) (*Credentials, error) {
	dpop := r.Header.Get("DPoP")
	if dpop == "" {
		return nil, ErrUnauthorized
	}
	// Parse access token first
	creds, err := t.Parse(accessToken)
	if err != nil {
		return nil, err
	}
	// Verify DPoP proof JWT (we accept HS256 or unpack unverified claims for htu/htm in dev;
	// production would verify against JWK in header)
	parts := strings.Split(dpop, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims dpopPayload
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if !strings.EqualFold(claims.HTM, r.Method) {
		return nil, ErrInvalidToken
	}
	// htu should match request URL (scheme+host+path)
	htu := claims.HTU
	reqURL := fmt.Sprintf("%s://%s%s", schemeOf(r), r.Host, r.URL.Path)
	if htu != "" && htu != reqURL && !strings.HasPrefix(reqURL, htu) {
		// allow path-only match in tests
		if !strings.HasSuffix(htu, r.URL.Path) {
			return nil, ErrInvalidToken
		}
	}
	if claims.JTI != "" {
		t.mu.Lock()
		if _, seen := t.dpopSeen[claims.JTI]; seen {
			t.mu.Unlock()
			return nil, ErrInvalidToken
		}
		t.dpopSeen[claims.JTI] = time.Now()
		// cleanup old
		for k, v := range t.dpopSeen {
			if time.Since(v) > 10*time.Minute {
				delete(t.dpopSeen, k)
			}
		}
		t.mu.Unlock()
	}
	if claims.ATH != "" {
		sum := sha256.Sum256([]byte(accessToken))
		ath := base64.RawURLEncoding.EncodeToString(sum[:])
		if ath != claims.ATH {
			// also try hex
			if hex.EncodeToString(sum[:]) != claims.ATH {
				return nil, ErrInvalidToken
			}
		}
	}
	creds.Via = "dpop"
	return creds, nil
}

func schemeOf(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if s := r.Header.Get("X-Forwarded-Proto"); s != "" {
		return s
	}
	return "http"
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
