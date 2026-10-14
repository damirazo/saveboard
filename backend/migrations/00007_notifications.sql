-- +goose Up
ALTER TABLE post_captures ADD COLUMN deadline_at timestamptz NOT NULL DEFAULT (now() + interval '15 minutes');
UPDATE post_captures SET deadline_at=created_at + interval '15 minutes';
CREATE TABLE notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_id uuid REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 300),
    text text NOT NULL CHECK (length(text) BETWEEN 1 AND 10000),
    importance text NOT NULL DEFAULT 'normal' CHECK (importance IN ('normal','important','critical')),
    post_id uuid REFERENCES posts(id) ON DELETE SET NULL,
    event_key text UNIQUE
);
CREATE INDEX notifications_recipient_created ON notifications(recipient_id,created_at DESC,id DESC);
CREATE TABLE notification_reads (
    notification_id uuid NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    read_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(notification_id,user_id)
);
CREATE INDEX notification_reads_user ON notification_reads(user_id);

-- +goose Down
DROP TABLE notification_reads;
DROP TABLE notifications;
ALTER TABLE post_captures DROP COLUMN deadline_at;
