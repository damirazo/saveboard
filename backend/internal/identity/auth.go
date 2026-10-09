package identity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUnauthorized     = errors.New("invalid credentials or session")
	ErrBusy             = errors.New("too many login attempts")
	ErrOAuthUnavailable = errors.New("OAuth is not configured")
)

const (
	AccessLifetime  = 15 * time.Minute
	SessionLifetime = 30 * 24 * time.Hour
	FlowLifetime    = 10 * time.Minute
)

type AuthConfig struct {
	SigningKey    []byte
	EncryptionKey []byte
	Issuer        string
	Audience      string
	// ResolveProvider may lazily perform discovery. It must only use trusted configuration.
	ResolveProvider func(context.Context) (*Provider, error)
	OIDCIssuer      string
}

type AuthService struct {
	db            *sql.DB
	cfg           AuthConfig
	box           cipher.AEAD
	dummyHash     []byte
	passwordSlots chan struct{}
}

func NewAuthService(db *sql.DB, cfg AuthConfig) (*AuthService, error) {
	if db == nil || len(cfg.SigningKey) < 32 || len(cfg.EncryptionKey) != 32 || cfg.Issuer == "" || cfg.Audience == "" {
		return nil, fmt.Errorf("auth requires a database, signing key (at least 32 bytes), encryption key (32 bytes), issuer and audience")
	}
	block, err := aes.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	box, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte(randomToken()), 12)
	if err != nil {
		return nil, err
	}
	return &AuthService{db: db, cfg: cfg, box: box, dummyHash: dummy, passwordSlots: make(chan struct{}, 8)}, nil
}

// HashPassword produces the format accepted by Login. Never store raw passwords.
func HashPassword(password string) (string, error) {
	if len(password) < 8 || len(password) > 72 {
		return "", fmt.Errorf("password must contain 8 to 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), err
}

type Tokens struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`
	ExpiresIn        int64     `json:"expires_in"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	SessionID        uuid.UUID `json:"session_id"`
}

type Principal struct {
	UserID    uuid.UUID `json:"user_id"`
	SessionID uuid.UUID `json:"session_id"`
}

type accessClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
	TokenUse  string `json:"token_use"`
}

func randomToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

func digest(value string) []byte {
	h := sha256.Sum256([]byte(value))
	return h[:]
}

func validToken(value string) bool {
	b, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == value
}

func (s *AuthService) tokenPair(user, session uuid.UUID, refresh string, expires time.Time) (Tokens, error) {
	now := time.Now()
	accessExpires := now.Add(AccessLifetime)
	if expires.Before(accessExpires) {
		accessExpires = expires
	}
	claims := accessClaims{RegisteredClaims: jwt.RegisteredClaims{
		Issuer: s.cfg.Issuer, Audience: jwt.ClaimStrings{s.cfg.Audience}, Subject: user.String(),
		ID: uuid.NewString(), IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(accessExpires),
	}, SessionID: session.String(), TokenUse: "access"}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.cfg.SigningKey)
	return Tokens{AccessToken: token, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: accessExpires.Unix() - now.Unix(), RefreshExpiresAt: expires, SessionID: session}, err
}

func (s *AuthService) parseAccess(raw string) (Principal, error) {
	if len(raw) > 4096 {
		return Principal{}, ErrUnauthorized
	}
	c := &accessClaims{}
	_, err := jwt.ParseWithClaims(raw, c, func(_ *jwt.Token) (any, error) { return s.cfg.SigningKey, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.cfg.Issuer), jwt.WithAudience(s.cfg.Audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || c.TokenUse != "access" || c.IssuedAt == nil || c.ID == "" {
		return Principal{}, ErrUnauthorized
	}
	user, err := uuid.Parse(c.Subject)
	if err != nil || user == uuid.Nil {
		return Principal{}, ErrUnauthorized
	}
	session, err := uuid.Parse(c.SessionID)
	if err != nil || session == uuid.Nil {
		return Principal{}, ErrUnauthorized
	}
	return Principal{UserID: user, SessionID: session}, nil
}

// Authenticate checks the database on every request, making revocation immediate.
func (s *AuthService) Authenticate(ctx context.Context, raw string) (Principal, error) {
	p, err := s.parseAccess(raw)
	if err != nil {
		return Principal{}, err
	}
	var ok bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.id=$1 AND u.id=$2 AND s.revoked_at IS NULL AND s.expires_at>now()
		AND u.status='active' AND u.disabled_at IS NULL)`, p.SessionID, p.UserID).Scan(&ok)
	if err != nil {
		return Principal{}, err
	}
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	return p, nil
}

func (s *AuthService) Login(ctx context.Context, login, password string) (Tokens, error) {
	if len(login) > 254 || len(password) == 0 || len(password) > 72 {
		return Tokens{}, ErrUnauthorized
	}
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
	default:
		return Tokens{}, ErrBusy
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	var user uuid.UUID
	var hash sql.NullString
	var status string
	var locked sql.NullTime
	var failed int
	err = tx.QueryRowContext(ctx, `SELECT id, password_hash, status, locked_until, failed_login_attempts FROM users
		WHERE lower(username)=lower($1) OR lower(email)=lower($1) FOR UPDATE`, strings.TrimSpace(login)).Scan(&user, &hash, &status, &locked, &failed)
	if errors.Is(err, sql.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return Tokens{}, ErrUnauthorized
	}
	if err != nil {
		return Tokens{}, err
	}
	if status != "active" || (locked.Valid && locked.Time.After(time.Now())) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return Tokens{}, ErrUnauthorized
	}
	check := []byte(hash.String)
	if !hash.Valid {
		check = s.dummyHash
	}
	if bcrypt.CompareHashAndPassword(check, []byte(password)) != nil || !hash.Valid {
		// Restart the counter after the previous lock expires.
		if locked.Valid {
			failed = 0
		}
		_, err = tx.ExecContext(ctx, `UPDATE users SET failed_login_attempts=$2,
			locked_until=CASE WHEN $2>=5 THEN now()+interval '15 minutes' ELSE NULL END WHERE id=$1`, user, failed+1)
		if err != nil {
			return Tokens{}, err
		}
		if err := tx.Commit(); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, ErrUnauthorized
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

// All session mutations lock the user first, then the session. This serializes
// refresh/logout-all/login without deadlocks or resurrection of revoked sessions.
func lockUser(ctx context.Context, tx *sql.Tx, user uuid.UUID) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id=$1 FOR UPDATE`, user).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && status != "active") {
		return ErrUnauthorized
	}
	return err
}

func (s *AuthService) newSession(ctx context.Context, tx *sql.Tx, user uuid.UUID) (Tokens, error) {
	session, refresh, expires := uuid.New(), randomToken(), time.Now().Add(SessionLifetime)
	pair, err := s.tokenPair(user, session, refresh, expires)
	if err != nil {
		return Tokens{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sessions(id, token_hash, user_id, expires_at, last_seen_at) VALUES($1,$2,$3,$4,now())`, session, digest(refresh), user, expires)
	if err != nil {
		return Tokens{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO refresh_tokens(token_hash, session_id, expires_at) VALUES($1,$2,$3)`, digest(refresh), session, expires)
	if err != nil {
		return Tokens{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET last_login_at=now(), failed_login_attempts=0, locked_until=NULL WHERE id=$1`, user)
	return pair, err
}

func (s *AuthService) Refresh(ctx context.Context, raw string) (Tokens, error) {
	if !validToken(raw) {
		return Tokens{}, ErrUnauthorized
	}
	var p Principal
	err := s.db.QueryRowContext(ctx, `SELECT s.user_id, s.id FROM refresh_tokens r JOIN sessions s ON s.id=r.session_id WHERE r.token_hash=$1`, digest(raw)).Scan(&p.UserID, &p.SessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrUnauthorized
	}
	if err != nil {
		return Tokens{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	if err := lockUser(ctx, tx, p.UserID); err != nil {
		return Tokens{}, err
	}
	var expires time.Time
	var revoked, consumed sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT s.expires_at, s.revoked_at, r.consumed_at FROM sessions s
		JOIN refresh_tokens r ON r.session_id=s.id WHERE r.token_hash=$1 AND r.expires_at>now() FOR UPDATE OF s,r`, digest(raw)).Scan(&expires, &revoked, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return Tokens{}, ErrUnauthorized
	}
	if err != nil {
		return Tokens{}, err
	}
	if revoked.Valid || !expires.After(time.Now()) {
		return Tokens{}, ErrUnauthorized
	}
	if consumed.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=now() WHERE id=$1`, p.SessionID); err != nil {
			return Tokens{}, err
		}
		if err := tx.Commit(); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, ErrUnauthorized
	}
	refresh := randomToken()
	pair, err := s.tokenPair(p.UserID, p.SessionID, refresh, expires)
	if err != nil {
		return Tokens{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE refresh_tokens SET consumed_at=now() WHERE token_hash=$1`, digest(raw)); err != nil {
		return Tokens{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO refresh_tokens(token_hash, session_id, expires_at) VALUES($1,$2,$3)`, digest(refresh), p.SessionID, expires); err != nil {
		return Tokens{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET last_seen_at=now() WHERE id=$1`, p.SessionID); err != nil {
		return Tokens{}, err
	}
	if err := tx.Commit(); err != nil {
		return Tokens{}, err
	}
	return pair, nil
}

// Revoke with others=true leaves exactly the caller's current session intact.
func (s *AuthService) Revoke(ctx context.Context, raw string, others bool) (int64, error) {
	p, err := s.parseAccess(raw)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := lockUser(ctx, tx, p.UserID); err != nil {
		return 0, err
	}
	var id uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, p.SessionID, p.UserID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrUnauthorized
	}
	if err != nil {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL AND expires_at>now()
		AND (($3 AND id<>$2) OR (NOT $3 AND id=$2))`, p.UserID, p.SessionID, others)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, tx.Commit()
}
