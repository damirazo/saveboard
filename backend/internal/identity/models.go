package identity

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type UserStatus string

const (
	UserPending  UserStatus = "pending"
	UserActive   UserStatus = "active"
	UserDisabled UserStatus = "disabled"
)

// User supports local credentials and accounts that only use OAuth/OIDC.
// PasswordHash contains an encoded password hash with its salt and parameters.
// Username excludes @ so username/email login lookup is unambiguous.
type User struct {
	bun.BaseModel       `bun:"table:users"`
	ID                  uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()" json:"id"`
	Username            string     `bun:"username,notnull" json:"username"`
	DisplayName         string     `bun:"display_name,notnull" json:"display_name"`
	Email               *string    `bun:"email" json:"email,omitempty"`
	EmailVerifiedAt     *time.Time `bun:"email_verified_at" json:"email_verified_at,omitempty"`
	PasswordHash        *string    `bun:"password_hash" json:"-"`
	PasswordChangedAt   *time.Time `bun:"password_changed_at" json:"-"`
	Status              UserStatus `bun:"status,notnull,default:'active'" json:"status"`
	CreatedAt           time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt           time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
	LastLoginAt         *time.Time `bun:"last_login_at" json:"last_login_at,omitempty"`
	DisabledAt          *time.Time `bun:"disabled_at" json:"disabled_at,omitempty"`
	FailedLoginAttempts int        `bun:"failed_login_attempts,notnull,default:0" json:"-"`
	LockedUntil         *time.Time `bun:"locked_until" json:"-"`
}

// OAuthIdentity is identified by issuer + subject, never by provider email.
// Encrypted fields hold authenticated ciphertext including key/version metadata.
// Provider configuration and client secrets remain in server configuration.
type OAuthIdentity struct {
	bun.BaseModel         `bun:"table:identities"`
	UserID                uuid.UUID  `bun:"user_id,type:uuid,notnull" json:"user_id"`
	Issuer                string     `bun:"issuer,pk" json:"issuer"`
	Subject               string     `bun:"subject,pk" json:"subject"`
	CreatedAt             time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt             time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`
	LastLoginAt           *time.Time `bun:"last_login_at" json:"last_login_at,omitempty"`
	ProviderEmail         *string    `bun:"provider_email" json:"provider_email,omitempty"`
	ProviderEmailVerified bool       `bun:"provider_email_verified,notnull,default:false" json:"provider_email_verified"`
	AccessTokenEncrypted  []byte     `bun:"access_token_encrypted" json:"-"`
	RefreshTokenEncrypted []byte     `bun:"refresh_token_encrypted" json:"-"`
	TokenType             *string    `bun:"token_type" json:"-"`
	Scopes                []string   `bun:"scopes,array,notnull,default:'{}'" json:"-"`
	AccessTokenExpiresAt  *time.Time `bun:"access_token_expires_at" json:"-"`
	RefreshTokenExpiresAt *time.Time `bun:"refresh_token_expires_at" json:"-"`
}

type Session struct {
	bun.BaseModel `bun:"table:sessions"`
	ID            uuid.UUID  `bun:"id,type:uuid,notnull,default:gen_random_uuid()" json:"id"`
	TokenHash     []byte     `bun:"token_hash,pk" json:"-"`
	UserID        uuid.UUID  `bun:"user_id,type:uuid,notnull" json:"user_id"`
	ExpiresAt     time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	RevokedAt     *time.Time `bun:"revoked_at" json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	LastSeenAt    *time.Time `bun:"last_seen_at" json:"last_seen_at,omitempty"`
	UserAgent     *string    `bun:"user_agent" json:"-"`
	IPAddress     *string    `bun:"ip_address,type:inet" json:"-"`
}

// Consumed refresh hashes are retained until session expiry to detect replay.
type RefreshToken struct {
	bun.BaseModel `bun:"table:refresh_tokens"`
	TokenHash     []byte     `bun:"token_hash,pk" json:"-"`
	SessionID     uuid.UUID  `bun:"session_id,type:uuid,notnull" json:"session_id"`
	CreatedAt     time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	ExpiresAt     time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt    *time.Time `bun:"consumed_at" json:"-"`
}

type TokenPurpose string

const (
	TokenEmailVerification TokenPurpose = "email_verification"
	TokenPasswordReset     TokenPurpose = "password_reset"
)

// UserToken must be consumed atomically, checking expiry and consumed_at.
// Email binds a verification token to the exact address being verified.
type UserToken struct {
	bun.BaseModel `bun:"table:user_tokens"`
	TokenHash     []byte       `bun:"token_hash,pk" json:"-"`
	UserID        uuid.UUID    `bun:"user_id,type:uuid,notnull" json:"user_id"`
	Purpose       TokenPurpose `bun:"purpose,notnull" json:"purpose"`
	Email         *string      `bun:"email" json:"-"`
	CreatedAt     time.Time    `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	ExpiresAt     time.Time    `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt    *time.Time   `bun:"consumed_at" json:"consumed_at,omitempty"`
}

// OAuthFlow is short-lived and single-use. NonceHash is required for OIDC,
// nullable for plain OAuth2. BrowserTokenHash binds state to an HttpOnly cookie.
type OAuthFlow struct {
	bun.BaseModel         `bun:"table:oauth_flows"`
	StateHash             []byte     `bun:"state_hash,pk" json:"-"`
	BrowserTokenHash      []byte     `bun:"browser_token_hash,notnull" json:"-"`
	Issuer                string     `bun:"issuer,notnull" json:"issuer"`
	NonceHash             []byte     `bun:"nonce_hash" json:"-"`
	PKCEVerifierEncrypted []byte     `bun:"pkce_verifier_encrypted,notnull" json:"-"`
	RedirectURI           string     `bun:"redirect_uri,notnull" json:"-"`
	ReturnPath            string     `bun:"return_path,notnull,default:'/'" json:"-"`
	LinkUserID            *uuid.UUID `bun:"link_user_id,type:uuid" json:"-"`
	CreatedAt             time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	ExpiresAt             time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	ConsumedAt            *time.Time `bun:"consumed_at" json:"consumed_at,omitempty"`
}
