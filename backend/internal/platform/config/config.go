package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	ServiceName        string
	AuthSigningKey     []byte
	AuthEncryptionKey  []byte
	PublicURL          string
	SecureCookies      bool
	OIDCIssuer         string
	OIDCClientID       string
	OIDCClientSecret   string
	OIDCBackchannelURL string
}

func Load(service string) (Config, error) {
	c := Config{HTTPAddr: env("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), ServiceName: service}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL connection URL")
	}
	if service == "api" {
		c.AuthSigningKey, err = hex.DecodeString(os.Getenv("AUTH_JWT_KEY"))
		if err != nil || len(c.AuthSigningKey) != 32 {
			return c, fmt.Errorf("AUTH_JWT_KEY must contain 32 random bytes encoded as 64 hex characters")
		}
		c.AuthEncryptionKey, err = hex.DecodeString(os.Getenv("AUTH_ENCRYPTION_KEY"))
		if err != nil || len(c.AuthEncryptionKey) != 32 {
			return c, fmt.Errorf("AUTH_ENCRYPTION_KEY must contain 32 random bytes encoded as 64 hex characters")
		}
		if string(c.AuthSigningKey) == string(c.AuthEncryptionKey) {
			return c, fmt.Errorf("auth signing and encryption keys must be different")
		}
		c.PublicURL = strings.TrimRight(env("AUTH_PUBLIC_URL", "http://localhost:8080"), "/")
		public, err := url.Parse(c.PublicURL)
		if err != nil || public.Host == "" || public.User != nil || public.RawQuery != "" || public.Fragment != "" || public.Path != "" ||
			(public.Scheme != "https" && !(public.Scheme == "http" && (public.Hostname() == "localhost" || public.Hostname() == "127.0.0.1" || public.Hostname() == "::1"))) {
			return c, fmt.Errorf("AUTH_PUBLIC_URL must be an HTTPS origin (HTTP allowed only on loopback)")
		}
		c.SecureCookies = public.Scheme == "https"
		c.OIDCIssuer, c.OIDCClientID, c.OIDCClientSecret = os.Getenv("OIDC_ISSUER"), os.Getenv("OIDC_CLIENT_ID"), os.Getenv("OIDC_CLIENT_SECRET")
		c.OIDCBackchannelURL = os.Getenv("OIDC_BACKCHANNEL_URL")
		if c.OIDCIssuer != "" {
			u, err := url.Parse(c.OIDCIssuer)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || c.OIDCClientID == "" || c.OIDCClientSecret == "" {
				return c, fmt.Errorf("OIDC requires a valid issuer URL, client ID and secret")
			}
		}
		if c.OIDCBackchannelURL != "" {
			u, err := url.Parse(c.OIDCBackchannelURL)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				return c, fmt.Errorf("OIDC_BACKCHANNEL_URL must be an HTTP(S) origin")
			}
		}
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
