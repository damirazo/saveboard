package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"os"
	"os/signal"
	"saveboard/internal/jobs"
	"saveboard/internal/platform/config"
	"saveboard/internal/platform/db"
	"saveboard/internal/platform/telemetry"
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
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Workers: workers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	shutdownCtx, finish := context.WithTimeout(context.Background(), 20*time.Second)
	defer finish()
	return client.Stop(shutdownCtx)
}
