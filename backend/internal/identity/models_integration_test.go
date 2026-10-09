//go:build integration

package identity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	platformdb "saveboard/internal/platform/db"
	"saveboard/migrations"
)

func TestIdentitySchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("identity_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Log(err)
		}
	})
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db.DB, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	legacyID := uuid.New()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO users(id, display_name, disabled_at) VALUES ($1, 'Legacy', now())`, legacyID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	var legacy User
	if err := db.NewSelect().Model(&legacy).Where("id = ?", legacyID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if legacy.Status != UserDisabled || legacy.Username != "user_"+strings.ReplaceAll(legacyID.String(), "-", "") || legacy.PasswordHash != nil {
		t.Fatalf("incorrect backfill: %+v", legacy)
	}

	email := "Alice@example.com"
	u := User{Username: "Alice", DisplayName: "Alice", Email: &email}
	if _, err := db.NewInsert().Model(&u).Returning("*").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if u.ID == uuid.Nil || u.CreatedAt.IsZero() || u.Status != UserActive {
		t.Fatalf("missing defaults: %+v", u)
	}
	for _, query := range []string{
		`INSERT INTO users(username, display_name) VALUES ('alice', 'Duplicate')`,
		`INSERT INTO users(username, display_name, email) VALUES ('other', 'Duplicate', 'alice@EXAMPLE.COM')`,
		`INSERT INTO users(username, display_name) VALUES ('alice@example.com', 'Ambiguous')`,
		`INSERT INTO users(username, display_name, status) VALUES ('badstatus', 'Invalid', 'unknown')`,
		`INSERT INTO users(username, display_name, status) VALUES ('disabled', 'Invalid', 'disabled')`,
	} {
		if _, err := db.DB.ExecContext(ctx, query); err == nil {
			t.Fatalf("accepted invalid data: %s", query)
		}
	}
	before := u.UpdatedAt
	if _, err := db.DB.ExecContext(ctx, `UPDATE users SET display_name = 'Updated' WHERE id = $1`, u.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&u).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if !u.UpdatedAt.After(before) {
		t.Fatal("updated_at did not advance")
	}

	account := OAuthIdentity{UserID: u.ID, Issuer: "https://issuer.example", Subject: "subject-1", Scopes: []string{"openid", "email"}, AccessTokenEncrypted: []byte("test ciphertext")}
	if _, err := db.NewInsert().Model(&account).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewInsert().Model(&account).Exec(ctx); err == nil {
		t.Fatal("duplicate OAuth identity accepted")
	}
	if err := db.NewSelect().Model(&account).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if len(account.Scopes) != 2 || string(account.AccessTokenEncrypted) != "test ciphertext" {
		t.Fatal("OAuth fields did not round trip")
	}
	expires := time.Now().Add(time.Hour)
	ip := "127.0.0.1"
	session := Session{UserID: u.ID, TokenHash: make([]byte, 32), ExpiresAt: expires, IPAddress: &ip}
	if _, err := db.NewInsert().Model(&session).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&session).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	flow := OAuthFlow{StateHash: make([]byte, 32), BrowserTokenHash: make([]byte, 32), Issuer: account.Issuer, NonceHash: make([]byte, 32), PKCEVerifierEncrypted: []byte("test ciphertext"), RedirectURI: "https://app.example/callback", ExpiresAt: expires}
	if _, err := db.NewInsert().Model(&flow).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&flow).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if flow.ReturnPath != "/" {
		t.Fatal("missing return path default")
	}
	for _, path := range []string{"//evil.example", "https://evil.example", `/\evil.example`} {
		if _, err := db.DB.ExecContext(ctx, `UPDATE oauth_flows SET return_path = $1`, path); err == nil {
			t.Fatalf("accepted unsafe return path %q", path)
		}
	}
	token := UserToken{UserID: u.ID, TokenHash: make([]byte, 32), Purpose: TokenEmailVerification, Email: &email, ExpiresAt: expires}
	if _, err := db.NewInsert().Model(&token).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&token).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE user_tokens SET token_hash = decode('00', 'hex')`); err == nil {
		t.Fatal("invalid token hash accepted")
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE user_tokens SET expires_at = created_at`); err == nil {
		t.Fatal("invalid expiry accepted")
	}
	if _, err := p.DownTo(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
}
