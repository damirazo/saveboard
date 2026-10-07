package blobstore

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestImmutableBlobsAndLimits(t *testing.T) {
	store, err := NewLocal(t.TempDir(), 16)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a, err := store.Put(ctx, strings.NewReader("version one"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Put(ctx, strings.NewReader("version one"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := store.Put(ctx, strings.NewReader("version two"))
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a.Key == c.Key {
		t.Fatal("incorrect content addressing")
	}
	r, err := store.Get(ctx, a.Key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil || string(data) != "version one" {
		t.Fatal("old version changed")
	}
	if _, err := store.Put(ctx, strings.NewReader(strings.Repeat("x", 17))); err == nil {
		t.Fatal("size limit ignored")
	}
	if _, err := store.Get(ctx, "../secret"); err == nil {
		t.Fatal("path traversal accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.Put(cancelled, strings.NewReader("test")); err == nil {
		t.Fatal("cancellation ignored")
	}
}
