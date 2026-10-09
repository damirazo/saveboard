package config

import (
	"strings"
	"testing"
)

func TestAuthConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/saveboard")
	t.Setenv("AUTH_JWT_KEY", strings.Repeat("ab", 32))
	t.Setenv("AUTH_ENCRYPTION_KEY", strings.Repeat("cd", 32))
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_BACKCHANNEL_URL", "")
	for _, origin := range []string{"http://localhost:8080", "https://saveboard.example"} {
		t.Setenv("AUTH_PUBLIC_URL", origin)
		cfg, err := Load("api")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SecureCookies != strings.HasPrefix(origin, "https:") {
			t.Fatal("incorrect cookie security")
		}
	}
	for _, origin := range []string{"http://saveboard.example", "https://user:pass@example.com", "https://example.com/path", "https://example.com?query=1"} {
		t.Setenv("AUTH_PUBLIC_URL", origin)
		if _, err := Load("api"); err == nil {
			t.Fatalf("unsafe origin accepted: %s", origin)
		}
	}
	t.Setenv("AUTH_PUBLIC_URL", "https://saveboard.example")
	t.Setenv("AUTH_JWT_KEY", "")
	if _, err := Load("api"); err == nil {
		t.Fatal("missing signing key accepted")
	}
	if _, err := Load("worker"); err != nil {
		t.Fatal("worker unexpectedly requires auth keys")
	}
}
