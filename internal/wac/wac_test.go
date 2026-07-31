package wac

import (
	"context"
	"testing"

	"solid-go/internal/resourcestore"
	"solid-go/internal/storage"
)

func TestACLEvaluation(t *testing.T) {
	dir := t.TempDir()
	fs, _ := storage.NewFileStorage(dir)
	store := resourcestore.New(fs, dir)
	ctx := context.Background()
	_ = store.EnsureContainer(ctx, "pod/")
	owner := "http://localhost/pod/profile/card#me"
	acl := DefaultPublicACL("http://localhost/pod/", owner)
	_, err := store.Put(ctx, "pod/.acl", "text/turtle", []byte(acl), "", "")
	if err != nil {
		t.Fatal(err)
	}
	c := NewChecker(store)
	modes, ok, err := c.Allowed(ctx, "pod/secret.txt", owner, Mode{Write: true})
	if err != nil || !ok {
		t.Fatalf("owner write: %v ok=%v modes=%+v", err, ok, modes)
	}
	_, ok, err = c.Allowed(ctx, "pod/secret.txt", "", Mode{Write: true})
	if err != nil || ok {
		t.Fatalf("public write should fail: ok=%v", ok)
	}
	_, ok, err = c.Allowed(ctx, "pod/secret.txt", "", Mode{Read: true})
	if err != nil || !ok {
		t.Fatalf("public read: ok=%v", ok)
	}
}
