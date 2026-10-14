package posts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"io"
	"saveboard/internal/jobs"
)

func (s *Service) saveCapture(ctx context.Context, tx *sql.Tx, id string, w Write) error {
	if w.Type != "link" || (!w.SaveCopy && !w.CreateThumbnail) {
		_, err := tx.ExecContext(ctx, "DELETE FROM post_captures WHERE post_id=$1", id)
		return err
	}
	var address, state string
	var copyPage, thumb bool
	err := tx.QueryRowContext(ctx, "SELECT url,save_copy,create_thumbnail,state FROM post_captures WHERE post_id=$1", id).Scan(&address, &copyPage, &thumb, &state)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && address == w.URL && copyPage == w.SaveCopy && thumb == w.CreateThumbnail && state != "failed" {
		return nil
	}
	revision := uuid.NewString()
	_, err = tx.ExecContext(ctx, `INSERT INTO post_captures(post_id,revision,url,save_copy,create_thumbnail) VALUES($1,$2,$3,$4,$5) ON CONFLICT(post_id) DO UPDATE SET revision=$2,url=$3,save_copy=$4,create_thumbnail=$5,state='pending',manifest=NULL,thumbnail_key=NULL,created_at=now(),deadline_at=now()+interval '15 minutes',completed_at=NULL`, id, revision, w.URL, w.SaveCopy, w.CreateThumbnail)
	if err != nil {
		return err
	}
	queue, err := jobs.Producer(s.DB)
	if err != nil {
		return err
	}
	_, err = queue.InsertTx(ctx, tx, jobs.CaptureArgs{PostID: id, Revision: revision}, &river.InsertOpts{MaxAttempts: 3, Queue: "capture"})
	return err
}

func (s *Service) captureView(ctx context.Context, v *View) error {
	if v.Type != "link" {
		return nil
	}
	var state string
	err := s.DB.QueryRowContext(ctx, "SELECT save_copy,create_thumbnail,state,COALESCE(manifest->>'warning','') FROM post_captures WHERE post_id=$1", v.ID).Scan(&v.SaveCopy, &v.CreateThumbnail, &state, &v.CaptureWarning)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	v.CaptureState = state
	if state == "ready" {
		if v.SaveCopy {
			v.CopyURL = "/api/v1/posts/" + v.ID + "/copy"
		}
		if v.CreateThumbnail {
			v.ThumbnailURL = "/api/v1/posts/" + v.ID + "/thumbnail"
		}
	}
	return nil
}
func (s *Service) captured(ctx context.Context, viewer, id string) (jobs.CaptureManifest, string, error) {
	var manifest jobs.CaptureManifest
	var data []byte
	var thumbnail string
	err := s.DB.QueryRowContext(ctx, `SELECT c.manifest,COALESCE(c.thumbnail_key,'') FROM post_captures c JOIN posts p ON p.id=c.post_id WHERE c.post_id=$1 AND c.state='ready' AND p.type='link' AND (p.author_id::text=$2 OR (p.status='published' AND p.visibility='public'))`, id, viewer).Scan(&data, &thumbnail)
	if errors.Is(err, sql.ErrNoRows) {
		return manifest, "", ErrNotFound
	}
	if err != nil {
		return manifest, "", err
	}
	err = json.Unmarshal(data, &manifest)
	return manifest, thumbnail, err
}
func (s *Service) readBlob(ctx context.Context, key string) ([]byte, error) {
	if s.Blobs == nil {
		return nil, errors.New("archive storage unavailable")
	}
	r, err := s.Blobs.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 32<<20))
}
func (s *Service) Copy(ctx context.Context, viewer, id string) (jobs.CaptureResult, error) {
	out := jobs.CaptureResult{Assets: map[string]jobs.CaptureAsset{}}
	m, _, err := s.captured(ctx, viewer, id)
	if err != nil {
		return out, err
	}
	if m.HTML == "" {
		return out, ErrNotFound
	}
	html, err := s.readBlob(ctx, m.HTML)
	if err != nil {
		return out, err
	}
	out.HTML = string(html)
	for id, a := range m.Assets {
		data, e := s.readBlob(ctx, a.Key)
		if e != nil {
			return out, e
		}
		out.Assets[id] = jobs.CaptureAsset{MIME: a.MIME, Data: data, Path: a.Path}
	}
	return out, nil
}
func (s *Service) Thumbnail(ctx context.Context, viewer, id string) ([]byte, error) {
	_, key, err := s.captured(ctx, viewer, id)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrNotFound
	}
	return s.readBlob(ctx, key)
}
