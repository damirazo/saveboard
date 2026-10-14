package jobs

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"github.com/riverqueue/river"
	"io"
	"net/http"
	modelconfig "saveboard/config"
	"saveboard/internal/notifications"
	"saveboard/internal/search"
	"time"
	"unicode/utf8"
)

type RAGArgs struct {
	PostID   string `json:"post_id"`
	Revision string `json:"revision"`
}

func (RAGArgs) Kind() string { return "post.rag" }

type RAGWorker struct {
	river.WorkerDefaults[RAGArgs]
	DB         *sql.DB
	Index      *search.Index
	URL, Token string
}

func (w *RAGWorker) Timeout(*river.Job[RAGArgs]) time.Duration { return 5 * time.Minute }
func (w *RAGWorker) NextRetry(job *river.Job[RAGArgs]) time.Time {
	if job.Attempt < 2 {
		return time.Now().Add(15 * time.Second)
	}
	return time.Now().Add(time.Minute)
}

// The generation lock serializes indexing and cleanup, without locking CRUD.
func generationLock(ctx context.Context, db *sql.DB, revision string) (func(), bool, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, revision).Scan(&locked); err != nil || !locked {
		conn.Close()
		return nil, false, err
	}
	return func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, e := conn.ExecContext(c, `SELECT pg_advisory_unlock(hashtextextended($1,0))`, revision); e != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}, true, nil
}
func (w *RAGWorker) Work(ctx context.Context, job *river.Job[RAGArgs]) error {
	release, locked, err := generationLock(ctx, w.DB, job.Args.Revision)
	if err != nil {
		return err
	}
	if !locked {
		return river.JobSnooze(15 * time.Second)
	}
	defer release()
	var url, owner, title, text string
	var deadline time.Time
	err = w.DB.QueryRowContext(ctx, `SELECT d.source_url,d.owner_id,d.title,d.text,d.deadline_at FROM rag_documents d JOIN post_rag r ON r.revision=d.id JOIN posts p ON p.id=r.post_id WHERE d.id=$1 AND d.post_id=$2 AND d.state='pending' AND p.status='published' AND p.type='link'`, job.Args.Revision, job.Args.PostID).Scan(&url, &owner, &title, &text, &deadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	err = w.index(ctx, job.Args, url, owner, title, text)
	if err != nil && (job.Attempt >= job.MaxAttempts || !time.Now().Before(deadline)) {
		c, done := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer done()
		if e := finishRAG(c, w.DB, job.Args, true); e != nil {
			return e
		}
	}
	return err
}
func (w *RAGWorker) index(ctx context.Context, args RAGArgs, url, owner, title, text string) error {
	if err := w.Index.VerifyModel(ctx); err != nil {
		return err
	}
	if text == "" {
		raw, _ := json.Marshal(map[string]string{"url": url})
		req, err := http.NewRequestWithContext(ctx, "POST", w.URL+"/v1/extract", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+w.Token)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 100 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		res, err := client.Do(req)
		if err != nil {
			return errors.New("text extraction unavailable")
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return errors.New("text extraction failed")
		}
		data, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
		if err != nil {
			return err
		}
		if len(data) > 1<<20 {
			return errors.New("article too large")
		}
		var article struct{ Title, Text string }
		if err = json.Unmarshal(data, &article); err != nil {
			return errors.New("invalid article")
		}
		if !utf8.ValidString(article.Text) || utf8.RuneCountInString(article.Text) < 80 || utf8.RuneCountInString(article.Text) > 100000 {
			return errors.New("article outside limits")
		}
		title, text = article.Title, article.Text
	}
	chunks := search.Chunks(text)
	encoded, _ := json.Marshal(chunks)
	// Persist the exact extracted document before external writes; retries reuse it.
	if _, err := w.DB.ExecContext(ctx, `UPDATE rag_documents SET title=$2,text=$3,chunks=$4,model_digest=$5,collection=$6 WHERE id=$1`, args.Revision, title, text, string(encoded), modelconfig.Model().Digest, search.Collection); err != nil {
		return err
	}
	if err := w.Index.Ensure(ctx); err != nil {
		return err
	}
	for start := 0; start < len(chunks); start += 8 {
		var current bool
		if err := w.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM post_rag r JOIN posts p ON p.id=r.post_id JOIN rag_documents d ON d.id=r.revision WHERE r.revision=$1 AND p.status='published' AND p.type='link' AND d.state='pending' AND d.deadline_at>clock_timestamp())`, args.Revision).Scan(&current); err != nil {
			return err
		}
		if !current {
			return nil
		}
		end := min(start+8, len(chunks))
		vectors, err := w.Index.Embed(ctx, chunks[start:end])
		if err != nil {
			return err
		}
		points := make([]search.Point, 0, end-start)
		for j, v := range vectors {
			ordinal := start + j
			points = append(points, search.Point{ID: search.PointID(args.Revision, ordinal), Vector: v, Payload: map[string]any{"post_id": args.PostID, "owner_id": owner, "generation_id": args.Revision, "chunk_index": ordinal, "text": chunks[ordinal], "title": title, "source_url": url, "model_digest": modelconfig.Model().Digest, "chunker_version": "runes-1600-overlap-200-v1"}})
		}
		if err = w.Index.Upsert(ctx, points); err != nil {
			return err
		}
	}
	return finishRAG(ctx, w.DB, args, false)
}
func finishRAG(ctx context.Context, db *sql.DB, args RAGArgs, failed bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner string
	err = tx.QueryRowContext(ctx, `SELECT p.author_id FROM posts p JOIN post_rag r ON r.post_id=p.id JOIN rag_documents d ON d.id=r.revision WHERE p.id=$1 AND r.revision=$2 AND p.type='link' AND p.status='published' AND d.state='pending' AND ($3 OR d.deadline_at>clock_timestamp()) FOR UPDATE OF p,d`, args.PostID, args.Revision, failed).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	// The first SELECT may have waited for an edit's post lock. Read the active
	// pointer again with a fresh READ COMMITTED snapshot while holding that lock.
	var current bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM post_rag WHERE post_id=$1 AND revision=$2)`, args.PostID, args.Revision).Scan(&current); err != nil {
		return err
	}
	if !current {
		return nil
	}
	state, importance, title, text := "ready", "normal", "Индексация для RAG завершена", "Текст страницы очищен и сохранён в поисковом индексе."
	if failed {
		state, importance, title, text = "failed", "critical", "Ошибка индексации для RAG", "Не удалось извлечь текст или сохранить индекс: исчерпаны попытки либо истёк срок обработки. Сохраните пост ещё раз для повторной попытки."
	}
	if _, err = tx.ExecContext(ctx, `UPDATE rag_documents SET state=$2,completed_at=now() WHERE id=$1`, args.Revision, state); err != nil {
		return err
	}
	key := "rag:" + args.Revision
	if err = notifications.PublishTx(ctx, tx, notifications.Event{RecipientID: &owner, Title: title, Text: text, Importance: importance, PostID: &args.PostID, Key: &key}); err != nil {
		return err
	}
	return tx.Commit()
}
