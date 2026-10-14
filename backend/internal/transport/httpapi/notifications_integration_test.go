//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
	"net/http/httptest"
	"saveboard/internal/identity"
	"saveboard/internal/notifications"
	platformdb "saveboard/internal/platform/db"
	"strings"
	"testing"
	"time"
)

func TestNotificationsAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("notifications"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
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
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	add := func(name string) (string, string) {
		t.Helper()
		id := uuid.NewString()
		if _, e := db.DB.ExecContext(ctx, `INSERT INTO users(id,username,display_name,status,password_hash) VALUES($1,$2,$2,'active',$3)`, id, name, string(hash)); e != nil {
			t.Fatal(e)
		}
		token, e := auth.Login(ctx, name, "test-password")
		if e != nil {
			t.Fatal(e)
		}
		return id, token.AccessToken
	}
	owner, token := add("notify-owner")
	_, other := add("notify-other")
	service := &notifications.Service{DB: db.DB}
	h, _ := New(Dependencies{Auth: auth, Notifications: service})
	call := func(method, path, bearer string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil).WithContext(ctx)
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	publish := func(recipient *string, key string) string {
		t.Helper()
		tx, e := db.DB.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		e = notifications.PublishTx(ctx, tx, notifications.Event{RecipientID: recipient, Title: "Title", Text: "Plain <script> text", Importance: "important", Key: &key})
		if e != nil {
			t.Fatal(e)
		}
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
		var id string
		if e = db.DB.QueryRowContext(ctx, `SELECT id FROM notifications WHERE event_key=$1`, key).Scan(&id); e != nil {
			t.Fatal(e)
		}
		return id
	}
	old := publish(nil, "old")
	if _, err = db.DB.ExecContext(ctx, `UPDATE notifications SET created_at=(SELECT created_at FROM users WHERE id=$1) WHERE id=$2`, owner, old); err != nil {
		t.Fatal(err)
	}
	private := publish(&owner, "private")
	broadcast := publish(nil, "all")
	publish(&owner, "private")
	call("GET", "/api/v1/notifications", "", 401)
	var page notifications.NotificationPage
	if err = json.Unmarshal(call("GET", "/api/v1/notifications?page=1&size=1", token, 200).Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 1 {
		t.Fatalf("pagination/filter/dedup: %+v", page)
	}
	call("GET", "/api/v1/notifications/"+old, token, 404)
	call("GET", "/api/v1/notifications/"+private, other, 404)
	call("POST", "/api/v1/notifications/"+private+"/read", other, 404)
	call("GET", "/api/v1/notifications/"+broadcast, token, 200)
	for i := 0; i < 2; i++ {
		call("POST", "/api/v1/notifications/"+broadcast+"/read", token, 204)
	}
	var count NotificationCountOutput
	if err = json.Unmarshal(call("GET", "/api/v1/notifications/unread", token, 200).Body.Bytes(), &count.Body); err != nil || count.Body.Count != 1 {
		t.Fatalf("unread %v %+v", err, count)
	}
	if err = json.Unmarshal(call("GET", "/api/v1/notifications/unread", other, 200).Body.Bytes(), &count.Body); err != nil || count.Body.Count != 1 {
		t.Fatalf("broadcast read leaked %v %+v", err, count)
	}
	n, err := service.Get(ctx, owner, broadcast)
	if err != nil || !n.Read {
		t.Fatal("read not persisted", err)
	}
	// A new account cannot see earlier broadcasts.
	_, newcomer := add("notify-new")
	call("GET", "/api/v1/notifications/"+broadcast, newcomer, 404)
	if _, err = db.DB.ExecContext(ctx, `UPDATE users SET status='disabled',disabled_at=now() WHERE id=$1`, owner); err != nil {
		t.Fatal(err)
	}
	call("GET", "/api/v1/notifications", token, 401)
}
