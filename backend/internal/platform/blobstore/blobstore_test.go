package blobstore

import (
	"context"
	"io"
	"os"
	"path/filepath"
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
	alias := strings.Repeat("a", 64) + "/" + a.Key + ".css"
	if err := store.PublishAlias(ctx, a.Key, alias); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishAlias(ctx, a.Key, alias); err != nil {
		t.Fatal(err)
	}
	source, _ := os.Stat(filepath.Join(store.root, a.Key[:2], a.Key))
	target, _ := os.Stat(filepath.Join(store.root, "sites", filepath.FromSlash(alias)))
	if source == nil || target == nil || !os.SameFile(source, target) {
		t.Fatal("hierarchy must reuse the CAS inode")
	}
	for _, path := range []string{"../secret", strings.Repeat("a", 64) + "/../../secret", `C:\secret`} {
		if store.PublishAlias(ctx, a.Key, path) == nil {
			t.Fatal("unsafe alias accepted")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.Put(cancelled, strings.NewReader("test")); err == nil {
		t.Fatal("cancellation ignored")
	}
}
