-- +goose Up
CREATE TABLE post_captures (
    post_id uuid PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
    revision uuid NOT NULL,
    url text NOT NULL,
    save_copy boolean NOT NULL DEFAULT false,
    create_thumbnail boolean NOT NULL DEFAULT false,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','ready','failed')),
    manifest jsonb,
    thumbnail_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

-- +goose Down
DROP TABLE post_captures;
