package conversations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"solid-go/internal/authn"
	"solid-go/internal/identityapi"
	"solid-go/internal/resourcestore"
	"solid-go/internal/storage"
	"solid-go/internal/wac"
)

func testService(t *testing.T) (*Service, *authn.TokenService, *resourcestore.Store) {
	t.Helper()
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := resourcestore.New(fs, dir)
	tokens := authn.NewTokenService("test-secret")
	idp := identityapi.New(store, tokens, "http://localhost")
	svc := New(store, tokens, idp, "http://localhost")
	var audited []string
	svc.OnAudit = func(ctx context.Context, agent, method, path string, body []byte) {
		audited = append(audited, method+" "+path)
	}
	t.Cleanup(func() { _ = audited })
	return svc, tokens, store
}

func registerHuman(t *testing.T, idp *identityapi.Service) (token string, who *actor) {
	t.Helper()
	body := `{"handle":"ada","password":"testpass123","name":"Ada","createPod":true}`
	req := httptest.NewRequest(http.MethodPost, "/idp/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	idp.Routes(http.NewServeMux())
	// use the service method via HTTP mux
	mux := http.NewServeMux()
	idp.Routes(mux)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("register %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	tok, _ := out["token"].(string)
	acc := out["account"].(map[string]any)
	if tok == "" {
		t.Fatalf("no token: %s", rec.Body.String())
	}
	return tok, &actor{
		WebID:   acc["webId"].(string),
		PodPath: acc["podPath"].(string),
		Handle:  acc["handle"].(string),
	}
}

func TestSaveListShareACL(t *testing.T) {
	svc, _, store := testService(t)
	tok, act := registerHuman(t, svc.IDP)
	_ = tok
	ctx := context.Background()
	c, err := svc.Save(ctx, act, SaveInput{
		Title: "Train",
		Messages: []Message{
			{Role: "user", Text: "Should I take the train?"},
			{Role: "assistant", Text: "Yes."},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" || c.Resource == "" || c.Source != SourceGeminiSpark {
		t.Fatalf("saved %#v", c)
	}

	listed, err := svc.List(ctx, act)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list %v %#v", err, listed)
	}

	checker := wac.NewChecker(store)
	_, ok, err := checker.Allowed(ctx, c.Resource, "", wac.Mode{Read: true})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("public should not read an unshared conversation")
	}
	_, ok, err = checker.Allowed(ctx, c.Resource, act.WebID, wac.Mode{Read: true})
	if err != nil || !ok {
		t.Fatalf("owner read: %v ok=%v", err, ok)
	}

	shared, err := svc.Share(ctx, act, c.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if shared.Shared == nil || shared.Shared.URL == "" || shared.Shared.Public {
		t.Fatalf("share %#v", shared.Shared)
	}
	got, err := svc.PublicGet(ctx, shared.Shared.Token)
	if err != nil || got.Title != "Train" {
		t.Fatalf("public get by token: %v %#v", err, got)
	}
	if _, err := svc.PublicGet(ctx, c.ID); err == nil {
		t.Fatal("unlisted share must not resolve by conversation id")
	}

	pub, err := svc.Share(ctx, act, c.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Shared.Public {
		t.Fatal("expected public")
	}
	_, ok, err = checker.Allowed(ctx, c.Resource, "", wac.Mode{Read: true})
	if err != nil || !ok {
		t.Fatalf("public read after public share: %v ok=%v", err, ok)
	}
	if _, err := svc.PublicGet(ctx, c.ID); err != nil {
		t.Fatalf("public get by id: %v", err)
	}

	if err := svc.Unshare(ctx, act, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicGet(ctx, shared.Shared.Token); err != ErrNotFound {
		t.Fatalf("unshare want not found, got %v", err)
	}
	_, ok, err = checker.Allowed(ctx, c.Resource, "", wac.Mode{Read: true})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("public read should be gone after unshare")
	}
}

func TestHTTPSaveShareUnshare(t *testing.T) {
	svc, tokens, _ := testService(t)
	tok, _ := registerHuman(t, svc.IDP)
	mux := http.NewServeMux()
	svc.Routes(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	_ = tokens

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/conversations", strings.NewReader(`{
		"title":"From HTTP",
		"messages":[{"role":"user","text":"hello spark"},{"role":"assistant","text":"hi"}]
	}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("save %d", resp.StatusCode)
	}
	var saved Conversation
	_ = json.NewDecoder(resp.Body).Decode(&saved)
	resp.Body.Close()

	listReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/conversations", nil)
	listReq.Header.Set("Authorization", "Bearer "+tok)
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	var listed map[string][]Conversation
	_ = json.NewDecoder(listResp.Body).Decode(&listed)
	listResp.Body.Close()
	if listResp.StatusCode != 200 || len(listed["conversations"]) != 1 {
		t.Fatalf("list %d %#v", listResp.StatusCode, listed)
	}

	shareReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/conversations/"+saved.ID+"/share", strings.NewReader(`{"public":false}`))
	shareReq.Header.Set("Authorization", "Bearer "+tok)
	shareReq.Header.Set("Content-Type", "application/json")
	shareResp, err := http.DefaultClient.Do(shareReq)
	if err != nil {
		t.Fatal(err)
	}
	var shared Conversation
	_ = json.NewDecoder(shareResp.Body).Decode(&shared)
	shareResp.Body.Close()
	if shareResp.StatusCode != 200 || shared.Shared == nil {
		t.Fatalf("share %d", shareResp.StatusCode)
	}

	pub, err := http.Get(ts.URL + SharePrefix + shared.Shared.Token)
	if err != nil {
		t.Fatal(err)
	}
	if pub.StatusCode != 200 {
		t.Fatalf("share get %d", pub.StatusCode)
	}
	pub.Body.Close()

	unReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/conversations/"+saved.ID+"/unshare", strings.NewReader(`{}`))
	unReq.Header.Set("Authorization", "Bearer "+tok)
	unResp, err := http.DefaultClient.Do(unReq)
	if err != nil {
		t.Fatal(err)
	}
	unResp.Body.Close()
	if unResp.StatusCode != 200 {
		t.Fatalf("unshare %d", unResp.StatusCode)
	}
	gone, err := http.Get(ts.URL + SharePrefix + shared.Shared.Token)
	if err != nil {
		t.Fatal(err)
	}
	gone.Body.Close()
	if gone.StatusCode != 404 && gone.StatusCode != 401 {
		t.Fatalf("unshared share url want 404/401 got %d", gone.StatusCode)
	}
}

type redirectTransport struct{}

func (redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusFound,
		Header:     http.Header{"Location": []string{"https://accounts.google.com/signin"}},
		Body:       http.NoBody,
		Request:    req.Clone(req.Context()),
	}, nil
}

func TestImportShareNotPublic(t *testing.T) {
	svc, _, _ := testService(t)
	svc.HTTP = &http.Client{Transport: redirectTransport{}, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	_, err := svc.ImportShareURL(context.Background(), "https://g.co/gemini/share/nope")
	if err == nil {
		t.Fatal("expected honest failure")
	}
	if !strings.Contains(err.Error(), "paste") {
		t.Fatalf("error should tell user to paste: %v", err)
	}
}

func TestPodFromWebID(t *testing.T) {
	pod, handle := podFromWebID("http://localhost/ada/profile/card#me", "http://localhost")
	if pod != "ada/" || handle != "ada" {
		t.Fatalf("%s %s", pod, handle)
	}
}

func TestMetadataTurtle(t *testing.T) {
	svc, _, _ := testService(t)
	ts := time.Date(2026, 9, 1, 1, 2, 3, 0, time.UTC)
	ttl := svc.metadataTurtle(&Conversation{
		ID:       "x",
		Title:    "Hello",
		Owner:    "http://localhost/ada/profile/card#me",
		Resource: "ada/conversations/spark/x.json",
		Created:  time.Unix(0, 0).UTC(),
		Updated:  time.Unix(0, 0).UTC(),
		Source:   SourceGeminiSpark,
		Messages: []Message{{Role: "user", Text: "hi", Timestamp: &ts}},
	})
	if !strings.Contains(ttl, "Hello") || (!strings.Contains(ttl, "schema:Conversation") && !strings.Contains(ttl, "schema.org/Conversation")) {
		t.Fatalf("ttl %s", ttl)
	}
	if !strings.Contains(ttl, "2026-09-01T01:02:03Z") {
		t.Fatalf("message timestamp missing from ttl: %s", ttl)
	}
	if !strings.Contains(ttl, "XMLSchema#dateTime") && !strings.Contains(ttl, "xsd:dateTime") {
		t.Fatalf("xsd:dateTime missing: %s", ttl)
	}
}

func TestSaveTimestampsJSONAndTurtle(t *testing.T) {
	svc, _, store := testService(t)
	_, act := registerHuman(t, svc.IDP)
	msgTime := time.Date(2026, 3, 15, 9, 30, 0, 0, time.FixedZone("AEST", 10*3600))
	c, err := svc.Save(context.Background(), act, SaveInput{
		Title: "Timed",
		Messages: []Message{
			{Role: "user", Text: "morning", Timestamp: &msgTime},
			{Role: "assistant", Content: "noted"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Created.IsZero() || c.Updated.IsZero() || c.DateCreated == "" || c.DateModified == "" {
		t.Fatalf("conversation times: %#v", c)
	}
	if c.Messages[0].Timestamp == nil || !c.Messages[0].Timestamp.UTC().Equal(msgTime.UTC()) {
		t.Fatalf("message timestamp %#v", c.Messages[0].Timestamp)
	}
	if !strings.HasSuffix(c.Resource, ".json") || !strings.Contains(c.Resource, "conversations/spark/") {
		t.Fatalf("resource path %s", c.Resource)
	}

	got, err := store.Get(context.Background(), c.Resource)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got.Body)
	if !strings.Contains(body, `"created"`) || !strings.Contains(body, `"dateCreated"`) || !strings.Contains(body, `"source": "gemini-spark"`) {
		t.Fatalf("json missing time/source fields\n%s", body)
	}
	if !strings.Contains(body, "2026-03-15T09:30:00+10:00") && !strings.Contains(body, "2026-03-14T23:30:00Z") {
		t.Fatalf("json missing message timestamp\n%s", body)
	}

	ttlRes, err := store.Get(context.Background(), svc.metaPath(act.PodPath, c.ID))
	if err != nil {
		t.Fatal(err)
	}
	ttl := string(ttlRes.Body)
	for _, needle := range []string{"purl.org/dc/terms/created", "purl.org/dc/terms/modified", "schema.org/dateCreated", "gemini-spark", act.WebID} {
		if !strings.Contains(ttl, needle) && !strings.Contains(ttl, strings.ReplaceAll(needle, "purl.org/dc/terms/", "dcterms:")) {
			t.Fatalf("ttl missing %s\n%s", needle, ttl)
		}
	}
	if !strings.Contains(ttl, "2026-03-14T23:30:00Z") && !strings.Contains(ttl, "2026-03-15T09:30:00+10:00") {
		t.Fatalf("ttl missing message timestamp\n%s", ttl)
	}

	dir, err := store.Get(context.Background(), act.PodPath+"conversations/")
	if err != nil || !dir.IsContainer {
		t.Fatalf("conversations/ container: %v %#v", err, dir)
	}
	spark, err := store.Get(context.Background(), act.PodPath+"conversations/spark/")
	if err != nil || !spark.IsContainer {
		t.Fatalf("conversations/spark/ container: %v %#v", err, spark)
	}

	result := svc.ResultOf(c)
	if result.ResourceURL == "" || result.WebID != act.WebID || result.Confirmation == "" {
		t.Fatalf("result %#v", result)
	}
}
