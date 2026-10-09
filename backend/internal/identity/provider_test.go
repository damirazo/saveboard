package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderBackchannelPreservesPublicIssuer(t *testing.T) {
	const issuer = "https://public.example/identity/realms/test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity/realms/test/.well-known/openid-configuration" {
			t.Errorf("incorrect backchannel path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": issuer + "/auth", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys"})
	}))
	defer server.Close()
	resolve := ProviderResolver(issuer, "client", "secret", "https://app.example/auth/callback", server.URL)
	p, err := resolve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.OAuth.Endpoint.AuthURL != issuer+"/auth" || p.OAuth.Endpoint.TokenURL != issuer+"/token" {
		t.Fatal("backchannel changed public endpoints")
	}
	server.Close()
	if cached, err := resolve(context.Background()); err != nil || cached != p {
		t.Fatal("provider was not cached")
	}
}
