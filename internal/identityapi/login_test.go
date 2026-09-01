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

	"solid-go/internal/logging"
	"solid-go/internal/server"
	"solid-go/internal/storage"
)

func TestRegisterLoginMeLogout(t *testing.T) {
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

	reg, err := http.Post(ts.URL+"/idp/register", "application/json", bytes.NewBufferString(`{"handle":"ida","password":"testpass123","name":"Ida","createPod":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var regBody map[string]any
	_ = json.NewDecoder(reg.Body).Decode(&regBody)
	reg.Body.Close()
	if reg.StatusCode != 200 {
		t.Fatalf("register %d", reg.StatusCode)
	}
	token, _ := regBody["token"].(string)
	webID, _ := regBody["webId"].(string)
	if token == "" || webID == "" {
		t.Fatalf("register body %#v", regBody)
	}
	if got := reg.Header.Get("Set-Cookie"); got == "" || !bytes.Contains([]byte(got), []byte("solid-session")) {
		t.Fatalf("register cookie %q", got)
	}

	login, err := http.Post(ts.URL+"/idp/login", "application/json", bytes.NewBufferString(`{"handle":"ida","password":"testpass123"}`))
	if err != nil {
		t.Fatal(err)
	}
	var loginBody map[string]any
	_ = json.NewDecoder(login.Body).Decode(&loginBody)
	login.Body.Close()
	if login.StatusCode != 200 {
		t.Fatalf("login %d", login.StatusCode)
	}
	token, _ = loginBody["token"].(string)
	if token == "" {
		t.Fatal("login token")
	}

	meReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/idp/accounts/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meResp, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	meRaw, _ := io.ReadAll(meResp.Body)
	meResp.Body.Close()
	if meResp.StatusCode != 200 || !bytes.Contains(meRaw, []byte(`"ida"`)) || !bytes.Contains(meRaw, []byte("webId")) {
		t.Fatalf("me %d %s", meResp.StatusCode, meRaw)
	}

	outReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/idp/logout", nil)
	outReq.Header.Set("Authorization", "Bearer "+token)
	outResp, err := http.DefaultClient.Do(outReq)
	if err != nil {
		t.Fatal(err)
	}
	outResp.Body.Close()
	if outResp.StatusCode != 204 && outResp.StatusCode != 200 {
		t.Fatalf("logout %d", outResp.StatusCode)
	}
}
