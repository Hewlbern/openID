package solid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"solid-go/internal/authn"
	"solid-go/internal/logging"
	"solid-go/internal/resourcestore"
	"solid-go/internal/storage"
	"solid-go/internal/wac"
)

func TestCleanAndInternalPaths(t *testing.T) {
	if !IsInternalServerPath("/.openid/accounts.json") {
		t.Fatal("accounts.json")
	}
	if !IsInternalServerPath("foo/../.openid/local-auth.json") {
		t.Fatal("traversal")
	}
	if !IsInternalServerPath("/.OpenID/accounts.json") {
		t.Fatal("case")
	}
	if !IsInternalServerPath("/.openid/") {
		t.Fatal("container")
	}
	if IsInternalServerPath("/ada/.openid/spark-grants.json") {
		t.Fatal("pod-local .openid must stay addressable")
	}
	if IsInternalServerPath("/ada/profile/card") {
		t.Fatal("profile")
	}
	if got := CleanResourcePath("/ada/conversations/spark/"); got != "ada/conversations/spark/" {
		t.Fatalf("trailing slash: %q", got)
	}
}

func TestLDPHidesOpenIDState(t *testing.T) {
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := resourcestore.New(fs, dir)
	ctx := context.Background()
	secret := []byte(`{"password":"plaintext-should-not-leak","secret":"agent-key-should-not-leak"}`)
	if _, err := store.Put(ctx, ".openid/accounts.json", "application/json", secret, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(ctx, ".openid/local-auth.json", "application/json", secret, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(ctx, "ada/profile/card", "text/turtle", []byte("public profile"), "", ""); err != nil {
		t.Fatal(err)
	}
	tokens := authn.NewTokenService("test-secret")
	tok, err := tokens.Issue("http://localhost/ada/profile/card#me", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &LDPHandler{
		Store:   store,
		WAC:     wac.NewChecker(store),
		Tokens:  tokens,
		BaseURL: "http://localhost",
		Logger:  logging.NewBasicLogger(logging.Error),
	}
	blocked := []string{
		"GET /.openid/accounts.json",
		"GET /.openid/local-auth.json",
		"GET /.openid/",
		"HEAD /.openid/accounts.json",
		"PUT /.openid/accounts.json",
		"POST /.openid/",
		"DELETE /.openid/accounts.json",
		"PATCH /.openid/accounts.json",
		"OPTIONS /.openid/accounts.json",
	}
	for _, c := range blocked {
		method, p, _ := strings.Cut(c, " ")
		req := httptest.NewRequest(method, p, strings.NewReader("x"))
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: got %d body %s", c, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "plaintext") || strings.Contains(rec.Body.String(), "agent-key") {
			t.Fatalf("%s leaked body", c)
		}
	}
	trav := httptest.NewRequest(http.MethodGet, "/ada/profile/card", nil)
	trav.URL.Path = "/ada/../.openid/accounts.json"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, trav)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("traversal %d %s", rec.Code, rec.Body.String())
	}

	pub := httptest.NewRequest(http.MethodGet, "/ada/profile/card", nil)
	pubRec := httptest.NewRecorder()
	h.ServeHTTP(pubRec, pub)
	if pubRec.Code != http.StatusOK || !strings.Contains(pubRec.Body.String(), "public profile") {
		t.Fatalf("public profile %d %s", pubRec.Code, pubRec.Body.String())
	}

	root := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	h.ServeHTTP(rootRec, root)
	if strings.Contains(rootRec.Body.String(), "accounts.json") || strings.Contains(rootRec.Body.String(), ".openid") || strings.Contains(rootRec.Body.String(), "plaintext") {
		t.Fatalf("root listing leaked internal state: %s", rootRec.Body.String())
	}
}
