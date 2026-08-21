package smoke_test

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
