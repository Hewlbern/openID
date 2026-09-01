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

func TestOwnerOnlyAndPublicReadACL(t *testing.T) {
	dir := t.TempDir()
	fs, _ := storage.NewFileStorage(dir)
	store := resourcestore.New(fs, dir)
	ctx := context.Background()
	_ = store.EnsureContainer(ctx, "pod/conversations/spark/")
	owner := "http://localhost/pod/profile/card#me"
	resource := "pod/conversations/spark/c.json"
	_, err := store.Put(ctx, resource, "application/json", []byte(`{"id":"c"}`), "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Put(ctx, resource+".acl", "text/turtle", []byte(OwnerOnlyACL("http://localhost/"+resource, owner)), "", "")
	if err != nil {
		t.Fatal(err)
	}
	c := NewChecker(store)
	_, ok, err := c.Allowed(ctx, resource, "", Mode{Read: true})
	if err != nil || ok {
		t.Fatalf("owner-only should deny public read: ok=%v err=%v", ok, err)
	}
	_, ok, err = c.Allowed(ctx, resource, owner, Mode{Write: true})
	if err != nil || !ok {
		t.Fatalf("owner write: ok=%v err=%v", ok, err)
	}
	_, err = store.Put(ctx, resource+".acl", "text/turtle", []byte(PublicReadACL("http://localhost/"+resource, owner)), "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err = c.Allowed(ctx, resource, "", Mode{Read: true})
	if err != nil || !ok {
		t.Fatalf("public read after share ACL: ok=%v err=%v", ok, err)
	}
	_, ok, err = c.Allowed(ctx, resource, "", Mode{Write: true})
	if err != nil || ok {
		t.Fatalf("public write should fail: ok=%v", ok)
	}
}
