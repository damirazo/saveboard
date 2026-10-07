package notes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/uptrace/bun"
	"saveboard/internal/jobs"
	"saveboard/internal/platform/telemetry"
	"strings"
	"unicode/utf8"
)

type Note struct {
	bun.BaseModel `bun:"table:notes"`
	ID            uuid.UUID `bun:"id,pk"`
	OwnerID       uuid.UUID `bun:"owner_id"`
	Kind          string    `bun:"kind"`
	Visibility    string    `bun:"visibility"`
	Title         string    `bun:"title"`
}

type Service struct {
	DB    *bun.DB
	Queue *river.Client[*sql.Tx]
}

// Internal application boundary. HTTP mutations remain unavailable until BFF
// authentication and CSRF checks exist; OwnerID must come from a verified session.
func (s *Service) CreateText(ctx context.Context, owner uuid.UUID, title, text string) (Note, error) {
	ctx, span := telemetry.Tracer().Start(ctx, "notes.CreateText")
	defer span.End()
	var note Note
	err := s.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		note, err = s.CreateTextTx(ctx, tx, owner, title, text)
		return err
	})
	if err != nil {
		return Note{}, err
	}
	return note, err
}

func (s *Service) CreateTextTx(ctx context.Context, tx bun.Tx, owner uuid.UUID, title, text string) (Note, error) {
	title = strings.TrimSpace(title)
	if owner == uuid.Nil || title == "" || utf8.RuneCountInString(title) > 300 || len(text) > 1<<20 {
		return Note{}, errors.New("invalid text note")
	}
	n := Note{ID: uuid.New(), OwnerID: owner, Kind: "text", Visibility: "private", Title: title}
	if _, err := tx.NewInsert().Model(&n).Exec(ctx); err != nil {
		return Note{}, err
	}
	content := []any{}
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	doc, err := json.Marshal(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": content}}})
	if err != nil {
		return Note{}, err
	}
	// Raw pgx SQL and Bun use the exact same transaction.
	if _, err := tx.Tx.ExecContext(ctx, `INSERT INTO text_notes(note_id, content_json, plain_text) VALUES ($1,$2,$3)`, n.ID, string(doc), text); err != nil {
		return Note{}, err
	}
	if _, err := s.Queue.InsertTx(ctx, tx.Tx, jobs.NoteCreatedArgs{NoteID: n.ID}, nil); err != nil {
		return Note{}, err
	}
	return n, nil
}
