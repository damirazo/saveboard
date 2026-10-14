package posts

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"saveboard/internal/jobs"
)

func (s *Service) saveRAG(ctx context.Context, tx *sql.Tx, id, owner string, w Write) error {
	var published bool
	if err := tx.QueryRowContext(ctx, `SELECT status='published' FROM posts WHERE id=$1`, id).Scan(&published); err != nil {
		return err
	}
	if !w.IndexRAG || w.Type != "link" || !published {
		_, err := tx.ExecContext(ctx, `DELETE FROM post_rag WHERE post_id=$1`, id)
		return err
	}
	var url, state string
	err := tx.QueryRowContext(ctx, `SELECT r.url,d.state FROM post_rag r JOIN rag_documents d ON d.id=r.revision WHERE r.post_id=$1`, id).Scan(&url, &state)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && url == w.URL && (state == "pending" || state == "ready") {
		return nil
	}
	revision := uuid.NewString()
	if _, err = tx.ExecContext(ctx, `INSERT INTO rag_documents(id,post_id,owner_id,source_url) VALUES($1,$2,$3,$4)`, revision, id, owner, w.URL); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO post_rag(post_id,revision,url) VALUES($1,$2,$3) ON CONFLICT(post_id) DO UPDATE SET revision=$2,url=$3`, id, revision, w.URL); err != nil {
		return err
	}
	producer, err := jobs.Producer(s.DB)
	if err != nil {
		return err
	}
	_, err = producer.InsertTx(ctx, tx, jobs.RAGArgs{PostID: id, Revision: revision}, &river.InsertOpts{MaxAttempts: 3, Queue: "capture"})
	return err
}
func (s *Service) ragView(ctx context.Context, v *View) error {
	err := s.DB.QueryRowContext(ctx, `SELECT true,d.state FROM post_rag r JOIN rag_documents d ON d.id=r.revision WHERE r.post_id=$1`, v.ID).Scan(&v.IndexRAG, &v.RAGState)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}
