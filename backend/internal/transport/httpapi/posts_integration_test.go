//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
	"saveboard/internal/identity"
	platformdb "saveboard/internal/platform/db"
	"saveboard/internal/posts"
)

func TestPostsAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("posts_api"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = platformdb.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	auth, err := identity.NewAuthService(db.DB, identity.AuthConfig{SigningKey: []byte(strings.Repeat("s", 32)), EncryptionKey: []byte(strings.Repeat("e", 32)), Issuer: "test", Audience: "test"})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := New(Dependencies{Auth: auth, Posts: posts.NewService(db.DB)})
	call := func(method, path, token string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: want %d got %d: %s", method, path, want, w.Code, w.Body.String())
		}
		return w
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	add := func(name string) (string, string) {
		t.Helper()
		id := uuid.NewString()
		if _, e := db.DB.ExecContext(ctx, "INSERT INTO users(id,username,display_name,status,password_hash) VALUES($1,$2,$2,'active',$3)", id, name, string(hash)); e != nil {
			t.Fatal(e)
		}
		tokens, e := auth.Login(ctx, name, "test-password")
		if e != nil {
			t.Fatal(e)
		}
		return id, tokens.AccessToken
	}
	owner, token := add("owner")
	_, other := add("other")
	draft := posts.Write{Title: "Заметка", Type: "note", Visibility: "public", Markdown: "# Hello\n**world**", Tags: []posts.TagInput{{Title: "Привет мир"}}, PhotoIDs: []string{}}
	call("POST", "/api/v1/posts", "", draft, 401)
	w := call("POST", "/api/v1/posts", token, draft, 200)
	var post posts.View
	if e := json.Unmarshal(w.Body.Bytes(), &post); e != nil {
		t.Fatal(e)
	}
	if post.ID == "" || len(post.Tags) != 1 || post.Tags[0].Name != "privet-mir" {
		t.Fatalf("bad post %+v", post)
	}
	path := "/api/v1/posts/" + post.ID
	call("PUT", path, other, draft, 403)
	call("DELETE", path, other, nil, 403)
	call("GET", path, "", nil, 200)
	bad := draft
	bad.Markdown = "<script>alert(1)</script>"
	call("PUT", path, token, bad, 422)
	bad = draft
	bad.Tags = nil
	call("POST", "/api/v1/posts", token, bad, 422)
	w = call("GET", "/api/v1/post-tags?q=%D0%9F%D1%80", token, nil, 200)
	if !strings.Contains(w.Body.String(), "privet-mir") {
		t.Fatal(w.Body.String())
	}
	draft.Visibility = "private"
	call("PUT", path, token, draft, 200)
	call("GET", path, "", nil, 404)
	call("GET", path, other, nil, 404)
	w = call("GET", "/api/v1/posts?scope=public", "", nil, 200)
	var page posts.Page
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if page.Total != 0 {
		t.Fatal("private leak")
	}
	call("GET", "/api/v1/posts?scope=mine", "", nil, 401)
	// Photo ownership and private delivery; invalid content is rejected independent of its extension.
	call("POST", "/api/v1/photos", token, map[string]string{"data": base64.StdEncoding.EncodeToString([]byte("<svg/>"))}, 422)
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	w = call("POST", "/api/v1/photos", token, map[string]string{"data": base64.StdEncoding.EncodeToString(pngData.Bytes())}, 200)
	var photo posts.PhotoView
	_ = json.Unmarshal(w.Body.Bytes(), &photo)
	call("GET", photo.URL, "", nil, 404)
	call("GET", photo.URL, other, nil, 404)
	w = call("GET", photo.URL, token, nil, 200)
	if w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), pngData.Bytes()) {
		t.Fatalf("invalid binary response: %s", w.Header())
	}
	album := posts.Write{Title: "Фото", Type: "photo", Visibility: "public", PhotoIDs: []string{photo.ID}, Tags: draft.Tags}
	call("POST", "/api/v1/posts", other, album, 422)
	w = call("POST", "/api/v1/posts", token, album, 200)
	var photoPost posts.View
	_ = json.Unmarshal(w.Body.Bytes(), &photoPost)
	call("GET", photo.URL, "", nil, 200)
	// The inclusive limit is valid; preserve all 32 attachment identities and order.
	for len(album.PhotoIDs) < 32 {
		w = call("POST", "/api/v1/photos", token, map[string]string{"data": base64.StdEncoding.EncodeToString(pngData.Bytes())}, 200)
		var next posts.PhotoView
		if err := json.Unmarshal(w.Body.Bytes(), &next); err != nil {
			t.Fatal(err)
		}
		album.PhotoIDs = append(album.PhotoIDs, next.ID)
	}
	w = call("PUT", "/api/v1/posts/"+photoPost.ID, token, album, 200)
	if err := json.Unmarshal(w.Body.Bytes(), &photoPost); err != nil {
		t.Fatal(err)
	}
	if len(photoPost.Photos) != 32 || photoPost.Photos[31].ID != album.PhotoIDs[31] {
		t.Fatal("32 photo boundary/order failed")
	}
	tooMany := album
	tooMany.PhotoIDs = make([]string, 33)
	for i := range tooMany.PhotoIDs {
		tooMany.PhotoIDs[i] = uuid.NewString()
	}
	call("POST", "/api/v1/posts", token, tooMany, 422)
	call("DELETE", "/api/v1/posts/"+photoPost.ID, token, nil, 204)
	call("GET", photo.URL, "", nil, 404)
	call("DELETE", path, token, nil, 204)
	w = call("GET", "/api/v1/posts?scope=mine", token, nil, 200)
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatal("draft in feed")
	}
	w = call("PUT", path, token, draft, 200)
	_ = json.Unmarshal(w.Body.Bytes(), &post)
	if post.Status != "draft" {
		t.Fatal("edit republished draft")
	}
	if _, err = db.DB.ExecContext(ctx, "UPDATE users SET status='disabled',disabled_at=now() WHERE id=$1", owner); err != nil {
		t.Fatal(err)
	}
	call("POST", "/api/v1/posts", token, draft, 401)
	call("PUT", path, token, draft, 401)
	call("DELETE", path, token, nil, 401)
	call("POST", "/api/v1/photos", token, map[string]string{"data": "AA=="}, 401)
	if _, err = db.DB.ExecContext(ctx, "UPDATE users SET status='pending',disabled_at=NULL WHERE id=$1", owner); err != nil {
		t.Fatal(err)
	}
	call("POST", "/api/v1/posts", token, draft, 401)
}
