-- +goose Up
ALTER TABLE users
    ALTER COLUMN id SET DEFAULT gen_random_uuid(),
    ADD COLUMN username text,
    ADD COLUMN email text,
    ADD COLUMN email_verified_at timestamptz,
    ADD COLUMN password_hash text,
    ADD COLUMN password_changed_at timestamptz,
    ADD COLUMN status text NOT NULL DEFAULT 'active',
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN last_login_at timestamptz,
    ADD COLUMN failed_login_attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN locked_until timestamptz;

-- Preserve existing users without inventing email addresses or credentials.
UPDATE users SET username = 'user_' || replace(id::text, '-', ''),
    status = CASE WHEN disabled_at IS NULL THEN 'active' ELSE 'disabled' END;
ALTER TABLE users
    ALTER COLUMN username SET NOT NULL,
    ADD CONSTRAINT users_username_check CHECK (username ~ '^[a-zA-Z0-9_][a-zA-Z0-9_.-]{2,63}$'),
    ADD CONSTRAINT users_email_check CHECK (email IS NULL OR (email = btrim(email) AND email ~ '^[^[:space:]@]+@[^[:space:]@]+$')),
    ADD CONSTRAINT users_email_verified_check CHECK (email_verified_at IS NULL OR email IS NOT NULL),
    ADD CONSTRAINT users_password_check CHECK (password_hash IS NULL OR length(btrim(password_hash)) > 0),
    ADD CONSTRAINT users_status_check CHECK (status IN ('pending', 'active', 'disabled')),
    ADD CONSTRAINT users_disabled_check CHECK ((status = 'disabled') = (disabled_at IS NOT NULL)),
    ADD CONSTRAINT users_failed_login_check CHECK (failed_login_attempts >= 0);
CREATE UNIQUE INDEX users_username_unique ON users(lower(username));
CREATE UNIQUE INDEX users_email_unique ON users(lower(email)) WHERE email IS NOT NULL;

-- Database-managed timestamps also cover updates outside Bun.
-- +goose StatementBegin
CREATE FUNCTION identity_set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION identity_set_updated_at();

ALTER TABLE identities
    ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN last_login_at timestamptz,
    ADD COLUMN provider_email text,
    ADD COLUMN provider_email_verified boolean NOT NULL DEFAULT false,
    ADD COLUMN access_token_encrypted bytea,
    ADD COLUMN refresh_token_encrypted bytea,
    ADD COLUMN token_type text,
    ADD COLUMN scopes text[] NOT NULL DEFAULT '{}',
    ADD COLUMN access_token_expires_at timestamptz,
    ADD COLUMN refresh_token_expires_at timestamptz,
    ADD CONSTRAINT identities_issuer_check CHECK (length(btrim(issuer)) > 0),
    ADD CONSTRAINT identities_subject_check CHECK (length(btrim(subject)) > 0);
CREATE INDEX identities_user_idx ON identities(user_id);
CREATE TRIGGER identities_updated_at BEFORE UPDATE ON identities
    FOR EACH ROW EXECUTE FUNCTION identity_set_updated_at();

ALTER TABLE sessions
    ADD COLUMN last_seen_at timestamptz,
    ADD COLUMN user_agent text,
    ADD COLUMN ip_address inet;
CREATE INDEX sessions_user_idx ON sessions(user_id);

-- One-time verification/reset tokens: store SHA-256 digests, never raw tokens.
CREATE TABLE user_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose text NOT NULL CHECK (purpose IN ('email_verification', 'password_reset')),
    email text,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at),
    CHECK (purpose <> 'email_verification' OR (email IS NOT NULL AND length(btrim(email)) > 0))
);
CREATE INDEX user_tokens_user_idx ON user_tokens(user_id, purpose);
CREATE INDEX user_tokens_expiry_idx ON user_tokens(expires_at);

-- Bind each callback to the initiating browser; linking also binds the user.
CREATE TABLE oauth_flows (
    state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash) = 32),
    browser_token_hash bytea NOT NULL CHECK (octet_length(browser_token_hash) = 32),
    issuer text NOT NULL CHECK (length(btrim(issuer)) > 0),
    nonce_hash bytea CHECK (nonce_hash IS NULL OR octet_length(nonce_hash) = 32),
    pkce_verifier_encrypted bytea NOT NULL CHECK (octet_length(pkce_verifier_encrypted) > 0),
    redirect_uri text NOT NULL CHECK (length(btrim(redirect_uri)) > 0),
    return_path text NOT NULL DEFAULT '/' CHECK (return_path ~ '^/([^/\\]|$)' AND position(chr(92) in return_path) = 0),
    link_user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX oauth_flows_expiry_idx ON oauth_flows(expires_at);
CREATE INDEX oauth_flows_link_user_idx ON oauth_flows(link_user_id) WHERE link_user_id IS NOT NULL;

-- +goose Down
DROP TABLE oauth_flows, user_tokens;
DROP INDEX sessions_user_idx;
ALTER TABLE sessions DROP COLUMN last_seen_at, DROP COLUMN user_agent, DROP COLUMN ip_address;
DROP TRIGGER identities_updated_at ON identities;
DROP INDEX identities_user_idx;
ALTER TABLE identities
    DROP CONSTRAINT identities_issuer_check, DROP CONSTRAINT identities_subject_check,
    DROP COLUMN created_at, DROP COLUMN updated_at, DROP COLUMN last_login_at,
    DROP COLUMN provider_email, DROP COLUMN provider_email_verified,
    DROP COLUMN access_token_encrypted, DROP COLUMN refresh_token_encrypted,
    DROP COLUMN token_type, DROP COLUMN scopes,
    DROP COLUMN access_token_expires_at, DROP COLUMN refresh_token_expires_at;
DROP TRIGGER users_updated_at ON users;
DROP FUNCTION identity_set_updated_at();
ALTER TABLE users
    ALTER COLUMN id DROP DEFAULT,
    DROP COLUMN username, DROP COLUMN email, DROP COLUMN email_verified_at,
    DROP COLUMN password_hash, DROP COLUMN password_changed_at, DROP COLUMN status,
    DROP COLUMN updated_at, DROP COLUMN last_login_at,
    DROP COLUMN failed_login_attempts, DROP COLUMN locked_until;
