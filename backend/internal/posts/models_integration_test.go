//go:build integration

package posts

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"saveboard/internal/identity"
	platformdb "saveboard/internal/platform/db"
	"saveboard/migrations"
)

func TestPostSchema(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18.6-trixie@sha256:74935e72241653ca55e0414067e6d8763aceb8a810eb51b452253ec3dcfc4336", postgres.WithDatabase("posts_test"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(context.Background()); err != nil {
			t.Log(err)
		}
	})
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db.DB, migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	u := identity.User{Username: "author", DisplayName: "Author"}
	if _, err := db.NewInsert().Model(&u).Returning("*").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	insert := func(model any) {
		t.Helper()
		if _, err := db.NewInsert().Model(model).Returning("*").Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	note := Post{AuthorID: u.ID, Type: TypeNote}
	insert(&note)
	if note.ID == uuid.Nil || note.CreatedAt.IsZero() || note.UpdatedAt.IsZero() || note.Status != StatusDraft || note.Visibility != VisibilityPrivate {
		t.Fatalf("missing defaults: %+v", note)
	}
	insert(&Note{PostID: note.ID, Text: "A note"})
	link := Post{AuthorID: u.ID, Type: TypeLink, Status: StatusPublished, Visibility: VisibilityUnlisted}
	insert(&link)
	insert(&Link{PostID: link.ID, URL: "https://example.com"})
	photo := Post{AuthorID: u.ID, Type: TypePhoto}
	insert(&photo)
	for _, url := range []string{"https://example.com/1.jpg", "https://example.com/2.jpg"} {
		insert(&Photo{PostID: photo.ID, URL: url})
	}
	tag := Tag{Name: "travel", Title: "Travel"}
	insert(&tag)
	insert(&PostTag{PostID: photo.ID, TagID: tag.ID})
	insert(&PostTag{PostID: note.ID, TagID: tag.ID})
	if err := db.NewSelect().Model(&photo).WherePK().Relation("Photos").Relation("TagLinks.Tag").Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if len(photo.Photos) != 2 || len(photo.TagLinks) != 1 || photo.TagLinks[0].Tag.Name != tag.Name {
		t.Fatalf("incorrect relations: %+v", photo)
	}
	if photo.Photos[0].ID == uuid.Nil || photo.Photos[0].ID == photo.Photos[1].ID {
		t.Fatal("photos need distinct UUIDs")
	}
	if err := db.NewSelect().Model(&note).WherePK().Relation("Note").Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if note.Note == nil || note.Note.Text != "A note" {
		t.Fatal("note did not round trip")
	}
	if err := db.NewSelect().Model(&link).WherePK().Relation("Link").Relation("TagLinks").Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if link.Link == nil || link.Link.URL != "https://example.com" || len(link.TagLinks) != 0 {
		t.Fatal("link without tags did not round trip")
	}
	for _, query := range []string{
		`UPDATE posts SET status = 'invalid'`,
		`UPDATE posts SET type = 'invalid'`,
		`UPDATE posts SET visibility = 'invalid'`,
		`UPDATE posts SET author_id = gen_random_uuid()`,
		`INSERT INTO post_tags(name, title) VALUES ('travel', 'Duplicate')`,
		`INSERT INTO post_notes(post_id) SELECT id FROM posts WHERE type = 'photo'`,
		`INSERT INTO post_photos(post_id, url) SELECT id, 'photo.jpg' FROM posts WHERE type = 'note'`,
		`INSERT INTO post_notes(post_id) SELECT id FROM posts WHERE type = 'note'`,
		`INSERT INTO post_tag_links SELECT * FROM post_tag_links`,
		`INSERT INTO post_tag_links VALUES (gen_random_uuid(), gen_random_uuid())`,
		`UPDATE post_links SET url = ''`,
		`UPDATE post_photos SET url = ' '`,
	} {
		if _, err := db.DB.ExecContext(ctx, query); err == nil {
			t.Fatalf("accepted invalid data: %s", query)
		}
	}
	before := note.UpdatedAt
	if _, err := db.DB.ExecContext(ctx, `UPDATE post_notes SET text = 'Edited' WHERE post_id = $1`, note.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.NewSelect().Model(&note).WherePK().Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if !note.UpdatedAt.After(before) {
		t.Fatal("content edit did not update post timestamp")
	}
	if _, err := db.NewDelete().Model(&photo).WherePK().Exec(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"post_photos", "post_tag_links"} {
		count, err := db.NewSelect().Table(table).Where("post_id = ?", photo.ID).Count(ctx)
		if err != nil || count != 0 {
			t.Fatalf("cascade for %s: count=%d, err=%v", table, count, err)
		}
	}
	if _, err := db.NewDelete().Model(&tag).WherePK().Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := db.NewSelect().Model((*PostTag)(nil)).Count(ctx); err != nil || count != 0 {
		t.Fatalf("tag cascade: count=%d, err=%v", count, err)
	}
	if _, err := p.DownTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
}
