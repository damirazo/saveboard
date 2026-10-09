-- +goose Up
ALTER TABLE sessions ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE sessions ADD CONSTRAINT sessions_id_unique UNIQUE (id);
CREATE TABLE refresh_tokens (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    session_id uuid NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX refresh_tokens_session_idx ON refresh_tokens(session_id);
CREATE INDEX refresh_tokens_expiry_idx ON refresh_tokens(expires_at);

-- +goose Down
DROP TABLE refresh_tokens;
ALTER TABLE sessions DROP COLUMN id;
