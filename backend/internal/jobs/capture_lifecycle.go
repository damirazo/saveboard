package jobs

import (
	"context"
	"database/sql"
	"errors"
	"saveboard/internal/notifications"
)

// FinishCapture publishes one durable outcome, together with its notification.
// A late/stale worker cannot overwrite a newer revision or publish expired output.
func FinishCapture(ctx context.Context, db *sql.DB, args CaptureArgs, manifest []byte, thumbnail, failure string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	state, importance, title, text := "ready", "normal", "Фоновая обработка завершена", "Копия и/или эскиз страницы готовы. Откройте пост, чтобы посмотреть результат."
	if failure != "" {
		state, importance, title, text = "failed", "critical", "Ошибка фоновой обработки", failure
	}
	var recipient string
	err = tx.QueryRowContext(ctx, `UPDATE post_captures c SET state=$3,manifest=$4,thumbnail_key=NULLIF($5,''),completed_at=now() FROM posts p WHERE c.post_id=$1 AND c.revision=$2 AND c.state='pending' AND p.id=c.post_id AND p.type='link' AND p.status='published' AND ($3='failed' OR c.deadline_at>clock_timestamp()) RETURNING p.author_id`, args.PostID, args.Revision, state, nullableJSON(manifest), thumbnail).Scan(&recipient)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	key := "capture:" + args.Revision
	if err = notifications.PublishTx(ctx, tx, notifications.Event{RecipientID: &recipient, Title: title, Text: text, Importance: importance, PostID: &args.PostID, Key: &key}); err != nil {
		return err
	}
	return tx.Commit()
}
func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}

// ReconcileCaptures also handles a process dying during its final attempt, when
// Work cannot report failure. River first rescues/discards the abandoned job.
// cancelJob must use River's API so running work receives cancellation.
func ReconcileCaptures(ctx context.Context, db *sql.DB, cancelJob func(context.Context, int64) error) error {
	rows, err := db.QueryContext(ctx, `SELECT c.post_id,c.revision FROM post_captures c JOIN posts p ON p.id=c.post_id WHERE c.state='pending' AND p.status='published' AND p.type='link' AND (c.deadline_at<=clock_timestamp() OR NOT EXISTS (SELECT 1 FROM river_job j WHERE j.kind='post.capture' AND j.args->>'post_id'=c.post_id::text AND j.args->>'revision'=c.revision::text AND j.state IN ('available','running','retryable','scheduled','pending'))) LIMIT 100`)
	if err != nil {
		return err
	}
	var failed []CaptureArgs
	for rows.Next() {
		var a CaptureArgs
		if err = rows.Scan(&a.PostID, &a.Revision); err != nil {
			rows.Close()
			return err
		}
		failed = append(failed, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range failed {
		if err = FinishCapture(ctx, db, a, nil, "", "Фоновая обработка не завершилась: превышено время ожидания или исчерпаны попытки восстановления. Можно повторить её, сохранив пост ещё раз."); err != nil {
			return err
		}
	}
	// Obsolete tasks must not consume a browser slot after an edit/deletion/expiry.
	rows, err = db.QueryContext(ctx, `SELECT j.id FROM river_job j WHERE j.kind='post.capture' AND j.state IN ('available','running','retryable','scheduled','pending') AND NOT EXISTS (SELECT 1 FROM post_captures c JOIN posts p ON p.id=c.post_id WHERE c.post_id::text=j.args->>'post_id' AND c.revision::text=j.args->>'revision' AND c.state='pending' AND c.deadline_at>clock_timestamp() AND p.type='link' AND p.status='published') LIMIT 100`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = cancelJob(ctx, id); err != nil {
			return err
		}
	}
	// Never delete running rows: retain their leases until River finalizes them.
	_, err = db.ExecContext(ctx, `DELETE FROM river_job WHERE id IN (SELECT j.id FROM river_job j WHERE j.kind='post.capture' AND j.state IN ('completed','cancelled','discarded') AND NOT EXISTS (SELECT 1 FROM post_captures c JOIN posts p ON p.id=c.post_id WHERE c.post_id::text=j.args->>'post_id' AND c.revision::text=j.args->>'revision' AND c.state='pending' AND p.status='published' AND p.type='link') LIMIT 100)`)
	return err
}
