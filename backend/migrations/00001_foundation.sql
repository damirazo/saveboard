-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    disabled_at timestamptz
);
CREATE TABLE identities (
    user_id uuid NOT NULL REFERENCES users(id),
    issuer text NOT NULL,
    subject text NOT NULL,
    PRIMARY KEY (issuer, subject)
);
CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id uuid NOT NULL REFERENCES users(id),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_expiry_idx ON sessions(expires_at);
CREATE TABLE notes (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    kind text NOT NULL CHECK (kind IN ('text', 'web', 'file')),
    visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'friends', 'unlisted', 'public')),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 300),
    version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE(id, owner_id)
);
CREATE INDEX notes_owner_cursor_idx ON notes(owner_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE TABLE text_notes (
    note_id uuid PRIMARY KEY REFERENCES notes(id) ON DELETE CASCADE,
    content_json jsonb NOT NULL,
    schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
    plain_text text NOT NULL DEFAULT '',
    search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', plain_text)) STORED
);
CREATE INDEX text_notes_search_idx ON text_notes USING gin(search_vector);
CREATE TABLE tags (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES users(id),
    name text NOT NULL,
    normalized_name text NOT NULL,
    UNIQUE(owner_id, normalized_name),
    UNIQUE(id, owner_id)
);
CREATE TABLE note_tags (
    note_id uuid NOT NULL,
    tag_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    PRIMARY KEY(note_id, tag_id),
    FOREIGN KEY(note_id, owner_id) REFERENCES notes(id, owner_id) ON DELETE CASCADE,
    FOREIGN KEY(tag_id, owner_id) REFERENCES tags(id, owner_id) ON DELETE CASCADE
);
CREATE INDEX note_tags_reverse_idx ON note_tags(tag_id, note_id);
-- Foundation event receipt, not a capture/indexing completion marker.
CREATE TABLE job_receipts (
    job_id bigint PRIMARY KEY,
    kind text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE job_receipts, note_tags, tags, text_notes, notes, sessions, identities, users;
