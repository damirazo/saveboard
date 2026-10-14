//go:build integration

package posts

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"net/http"
	"net/http/httptest"
	"saveboard/internal/jobs"
	"saveboard/internal/platform/blobstore"
	platformdb "saveboard/internal/platform/db"
	"sync/atomic"
	"testing"
	"time"
)

func TestCaptureQueueRestartAndCapacity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("capture_queue"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
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
	if _, err = db.DB.ExecContext(ctx, `INSERT INTO users(id,username,display_name,status) VALUES($1,'queue-owner','Queue owner','active')`, owner); err != nil {
		t.Fatal(err)
	}
	service := NewService(db.DB)
	for i := 0; i < 2; i++ {
		if _, err = service.Save(ctx, owner, "", Write{Title: "Queue", Type: "link", URL: "https://example.com", Visibility: "private", SaveCopy: true, Tags: []TagInput{{Title: "Queue"}}}); err != nil {
			t.Fatal(err)
		}
	}
	var requests atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var released atomic.Bool
	defer func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
	}()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_ = json.NewEncoder(w).Encode(jobs.CaptureResult{HTML: "<p>Ready</p>", Assets: map[string]jobs.CaptureAsset{}})
	}))
	defer remote.Close()
	store, err := blobstore.NewLocal(t.TempDir(), 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.CaptureWorker{DB: db.DB, Store: store, URL: remote.URL, Token: "test"})
	config := &river.Config{Workers: workers, Queues: map[string]river.QueueConfig{"capture": {MaxWorkers: 1}}, FetchPollInterval: 100 * time.Millisecond, FetchCooldown: 10 * time.Millisecond}
	first, err := river.NewClient(riverpgxv5.New(pool), config)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stop, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = first.StopAndCancel(stop)
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var running, waiting int
	if err = db.DB.QueryRowContext(ctx, `SELECT count(*) FILTER(WHERE state='running'),count(*) FILTER(WHERE state='available') FROM river_job WHERE kind='post.capture'`).Scan(&running, &waiting); err != nil || running != 1 || waiting != 1 {
		t.Fatalf("capacity running=%d waiting=%d: %v", running, waiting, err)
	}
	stop, done := context.WithTimeout(ctx, 10*time.Second)
	err = first.StopAndCancel(stop)
	done()
	if err != nil {
		t.Fatal(err)
	}
	if released.CompareAndSwap(false, true) {
		close(release)
	}
	// Expedite only the test's retry schedule; the persisted args/attempt survive.
	if _, err = db.DB.ExecContext(ctx, `UPDATE river_job SET scheduled_at=now() WHERE state='retryable'`); err != nil {
		t.Fatal(err)
	}
	second, err := river.NewClient(riverpgxv5.New(pool), config)
	if err != nil {
		t.Fatal(err)
	}
	if err = second.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stop, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = second.StopAndCancel(stop)
	}()
	deadline := time.Now().Add(40 * time.Second)
	for {
		var ready, notifications int
		err = db.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM post_captures WHERE state='ready'),(SELECT count(*) FROM notifications WHERE importance='normal')`).Scan(&ready, &notifications)
		if err != nil {
			t.Fatal(err)
		}
		if ready == 2 && notifications == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("restart failed: ready=%d notifications=%d requests=%d", ready, notifications, requests.Load())
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	if requests.Load() != 3 {
		t.Fatalf("expected interrupted attempt + two completions, got %d", requests.Load())
	}
}
