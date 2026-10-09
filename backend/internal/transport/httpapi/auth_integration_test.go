//go:build integration

package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/bcrypt"
	"saveboard/internal/identity"
	platformdb "saveboard/internal/platform/db"
)

func TestAuthAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("auth_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
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
	if err := platformdb.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	provider := newFakeOIDC(t)
	key := []byte(strings.Repeat("s", 32))
	svc, err := identity.NewAuthService(db.DB, identity.AuthConfig{
		SigningKey: key, EncryptionKey: []byte(strings.Repeat("e", 32)), Issuer: "https://app.example", Audience: "saveboard-api",
		OIDCIssuer:      provider.server.URL,
		ResolveProvider: identity.ProviderResolver(provider.server.URL, "test-client", "test-secret", "https://app.example/auth/callback", ""),
	})
	if err != nil {
		t.Fatal(err)
	}
	h, _ := New(Dependencies{Auth: svc, SecureCookies: true})
	password := "correct-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	addUser := func(name, status string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := db.DB.ExecContext(ctx, `INSERT INTO users(id, username, display_name, email, password_hash, status, disabled_at)
			VALUES($1,$2,$2,$3,$4,$5,CASE WHEN $5='disabled' THEN now() END)`, id, name, name+"@example.com", string(hash), status)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	call := func(method, path, body, access, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if access != "" {
			req.Header.Set("Authorization", "Bearer "+access)
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("expected %d, got %d: %s", status, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("auth response must not be cached")
		}
	}
	pair := func(w *httptest.ResponseRecorder) identity.Tokens {
		t.Helper()
		expect(w, 200)
		var p identity.Tokens
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.AccessToken == "" || len(p.RefreshToken) != 43 || p.SessionID == uuid.Nil || p.TokenType != "Bearer" {
			t.Fatal("incomplete token response")
		}
		return p
	}
	login := func(login string) identity.Tokens {
		t.Helper()
		return pair(call("POST", "/api/v1/auth/login", fmt.Sprintf(`{"login":%q,"password":%q}`, login, password), "", ""))
	}
	refresh := func(p identity.Tokens) *httptest.ResponseRecorder {
		return call("POST", "/api/v1/auth/refresh", fmt.Sprintf(`{"refresh_token":%q}`, p.RefreshToken), "", "")
	}
	me := func(p identity.Tokens) *httptest.ResponseRecorder {
		return call("GET", "/api/v1/auth/me", "", p.AccessToken, "")
	}
	expect(call("GET", "/api/v1/auth/me", "", "", ""), 401)
	expect(call("POST", "/api/v1/auth/logout", "", "", ""), 401)

	t.Run("credentials rotation and replay", func(t *testing.T) {
		id := addUser("Alice", "active")
		a := login("aLiCe")
		b := login("ALICE@EXAMPLE.COM")
		expect(me(a), 200)
		rotated := pair(refresh(a))
		if rotated.SessionID != a.SessionID || rotated.RefreshToken == a.RefreshToken || rotated.AccessToken == a.AccessToken {
			t.Fatal("rotation did not replace tokens in the same session")
		}
		expect(refresh(a), 401)
		expect(me(rotated), 401)
		expect(refresh(rotated), 401)
		expect(me(b), 200)
		var hash []byte
		if err := db.DB.QueryRowContext(ctx, `SELECT r.token_hash FROM refresh_tokens r WHERE session_id=$1 AND consumed_at IS NULL`, rotated.SessionID).Scan(&hash); err != nil {
			t.Fatal(err)
		}
		expectedHash := sha256.Sum256([]byte(rotated.RefreshToken))
		if string(hash) != string(expectedHash[:]) {
			t.Fatal("refresh token not stored as digest")
		}
		if _, err := db.DB.ExecContext(ctx, `UPDATE users SET status='disabled',disabled_at=now() WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		expect(me(b), 401)
		expect(refresh(b), 401)
	})

	t.Run("logout others and current", func(t *testing.T) {
		addUser("Bob", "active")
		addUser("Carol", "active")
		a, b, other := login("Bob"), login("Bob"), login("Carol")
		w := call("POST", "/api/v1/auth/logout-others", "", a.AccessToken, "")
		expect(w, 200)
		if !strings.Contains(w.Body.String(), `"revoked":1`) {
			t.Fatal("incorrect revoked count")
		}
		expect(me(a), 200)
		expect(me(b), 401)
		expect(refresh(b), 401)
		expect(me(other), 200)
		a = pair(refresh(a))
		expect(call("POST", "/api/v1/auth/logout", "", a.AccessToken, ""), 200)
		expect(me(a), 401)
		expect(refresh(a), 401)
		expect(call("POST", "/api/v1/auth/logout-others", "", a.AccessToken, ""), 401)
	})

	t.Run("simultaneous refresh revokes reused family", func(t *testing.T) {
		addUser("Race", "active")
		a := login("Race")
		results := make(chan *httptest.ResponseRecorder, 2)
		for range 2 {
			go func() { results <- refresh(a) }()
		}
		x, y := <-results, <-results
		if x.Code == 401 {
			x, y = y, x
		}
		rotated := pair(x)
		expect(y, 401)
		expect(me(rotated), 401)
	})

	t.Run("password failures and locked accounts", func(t *testing.T) {
		id := addUser("Locked", "active")
		for range 5 {
			expect(call("POST", "/api/v1/auth/login", `{"login":"Locked","password":"wrong"}`, "", ""), 401)
		}
		expect(call("POST", "/api/v1/auth/login", `{"login":"Locked","password":"correct-password"}`, "", ""), 401)
		var until sql.NullTime
		if err := db.DB.QueryRowContext(ctx, `SELECT locked_until FROM users WHERE id=$1`, id).Scan(&until); err != nil {
			t.Fatal(err)
		}
		if !until.Valid || until.Time.Before(time.Now()) {
			t.Fatal("account was not locked")
		}
		if _, err := db.DB.ExecContext(ctx, `UPDATE users SET locked_until=now()-interval '1 minute' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		login("Locked")
		for _, name := range []string{"Absent", "Pending", "Disabled"} {
			if name != "Absent" {
				addUser(name, strings.ToLower(name))
			}
			expect(call("POST", "/api/v1/auth/login", fmt.Sprintf(`{"login":%q,"password":"correct-password"}`, name), "", ""), 401)
		}
	})

	t.Run("JWT validation and expired sessions", func(t *testing.T) {
		addUser("Claims", "active")
		a := login("Claims")
		parsed, _, err := jwt.NewParser().ParseUnverified(a.AccessToken, jwt.MapClaims{})
		if err != nil {
			t.Fatal(err)
		}
		wrongSignature, err := jwt.NewWithClaims(jwt.SigningMethodHS256, parsed.Claims).SignedString([]byte(strings.Repeat("z", 32)))
		if err != nil {
			t.Fatal(err)
		}
		expect(call("GET", "/api/v1/auth/me", "", wrongSignature, ""), 401)
		for _, field := range []string{"iss", "aud", "exp", "token_use", "sub", "sid"} {
			claims := jwt.MapClaims{}
			for k, v := range parsed.Claims.(jwt.MapClaims) {
				claims[k] = v
			}
			claims[field] = "invalid"
			if field == "exp" {
				claims[field] = time.Now().Add(-time.Hour).Unix()
			}
			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			expect(call("GET", "/api/v1/auth/me", "", raw, ""), 401)
		}
		for _, method := range []jwt.SigningMethod{jwt.SigningMethodNone, jwt.SigningMethodHS384} {
			var signingKey any = key
			if method == jwt.SigningMethodNone {
				signingKey = jwt.UnsafeAllowNoneSignatureType
			}
			raw, err := jwt.NewWithClaims(method, parsed.Claims).SignedString(signingKey)
			if err != nil {
				t.Fatal(err)
			}
			expect(call("GET", "/api/v1/auth/me", "", raw, ""), 401)
		}
		if _, err := db.DB.ExecContext(ctx, `UPDATE sessions SET expires_at=now()-interval '1 second' WHERE id=$1`, a.SessionID); err != nil {
			t.Fatal(err)
		}
		expect(me(a), 401)
		expect(refresh(a), 401)
	})

	t.Run("OAuth PKCE browser binding nonce and replay", func(t *testing.T) {
		addUser("EmailMatch", "active")
		start := func() (*httptest.ResponseRecorder, string, string) {
			t.Helper()
			w := call("GET", "/auth/login", "", "", "")
			expect(w, 302)
			cookies := w.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
				t.Fatal("unsafe OAuth cookie")
			}
			u, err := url.Parse(w.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			if u.Query().Get("code_challenge_method") != "S256" || u.Query().Get("nonce") == "" {
				t.Fatal("PKCE/nonce missing")
			}
			return w, cookies[0].Name + "=" + cookies[0].Value, u.Query().Get("state")
		}
		w, cookie, state := start()
		code := provider.issue(t, w.Header().Get("Location"), nil)
		path := "/auth/callback?state=" + state + "&code=" + code
		expect(call("GET", path, "", "", ""), 401)
		expect(call("GET", path, "", "", "saveboard_oauth="+strings.Repeat("x", 43)), 401)
		response := call("GET", path, "", "", cookie)
		a := pair(response)
		if response.Result().Cookies()[0].MaxAge != -1 {
			t.Fatal("OAuth cookie not cleared")
		}
		expect(me(a), 200)
		expect(call("GET", path, "", "", cookie), 401)
		var linkedEmail sql.NullString
		if err := db.DB.QueryRowContext(ctx, `SELECT u.email FROM users u JOIN sessions s ON s.user_id=u.id WHERE s.id=$1`, a.SessionID).Scan(&linkedEmail); err != nil {
			t.Fatal(err)
		}
		if linkedEmail.Valid {
			t.Fatal("OAuth auto-linked a login email")
		}
		// Same issuer+subject logs into the same local user on subsequent attempts.
		w, cookie, state = start()
		code = provider.issue(t, w.Header().Get("Location"), nil)
		b := pair(call("GET", "/auth/callback?state="+state+"&code="+code, "", "", cookie))
		pa, _ := svc.Authenticate(ctx, a.AccessToken)
		pb, _ := svc.Authenticate(ctx, b.AccessToken)
		if pa.UserID != pb.UserID || pa.SessionID == pb.SessionID {
			t.Fatal("incorrect OAuth account/session reuse")
		}
		for _, field := range []string{"nonce", "iss", "aud", "exp", "azp", "at_hash"} {
			w, cookie, state = start()
			code = provider.issue(t, w.Header().Get("Location"), func(c jwt.MapClaims) {
				c[field] = "invalid"
				if field == "exp" {
					c[field] = time.Now().Add(-time.Hour).Unix()
				}
			})
			expect(call("GET", "/auth/callback?state="+state+"&code="+code, "", "", cookie), 401)
		}
		w, cookie, state = start()
		code = provider.issue(t, w.Header().Get("Location"), nil)
		if _, err := db.DB.ExecContext(ctx, `UPDATE oauth_flows SET expires_at=created_at+interval '1 microsecond' WHERE consumed_at IS NULL`); err != nil {
			t.Fatal(err)
		}
		expect(call("GET", "/auth/callback?state="+state+"&code="+code, "", "", cookie), 401)
	})
}

type fakeCode struct{ token, challenge string }
type fakeOIDC struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	mu     sync.Mutex
	codes  map[string]fakeCode
}

func newFakeOIDC(t *testing.T) *fakeOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeOIDC{key: key, codes: map[string]fakeCode{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": f.server.URL, "authorization_endpoint": f.server.URL + "/authorize", "token_endpoint": f.server.URL + "/token", "jwks_uri": f.server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			_ = r.ParseForm()
			client, secret, _ := r.BasicAuth()
			if client == "" {
				client, secret = r.Form.Get("client_id"), r.Form.Get("client_secret")
			}
			f.mu.Lock()
			code, exists := f.codes[r.Form.Get("code")]
			delete(f.codes, r.Form.Get("code"))
			f.mu.Unlock()
			challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !exists || client != "test-client" || secret != "test-secret" || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("redirect_uri") != "https://app.example/auth/callback" || base64.RawURLEncoding.EncodeToString(challenge[:]) != code.challenge {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "provider-access", "token_type": "Bearer", "expires_in": 300, "id_token": code.token})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeOIDC) issue(t *testing.T, authorizeURL string, mutate func(jwt.MapClaims)) string {
	t.Helper()
	u, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{"iss": f.server.URL, "aud": "test-client", "sub": "external-user", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "nonce": u.Query().Get("nonce"), "email": "EmailMatch@example.com", "email_verified": true, "name": "External User"}
	if mutate != nil {
		mutate(claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test"
	signed, err := token.SignedString(f.key)
	if err != nil {
		t.Fatal(err)
	}
	code := uuid.NewString()
	f.mu.Lock()
	f.codes[code] = fakeCode{token: signed, challenge: u.Query().Get("code_challenge")}
	f.mu.Unlock()
	return code
}
