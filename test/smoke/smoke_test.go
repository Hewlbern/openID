package smoke_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"solid-go/internal/logging"
	"solid-go/internal/server"
	"solid-go/internal/storage"
)

func TestSolidAgentAuditSmoke(t *testing.T) {
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}

	var srv *server.Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.Handler().ServeHTTP(w, r)
	}))
	defer ts.Close()

	srv = server.NewServer(&server.ServerOptions{
		Port:            0,
		Storage:         fs,
		StoragePath:     dir,
		Logger:          logging.NewBasicLogger(logging.Error),
		BaseURL:         ts.URL,
		AuditBatchEvery: time.Hour,
	})
	srv.Bootstrap(context.Background())

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("health %d", resp.StatusCode)
	}

	htmlReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	htmlReq.Header.Set("Accept", "text/html")
	htmlResp, err := http.DefaultClient.Do(htmlReq)
	if err != nil {
		t.Fatal(err)
	}
	htmlBody, _ := io.ReadAll(htmlResp.Body)
	htmlResp.Body.Close()
	if htmlResp.StatusCode != 200 || !bytes.Contains(htmlBody, []byte("Solid server")) {
		t.Fatalf("dashboard missing: %d %s", htmlResp.StatusCode, htmlBody[:min(200, len(htmlBody))])
	}

	recReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/records", nil)
	recReq.Header.Set("Accept", "text/html")
	recResp, err := http.DefaultClient.Do(recReq)
	if err != nil {
		t.Fatal(err)
	}
	recBody, _ := io.ReadAll(recResp.Body)
	recResp.Body.Close()
	if recResp.StatusCode != 200 || !bytes.Contains(recBody, []byte("Your records")) {
		t.Fatalf("records UI missing: %d %s", recResp.StatusCode, recBody[:min(200, len(recBody))])
	}

	welcomeReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/welcome", nil)
	welcomeReq.Header.Set("Accept", "text/html")
	welcomeResp, err := http.DefaultClient.Do(welcomeReq)
	if err != nil {
		t.Fatal(err)
	}
	welcomeBody, _ := io.ReadAll(welcomeResp.Body)
	welcomeResp.Body.Close()
	if welcomeResp.StatusCode != 200 || !bytes.Contains(welcomeBody, []byte("Claim this handle")) {
		t.Fatalf("landing page missing claim UI: %d %s", welcomeResp.StatusCode, welcomeBody[:min(200, len(welcomeBody))])
	}

	mcpResp, err := http.Get(ts.URL + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	mcpBody, _ := io.ReadAll(mcpResp.Body)
	mcpResp.Body.Close()
	if mcpResp.StatusCode != 200 || !bytes.Contains(mcpBody, []byte(`"open"`)) {
		t.Fatalf("mcp openable: %d %s", mcpResp.StatusCode, mcpBody)
	}
	mcpCall, err := http.Post(ts.URL+"/mcp", "application/json", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"openid_status","arguments":{}}}`))
	if err != nil {
		t.Fatal(err)
	}
	mcpCallBody, _ := io.ReadAll(mcpCall.Body)
	mcpCall.Body.Close()
	if mcpCall.StatusCode != 200 || !bytes.Contains(mcpCallBody, []byte("ok")) {
		t.Fatalf("mcp status tool: %d %s", mcpCall.StatusCode, mcpCallBody)
	}

	aresp, err := http.Post(ts.URL+"/agents", "application/json", bytes.NewBufferString(`{"name":"SmokeBot"}`))
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]interface{}
	_ = json.NewDecoder(aresp.Body).Decode(&reg)
	aresp.Body.Close()
	token, _ := reg["token"].(string)
	agent, _ := reg["agent"].(map[string]interface{})
	pod, _ := agent["podPath"].(string)
	if token == "" || pod == "" {
		t.Fatalf("register: %#v", reg)
	}

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/"+pod+"memo.txt", bytes.NewBufferString("hello"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	presp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	presp.Body.Close()
	if presp.StatusCode != 201 && presp.StatusCode != 200 {
		t.Fatalf("put %d", presp.StatusCode)
	}

	fresp, err := http.Post(ts.URL+"/audit/flush", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(fresp.Body)
	fresp.Body.Close()
	if fresp.StatusCode != 200 {
		t.Fatalf("flush %d %s", fresp.StatusCode, raw)
	}
	var batch map[string]interface{}
	_ = json.Unmarshal(raw, &batch)
	if batch["merkleRoot"] == nil || batch["ots"] == nil {
		t.Fatalf("batch: %s", raw)
	}

	eresp, err := http.Get(ts.URL + "/audit/events/")
	if err != nil {
		t.Fatal(err)
	}
	var events []map[string]interface{}
	_ = json.NewDecoder(eresp.Body).Decode(&events)
	eresp.Body.Close()
	if len(events) == 0 {
		t.Fatal("no audit events")
	}
	id, _ := events[0]["id"].(string)
	vresp, err := http.Get(ts.URL + "/audit/events/" + id + "/verify")
	if err != nil {
		t.Fatal(err)
	}
	var ver map[string]interface{}
	_ = json.NewDecoder(vresp.Body).Decode(&ver)
	vresp.Body.Close()
	if ver["verified"] != true {
		t.Fatalf("verify: %#v", ver)
	}
}

func TestReplicaAdoptSameAccount(t *testing.T) {
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	var srv *server.Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.Handler().ServeHTTP(w, r)
	}))
	defer ts.Close()
	srv = server.NewServer(&server.ServerOptions{
		Storage:         fs,
		StoragePath:     dir,
		Logger:          logging.NewBasicLogger(logging.Error),
		BaseURL:         ts.URL,
		AuditBatchEvery: time.Hour,
	})
	srv.Bootstrap(context.Background())

	regBody := `{"handle":"mike","password":"grokbot-dev-2026","name":"Temp","createPod":true}`
	regResp, err := http.Post(ts.URL+"/idp/register", "application/json", bytes.NewBufferString(regBody))
	if err != nil {
		t.Fatal(err)
	}
	regResp.Body.Close()
	if regResp.StatusCode != 200 {
		t.Fatalf("register %d", regResp.StatusCode)
	}

	loginResp, err := http.Post(ts.URL+"/idp/login", "application/json", bytes.NewBufferString(`{"handle":"mike","password":"grokbot-dev-2026"}`))
	if err != nil {
		t.Fatal(err)
	}
	var login map[string]interface{}
	_ = json.NewDecoder(loginResp.Body).Decode(&login)
	loginResp.Body.Close()
	token, _ := login["token"].(string)
	if token == "" {
		t.Fatal("login token")
	}

	adopt := map[string]interface{}{
		"password": "grokbot-dev-2026",
		"account": map[string]interface{}{
			"id":           "c06cdb70-9b3f-4fb9-9a7d-a70e41e0756f",
			"handle":       "mike",
			"name":         "Mike Holborn",
			"bio":          "Local Solid pod for Grok Bot.",
			"passwordHash": loginAccountHash(t, dir, "mike"),
			"podPath":      "mike/",
			"created":      "2026-08-19T08:15:03.606875Z",
		},
		"clients": []map[string]string{{
			"id":        "cc_74b671ce-3444-4b1f-bbf1-f1c8599047cf",
			"secret":    "test-secret",
			"accountId": "c06cdb70-9b3f-4fb9-9a7d-a70e41e0756f",
			"name":      "grokbot",
		}},
	}
	raw, _ := json.Marshal(adopt)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/idp/replica/adopt", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	adoptResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	adoptBody, _ := io.ReadAll(adoptResp.Body)
	adoptResp.Body.Close()
	if adoptResp.StatusCode != 200 {
		t.Fatalf("adopt %d %s", adoptResp.StatusCode, adoptBody)
	}
	var acc map[string]interface{}
	_ = json.Unmarshal(adoptBody, &acc)
	if acc["id"] != "c06cdb70-9b3f-4fb9-9a7d-a70e41e0756f" {
		t.Fatalf("id not adopted: %s", adoptBody)
	}
	if acc["name"] != "Mike Holborn" {
		t.Fatalf("name: %s", adoptBody)
	}
}

func loginAccountHash(t *testing.T, dir, handle string) string {
	t.Helper()
	raw, err := os.ReadFile(dir + "/.openid/accounts.json")
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Accounts []struct {
			Handle       string `json:"handle"`
			PasswordHash string `json:"passwordHash"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	for _, a := range st.Accounts {
		if a.Handle == handle {
			return a.PasswordHash
		}
	}
	t.Fatal("hash missing")
	return ""
}
