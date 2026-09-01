package authn

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBearerRoundTrip(t *testing.T) {
	ts := NewTokenService("test-secret")
	tok, err := ts.Issue("https://ex/card#me", "client1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	creds, err := ts.Parse(tok)
	if err != nil || creds.WebID != "https://ex/card#me" {
		t.Fatalf("%v %#v", err, creds)
	}
	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	got, err := ts.Extract(req)
	if err != nil || got.WebID != "https://ex/card#me" || got.Via != "bearer" {
		t.Fatalf("%v %#v", err, got)
	}
}

func TestAgentSignature(t *testing.T) {
	ts := NewTokenService("test-secret")
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	webID := "https://ex/agent#me"
	tsStr := fmt.Sprintf("%d", time.Now().Unix())
	msg := []byte("PUT|/data|" + tsStr + "|" + webID)
	sig := ed25519.Sign(priv, msg)
	req := httptest.NewRequest("PUT", "http://localhost/data", nil)
	req.Header.Set("X-Agent-WebID", webID)
	req.Header.Set("X-Agent-Timestamp", tsStr)
	req.Header.Set("X-Agent-Public-Key", base64.RawURLEncoding.EncodeToString(pub))
	req.Header.Set("X-Agent-Signature", base64.RawURLEncoding.EncodeToString(sig))
	creds, err := ts.Extract(req)
	if err != nil || creds.WebID != webID || creds.Via != "agent_sig" {
		t.Fatalf("%v %#v", err, creds)
	}
}

func TestSparkConnectToken(t *testing.T) {
	ts := NewTokenService("test-secret")
	tok, jti, exp, err := ts.IssueSpark("https://ex/ada/profile/card#me", time.Hour)
	if err != nil || tok == "" || jti == "" || exp.Before(time.Now()) {
		t.Fatalf("%v %s %s %v", err, tok, jti, exp)
	}
	creds, err := ts.Parse(tok)
	if err != nil || !creds.IsSpark() || creds.WebID != "https://ex/ada/profile/card#me" || creds.JTI != jti {
		t.Fatalf("%v %#v", err, creds)
	}
	if HandleFromWebID(creds.WebID) != "ada" {
		t.Fatalf("handle %s", HandleFromWebID(creds.WebID))
	}
	if !SparkPathAllowed(creds.WebID, "ada/conversations/spark/x.json", http.MethodPut) {
		t.Fatal("owner conversations should be allowed")
	}
	if SparkPathAllowed(creds.WebID, "bob/conversations/spark/x.json", http.MethodPut) {
		t.Fatal("other pod must be denied")
	}
	if SparkPathAllowed(creds.WebID, "ada/inbox/note.json", http.MethodPut) {
		t.Fatal("inbox must be denied")
	}
	ts.RevokeJTI(jti)
	if _, err := ts.Parse(tok); err != ErrRevoked {
		t.Fatalf("revoked want ErrRevoked got %v", err)
	}
	session, _ := ts.Issue("https://ex/ada/profile/card#me", "", time.Hour)
	sess, err := ts.Parse(session)
	if err != nil || sess.IsSpark() {
		t.Fatalf("session token should not be spark: %v %#v", err, sess)
	}
}

func TestDPoP(t *testing.T) {
	ts := NewTokenService("test-secret")
	tok, _ := ts.Issue("https://ex/card#me", "", time.Hour)
	// minimal DPoP proof payload (header.payload.sig) — verification unpacks payload claims
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"htm":"GET","htu":"http://localhost/x","jti":"abc123"}`))
	dpop := "eyJhbGciOiJub25lIn0." + payload + ".x"
	req := httptest.NewRequest("GET", "http://localhost/x", nil)
	req.Header.Set("Authorization", "DPoP "+tok)
	req.Header.Set("DPoP", dpop)
	creds, err := ts.Extract(req)
	if err != nil || creds.Via != "dpop" {
		t.Fatalf("%v %#v", err, creds)
	}
}
