package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"log/slog"
	"os"
	"os/signal"
	"saveboard/internal/jobs"
	"saveboard/internal/platform/blobstore"
	"saveboard/internal/platform/config"
	"saveboard/internal/platform/db"
	"saveboard/internal/platform/telemetry"
	"saveboard/internal/search"
	"syscall"
	"time"
)

func main() {
	log := telemetry.Init("worker")
	if err := run(); err != nil {
		log.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cfg, err := config.Load("worker")
	if err != nil {
		return err
	}
	connectCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	database, err := db.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.NoteCreatedWorker{DB: database.DB})
	store, err := blobstore.NewLocal(os.Getenv("BLOB_ROOT"), 32<<20)
	if err != nil {
		return err
	}
	river.AddWorker(workers, &jobs.CaptureWorker{DB: database.DB, Store: store, URL: os.Getenv("CAPTURE_URL"), Token: os.Getenv("CAPTURE_TOKEN")})
	index := &search.Index{OllamaURL: os.Getenv("OLLAMA_URL"), QdrantURL: "http://" + os.Getenv("QDRANT_HOST") + ":6333", APIKey: os.Getenv("QDRANT_API_KEY")}
	river.AddWorker(workers, &jobs.RAGWorker{DB: database.DB, Index: index, URL: os.Getenv("CAPTURE_URL"), Token: os.Getenv("CAPTURE_TOKEN")})
	stuck := make(chan struct{}, 1)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Workers: workers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}, "capture": {MaxWorkers: 1}},
		JobTimeout: 5 * time.Minute, RescueStuckJobsAfter: 6 * time.Minute, SoftStopTimeout: 10 * time.Second,
		CancelledJobRetentionPeriod: time.Hour, CompletedJobRetentionPeriod: time.Hour, DiscardedJobRetentionPeriod: time.Hour,
		JobStuckHandler: func(context.Context, river.JobStuckHandlerParams) river.JobStuckHandlerResult {
			select {
			case stuck <- struct{}{}:
			default:
			}
			return river.JobStuckHandlerResult{}
		},
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	reconcile := func() {
		checkCtx, done := context.WithTimeout(ctx, 20*time.Second)
		defer done()
		if e := jobs.ReconcileCaptures(checkCtx, database.DB, func(c context.Context, id int64) error { _, e := client.JobCancel(c, id); return e }); e != nil && ctx.Err() == nil {
			slog.Error("capture reconciliation failed", "error", e)
		}
		if e := jobs.ReconcileRAG(checkCtx, database.DB, index, func(c context.Context, id int64) error { _, e := client.JobCancel(c, id); return e }); e != nil && ctx.Err() == nil {
			slog.Error("RAG reconciliation failed", "error", e)
		}
	}
	reconcile()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var cause error
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-stuck:
			cause = errors.New("stuck worker: restart required")
			cancel()
			break loop
		case <-ticker.C:
			reconcile()
		}
	}
	shutdownCtx, finish := context.WithTimeout(context.Background(), 20*time.Second)
	defer finish()
	if err = client.Stop(shutdownCtx); err != nil {
		// A non-cooperative goroutine must not block pool.Close forever. Docker
		// restarts the process and River recovers the durable lease.
		slog.Error("worker shutdown timed out", "error", err)
		os.Exit(1)
	}
	return cause
}
