package audit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"solid-go/internal/ipfs"
	"solid-go/internal/ots"
	"solid-go/internal/resourcestore"
	"solid-go/internal/storage"
)

func TestMerkleAndOTS(t *testing.T) {
	dir := t.TempDir()
	fs, err := storage.NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := resourcestore.New(fs, dir)
	log := New(store, ipfs.New(""), ots.New(), time.Hour)

	ctx := context.Background()
	e1, err := log.Append(ctx, "https://ex/agent#me", "PUT", "x.txt", []byte("a"), nil)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := log.Append(ctx, "https://ex/agent#me", "PUT", "y.txt", []byte("b"), nil)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := log.FlushBatch(ctx)
	if err != nil || batch == nil {
		t.Fatalf("batch: %v %#v", err, batch)
	}
	if batch.OTS == nil || batch.OTS.Digest == "" {
		t.Fatal("expected OTS proof")
	}
	if batch.IPFSCID == "" {
		t.Fatal("expected IPFS CID")
	}
	v, err := log.VerifyEvent(ctx, e1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v["verified"] != true {
		t.Fatalf("expected verified: %#v", v)
	}
	v2, err := log.VerifyEvent(ctx, e2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v2["merkleIncluded"] != true {
		t.Fatalf("e2 not included: %#v", v2)
	}
	// events persisted on disk
	if ok, _ := store.Exists(ctx, filepath.ToSlash("audit/events/"+e1.ID+".json")); !ok {
		t.Fatal("event not stored")
	}
}
