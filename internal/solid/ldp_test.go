package solid

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"solid-go/internal/authn"
	"solid-go/internal/identityapi"
	"solid-go/internal/logging"
	"solid-go/internal/resourcestore"
	"solid-go/internal/storage"
	"solid-go/internal/wac"
)

func TestPOSTOnlyToContainers(t *testing.T) {
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := resourcestore.New(fs, dir)
	tokens := authn.NewTokenService("test-secret")
	idp := identityapi.New(store, tokens, "http://localhost")
	h := &LDPHandler{
		Store:   store,
		WAC:     wac.NewChecker(store),
		Tokens:  tokens,
		BaseURL: "http://localhost",
		Logger:  logging.NewBasicLogger(logging.Error),
	}

	req := httptest.NewRequest(http.MethodPost, "/idp/register", strings.NewReader(
		`{"handle":"ada","password":"testpass123","name":"Ada","createPod":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	idp.Routes(mux)
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("register %d %s", rec.Code, rec.Body.String())
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tok); err != nil || tok.Token == "" {
		t.Fatalf("token: %s", rec.Body.String())
	}

	ctx := context.Background()
	if _, err := store.Put(ctx, "ada/conversations/spark/doc.json", "application/json", []byte(`{"ok":true}`), "", ""); err != nil {
		t.Fatal(err)
	}

	postDoc := httptest.NewRequest(http.MethodPost, "/ada/conversations/spark/doc.json", strings.NewReader(`{}`))
	postDoc.Header.Set("Authorization", "Bearer "+tok.Token)
	postDoc.Header.Set("Content-Type", "application/json")
	docRec := httptest.NewRecorder()
	h.ServeHTTP(docRec, postDoc)
	if docRec.Code != http.StatusBadRequest {
		t.Fatalf("POST to document want 400, got %d %s", docRec.Code, docRec.Body.String())
	}
	if !strings.Contains(docRec.Body.String(), "Can only POST to containers") {
		t.Fatalf("expected container POST error, got %s", docRec.Body.String())
	}

	if err := store.EnsureContainer(ctx, "ada/conversations/spark/"); err != nil {
		t.Fatal(err)
	}
	postDir := httptest.NewRequest(http.MethodPost, "/ada/conversations/spark/", strings.NewReader(`{"ok":true}`))
	postDir.Header.Set("Authorization", "Bearer "+tok.Token)
	postDir.Header.Set("Content-Type", "application/json")
	postDir.Header.Set("Slug", "posted")
	dirRec := httptest.NewRecorder()
	h.ServeHTTP(dirRec, postDir)
	if dirRec.Code != http.StatusCreated {
		t.Fatalf("POST to container want 201, got %d %s", dirRec.Code, dirRec.Body.String())
	}
}
