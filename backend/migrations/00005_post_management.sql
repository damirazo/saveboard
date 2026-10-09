-- +goose Up
ALTER TABLE posts ADD COLUMN title text NOT NULL DEFAULT '' CHECK (length(title) <= 200);
ALTER TABLE post_photos ADD COLUMN position integer NOT NULL DEFAULT 0;
CREATE TABLE post_uploads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES users(id),
    post_id uuid REFERENCES posts(id) ON DELETE CASCADE,
    content bytea NOT NULL,
    media_type text NOT NULL CHECK (media_type IN ('image/png','image/jpeg')),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (octet_length(content) <= 10485760)
);
CREATE INDEX post_uploads_owner_idx ON post_uploads(owner_id);
CREATE INDEX post_uploads_post_idx ON post_uploads(post_id);
CREATE INDEX post_tags_title_prefix_idx ON post_tags(lower(title) text_pattern_ops);
CREATE INDEX posts_public_feed_idx ON posts(created_at DESC,id DESC) WHERE status='published' AND visibility='public';

-- +goose Down
DROP INDEX posts_public_feed_idx;
DROP INDEX post_tags_title_prefix_idx;
DROP TABLE post_uploads;
ALTER TABLE post_photos DROP COLUMN position;
ALTER TABLE posts DROP COLUMN title;
