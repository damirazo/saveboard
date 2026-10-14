package blobstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Info struct {
	Key  string
	Size int64
}
type Store interface {
	Put(context.Context, io.Reader) (Info, error)
	Get(context.Context, string) (io.ReadCloser, error)
	Stat(context.Context, string) (Info, error)
	Delete(context.Context, string) error
	PublishAlias(context.Context, string, string) error
}

type Local struct {
	root     string
	maxBytes int64
}

var validKey = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ReadOnly opens existing storage without creating directories; mount it read-only in API.
func ReadOnly(root string) *Local { return &Local{root: root} }

// PublishAlias preserves the salted site/path hierarchy without duplicating CAS bytes.
// Call only from the worker; callers never provide raw URL path components.
func (s *Local) PublishAlias(ctx context.Context, key, relative string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	parts := strings.Split(relative, "/")
	if len(parts) < 2 || len(parts) > 128 {
		return fmt.Errorf("invalid archive path")
	}
	component := regexp.MustCompile(`^[a-f0-9]{64}(\.(html|css))?$`)
	for _, part := range parts {
		if !component.MatchString(part) {
			return fmt.Errorf("invalid archive path")
		}
	}
	source, err := s.path(key)
	if err != nil {
		return err
	}
	target := filepath.Join(s.root, "sites", filepath.FromSlash(relative))
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	if err = os.Link(source, target); err != nil && !os.IsExist(err) {
		return err
	}
	return nil
}

func NewLocal(root string, maxBytes int64) (*Local, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("blob limit must be positive")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, ".staging"), 0700); err != nil {
		return nil, err
	}
	return &Local{root: abs, maxBytes: maxBytes}, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// The storage directory is private to the worker. Hard-link publication is
// atomic and never overwrites an existing immutable blob, even under races.
func (s *Local) Put(ctx context.Context, src io.Reader) (Info, error) {
	f, err := os.CreateTemp(filepath.Join(s.root, ".staging"), "blob-")
	if err != nil {
		return Info{}, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(contextReader{ctx, src}, s.maxBytes+1))
	if err != nil {
		return Info{}, err
	}
	if n > s.maxBytes {
		return Info{}, fmt.Errorf("blob exceeds size limit")
	}
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	if err := f.Sync(); err != nil {
		return Info{}, err
	}
	if err := f.Close(); err != nil {
		return Info{}, err
	}
	key := hex.EncodeToString(h.Sum(nil))
	path, _ := s.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Info{}, err
	}
	if err := os.Link(f.Name(), path); err != nil && !os.IsExist(err) {
		return Info{}, err
	}
	return Info{Key: key, Size: n}, nil
}
func (s *Local) path(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fmt.Errorf("invalid blob key")
	}
	return filepath.Join(s.root, key[:2], key), nil
}
func (s *Local) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s *Local) Stat(ctx context.Context, key string) (Info, error) {
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	path, err := s.path(key)
	if err != nil {
		return Info{}, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return Info{}, err
	}
	return Info{Key: key, Size: stat.Size()}, nil
}

// Delete is only for future reference-aware GC after its lease/grace checks.
func (s *Local) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
