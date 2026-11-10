package conversations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func twoMsgs() []Message {
	return []Message{{Role: "user", Text: "hi"}, {Role: "assistant", Text: "hello"}}
}

func TestSaveRejectsUnsafeIDs(t *testing.T) {
	svc, _, store := testService(t)
	_, act := registerHuman(t, svc.IDP)
	ctx := context.Background()
	for _, id := range []string{"../../escape", "a/b", "..", ".hidden", "x.json", "id with space", strings.Repeat("a", 129)} {
		_, err := svc.Save(ctx, act, SaveInput{ID: id, Messages: twoMsgs()})
		if !errors.Is(err, ErrInvalidID) {
			t.Errorf("Save(id=%q) err = %v, want ErrInvalidID", id, err)
		}
	}
	if _, err := store.Get(ctx, "escape.json"); err == nil {
		t.Fatal("traversal id wrote outside the pod")
	}
	if _, err := svc.Get(ctx, act, "../../escape"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get traversal id err = %v, want ErrNotFound", err)
	}
}

func TestSaveRejectsUnsafeSources(t *testing.T) {
	svc, _, _ := testService(t)
	_, act := registerHuman(t, svc.IDP)
	for _, src := range []string{"../x", "a/b", "Claude Code", ".acl"} {
		_, err := svc.Save(context.Background(), act, SaveInput{Source: src, Messages: twoMsgs()})
		if !errors.Is(err, ErrInvalidSource) {
			t.Errorf("Save(source=%q) err = %v, want ErrInvalidSource", src, err)
		}
	}
}

func TestSourcesGetOwnContainersAndFilter(t *testing.T) {
	svc, _, store := testService(t)
	_, act := registerHuman(t, svc.IDP)
	ctx := context.Background()

	spark, err := svc.Save(ctx, act, SaveInput{ID: "s1", Messages: twoMsgs()})
	if err != nil {
		t.Fatal(err)
	}
	if want := act.PodPath + "conversations/spark/s1.json"; spark.Resource != want {
		t.Fatalf("spark resource %q, want %q", spark.Resource, want)
	}
	claude, err := svc.Save(ctx, act, SaveInput{ID: "claude-1", Source: "claude-code", Messages: twoMsgs()})
	if err != nil {
		t.Fatal(err)
	}
	if want := act.PodPath + "conversations/claude-code/claude-1.json"; claude.Resource != want {
		t.Fatalf("claude resource %q, want %q", claude.Resource, want)
	}
	if _, err := store.Get(ctx, act.PodPath+"conversations/claude-code/claude-1.ttl"); err != nil {
		t.Fatalf("turtle sidecar missing: %v", err)
	}

	all, err := svc.List(ctx, act, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("List all = %d, %v; want 2", len(all), err)
	}
	only, err := svc.List(ctx, act, "claude-code")
	if err != nil || len(only) != 1 || only[0].ID != "claude-1" {
		t.Fatalf("List claude-code = %#v, %v", only, err)
	}
	if _, err := svc.List(ctx, act, "../x"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("List bad source err = %v", err)
	}

	got, err := svc.Get(ctx, act, "claude-1")
	if err != nil || got.Source != "claude-code" {
		t.Fatalf("Get claude-1 = %#v, %v", got, err)
	}

	// Re-saving an existing id updates it in place, even under another source.
	again, err := svc.Save(ctx, act, SaveInput{ID: "claude-1", Source: "codex", Messages: twoMsgs()})
	if err != nil {
		t.Fatal(err)
	}
	if again.Resource != claude.Resource || !again.Created.Equal(claude.Created) {
		t.Fatalf("resave moved or reset: %q created %v", again.Resource, again.Created)
	}

	// Share/unshare works outside spark/.
	if _, err := svc.Share(ctx, act, "claude-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, act.PodPath+"conversations/claude-code/claude-1.share.json"); err != nil {
		t.Fatalf("share record not beside conversation: %v", err)
	}
	if err := svc.Unshare(ctx, act, "claude-1"); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPListBadSource(t *testing.T) {
	svc, _, _ := testService(t)
	tok, _ := registerHuman(t, svc.IDP)
	mux := http.NewServeMux()
	svc.Routes(mux)
	req := httptest.NewRequest(http.MethodGet, "/conversations?source=..%2Fx", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}
