-- +goose Up
CREATE TABLE post_rag (
    post_id uuid PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
    revision uuid NOT NULL UNIQUE,
    url text NOT NULL
);
-- Retain an outbox/tombstone even after hard deletion of a post: Qdrant cleanup
-- must be retried independently of availability of external services.
CREATE TABLE rag_documents (
    id uuid PRIMARY KEY,
    post_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    source_url text NOT NULL,
    title text NOT NULL DEFAULT '',
    text text NOT NULL DEFAULT '',
    chunks jsonb NOT NULL DEFAULT '[]',
    state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','ready','failed','obsolete')),
    created_at timestamptz NOT NULL DEFAULT now(),
    deadline_at timestamptz NOT NULL DEFAULT (now()+interval '30 minutes'),
    completed_at timestamptz,
    purged_at timestamptz,
    model_digest text NOT NULL DEFAULT '',
    collection text NOT NULL DEFAULT '',
    chunker_version text NOT NULL DEFAULT 'runes-1600-overlap-200-v1'
);
CREATE INDEX rag_documents_post ON rag_documents(post_id);
CREATE INDEX rag_documents_pending ON rag_documents(deadline_at) WHERE state='pending';

-- +goose Down
DROP TABLE post_rag;
DROP TABLE rag_documents;
