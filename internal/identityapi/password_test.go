package identityapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solid-go/internal/logging"
	"solid-go/internal/server"
	"solid-go/internal/storage"
)

func TestPlaintextAuthRemovedAndPathsBlocked(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, ".openid", "local-auth.json")
	if err := os.MkdirAll(filepath.Dir(authPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const leaked = "leaked-luk-password"
	if err := os.WriteFile(authPath, []byte(`{"handle":"luk","password":"`+leaked+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ts, _ := newTestPod(t, dir)
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("local-auth.json still on disk: %v", err)
	}
	meta := filepath.Join(dir, ".openid", "local-auth.json.meta.json")
	if _, err := os.Stat(meta); !os.IsNotExist(err) {
		t.Fatalf("local-auth meta still on disk: %v", err)
	}

	reg := postJSON(t, ts.URL+"/idp/register", `{"handle":"ada","email":"ada@example.com","password":"testpass123","name":"Ada","createPod":true}`)
	if reg.Status != 200 || reg.Token == "" {
		t.Fatalf("register %d", reg.Status)
	}
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatal("register recreated local-auth.json")
	}
	accounts := readFile(t, filepath.Join(dir, ".openid", "accounts.json"))
	if strings.Contains(accounts, "testpass123") || strings.Contains(accounts, leaked) {
		t.Fatal("plaintext password stored in accounts.json")
	}

	paths := []string{
		"/.openid/accounts.json",
		"/.openid/local-auth.json",
		"/.openid/password-resets.json",
		"/.openid/security-audit.json",
		"/.openid/",
	}
	for _, p := range paths {
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodOptions} {
			req, err := http.NewRequest(method, ts.URL+p, strings.NewReader(`{"password":"nope"}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+reg.Token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("%s %s → %d", method, p, resp.StatusCode)
			}
			if strings.Contains(string(body), "passwordHash") || strings.Contains(string(body), leaked) || strings.Contains(string(body), "testpass123") {
				t.Fatalf("%s %s leaked", method, p)
			}
		}
	}
	trav, err := http.NewRequest(http.MethodGet, ts.URL+"/ada/profile/card", nil)
	if err != nil {
		t.Fatal(err)
	}
	trav.URL.Path = "/ada/../.openid/accounts.json"
	trav.URL.RawPath = "/ada/../.openid/accounts.json"
	rec := httptest.NewRecorder()
	ts.Config.Handler.ServeHTTP(rec, trav)
	if rec.Code == http.StatusMovedPermanently || rec.Code == http.StatusFound {
		loc := rec.Header().Get("Location")
		if !strings.Contains(loc, "/.openid/accounts.json") {
			t.Fatalf("traversal redirect %s", loc)
		}
		follow, err := http.Get(ts.URL + loc)
		if err != nil {
			t.Fatal(err)
		}
		followBody, _ := io.ReadAll(follow.Body)
		follow.Body.Close()
		if follow.StatusCode != http.StatusNotFound || strings.Contains(string(followBody), "passwordHash") {
			t.Fatalf("traversal follow %d", follow.StatusCode)
		}
	} else if rec.Code != http.StatusNotFound {
		t.Fatalf("traversal %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "passwordHash") || strings.Contains(rec.Body.String(), "testpass123") {
		t.Fatal("traversal response leaked accounts")
	}

	pub, err := http.Get(ts.URL + "/ada/profile/card")
	if err != nil {
		t.Fatal(err)
	}
	pubBody, _ := io.ReadAll(pub.Body)
	pub.Body.Close()
	if pub.StatusCode != http.StatusOK || !bytes.Contains(pubBody, []byte("Ada")) {
		t.Fatalf("profile still public: %d %s", pub.StatusCode, pubBody)
	}
}

func TestPasswordResetFlow(t *testing.T) {
	dir := t.TempDir()
	mailDir := t.TempDir()
	t.Setenv("OPENID_EMAIL_DIR", mailDir)
	t.Setenv("OPENID_ADMIN_RESET_SECRET", "test-admin-secret")
	logs := &memLog{}
	ts, _ := newTestPod(t, dir, logs)

	ada := postJSON(t, ts.URL+"/idp/register", `{"handle":"ada","email":"ada@example.com","password":"testpass123","name":"Ada","createPod":true}`)
	mike := postJSON(t, ts.URL+"/idp/register", `{"handle":"mike","email":"mike@localhost","password":"old-mike-pass","name":"Mike","createPod":true}`)
	if ada.Status != 200 || mike.Status != 200 {
		t.Fatalf("register ada %d mike %d", ada.Status, mike.Status)
	}

	unknown := postJSON(t, ts.URL+"/idp/password/forgot", `{"email":"nobody@example.com"}`)
	known := postJSON(t, ts.URL+"/idp/password/forgot", `{"email":"ada@example.com"}`)
	if unknown.Status != 200 || known.Status != 200 || unknown.Raw != known.Raw {
		t.Fatalf("forgot responses differ: %d %q vs %d %q", unknown.Status, unknown.Raw, known.Status, known.Raw)
	}
	if !strings.Contains(known.Raw, "deliverable email") {
		t.Fatalf("forgot body %s", known.Raw)
	}
	local := postJSON(t, ts.URL+"/idp/password/forgot", `{"handle":"mike"}`)
	if local.Status != 200 || local.Raw != known.Raw {
		t.Fatalf("localhost forgot revealed a difference: %s", local.Raw)
	}

	mails := readMails(t, mailDir)
	if len(mails) != 1 || mails[0]["to"] != "ada@example.com" {
		t.Fatalf("mails %#v", mails)
	}
	token := tokenFromMail(mails[0]["text"])
	if token == "" || strings.Contains(known.Raw, token) {
		t.Fatal("reset token missing from mail or echoed in the HTTP response")
	}
	assertNoSecret(t, dir, logs, token, "testpass123", "old-mike-pass")

	bad := postJSON(t, ts.URL+"/idp/password/reset", `{"token":"`+token+`","password":"short"}`)
	if bad.Status != http.StatusBadRequest {
		t.Fatalf("short password %d", bad.Status)
	}
	reset := postJSON(t, ts.URL+"/idp/password/reset", `{"token":"`+token+`","password":"newpass1234"}`)
	if reset.Status != 200 {
		t.Fatalf("reset %d %s", reset.Status, reset.Raw)
	}
	again := postJSON(t, ts.URL+"/idp/password/reset", `{"token":"`+token+`","password":"newpass1234"}`)
	if again.Status != http.StatusBadRequest {
		t.Fatalf("reuse %d", again.Status)
	}
	if code := loginStatus(t, ts.URL, "ada", "testpass123"); code != http.StatusUnauthorized {
		t.Fatalf("old password %d", code)
	}
	ada2 := login(t, ts.URL, "ada", "newpass1234")
	if ada2.Status != 200 || ada2.Token == "" {
		t.Fatalf("new login %d", ada2.Status)
	}
	if meStatus(t, ts.URL, ada.Token) != http.StatusUnauthorized {
		t.Fatal("old bearer still accepted")
	}
	if meStatus(t, ts.URL, ada2.Token) != 200 {
		t.Fatal("new bearer rejected")
	}
	podReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/ada/profile/card", nil)
	podReq.Header.Set("Authorization", "Bearer "+ada.Token)
	podResp, err := http.DefaultClient.Do(podReq)
	if err != nil {
		t.Fatal(err)
	}
	podResp.Body.Close()
	if podResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old bearer still read the pod: %d", podResp.StatusCode)
	}

	hiddenDir := t.TempDir()
	t.Setenv("OPENID_ADMIN_RESET_SECRET", "")
	hidden, _ := newTestPod(t, hiddenDir)
	miss := adminPost(t, hidden.URL, "", `{"handle":"mike"}`, "/idp/password/admin-reset")
	if miss.Status != http.StatusNotFound {
		t.Fatalf("unset secret %d", miss.Status)
	}

	wrong := adminPost(t, ts.URL, "nope", `{"handle":"mike"}`, "/idp/password/admin-reset")
	if wrong.Status != http.StatusUnauthorized {
		t.Fatalf("bad secret %d", wrong.Status)
	}
	issued := adminPost(t, ts.URL, "test-admin-secret", `{"handle":"mike"}`, "/idp/password/admin-reset")
	if issued.Status != 200 {
		t.Fatalf("admin reset %d %s", issued.Status, issued.Raw)
	}
	var issuedBody struct {
		ResetURL string `json:"resetUrl"`
	}
	if err := json.Unmarshal([]byte(issued.Raw), &issuedBody); err != nil {
		t.Fatal(err)
	}
	mikeToken := tokenFromMail(issuedBody.ResetURL)
	if mikeToken == "" || !strings.Contains(issuedBody.ResetURL, "#token=") {
		t.Fatal("admin link missing fragment token")
	}
	if strings.Contains(logs.String(), mikeToken) || strings.Contains(logs.String(), "old-mike-pass") {
		t.Fatal("admin token or password written to logs")
	}
	done := postJSON(t, ts.URL+"/idp/password/reset", `{"token":"`+mikeToken+`","password":"mike-new-pass"}`)
	if done.Status != 200 {
		t.Fatalf("mike reset %d %s", done.Status, done.Raw)
	}
	if meStatus(t, ts.URL, mike.Token) != http.StatusUnauthorized {
		t.Fatal("mike bearer survived reset")
	}
	if loginStatus(t, ts.URL, "mike", "mike-new-pass") != 200 {
		t.Fatal("mike new password failed")
	}
	assertNoSecret(t, dir, logs, token, mikeToken, "testpass123", "old-mike-pass", "newpass1234", "mike-new-pass")

	changed := authedJSON(t, ts.URL+"/idp/profile", ada2.Token, `{"email":"ada.real@example.com","currentPassword":"newpass1234"}`)
	if changed.Status != 200 || !strings.Contains(changed.Raw, "ada.real@example.com") {
		t.Fatalf("email change %d %s", changed.Status, changed.Raw)
	}
	noCur := authedJSON(t, ts.URL+"/idp/profile", ada2.Token, `{"password":"another-pass-9"}`)
	if noCur.Status != http.StatusUnauthorized {
		t.Fatalf("password change without current %d", noCur.Status)
	}
	pw := authedJSON(t, ts.URL+"/idp/profile", ada2.Token, `{"password":"another-pass-9","currentPassword":"newpass1234"}`)
	if pw.Status != 200 || !strings.Contains(pw.Raw, `"token"`) {
		t.Fatalf("password change %d %s", pw.Status, redact(pw.Raw))
	}
	if meStatus(t, ts.URL, ada2.Token) != http.StatusUnauthorized {
		t.Fatal("previous bearer survived password change")
	}

	cc := authedJSON(t, ts.URL+"/idp/client-credentials", jsonString(t, pw.Raw, "token"), `{"name":"agent"}`)
	if cc.Status != 200 {
		t.Fatalf("client %d %s", cc.Status, cc.Raw)
	}
	var cred struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal([]byte(cc.Raw), &cred); err != nil || cred.Secret == "" {
		t.Fatalf("cred %s", cc.Raw)
	}
	if clientToken(t, ts.URL, cred.ID, cred.Secret) != 200 {
		t.Fatal("client credentials did not mint")
	}
	revoked := adminPost(t, ts.URL, "test-admin-secret", `{"all":true}`, "/idp/admin/revoke-clients")
	if revoked.Status != 200 || !strings.Contains(revoked.Raw, `"revoked"`) {
		t.Fatalf("revoke %d %s", revoked.Status, revoked.Raw)
	}
	if clientToken(t, ts.URL, cred.ID, cred.Secret) != http.StatusUnauthorized {
		t.Fatal("revoked client secret still works")
	}
	assertNoSecret(t, dir, logs, token, mikeToken, cred.Secret, "testpass123", "another-pass-9")
}

func TestForgotPasswordRateLimit(t *testing.T) {
	dir := t.TempDir()
	ts, _ := newTestPod(t, dir)
	var last apiResult
	for i := 0; i < 5; i++ {
		last = postJSON(t, ts.URL+"/idp/password/forgot", `{"email":"nobody@example.com"}`)
		if last.Status != 200 {
			t.Fatalf("attempt %d: %d", i, last.Status)
		}
	}
	blocked := postJSON(t, ts.URL+"/idp/password/forgot", `{"email":"nobody@example.com"}`)
	if blocked.Status != http.StatusTooManyRequests {
		t.Fatalf("rate limit %d", blocked.Status)
	}
	if blocked.Raw == "" || strings.Contains(blocked.Raw, "nobody@example.com") {
		t.Fatalf("rate limit body %s", blocked.Raw)
	}
}

type apiResult struct {
	Status int
	Raw    string
	Token  string
}

type memLog struct{ bytes.Buffer }

func (m *memLog) Debug(string, ...interface{}) {}
func (m *memLog) Info(format string, args ...interface{}) {
	fmt.Fprintf(&m.Buffer, format+"\n", args...)
}
func (m *memLog) Warn(format string, args ...interface{})  { m.Info(format, args...) }
func (m *memLog) Error(format string, args ...interface{}) { m.Info(format, args...) }
func (m *memLog) Fatal(format string, args ...interface{}) { m.Info(format, args...) }

func newTestPod(t *testing.T, dir string, loggers ...logging.Logger) (*httptest.Server, *server.Server) {
	t.Helper()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	var logger logging.Logger = logging.NewBasicLogger(logging.Error)
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	var srv *server.Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.Handler().ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	srv = server.NewServer(&server.ServerOptions{
		Storage:         fs,
		StoragePath:     dir,
		Logger:          logger,
		BaseURL:         ts.URL,
		AuditBatchEvery: time.Hour,
	})
	srv.Bootstrap(context.Background())
	return ts, srv
}

func postJSON(t *testing.T, url, body string) apiResult {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var doc struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(raw, &doc)
	return apiResult{Status: resp.StatusCode, Raw: string(raw), Token: doc.Token}
}

func authedJSON(t *testing.T, url, token, body string) apiResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url, "/idp/profile") {
		req.Method = http.MethodPost
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return apiResult{Status: resp.StatusCode, Raw: string(raw), Token: jsonString(t, string(raw), "token")}
}

func adminPost(t *testing.T, base, secret, body, path string) apiResult {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-OpenID-Admin-Secret", secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return apiResult{Status: resp.StatusCode, Raw: string(raw)}
}

func login(t *testing.T, base, handle, password string) apiResult {
	t.Helper()
	return postJSON(t, base+"/idp/login", `{"handle":"`+handle+`","password":"`+password+`"}`)
}

func loginStatus(t *testing.T, base, handle, password string) int {
	t.Helper()
	return login(t, base, handle, password).Status
}

func meStatus(t *testing.T, base, token string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/idp/accounts/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func clientToken(t *testing.T, base, id, secret string) int {
	t.Helper()
	resp, err := http.Post(base+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader("grant_type=client_credentials&client_id="+id+"&client_secret="+secret))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func readMails(t *testing.T, dir string) []map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]string
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]string
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		out = append(out, doc)
	}
	return out
}

func tokenFromMail(text string) string {
	const key = "#token="
	i := strings.Index(text, key)
	if i < 0 {
		return ""
	}
	rest := text[i+len(key):]
	if n := strings.IndexAny(rest, " \r\n\t"); n >= 0 {
		rest = rest[:n]
	}
	return strings.TrimSpace(rest)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertNoSecret(t *testing.T, dir string, logs *memLog, secrets ...string) {
	t.Helper()
	var blobs []string
	if logs != nil {
		blobs = append(blobs, logs.String())
	}
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		blobs = append(blobs, string(raw))
		return nil
	})
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		for _, blob := range blobs {
			if strings.Contains(blob, secret) {
				t.Fatalf("secret leaked into storage or logs")
			}
		}
	}
}

func jsonString(t *testing.T, raw, key string) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return ""
	}
	s, _ := doc[key].(string)
	return s
}

func redact(raw string) string {
	var doc map[string]any
	if json.Unmarshal([]byte(raw), &doc) != nil {
		return "unparsed"
	}
	if _, ok := doc["token"]; ok {
		doc["token"] = "redacted"
	}
	out, _ := json.Marshal(doc)
	return string(out)
}
