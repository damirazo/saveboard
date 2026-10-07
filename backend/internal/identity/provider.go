package identity

import (
	"context"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Provider struct {
	OAuth    oauth2.Config
	Verifier *oidc.IDTokenVerifier
}

// Discovery/verification adapter only. Login, PKCE state, nonce, sessions and
// CSRF middleware must be implemented together before mounting auth handlers.
func Discover(ctx context.Context, issuer, clientID, secret, redirectURL string) (*Provider, error) {
	if issuer == "" || clientID == "" || secret == "" || redirectURL == "" {
		return nil, fmt.Errorf("incomplete OIDC configuration")
	}
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &Provider{OAuth: oauth2.Config{ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, Verifier: p.Verifier(&oidc.Config{ClientID: clientID})}, nil
}
