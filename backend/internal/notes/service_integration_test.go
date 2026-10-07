//go:build integration

package notes

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/uptrace/bun"
	"saveboard/internal/jobs"
	platformdb "saveboard/internal/platform/db"
	"testing"
	"time"
)

func TestTransactionalEnqueueAndWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("saveboard_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Log(err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	database, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := platformdb.Migrate(ctx, database.DB); err != nil {
		t.Fatal(err)
	}
	// Re-running the one-shot process must be harmless.
	if err := platformdb.Migrate(ctx, database.DB); err != nil {
		t.Fatal(err)
	}
	producer, err := jobs.Producer(database.DB)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{DB: database, Queue: producer}
	owner := uuid.New()
	if _, err := database.DB.ExecContext(ctx, `INSERT INTO users(id, display_name) VALUES ($1, 'Test')`, owner); err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("deliberate rollback")
	err = database.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := service.CreateTextTx(ctx, tx, owner, "Rollback", "Must not persist"); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("rollback: %v", err)
	}
	for _, table := range []string{"notes", "text_notes", "river_job"} {
		var count int
		if err := database.DB.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("rollback leaked %d rows into %s", count, table)
		}
	}
	note, err := service.CreateText(ctx, owner, "Committed", "Hello, Saveboard")
	if err != nil {
		t.Fatal(err)
	}
	var visibility, plainText string
	if err := database.DB.QueryRowContext(ctx, `SELECT n.visibility, t.plain_text FROM notes n JOIN text_notes t ON t.note_id=n.id WHERE n.id=$1`, note.ID).Scan(&visibility, &plainText); err != nil {
		t.Fatal(err)
	}
	if visibility != "private" || plainText != "Hello, Saveboard" {
		t.Fatal("incorrect persisted note")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.NoteCreatedWorker{DB: database.DB})
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Workers: workers, Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	}()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		if err := database.DB.QueryRowContext(ctx, `SELECT count(*) FROM river_job j JOIN job_receipts r ON r.job_id=j.id WHERE j.state='completed'`).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("worker did not complete committed job")
}
