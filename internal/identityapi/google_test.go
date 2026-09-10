package identityapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"solid-go/internal/identityapi"
	"solid-go/internal/logging"
	"solid-go/internal/server"
	"solid-go/internal/storage"
)

func startIDP(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	var srv *server.Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.Handler().ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	srv = server.NewServer(&server.ServerOptions{
		Storage:         fs,
		StoragePath:     dir,
		Logger:          logging.NewBasicLogger(logging.Error),
		BaseURL:         ts.URL,
		AuditBatchEvery: time.Hour,
	})
	srv.Bootstrap(context.Background())
	return ts
}

func TestGoogleLoginCreatesAndRelinks(t *testing.T) {
	prev := identityapi.VerifyGoogleIDToken
	t.Cleanup(func() { identityapi.VerifyGoogleIDToken = prev })
	identityapi.VerifyGoogleIDToken = func(ctx context.Context, raw string) (*identityapi.GoogleIdentity, error) {
		if raw != "good-id-token" {
			return nil, errGoogle("bad token")
		}
		return &identityapi.GoogleIdentity{
			Sub:   "google-sub-1",
			Email: "ada@example.com",
			Name:  "Ada",
		}, nil
	}

	ts := startIDP(t)
	res, err := http.Post(ts.URL+"/idp/google", "application/json", bytes.NewBufferString(`{"idToken":"good-id-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("google create %d %s", res.StatusCode, raw)
	}
	var first map[string]any
	if err := json.Unmarshal(raw, &first); err != nil {
		t.Fatal(err)
	}
	token, _ := first["token"].(string)
	if token == "" {
		t.Fatalf("missing token %#v", first)
	}
	acc, _ := first["account"].(map[string]any)
	if acc["handle"] == "" || acc["email"] != "ada@example.com" {
		t.Fatalf("account %#v", acc)
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/idp/accounts/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	meRaw, _ := io.ReadAll(meResp.Body)
	meResp.Body.Close()
	if meResp.StatusCode != 200 || !bytes.Contains(meRaw, []byte("ada@example.com")) {
		t.Fatalf("me %d %s", meResp.StatusCode, meRaw)
	}

	again, err := http.Post(ts.URL+"/idp/google", "application/json", bytes.NewBufferString(`{"idToken":"good-id-token"}`))
	if err != nil {
		t.Fatal(err)
	}
	againRaw, _ := io.ReadAll(again.Body)
	again.Body.Close()
	if again.StatusCode != 200 {
		t.Fatalf("google relogin %d %s", again.StatusCode, againRaw)
	}
	var second map[string]any
	_ = json.Unmarshal(againRaw, &second)
	acc2, _ := second["account"].(map[string]any)
	if acc2["handle"] != acc["handle"] {
		t.Fatalf("expected same handle %v vs %v", acc["handle"], acc2["handle"])
	}
}

func TestGoogleRejectsBadToken(t *testing.T) {
	prev := identityapi.VerifyGoogleIDToken
	t.Cleanup(func() { identityapi.VerifyGoogleIDToken = prev })
	identityapi.VerifyGoogleIDToken = func(ctx context.Context, raw string) (*identityapi.GoogleIdentity, error) {
		return nil, errGoogle("invalid google id token")
	}
	ts := startIDP(t)
	res, err := http.Post(ts.URL+"/idp/google", "application/json", bytes.NewBufferString(`{"idToken":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestAgentPromptAndSession(t *testing.T) {
	ts := startIDP(t)
	promptRes, err := http.Get(ts.URL + "/api/agent/prompt")
	if err != nil {
		t.Fatal(err)
	}
	promptRaw, _ := io.ReadAll(promptRes.Body)
	promptRes.Body.Close()
	if promptRes.StatusCode != 200 || !bytes.Contains(promptRaw, []byte("Help me save conversation traces")) {
		t.Fatalf("prompt %d %s", promptRes.StatusCode, promptRaw)
	}
	var promptDoc map[string]any
	if err := json.Unmarshal(promptRaw, &promptDoc); err != nil {
		t.Fatal(err)
	}
	promptText, _ := promptDoc["prompt"].(string)
	if !bytes.Contains([]byte(promptText), []byte("Authorization: Bearer <TOKEN>")) {
		t.Fatalf("public prompt should use <TOKEN> placeholder: %s", promptText)
	}
	if promptDoc["auth"] != "Authorization: Bearer <TOKEN>" {
		t.Fatalf("auth field %#v", promptDoc["auth"])
	}
	if bytes.Contains(promptRaw, []byte("megabot.tech")) {
		t.Fatal("prompt leaked megabot")
	}

	llms, err := http.Get(ts.URL + "/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	llmsRaw, _ := io.ReadAll(llms.Body)
	llms.Body.Close()
	if llms.StatusCode != 200 || !bytes.Contains(llmsRaw, []byte("spark_save_conversation")) {
		t.Fatalf("llms %d %s", llms.StatusCode, llmsRaw)
	}

	unauth, err := http.Post(ts.URL+"/api/agent/session", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	unauth.Body.Close()
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unsigned session %d", unauth.StatusCode)
	}

	reg, err := http.Post(ts.URL+"/idp/register", "application/json", bytes.NewBufferString(`{"handle":"ada","password":"testpass123","name":"Ada"}`))
	if err != nil {
		t.Fatal(err)
	}
	var regBody map[string]any
	_ = json.NewDecoder(reg.Body).Decode(&regBody)
	reg.Body.Close()
	token, _ := regBody["token"].(string)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/agent/session", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	sess, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	sessRaw, _ := io.ReadAll(sess.Body)
	sess.Body.Close()
	if sess.StatusCode != 200 {
		t.Fatalf("session %d %s", sess.StatusCode, sessRaw)
	}
	var doc map[string]any
	if err := json.Unmarshal(sessRaw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["token"] == "" || doc["prompt"] == "" {
		t.Fatalf("session body %#v", doc)
	}
	if doc["tokenKind"] != "access_token" && doc["tokenKind"] != "agent_jwt" && doc["tokenKind"] != "spark-mcp" {
		t.Fatalf("session tokenKind %#v", doc["tokenKind"])
	}
	if !bytes.Contains([]byte(doc["prompt"].(string)), []byte(doc["token"].(string))) {
		t.Fatal("prompt should embed minted token")
	}
}

type googleErr string

func (e googleErr) Error() string { return string(e) }

func errGoogle(s string) error { return googleErr(s) }
