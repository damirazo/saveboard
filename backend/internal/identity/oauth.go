package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type OAuthStart struct {
	URL          string
	BrowserToken string
}

func (s *AuthService) StartOAuth(ctx context.Context) (OAuthStart, error) {
	if s.cfg.ResolveProvider == nil {
		return OAuthStart{}, ErrOAuthUnavailable
	}
	p, err := s.cfg.ResolveProvider(ctx)
	if err != nil {
		return OAuthStart{}, err
	}
	state, browser, nonce, verifier := randomToken(), randomToken(), randomToken(), oauth2.GenerateVerifier()
	iv := make([]byte, s.box.NonceSize())
	_, _ = rand.Read(iv)
	// Bind the ciphertext to this flow so ciphertext cannot be swapped between rows.
	encrypted := s.box.Seal(iv, iv, []byte(verifier), digest(state))
	_, err = s.db.ExecContext(ctx, `INSERT INTO oauth_flows(state_hash, browser_token_hash, issuer, nonce_hash,
		pkce_verifier_encrypted, redirect_uri, expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`,
		digest(state), digest(browser), s.cfg.OIDCIssuer, digest(nonce), encrypted, p.OAuth.RedirectURL, time.Now().Add(FlowLifetime))
	if err != nil {
		return OAuthStart{}, err
	}
	return OAuthStart{URL: p.OAuth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce)), BrowserToken: browser}, nil
}

func (s *AuthService) FinishOAuth(ctx context.Context, state, browser, code string) (Tokens, error) {
	if s.cfg.ResolveProvider == nil {
		return Tokens{}, ErrOAuthUnavailable
	}
	if !validToken(state) || !validToken(browser) || code == "" || len(code) > 8192 {
		return Tokens{}, ErrUnauthorized
	}
	p, err := s.cfg.ResolveProvider(ctx)
	if err != nil {
		return Tokens{}, err
	}
	var encrypted, nonceHash []byte
	// Consume before contacting the provider. A failed exchange requires a new flow.
	err = s.db.QueryRowContext(ctx, `UPDATE oauth_flows SET consumed_at=now()
		WHERE state_hash=$1 AND browser_token_hash=$2 AND issuer=$3 AND redirect_uri=$4
		AND consumed_at IS NULL AND expires_at>now() AND link_user_id IS NULL
		RETURNING pkce_verifier_encrypted, nonce_hash`, digest(state), digest(browser), s.cfg.OIDCIssuer, p.OAuth.RedirectURL).Scan(&encrypted, &nonceHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrUnauthorized
	}
	if err != nil {
		return Tokens{}, err
	}
	if len(encrypted) < s.box.NonceSize() {
		return Tokens{}, ErrUnauthorized
	}
	verifier, err := s.box.Open(nil, encrypted[:s.box.NonceSize()], encrypted[s.box.NonceSize():], digest(state))
	if err != nil {
		return Tokens{}, ErrUnauthorized
	}
	if p.Client != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, p.Client)
	}
	token, err := p.OAuth.Exchange(ctx, code, oauth2.VerifierOption(string(verifier)))
	if err != nil {
		return Tokens{}, ErrUnauthorized
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Tokens{}, ErrUnauthorized
	}
	idToken, err := p.Verifier.Verify(ctx, rawIDToken)
	if err != nil || idToken.Subject == "" || subtle.ConstantTimeCompare(digest(idToken.Nonce), nonceHash) != 1 {
		return Tokens{}, ErrUnauthorized
	}
	if idToken.AccessTokenHash != "" {
		if err := idToken.VerifyAccessToken(token.AccessToken); err != nil {
			return Tokens{}, ErrUnauthorized
		}
	}
	var claims struct {
		AuthorizedParty string `json:"azp"`
		Name            string `json:"name"`
		Email           string `json:"email"`
		EmailVerified   bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Tokens{}, ErrUnauthorized
	}
	if (len(idToken.Audience) > 1 && claims.AuthorizedParty == "") || (claims.AuthorizedParty != "" && claims.AuthorizedParty != p.OAuth.ClientID) {
		return Tokens{}, ErrUnauthorized
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	// Serialize first-time provisioning of the same external identity. Never link by email.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, s.cfg.OIDCIssuer+"\n"+idToken.Subject); err != nil {
		return Tokens{}, err
	}
	var user uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM identities WHERE issuer=$1 AND subject=$2`, s.cfg.OIDCIssuer, idToken.Subject).Scan(&user)
	if errors.Is(err, sql.ErrNoRows) {
		user = uuid.New()
		name := claims.Name
		if name == "" {
			name = "User"
		}
		if len(name) > 300 {
			name = "User"
		}
		// Keep provider email on the identity. Assigning a login email requires a
		// separate verified linking flow, even if the provider reports it verified.
		_, err = tx.ExecContext(ctx, `INSERT INTO users(id, username, display_name) VALUES($1,$2,$3)`, user, "user_"+strings.ReplaceAll(user.String(), "-", ""), name)
		if err != nil {
			return Tokens{}, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO identities(user_id, issuer, subject) VALUES($1,$2,$3)`, user, s.cfg.OIDCIssuer, idToken.Subject)
	}
	if err != nil {
		return Tokens{}, err
	}
	if err := lockUser(ctx, tx, user); err != nil {
		return Tokens{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE identities SET provider_email=NULLIF($3,''), provider_email_verified=$4, last_login_at=now()
		WHERE issuer=$1 AND subject=$2`, s.cfg.OIDCIssuer, idToken.Subject, claims.Email, claims.EmailVerified)
	if err != nil {
		return Tokens{}, err
	}
	pair, err := s.newSession(ctx, tx, user)
	if err != nil {
		return Tokens{}, err
	}
	if err := tx.Commit(); err != nil {
		return Tokens{}, err
	}
	return pair, nil
}
