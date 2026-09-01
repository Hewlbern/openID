package openidmcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"solid-go/internal/authn"
	"solid-go/internal/identityapi"
	"solid-go/internal/logging"
	"solid-go/internal/openidmcp"
	"solid-go/internal/resourcestore"
	"solid-go/internal/server"
	"solid-go/internal/solid"
	"solid-go/internal/storage"
	"solid-go/internal/wac"
)

func startOpenID(t *testing.T) (*httptest.Server, *openidmcp.Server) {
	t.Helper()
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	var solid *server.Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		solid.Handler().ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	solid = server.NewServer(&server.ServerOptions{
		Port:            0,
		Storage:         fs,
		StoragePath:     dir,
		Logger:          logging.NewBasicLogger(logging.Error),
		BaseURL:         ts.URL,
		AuditBatchEvery: time.Hour,
	})
	solid.Bootstrap(context.Background())
	mcp := openidmcp.New(ts.URL)
	mcp.HTTP = ts.Client()
	return ts, mcp
}

func mustRPC(t *testing.T, mcp *openidmcp.Server, method string, id int, params any) map[string]any {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	req := openidmcp.Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: method, Params: raw}
	if id != 1 {
		req.ID = json.RawMessage([]byte(`"` + string(rune('0'+id)) + `"`))
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp := mcp.Handle(body)
	if resp == nil {
		t.Fatalf("nil response for %s", method)
	}
	if resp.Error != nil {
		t.Fatalf("%s error: %v", method, resp.Error)
	}
	b, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func callTool(t *testing.T, mcp *openidmcp.Server, name string, args map[string]any) map[string]any {
	t.Helper()
	res := mustRPC(t, mcp, "tools/call", 1, map[string]any{"name": name, "arguments": args})
	if res["isError"] == true {
		t.Fatalf("tool %s isError: %v", name, res)
	}
	content, _ := res["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("tool %s empty content", name)
	}
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		t.Fatalf("tool %s not json: %s", name, text)
	}
	return parsed
}

func callToolErr(t *testing.T, mcp *openidmcp.Server, name string, args map[string]any) string {
	t.Helper()
	req := openidmcp.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/call",
	}
	p, _ := json.Marshal(map[string]any{"name": name, "arguments": args})
	req.Params = p
	body, _ := json.Marshal(req)
	resp := mcp.Handle(body)
	if resp == nil || resp.Error != nil {
		t.Fatalf("unexpected rpc error: %#v", resp)
	}
	b, _ := json.Marshal(resp.Result)
	var res openidmcp.CallResult
	_ = json.Unmarshal(b, &res)
	if !res.IsError {
		t.Fatalf("expected tool error for %s", name)
	}
	return res.Content[0].Text
}

func TestProtocolInitializeListPing(t *testing.T) {
	_, mcp := startOpenID(t)

	init := mustRPC(t, mcp, "initialize", 1, map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "grokbot", "version": "test"},
	})
	if init["protocolVersion"] != "2025-03-26" {
		t.Fatalf("protocol: %#v", init)
	}
	info, _ := init["serverInfo"].(map[string]any)
	if info["name"] != "openid" {
		t.Fatalf("serverInfo: %#v", info)
	}

	note := mcp.Handle([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if note != nil {
		t.Fatalf("notification should be silent: %#v", note)
	}

	pong := mustRPC(t, mcp, "ping", 1, nil)
	if pong == nil {
		t.Fatal("ping")
	}

	listed := mustRPC(t, mcp, "tools/list", 1, nil)
	tools, _ := listed["tools"].([]any)
	if len(tools) != len(openidmcp.Tools()) {
		t.Fatalf("tools %d want %d", len(tools), len(openidmcp.Tools()))
	}
	names := map[string]bool{}
	for _, raw := range tools {
		item := raw.(map[string]any)
		names[item["name"].(string)] = true
		if item["inputSchema"] == nil {
			t.Fatalf("missing schema for %s", item["name"])
		}
	}
	for _, want := range []string{
		"openid_open", "openid_status", "openid_discover", "openid_register",
		"openid_login", "openid_register_agent", "openid_pod_put", "openid_pod_get",
		"openid_client_credentials", "openid_token", "openid_audit_flush", "openid_audit_verify",
		"spark_save_conversation", "spark_list_conversations", "spark_share_conversation",
	} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}

	unknown := mcp.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"nope"}`))
	if unknown == nil || unknown.Error == nil || unknown.Error.Code != openidmcp.ErrNoMethod {
		t.Fatalf("unknown method: %#v", unknown)
	}
	badJSON := mcp.Handle([]byte(`{`))
	if badJSON == nil || badJSON.Error == nil || badJSON.Error.Code != openidmcp.ErrParse {
		t.Fatalf("bad json: %#v", badJSON)
	}
}

func TestEveryToolFlow(t *testing.T) {
	_, mcp := startOpenID(t)

	opened := callTool(t, mcp, "openid_open", nil)
	if opened["open"] != true {
		t.Fatalf("open: %#v", opened)
	}
	if !strings.Contains(opened["mcp"].(string), "/mcp") {
		t.Fatalf("mcp url: %#v", opened)
	}

	status := callTool(t, mcp, "openid_status", nil)
	health, _ := status["health"].(map[string]any)
	inner, _ := health["json"].(map[string]any)
	if inner["status"] != "ok" {
		t.Fatalf("status: %#v", status)
	}

	disco := callTool(t, mcp, "openid_discover", nil)
	oidcWrap, _ := disco["oidc"].(map[string]any)
	oidc, _ := oidcWrap["json"].(map[string]any)
	if oidc["token_endpoint"] == nil || oidc["issuer"] == nil {
		t.Fatalf("oidc: %#v", disco)
	}

	avail := callTool(t, mcp, "openid_handle_available", map[string]any{"handle": "ada"})
	aj, _ := avail["json"].(map[string]any)
	if aj["available"] != true {
		t.Fatalf("available: %#v", avail)
	}

	reg := callTool(t, mcp, "openid_register", map[string]any{
		"handle": "ada", "password": "testpass123", "name": "Ada", "email": "ada@example.com",
	})
	accWrap := reg["json"].(map[string]any)
	token, _ := accWrap["token"].(string)
	if token == "" {
		t.Fatalf("register: %#v", reg)
	}

	login := callTool(t, mcp, "openid_login", map[string]any{"handle": "ada", "password": "testpass123"})
	loginJSON := login["json"].(map[string]any)
	token, _ = loginJSON["token"].(string)
	if token == "" {
		t.Fatalf("login: %#v", login)
	}

	me := callTool(t, mcp, "openid_me", map[string]any{"token": token})
	meJSON := me["json"].(map[string]any)
	if meJSON["handle"] != "ada" {
		t.Fatalf("me: %#v", me)
	}

	prof := callTool(t, mcp, "openid_public_profile", map[string]any{"handle": "ada"})
	if prof["status"] != float64(200) {
		t.Fatalf("profile: %#v", prof)
	}

	put := callTool(t, mcp, "openid_pod_put", map[string]any{
		"path": "ada/public/from-mcp.json", "body": `{"via":"mcp"}`,
		"content_type": "application/json", "token": token,
	})
	if code := int(put["status"].(float64)); code != 201 && code != 200 {
		t.Fatalf("put: %#v", put)
	}

	got := callTool(t, mcp, "openid_pod_get", map[string]any{"path": "ada/public/from-mcp.json", "token": token})
	gotDump, _ := json.Marshal(got)
	if !strings.Contains(string(gotDump), "mcp") {
		t.Fatalf("get: %#v", got)
	}

	listed := callTool(t, mcp, "openid_pod_list", map[string]any{"path": "ada/public", "token": token})
	if !strings.Contains(listed["text"].(string), "from-mcp.json") {
		t.Fatalf("list: %#v", listed)
	}

	cc := callTool(t, mcp, "openid_client_credentials", map[string]any{"token": token, "name": "grokbot"})
	ccJSON := cc["json"].(map[string]any)
	cid, _ := ccJSON["id"].(string)
	csec, _ := ccJSON["secret"].(string)
	if cid == "" || csec == "" {
		t.Fatalf("cc: %#v", cc)
	}

	tok := callTool(t, mcp, "openid_token", map[string]any{"client_id": cid, "client_secret": csec})
	tokJSON := tok["json"].(map[string]any)
	ctoken, _ := tokJSON["access_token"].(string)
	if ctoken == "" {
		t.Fatalf("token: %#v", tok)
	}

	agent := callTool(t, mcp, "openid_register_agent", map[string]any{"name": "Grokbot"})
	agentJSON := agent["json"].(map[string]any)
	atoken, _ := agentJSON["token"].(string)
	agentObj, _ := agentJSON["agent"].(map[string]any)
	apod, _ := agentObj["podPath"].(string)
	if atoken == "" || apod == "" {
		t.Fatalf("agent: %#v", agent)
	}

	agents := callTool(t, mcp, "openid_list_agents", nil)
	if agents["json"] == nil {
		t.Fatalf("list agents: %#v", agents)
	}

	_ = callTool(t, mcp, "openid_pod_put", map[string]any{
		"path": apod + "inbox/job.json", "body": `{"job":1}`,
		"content_type": "application/json", "token": atoken,
	})

	flush := callTool(t, mcp, "openid_audit_flush", nil)
	flushJSON := flush["json"].(map[string]any)
	if flushJSON["merkleRoot"] == nil {
		t.Fatalf("flush: %#v", flush)
	}

	events := callTool(t, mcp, "openid_audit_events", nil)
	evs, _ := events["json"].([]any)
	if len(evs) == 0 {
		t.Fatal("no audit events")
	}
	eid := evs[0].(map[string]any)["id"].(string)
	ver := callTool(t, mcp, "openid_audit_verify", map[string]any{"id": eid})
	verJSON := ver["json"].(map[string]any)
	if verJSON["verified"] != true {
		t.Fatalf("verify: %#v", ver)
	}

	del := callTool(t, mcp, "openid_pod_delete", map[string]any{"path": "ada/public/from-mcp.json", "token": token})
	if int(del["status"].(float64)) != 204 {
		t.Fatalf("delete: %#v", del)
	}

	if msg := callToolErr(t, mcp, "openid_login", map[string]any{"handle": "ada", "password": "nope"}); !strings.Contains(msg, "401") {
		t.Fatalf("wrong password: %s", msg)
	}
	if msg := callToolErr(t, mcp, "openid_pod_get", nil); !strings.Contains(msg, "path") {
		t.Fatalf("missing path: %s", msg)
	}
	if msg := callToolErr(t, mcp, "does_not_exist", nil); !strings.Contains(msg, "unknown tool") {
		t.Fatalf("unknown tool: %s", msg)
	}
}

func TestHTTPMCPOpenableAndCall(t *testing.T) {
	_, mcp := startOpenID(t)
	ts := httptest.NewServer(mcp.Handler())
	t.Cleanup(ts.Close)

	get, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(get.Body)
	get.Body.Close()
	if get.StatusCode != 200 || !bytes.Contains(body, []byte(`"open"`)) {
		t.Fatalf("GET /mcp: %d %s", get.StatusCode, body)
	}

	opt, err := http.NewRequest(http.MethodOptions, ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	oresp, err := http.DefaultClient.Do(opt)
	if err != nil {
		t.Fatal(err)
	}
	oresp.Body.Close()
	if oresp.StatusCode != 204 {
		t.Fatalf("OPTIONS %d", oresp.StatusCode)
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"grokbot"}}}`
	presp, err := http.Post(ts.URL, "application/json", strings.NewReader(initBody))
	if err != nil {
		t.Fatal(err)
	}
	pbody, _ := io.ReadAll(presp.Body)
	presp.Body.Close()
	if presp.StatusCode != 200 || !bytes.Contains(pbody, []byte(`"openid"`)) {
		t.Fatalf("initialize: %d %s", presp.StatusCode, pbody)
	}

	sseReq, _ := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	sseReq.Header.Set("Content-Type", "application/json")
	sseReq.Header.Set("Accept", "text/event-stream")
	sresp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatal(err)
	}
	sbody, _ := io.ReadAll(sresp.Body)
	sresp.Body.Close()
	if sresp.StatusCode != 200 || !bytes.Contains(sbody, []byte("event: message")) || !bytes.Contains(sbody, []byte("openid_open")) {
		t.Fatalf("sse tools/list: %d %s", sresp.StatusCode, sbody)
	}

	nresp, err := http.Post(ts.URL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil {
		t.Fatal(err)
	}
	nresp.Body.Close()
	if nresp.StatusCode != 202 {
		t.Fatalf("notification %d", nresp.StatusCode)
	}
}

func TestStdioGrokbotClient(t *testing.T) {
	_, mcp := startOpenID(t)
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"grokbot"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"openid_open","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"openid_register_agent","arguments":{"name":"stdio-bot"}}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := mcp.ServeStdio(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("stdio lines %d: %s", len(lines), out.String())
	}
	if !strings.Contains(out.String(), `"openid"`) {
		t.Fatalf("missing server name: %s", out.String())
	}
	if !strings.Contains(out.String(), "openid_register_agent") {
		t.Fatalf("missing tools: %s", out.String())
	}
	if !strings.Contains(out.String(), `"stdio-bot"`) && !strings.Contains(out.String(), "agent-") {
		t.Fatalf("agent register missing: %s", out.String())
	}
}

func TestMountedMCPOnSolidServer(t *testing.T) {
	ts, _ := startOpenID(t)

	resp, err := http.Get(ts.URL + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Contains(body, []byte(`"open"`)) {
		t.Fatalf("mounted GET /mcp: %d %s", resp.StatusCode, body)
	}

	st, err := http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	sbody, _ := io.ReadAll(st.Body)
	st.Body.Close()
	if !bytes.Contains(sbody, []byte(`"mcp"`)) {
		t.Fatalf("status missing mcp: %s", sbody)
	}

	htmlReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	htmlReq.Header.Set("Accept", "text/html")
	html, err := http.DefaultClient.Do(htmlReq)
	if err != nil {
		t.Fatal(err)
	}
	hbody, _ := io.ReadAll(html.Body)
	html.Body.Close()
	if html.StatusCode != 200 || !bytes.Contains(hbody, []byte("Solid server")) {
		t.Fatalf("dashboard: %d %s", html.StatusCode, hbody[:min(200, len(hbody))])
	}
}

func TestSparkConversationTools(t *testing.T) {
	ts, mcp := startOpenID(t)
	reg := callTool(t, mcp, "openid_register", map[string]any{
		"handle": "spark", "password": "testpass123", "name": "Spark User",
	})
	token := reg["json"].(map[string]any)["token"].(string)
	saved := callTool(t, mcp, "spark_save_conversation", map[string]any{
		"token": token,
		"title": "From Spark",
		"messages": []any{
			map[string]any{"role": "user", "text": "hello from spark"},
			map[string]any{"role": "assistant", "text": "saved in your pod"},
		},
	})
	id := sparkSaveID(saved)
	if id == "" {
		t.Fatalf("save: %#v", saved)
	}
	if saved["confirmation"] == nil && saved["resourceUrl"] == nil {
		if inner, ok := saved["json"].(map[string]any); ok {
			saved = inner
		}
	}
	listed := callTool(t, mcp, "spark_list_conversations", map[string]any{"token": token})
	listDump, _ := json.Marshal(listed)
	if !strings.Contains(string(listDump), "From Spark") {
		t.Fatalf("list: %#v", listed)
	}
	got := callTool(t, mcp, "spark_get_conversation", map[string]any{"id": id, "token": token})
	if !strings.Contains(fmtJSON(got), "hello from spark") {
		t.Fatalf("get: %#v", got)
	}
	shared := callTool(t, mcp, "spark_share_conversation", map[string]any{"id": id, "token": token})
	shareObj := shared["json"].(map[string]any)
	share := shareObj["share"].(map[string]any)
	url, _ := share["url"].(string)
	if url == "" {
		t.Fatalf("share url: %#v", shared)
	}
	pub, err := http.Get(strings.Replace(url, ts.URL, ts.URL, 1))
	if err != nil {
		// share URL uses BaseURL from the solid server (ts.URL)
		pub, err = http.Get(url)
	}
	if err != nil {
		t.Fatal(err)
	}
	pbody, _ := io.ReadAll(pub.Body)
	pub.Body.Close()
	if pub.StatusCode != 200 || !bytes.Contains(pbody, []byte("hello from spark")) {
		t.Fatalf("public share %d %s", pub.StatusCode, pbody)
	}
	_ = callTool(t, mcp, "spark_unshare_conversation", map[string]any{"id": id, "token": token})
	gone, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	gone.Body.Close()
	if gone.StatusCode != 404 && gone.StatusCode != 401 {
		t.Fatalf("unshare status %d", gone.StatusCode)
	}
}

func fmtJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func sparkSaveID(saved map[string]any) string {
	if id, _ := saved["id"].(string); id != "" {
		return id
	}
	if inner, ok := saved["json"].(map[string]any); ok {
		id, _ := inner["id"].(string)
		return id
	}
	return ""
}

func TestSparkSaveTimestampsAndRDF(t *testing.T) {
	ts, mcp := startOpenID(t)
	reg := callTool(t, mcp, "openid_register", map[string]any{
		"handle": "timed", "password": "testpass123", "name": "Timed User",
	})
	token := reg["json"].(map[string]any)["token"].(string)
	userTS := "2026-09-01T20:15:30+10:00"
	assistTS := "2026-09-01T10:16:00Z"
	saved := callTool(t, mcp, "spark_save_conversation", map[string]any{
		"token": token,
		"title": "Timed Spark",
		"messages": []any{
			map[string]any{"role": "user", "content": "save this thread", "timestamp": userTS},
			map[string]any{"role": "assistant", "text": "uploaded to your pod", "timestamp": assistTS},
		},
	})
	id := sparkSaveID(saved)
	if id == "" {
		t.Fatalf("save id missing: %#v", saved)
	}
	resourceURL, _ := saved["resourceUrl"].(string)
	webID, _ := saved["webId"].(string)
	confirm, _ := saved["confirmation"].(string)
	if resourceURL == "" || webID == "" || confirm == "" {
		t.Fatalf("spark save result missing fields: %#v", saved)
	}
	if !strings.Contains(resourceURL, "conversations/spark/") || !strings.HasSuffix(resourceURL, ".json") {
		t.Fatalf("resourceUrl %s", resourceURL)
	}
	if saved["created"] == nil && saved["modified"] == nil {
		t.Fatalf("created/modified missing: %#v", saved)
	}

	jsonReq, _ := http.NewRequest(http.MethodGet, resourceURL, nil)
	jsonReq.Header.Set("Authorization", "Bearer "+token)
	jsonReq.Header.Set("Accept", "application/ld+json, application/json")
	jsonResp, err := ts.Client().Do(jsonReq)
	if err != nil {
		t.Fatal(err)
	}
	jbody, _ := io.ReadAll(jsonResp.Body)
	jsonResp.Body.Close()
	if jsonResp.StatusCode != 200 {
		t.Fatalf("GET json %d %s", jsonResp.StatusCode, jbody)
	}
	js := string(jbody)
	for _, needle := range []string{"dateCreated", "dateModified", `"created"`, "gemini-spark", "save this thread", "2026-09-01T10:16:00Z"} {
		if !strings.Contains(js, needle) {
			t.Fatalf("json missing %s\n%s", needle, js)
		}
	}
	if !strings.Contains(js, "2026-09-01T10:15:30Z") && !strings.Contains(js, "2026-09-01T20:15:30+10:00") {
		t.Fatalf("json missing user timestamp\n%s", js)
	}

	ttlURL := strings.TrimSuffix(resourceURL, ".json") + ".ttl"
	ttlReq, _ := http.NewRequest(http.MethodGet, ttlURL, nil)
	ttlReq.Header.Set("Authorization", "Bearer "+token)
	ttlReq.Header.Set("Accept", "text/turtle")
	ttlResp, err := ts.Client().Do(ttlReq)
	if err != nil {
		t.Fatal(err)
	}
	tbody, _ := io.ReadAll(ttlResp.Body)
	ttlResp.Body.Close()
	if ttlResp.StatusCode != 200 {
		t.Fatalf("GET ttl %d %s", ttlResp.StatusCode, tbody)
	}
	ttl := string(tbody)
	if !strings.Contains(ttl, "purl.org/dc/terms/created") && !strings.Contains(ttl, "dcterms:created") {
		t.Fatalf("ttl missing created\n%s", ttl)
	}
	if !strings.Contains(ttl, "schema.org/Conversation") && !strings.Contains(ttl, "schema:Conversation") {
		t.Fatalf("ttl missing Conversation type\n%s", ttl)
	}
	if !strings.Contains(ttl, "schema.org/Message") && !strings.Contains(ttl, "schema:Message") {
		t.Fatalf("ttl missing Message type\n%s", ttl)
	}
	if !strings.Contains(ttl, "gemini-spark") {
		t.Fatalf("ttl missing source\n%s", ttl)
	}
	if !strings.Contains(ttl, "2026-09-01T10:15:30Z") && !strings.Contains(ttl, "2026-09-01T20:15:30+10:00") {
		t.Fatalf("ttl missing message timestamp\n%s", ttl)
	}

	// Regression: LDP POST to the saved JSON document must fail with the container error.
	postReq, _ := http.NewRequest(http.MethodPost, resourceURL, strings.NewReader(`{}`))
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")
	postResp, err := ts.Client().Do(postReq)
	if err != nil {
		t.Fatal(err)
	}
	pbody, _ := io.ReadAll(postResp.Body)
	postResp.Body.Close()
	if postResp.StatusCode != http.StatusBadRequest || !bytes.Contains(pbody, []byte("Can only POST to containers")) {
		t.Fatalf("POST to document want 400 container error, got %d %s", postResp.StatusCode, pbody)
	}

	listed := mustRPC(t, mcp, "tools/list", 1, nil)
	dump, _ := json.Marshal(listed)
	if !bytes.Contains(dump, []byte("When the user asks to save")) {
		t.Fatalf("tool description should tell Spark to call this tool: %s", dump)
	}
}

func mcpHTTPCall(t *testing.T, ts *httptest.Server, name string, args map[string]any, bearer string) (map[string]any, bool, string) {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var rpc map[string]any
	if err := json.Unmarshal(raw, &rpc); err != nil {
		t.Fatalf("rpc %s", raw)
	}
	result, _ := rpc["result"].(map[string]any)
	isErr, _ := result["isError"].(bool)
	content, _ := result["content"].([]any)
	text := ""
	if len(content) > 0 {
		item, _ := content[0].(map[string]any)
		text, _ = item["text"].(string)
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(text), &parsed)
	return parsed, isErr, text
}

func TestSparkConnectTokenMintSaveRevokeAndIsolation(t *testing.T) {
	ts, _ := startOpenID(t)
	reg := httptest.NewRequest(http.MethodPost, "/idp/register", strings.NewReader(
		`{"handle":"alice","password":"testpass123","name":"Alice","createPod":true}`))
	reg.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	ts.Config.Handler.ServeHTTP(regRec, reg)
	if regRec.Code != 200 {
		t.Fatalf("alice register %d %s", regRec.Code, regRec.Body.String())
	}
	var alice map[string]any
	_ = json.Unmarshal(regRec.Body.Bytes(), &alice)
	aliceTok, _ := alice["token"].(string)

	mintReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/idp/spark-token", strings.NewReader(`{}`))
	mintReq.Header.Set("Authorization", "Bearer "+aliceTok)
	mintReq.Header.Set("Content-Type", "application/json")
	mintResp, err := ts.Client().Do(mintReq)
	if err != nil {
		t.Fatal(err)
	}
	mintRaw, _ := io.ReadAll(mintResp.Body)
	mintResp.Body.Close()
	if mintResp.StatusCode != 200 {
		t.Fatalf("mint %d %s", mintResp.StatusCode, mintRaw)
	}
	var minted map[string]any
	_ = json.Unmarshal(mintRaw, &minted)
	sparkTok, _ := minted["token"].(string)
	if sparkTok == "" || minted["aud"] != "spark-mcp" || minted["scope"] != "spark" {
		t.Fatalf("mint body %s", mintRaw)
	}

	saved, isErr, text := mcpHTTPCall(t, ts, "spark_save_conversation", map[string]any{
		"title": "Connect token thread",
		"messages": []any{
			map[string]any{"role": "user", "content": "save via spark token", "timestamp": "2026-09-01T12:00:00Z"},
			map[string]any{"role": "assistant", "text": "stored"},
		},
	}, sparkTok)
	if isErr || sparkSaveID(saved) == "" {
		t.Fatalf("spark save with connect token: err=%v %s %#v", isErr, text, saved)
	}
	if saved["webId"] == nil || saved["resourceUrl"] == nil {
		t.Fatalf("save result %#v", saved)
	}

	deniedMint, _ := http.NewRequest(http.MethodPost, ts.URL+"/idp/spark-token", strings.NewReader(`{}`))
	deniedMint.Header.Set("Authorization", "Bearer "+sparkTok)
	deniedMint.Header.Set("Content-Type", "application/json")
	deniedMintResp, err := ts.Client().Do(deniedMint)
	if err != nil {
		t.Fatal(err)
	}
	deniedMintResp.Body.Close()
	if deniedMintResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("active spark token must not mint another: %d", deniedMintResp.StatusCode)
	}

	_, isErr, text = mcpHTTPCall(t, ts, "openid_pod_put", map[string]any{
		"path": "alice/inbox/nope.json", "body": "{}", "content_type": "application/json",
	}, sparkTok)
	if !isErr || !strings.Contains(text, "spark connect token cannot call") {
		t.Fatalf("spark token must not call pod_put: %v %s", isErr, text)
	}

	regB := httptest.NewRequest(http.MethodPost, "/idp/register", strings.NewReader(
		`{"handle":"bob","password":"testpass123","name":"Bob","createPod":true}`))
	regB.Header.Set("Content-Type", "application/json")
	bobRec := httptest.NewRecorder()
	ts.Config.Handler.ServeHTTP(bobRec, regB)
	if bobRec.Code != 200 {
		t.Fatalf("bob register %d %s", bobRec.Code, bobRec.Body.String())
	}

	evil, _ := http.NewRequest(http.MethodPut, ts.URL+"/bob/conversations/spark/stolen.json", strings.NewReader(`{"no":true}`))
	evil.Header.Set("Authorization", "Bearer "+sparkTok)
	evil.Header.Set("Content-Type", "application/json")
	evilResp, err := ts.Client().Do(evil)
	if err != nil {
		t.Fatal(err)
	}
	evilBody, _ := io.ReadAll(evilResp.Body)
	evilResp.Body.Close()
	if evilResp.StatusCode != http.StatusForbidden {
		t.Fatalf("alice spark token writing bob pod want 403, got %d %s", evilResp.StatusCode, evilBody)
	}

	revReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/idp/spark-token", nil)
	revReq.Header.Set("Authorization", "Bearer "+aliceTok)
	revResp, err := ts.Client().Do(revReq)
	if err != nil {
		t.Fatal(err)
	}
	revRaw, _ := io.ReadAll(revResp.Body)
	revResp.Body.Close()
	if revResp.StatusCode != 200 || !bytes.Contains(revRaw, []byte(`"revoked"`)) {
		t.Fatalf("revoke %d %s", revResp.StatusCode, revRaw)
	}

	_, isErr, text = mcpHTTPCall(t, ts, "spark_save_conversation", map[string]any{
		"title":    "after revoke",
		"messages": []any{map[string]any{"role": "user", "text": "should fail"}},
	}, sparkTok)
	if !isErr || !strings.Contains(text, "revoked") {
		t.Fatalf("after revoke want failure, got err=%v %s", isErr, text)
	}

	sparkMint, _ := http.NewRequest(http.MethodPost, ts.URL+"/idp/spark-token", strings.NewReader(`{}`))
	sparkMint.Header.Set("Authorization", "Bearer "+sparkTok)
	sparkMint.Header.Set("Content-Type", "application/json")
	denied, err := ts.Client().Do(sparkMint)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked spark token minting another want 401, got %d", denied.StatusCode)
	}
}

// TestSparkSaveLDPFallbackCatchesContainerPOSTBug stands up identity+LDP only
// (no /conversations HTTP API). POST /conversations hits LDP and returns
// "Can only POST to containers". spark_save_conversation must still persist
// JSON-LD + Turtle via container PUTs.
func TestSparkSaveLDPFallbackCatchesContainerPOSTBug(t *testing.T) {
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := resourcestore.New(fs, dir)
	tokens := authn.NewTokenService("test-secret")
	var idp *identityapi.Service
	ldp := &solid.LDPHandler{
		Store:  store,
		WAC:    wac.NewChecker(store),
		Tokens: tokens,
		Logger: logging.NewBasicLogger(logging.Error),
	}
	mux := http.NewServeMux()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	idp = identityapi.New(store, tokens, ts.URL)
	ldp.BaseURL = ts.URL
	idp.Routes(mux)
	mux.HandleFunc("/", ldp.ServeHTTP)

	mcp := openidmcp.New(ts.URL)
	mcp.HTTP = ts.Client()

	reg := httptest.NewRequest(http.MethodPost, "/idp/register", strings.NewReader(
		`{"handle":"ldpuser","password":"testpass123","name":"LDP","createPod":true}`))
	reg.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	mux.ServeHTTP(regRec, reg)
	if regRec.Code != 200 {
		t.Fatalf("register %d %s", regRec.Code, regRec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(regRec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	token, _ := out["token"].(string)
	if token == "" {
		t.Fatalf("no token: %s", regRec.Body.String())
	}

	// Prove the bug still exists on this server: POST /conversations is LDP.
	bugReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/conversations", strings.NewReader(`{"title":"x"}`))
	bugReq.Header.Set("Authorization", "Bearer "+token)
	bugReq.Header.Set("Content-Type", "application/json")
	bugResp, err := ts.Client().Do(bugReq)
	if err != nil {
		t.Fatal(err)
	}
	bugBody, _ := io.ReadAll(bugResp.Body)
	bugResp.Body.Close()
	if bugResp.StatusCode != http.StatusBadRequest || !bytes.Contains(bugBody, []byte("Can only POST to containers")) {
		t.Fatalf("setup: expected LDP container POST bug, got %d %s", bugResp.StatusCode, bugBody)
	}

	saved := callTool(t, mcp, "spark_save_conversation", map[string]any{
		"token": token,
		"title": "Fallback thread",
		"messages": []any{
			map[string]any{"role": "user", "content": "upload this", "timestamp": "2026-01-02T03:04:05Z"},
			map[string]any{"role": "assistant", "text": "saved via LDP"},
		},
	})
	resourceURL, _ := saved["resourceUrl"].(string)
	if resourceURL == "" || !strings.Contains(resourceURL, "/ldpuser/conversations/spark/") {
		t.Fatalf("fallback save: %#v", saved)
	}
	if saved["webId"] == nil || saved["confirmation"] == nil {
		t.Fatalf("fallback result: %#v", saved)
	}

	got, err := ts.Client().Get(resourceURL)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(got.Body)
	got.Body.Close()
	if got.StatusCode != 200 || !bytes.Contains(raw, []byte("2026-01-02T03:04:05Z")) || !bytes.Contains(raw, []byte("gemini-spark")) {
		t.Fatalf("GET json %d %s", got.StatusCode, raw)
	}
	ttlURL := strings.TrimSuffix(resourceURL, ".json") + ".ttl"
	ttlResp, err := ts.Client().Get(ttlURL)
	if err != nil {
		t.Fatal(err)
	}
	ttl, _ := io.ReadAll(ttlResp.Body)
	ttlResp.Body.Close()
	if ttlResp.StatusCode != 200 || !bytes.Contains(ttl, []byte("dcterms:created")) {
		t.Fatalf("GET ttl %d %s", ttlResp.StatusCode, ttl)
	}
}
