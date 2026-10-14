package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/riverqueue/river"
	"saveboard/internal/platform/blobstore"
)

type CaptureArgs struct {
	PostID   string `json:"post_id"`
	Revision string `json:"revision"`
}

func (CaptureArgs) Kind() string { return "post.capture" }

type CaptureAsset struct {
	MIME string `json:"mime"`
	Data []byte `json:"data"`
	Path string `json:"path"`
}
type CaptureResult struct {
	Warning   string                  `json:"warning"`
	HTML      string                  `json:"html"`
	Assets    map[string]CaptureAsset `json:"assets"`
	Thumbnail []byte                  `json:"thumbnail"`
}
type StoredAsset struct {
	MIME string `json:"mime"`
	Key  string `json:"key"`
	Path string `json:"path"`
}
type CaptureManifest struct {
	HTMLPath string                 `json:"html_path"`
	Warning  string                 `json:"warning"`
	HTML     string                 `json:"html"`
	Assets   map[string]StoredAsset `json:"assets"`
}
type CaptureWorker struct {
	river.WorkerDefaults[CaptureArgs]
	DB         *sql.DB
	Store      blobstore.Store
	URL, Token string
}

func (w *CaptureWorker) Timeout(*river.Job[CaptureArgs]) time.Duration { return 2 * time.Minute }
func (w *CaptureWorker) NextRetry(job *river.Job[CaptureArgs]) time.Time {
	if job.Attempt < 2 {
		return time.Now().Add(15 * time.Second)
	}
	return time.Now().Add(time.Minute)
}
func (w *CaptureWorker) Work(ctx context.Context, job *river.Job[CaptureArgs]) error {
	var address string
	var copyPage, thumb bool
	var deadline time.Time
	err := w.DB.QueryRowContext(ctx, `SELECT c.url,c.save_copy,c.create_thumbnail,c.deadline_at FROM post_captures c JOIN posts p ON p.id=c.post_id WHERE c.post_id=$1 AND c.revision=$2 AND c.state='pending' AND p.type='link' AND p.status='published'`, job.Args.PostID, job.Args.Revision).Scan(&address, &copyPage, &thumb, &deadline)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	err = w.capture(ctx, job.Args, address, copyPage, thumb)
	if err != nil && (job.Attempt >= job.MaxAttempts || !time.Now().Before(deadline)) {
		updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		updateErr := FinishCapture(updateCtx, w.DB, job.Args, nil, "", "Не удалось создать копию и/или эскиз: исчерпаны попытки либо истёк срок обработки. Можно повторить обработку, сохранив пост ещё раз.")
		if updateErr != nil {
			return updateErr
		}
	}
	return err
}
func (w *CaptureWorker) capture(ctx context.Context, args CaptureArgs, address string, copyPage, thumb bool) error {
	body, _ := json.Marshal(map[string]any{"url": address, "save_copy": copyPage, "create_thumbnail": thumb})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL+"/v1/captures", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 100 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("capture unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("capture status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, (48<<20)+1))
	if err != nil {
		return err
	}
	if len(raw) > 48<<20 {
		return errors.New("capture too large")
	}
	var result CaptureResult
	if err = json.Unmarshal(raw, &result); err != nil {
		return errors.New("invalid capture response")
	}
	if (copyPage && result.HTML == "") || (thumb && len(result.Thumbnail) == 0) {
		return errors.New("incomplete capture")
	}
	manifest := CaptureManifest{Assets: map[string]StoredAsset{}, Warning: result.Warning}
	put := func(b []byte) (string, error) { info, e := w.Store.Put(ctx, bytes.NewReader(b)); return info.Key, e }
	if copyPage {
		manifest.HTML, err = put([]byte(result.HTML))
		if err != nil {
			return err
		}
		u, err := url.Parse(address)
		if err != nil {
			return err
		}
		hash := func(value string) string {
			h := hmac.New(sha256.New, []byte(w.Token))
			h.Write([]byte(value))
			return hex.EncodeToString(h.Sum(nil))
		}
		parts := []string{hash(u.Scheme + "://" + u.Host)}
		for _, part := range strings.Split(u.EscapedPath(), "/") {
			if part != "" {
				parts = append(parts, hash(part))
			}
		}
		parts = append(parts, hash(u.RawQuery), manifest.HTML+".html")
		manifest.HTMLPath = strings.Join(parts, "/")
		if err = w.Store.PublishAlias(ctx, manifest.HTML, manifest.HTMLPath); err != nil {
			return err
		}
		for id, a := range result.Assets {
			key, e := put(a.Data)
			if e != nil {
				return e
			}
			manifest.Assets[id] = StoredAsset{MIME: a.MIME, Key: key, Path: a.Path}
			if err = w.Store.PublishAlias(ctx, key, a.Path); err != nil {
				return err
			}
		}
	}
	var thumbnail string
	if thumb {
		thumbnail, err = put(result.Thumbnail)
		if err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	// Publish both outputs atomically, only to the exact revision that requested them.
	return FinishCapture(ctx, w.DB, args, encoded, thumbnail, "")
}
