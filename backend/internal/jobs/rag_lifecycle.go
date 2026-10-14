package jobs

import (
	"context"
	"database/sql"
	"saveboard/internal/search"
)

func ReconcileRAG(ctx context.Context, db *sql.DB, index *search.Index, cancelJob func(context.Context, int64) error) error {
	rows, err := db.QueryContext(ctx, `SELECT d.post_id,d.id FROM rag_documents d JOIN post_rag r ON r.revision=d.id JOIN posts p ON p.id=r.post_id WHERE d.state='pending' AND p.status='published' AND p.type='link' AND (d.deadline_at<=clock_timestamp() OR NOT EXISTS(SELECT 1 FROM river_job j WHERE j.kind='post.rag' AND j.args->>'revision'=d.id::text AND j.state IN ('available','running','retryable','scheduled','pending'))) LIMIT 100`)
	if err != nil {
		return err
	}
	var failed []RAGArgs
	for rows.Next() {
		var a RAGArgs
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
		if err = finishRAG(ctx, db, a, true); err != nil {
			return err
		}
	}
	rows, err = db.QueryContext(ctx, `SELECT j.id FROM river_job j WHERE j.kind='post.rag' AND j.state IN ('available','running','retryable','scheduled','pending') AND NOT EXISTS(SELECT 1 FROM post_rag r JOIN rag_documents d ON d.id=r.revision JOIN posts p ON p.id=r.post_id WHERE r.revision::text=j.args->>'revision' AND d.state='pending' AND d.deadline_at>clock_timestamp() AND p.status='published' AND p.type='link') LIMIT 100`)
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
	_, err = db.ExecContext(ctx, `DELETE FROM river_job WHERE id IN (SELECT j.id FROM river_job j WHERE j.kind='post.rag' AND j.state IN ('completed','cancelled','discarded') AND NOT EXISTS (SELECT 1 FROM post_rag r JOIN rag_documents d ON d.id=r.revision WHERE r.revision::text=j.args->>'revision' AND d.state='pending') LIMIT 100)`)
	if err != nil {
		return err
	}
	return PurgeRAG(ctx, db, index)
}

// Tombstones are retained and periodically swept again: even an ambiguous
// network timeout/process death during upsert cannot resurrect a retired index.
func PurgeRAG(ctx context.Context, db *sql.DB, index *search.Index) error {
	rows, err := db.QueryContext(ctx, `SELECT d.id FROM rag_documents d WHERE (d.state IN ('failed','obsolete') OR NOT EXISTS(SELECT 1 FROM post_rag r JOIN posts p ON p.id=r.post_id WHERE r.revision=d.id AND p.status='published' AND p.type='link')) AND (d.purged_at IS NULL OR d.purged_at<now()-interval '1 hour') ORDER BY d.purged_at NULLS FIRST LIMIT 10`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
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
		release, locked, e := generationLock(ctx, db, id)
		if e != nil {
			return e
		}
		if !locked {
			continue
		}
		e = index.Delete(ctx, id)
		if e == nil {
			_, e = db.ExecContext(ctx, `UPDATE rag_documents SET state=CASE WHEN state='failed' THEN 'failed' ELSE 'obsolete' END,title='',text='',chunks='[]',source_url='',purged_at=now() WHERE id=$1`, id)
		}
		release()
		if e != nil {
			return e
		}
	}
	return nil
}
