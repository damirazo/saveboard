//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
	"saveboard/internal/identity"
	platformdb "saveboard/internal/platform/db"
)

func TestRegistrationAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("registration_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
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
	h, _ := New(Dependencies{Auth: auth})
	call := func(path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", path, bytes.NewReader(data)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("want %d got %d: %s", status, w.Code, w.Body.String())
		}
	}
	body := map[string]string{"username": "Alice", "email": " ALICE@example.com ", "password": "secure-password"}
	w := call("/api/v1/auth/register", body)
	expect(w, 201)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("tokens must not be cached")
	}
	var tokens identity.Tokens
	if err = json.Unmarshal(w.Body.Bytes(), &tokens); err != nil {
		t.Fatal(err)
	}
	principal, err := auth.Authenticate(ctx, tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	var hash, email, status string
	var verified bool
	if err = db.DB.QueryRowContext(ctx, "SELECT password_hash,email,status,email_verified_at IS NOT NULL FROM users WHERE id=$1", principal.UserID).Scan(&hash, &email, &status, &verified); err != nil {
		t.Fatal(err)
	}
	if email != "alice@example.com" || status != "active" || verified || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body["password"])) != nil {
		t.Fatal("incorrect account state")
	}
	for _, login := range []string{"alice", "ALICE@EXAMPLE.COM"} {
		expect(call("/api/v1/auth/login", map[string]string{"login": login, "password": body["password"]}), 200)
	}
	expect(call("/api/v1/auth/login", map[string]string{"login": "alice", "password": "wrong"}), 401)
	expect(call("/api/v1/auth/register", map[string]string{"username": "ALICE", "email": "other@example.com", "password": body["password"]}), 409)
	expect(call("/api/v1/auth/register", map[string]string{"username": "other", "email": "ALICE@example.com", "password": body["password"]}), 409)
	expect(call("/api/v1/auth/register", map[string]string{"username": "bad@user", "email": "bad@example.com", "password": body["password"]}), 422)
	expect(call("/api/v1/auth/register", map[string]string{"username": "valid", "email": "no-email", "password": body["password"]}), 422)
	var count int
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
		t.Fatal("failed requests left accounts", count, err)
	}
	// Concurrent registrations race at the database uniqueness boundary, not a preflight check.
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses <- call("/api/v1/auth/register", map[string]string{"username": "race", "email": "race@example.com", "password": body["password"]}).Code
		}()
	}
	wg.Wait()
	close(statuses)
	got := map[int]int{}
	for status := range statuses {
		got[status]++
	}
	if got[201] != 1 || got[409] != 1 {
		t.Fatalf("race statuses: %v", got)
	}
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM sessions s JOIN users u ON u.id=s.user_id WHERE u.username='race'").Scan(&count); err != nil || count != 1 {
		t.Fatal("unexpected sessions", count, err)
	}
	if _, err = auth.Refresh(ctx, tokens.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.Revoke(ctx, tokens.AccessToken, false); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.Authenticate(ctx, tokens.AccessToken); err == nil {
		t.Fatal("logout did not revoke")
	}
}
