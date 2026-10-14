//go:build integration

package posts

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"net/http"
	"net/http/httptest"
	"saveboard/internal/jobs"
	"saveboard/internal/platform/blobstore"
	platformdb "saveboard/internal/platform/db"
	"testing"
	"time"
)

func TestLinkCaptureLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("capture_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = platformdb.Migrate(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	if _, err = db.DB.ExecContext(ctx, `INSERT INTO users(id,username,display_name,status) VALUES($1,'capture-author','Capture author','active')`, owner); err != nil {
		t.Fatal(err)
	}
	store, err := blobstore.NewLocal(t.TempDir(), 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(db.DB)
	s.Blobs = store
	w := Write{Title: "Link", Type: "link", Visibility: "public", URL: "https://example.com/a?x=1", Tags: []TagInput{{Title: "Capture"}}, SaveCopy: true, CreateThumbnail: true}
	saved, err := s.Save(ctx, owner, "", w)
	if err != nil {
		t.Fatal(err)
	}
	if saved.CaptureState != "pending" || saved.CopyURL != "" || saved.ThumbnailURL != "" {
		t.Fatalf("pending exposed results: %+v", saved)
	}
	if _, err = s.Copy(ctx, "", saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("pending copy: %v", err)
	}
	revision := func() string {
		t.Helper()
		var r string
		if e := db.DB.QueryRowContext(ctx, "SELECT revision FROM post_captures WHERE post_id=$1", saved.ID).Scan(&r); e != nil {
			t.Fatal(e)
		}
		return r
	}
	old := revision()
	var queued int
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM river_job WHERE kind='post.capture' AND args->>'post_id'=$1", saved.ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("queue count %d: %v", queued, err)
	}
	calls := 0
	fail := false
	var during func()
	remote := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		calls++
		if fail {
			rw.WriteHeader(503)
			return
		}
		if during != nil {
			during()
		}
		_ = json.NewEncoder(rw).Encode(jobs.CaptureResult{HTML: "<p>Rendered</p>", Assets: map[string]jobs.CaptureAsset{}, Thumbnail: []byte("png")})
	}))
	defer remote.Close()
	worker := jobs.CaptureWorker{DB: db.DB, Store: store, URL: remote.URL, Token: "test"}
	run := func(rev string) {
		t.Helper()
		if e := worker.Work(ctx, &river.Job[jobs.CaptureArgs]{JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}, Args: jobs.CaptureArgs{PostID: saved.ID, Revision: rev}}); e != nil {
			t.Fatal(e)
		}
	}
	// Change URL while the old capture is running, not just before its initial SELECT.
	during = func() {
		w.URL = "https://example.com/b"
		if _, e := s.Save(ctx, owner, saved.ID, w); e != nil {
			t.Error(e)
		}
	}
	run(old)
	during = nil
	current := revision()
	if current == old {
		t.Fatal("URL change reused revision")
	}
	view, err := s.Get(ctx, "", saved.ID)
	if err != nil || view.CopyURL != "" {
		t.Fatal("stale capture published")
	}
	run(current)
	run(current)
	if calls != 2 {
		t.Fatalf("retry recaptured completed result: %d", calls)
	}
	view, err = s.Get(ctx, "", saved.ID)
	if err != nil || view.CopyURL == "" || view.ThumbnailURL == "" {
		t.Fatalf("ready not published: %+v %v", view, err)
	}
	var notifications int
	if err = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE post_id=$1 AND importance='normal'`, saved.ID).Scan(&notifications); err != nil || notifications != 1 {
		t.Fatalf("success notification dedup: %d %v", notifications, err)
	}
	copy, err := s.Copy(ctx, "", saved.ID)
	if err != nil || copy.HTML != "<p>Rendered</p>" {
		t.Fatalf("copy: %+v %v", copy, err)
	}
	w.Visibility = "private"
	if _, err = s.Save(ctx, owner, saved.ID, w); err != nil {
		t.Fatal(err)
	}
	if revision() != current {
		t.Fatal("metadata edit recaptured")
	}
	if _, err = s.Copy(ctx, "", saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("private copy leaked")
	}
	if _, err = s.Thumbnail(ctx, uuid.NewString(), saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("private thumbnail leaked")
	}
	if _, err = s.Copy(ctx, owner, saved.ID); err != nil {
		t.Fatal(err)
	}
	w.Type = "note"
	w.Markdown = "Text"
	w.URL = ""
	w.SaveCopy = false
	w.CreateThumbnail = false
	if _, err = s.Save(ctx, owner, saved.ID, w); err != nil {
		t.Fatal(err)
	}
	run(current)
	if _, err = s.Copy(ctx, owner, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("type change retained copy")
	}
	// Transaction rollback also removes the queued task.
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.saveCapture(ctx, tx, saved.ID, Write{Type: "link", URL: "https://example.com", SaveCopy: true}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err = db.DB.QueryRowContext(ctx, "SELECT count(*) FROM post_captures WHERE post_id=$1", saved.ID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("rollback leaked capture")
	}
	// A final failure exposes neither result; saving the same options retries with a new revision.
	w.Type = "link"
	w.Markdown = ""
	w.URL = "https://example.com/failed"
	w.SaveCopy = true
	w.CreateThumbnail = false
	if _, err = s.Save(ctx, owner, saved.ID, w); err != nil {
		t.Fatal(err)
	}
	failedRevision := revision()
	fail = true
	if err = worker.Work(ctx, &river.Job[jobs.CaptureArgs]{JobRow: &rivertype.JobRow{Attempt: 3, MaxAttempts: 3}, Args: jobs.CaptureArgs{PostID: saved.ID, Revision: failedRevision}}); err == nil {
		t.Fatal("failed capture succeeded")
	}
	view, err = s.Get(ctx, owner, saved.ID)
	if err != nil || view.CaptureState != "failed" || view.CopyURL != "" || view.ThumbnailURL != "" {
		t.Fatalf("failure state: %+v %v", view, err)
	}
	if err = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE post_id=$1 AND importance='critical'`, saved.ID).Scan(&notifications); err != nil || notifications != 1 {
		t.Fatalf("failure notification: %d %v", notifications, err)
	}
	if _, err = s.Save(ctx, owner, saved.ID, w); err != nil {
		t.Fatal(err)
	}
	if revision() == failedRevision {
		t.Fatal("failed job not retried")
	}
	fail = false
	run(revision())
	view, err = s.Get(ctx, owner, saved.ID)
	if err != nil || view.CopyURL == "" || view.ThumbnailURL != "" {
		t.Fatal("independent copy flag failed")
	}
	// Recovery runs in a new service instance after a crash. Terminal jobs and
	// expired pending captures must produce exactly one failure, then be removed.
	for _, reason := range []string{"expired", "discarded", "missing"} {
		w.URL = "https://example.com/" + reason
		if _, err = s.Save(ctx, owner, saved.ID, w); err != nil {
			t.Fatal(err)
		}
		rev := revision()
		switch reason {
		case "expired":
			_, err = db.DB.ExecContext(ctx, `UPDATE post_captures SET deadline_at=now()-interval '1 second' WHERE post_id=$1`, saved.ID)
		case "discarded":
			_, err = db.DB.ExecContext(ctx, `UPDATE river_job SET state='discarded',finalized_at=now(),attempt=max_attempts WHERE args->>'revision'=$1`, rev)
		case "missing":
			_, err = db.DB.ExecContext(ctx, `DELETE FROM river_job WHERE args->>'revision'=$1`, rev)
		}
		if err != nil {
			t.Fatal(err)
		}
		producer, e := jobs.Producer(db.DB)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i < 2; i++ {
			if e = jobs.ReconcileCaptures(ctx, db.DB, func(c context.Context, id int64) error { _, e := producer.JobCancel(c, id); return e }); e != nil {
				t.Fatal(e)
			}
		}
		// A late browser result must not overwrite the failure.
		if e = jobs.FinishCapture(ctx, db.DB, jobs.CaptureArgs{PostID: saved.ID, Revision: rev}, []byte(`{}`), "", ""); e != nil {
			t.Fatal(e)
		}
		v, e := s.Get(ctx, owner, saved.ID)
		if e != nil || v.CaptureState != "failed" || v.CopyURL != "" {
			t.Fatalf("%s: %+v %v", reason, v, e)
		}
		if e = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE event_key=$1 AND importance='critical'`, "capture:"+rev).Scan(&notifications); e != nil || notifications != 1 {
			t.Fatalf("%s notification: %d %v", reason, notifications, e)
		}
		if e = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM river_job WHERE args->>'revision'=$1`, rev).Scan(&remaining); e != nil || remaining != 0 {
			t.Fatalf("%s queue cleanup: %d %v", reason, remaining, e)
		}
	}
	if err = s.Draft(ctx, owner, saved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Copy(ctx, "", saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("draft copy leaked")
	}
}
