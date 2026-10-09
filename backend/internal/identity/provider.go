package identity

import (
	"context"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Provider struct {
	OAuth    oauth2.Config
	Verifier *oidc.IDTokenVerifier
	Client   *http.Client
}

// Discover validates ID tokens using the issuer's advertised keys and client ID.
func Discover(ctx context.Context, issuer, clientID, secret, redirectURL string) (*Provider, error) {
	if issuer == "" || clientID == "" || secret == "" || redirectURL == "" {
		return nil, fmt.Errorf("incomplete OIDC configuration")
	}
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	client, _ := ctx.Value(oauth2.HTTPClient).(*http.Client)
	return &Provider{OAuth: oauth2.Config{ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, Endpoint: p.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, Verifier: p.Verifier(&oidc.Config{ClientID: clientID}), Client: client}, nil
}

// Lazy discovery avoids a startup dependency cycle (nginx waits for API health).
// Backchannel rewrites transport only; public issuer/audience verification stays intact.
func ProviderResolver(issuer, clientID, secret, redirectURL, backchannel string) func(context.Context) (*Provider, error) {
	var mu sync.Mutex
	var cached *Provider
	return func(ctx context.Context) (*Provider, error) {
		mu.Lock()
		defer mu.Unlock()
		if cached != nil {
			return cached, nil
		}
		client := &http.Client{Timeout: 10 * time.Second}
		if backchannel != "" {
			public, err := url.Parse(issuer)
			if err != nil {
				return nil, err
			}
			internal, err := url.Parse(backchannel)
			if err != nil || internal.Host == "" || (internal.Scheme != "http" && internal.Scheme != "https") {
				return nil, fmt.Errorf("invalid OIDC backchannel URL")
			}
			client.Transport = issuerTransport{public: public, internal: internal}
		}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
		p, err := Discover(ctx, issuer, clientID, secret, redirectURL)
		if err == nil {
			cached = p
		}
		return p, err
	}
}

type issuerTransport struct{ public, internal *url.URL }

func (t issuerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == t.public.Scheme && req.URL.Host == t.public.Host {
		req = req.Clone(req.Context())
		u := *req.URL
		u.Scheme, u.Host = t.internal.Scheme, t.internal.Host
		req.URL = &u
	}
	return http.DefaultTransport.RoundTrip(req)
}
