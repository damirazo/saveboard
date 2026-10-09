-- +goose Up
CREATE TABLE posts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    type text NOT NULL CHECK (type IN ('note', 'link', 'photo')),
    visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('public', 'unlisted', 'friends', 'private')),
    UNIQUE (id, type)
);
CREATE INDEX posts_author_cursor_idx ON posts(author_id, created_at DESC, id DESC);

-- +goose StatementBegin
CREATE FUNCTION posts_set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = clock_timestamp();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER posts_updated_at BEFORE UPDATE ON posts
    FOR EACH ROW EXECUTE FUNCTION posts_set_updated_at();

CREATE TABLE post_notes (
    post_id uuid PRIMARY KEY,
    type text NOT NULL DEFAULT 'note' CHECK (type = 'note'),
    text text NOT NULL DEFAULT '',
    FOREIGN KEY (post_id, type) REFERENCES posts(id, type) ON DELETE CASCADE
);
CREATE TABLE post_links (
    post_id uuid PRIMARY KEY,
    type text NOT NULL DEFAULT 'link' CHECK (type = 'link'),
    url text NOT NULL CHECK (length(btrim(url)) > 0),
    FOREIGN KEY (post_id, type) REFERENCES posts(id, type) ON DELETE CASCADE
);
CREATE TABLE post_photos (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id uuid NOT NULL,
    type text NOT NULL DEFAULT 'photo' CHECK (type = 'photo'),
    url text NOT NULL CHECK (length(btrim(url)) > 0),
    FOREIGN KEY (post_id, type) REFERENCES posts(id, type) ON DELETE CASCADE
);
CREATE INDEX post_photos_post_idx ON post_photos(post_id);

-- Global tags are separate from the legacy owner-scoped notes tags.
CREATE TABLE post_tags (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE CHECK (length(btrim(name)) > 0),
    title text NOT NULL
);
CREATE TABLE post_tag_links (
    post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES post_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);
CREATE INDEX post_tag_links_tag_idx ON post_tag_links(tag_id, post_id);

-- Content and tag changes also advance the parent post's timestamp.
-- +goose StatementBegin
CREATE FUNCTION posts_touch_parent() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP <> 'INSERT' THEN
        UPDATE posts SET updated_at = clock_timestamp() WHERE id = OLD.post_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        UPDATE posts SET updated_at = clock_timestamp() WHERE id = NEW.post_id;
    END IF;
    RETURN NULL;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER post_notes_touch AFTER INSERT OR UPDATE OR DELETE ON post_notes
    FOR EACH ROW EXECUTE FUNCTION posts_touch_parent();
CREATE TRIGGER post_links_touch AFTER INSERT OR UPDATE OR DELETE ON post_links
    FOR EACH ROW EXECUTE FUNCTION posts_touch_parent();
CREATE TRIGGER post_photos_touch AFTER INSERT OR UPDATE OR DELETE ON post_photos
    FOR EACH ROW EXECUTE FUNCTION posts_touch_parent();
CREATE TRIGGER post_tag_links_touch AFTER INSERT OR UPDATE OR DELETE ON post_tag_links
    FOR EACH ROW EXECUTE FUNCTION posts_touch_parent();

-- +goose Down
DROP TABLE post_tag_links, post_tags, post_photos, post_links, post_notes;
DROP FUNCTION posts_touch_parent();
DROP TABLE posts;
DROP FUNCTION posts_set_updated_at();
