package jobs

import (
	"context"
	"database/sql"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
)

type NoteCreatedArgs struct {
	NoteID uuid.UUID `json:"note_id"`
}

func (NoteCreatedArgs) Kind() string { return "note.created" }

func Producer(database *sql.DB) (*river.Client[*sql.Tx], error) {
	return river.NewClient(riverdatabasesql.New(database), &river.Config{})
}

type NoteCreatedWorker struct {
	river.WorkerDefaults[NoteCreatedArgs]
	DB *sql.DB
}

// A durable idempotent receipt verifies the queue boundary. Capture and indexing
// get their own job kinds after their P0 gates; this does not pretend to run them.
func (w *NoteCreatedWorker) Work(ctx context.Context, job *river.Job[NoteCreatedArgs]) error {
	_, err := w.DB.ExecContext(ctx, `INSERT INTO job_receipts (job_id, kind) VALUES ($1, $2) ON CONFLICT (job_id) DO NOTHING`, job.ID, job.Args.Kind())
	return err
}
