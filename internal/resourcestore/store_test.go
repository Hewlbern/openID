package resourcestore

import (
	"context"
	"testing"

	"solid-go/internal/storage"
)

func TestCRUD(t *testing.T) {
	dir := t.TempDir()
	fs, _ := storage.NewFileStorage(dir)
	s := New(fs, dir)
	ctx := context.Background()

	res, err := s.Put(ctx, "pods/doc.txt", "text/plain", []byte("hi"), "", "*")
	if err != nil {
		t.Fatal(err)
	}
	if res.ETag == "" {
		t.Fatal("etag")
	}
	got, err := s.Get(ctx, "pods/doc.txt")
	if err != nil || string(got.Body) != "hi" {
		t.Fatalf("get: %v %v", err, got)
	}
	children, err := s.List(ctx, "pods/")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range children {
		if c == "pods/doc.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("children: %v", children)
	}
	posted, err := s.Post(ctx, "pods/", "slug1", "text/plain", []byte("p"), false)
	if err != nil || posted.Path != "pods/slug1" {
		t.Fatalf("post: %v %#v", err, posted)
	}
	if err := s.Delete(ctx, "pods/doc.txt", ""); err != nil {
		t.Fatal(err)
	}
}
