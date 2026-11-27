package posts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"saveboard/internal/platform/blobstore"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	DB         *sql.DB
	Blobs      blobstore.Store
	photoSlots chan struct{}
}

func NewService(db *sql.DB) *Service { return &Service{DB: db, photoSlots: make(chan struct{}, 2)} }

type TagInput struct {
	ID    string `json:"id"`
	Title string `json:"title" maxLength:"80"`
}
type Write struct {
	IndexRAG        bool       `json:"index_rag,omitempty" default:"false"`
	SaveCopy        bool       `json:"save_copy,omitempty" default:"false"`
	CreateThumbnail bool       `json:"create_thumbnail,omitempty" default:"false"`
	Title           string     `json:"title" minLength:"1" maxLength:"200"`
	Type            string     `json:"type" enum:"note,link,photo"`
	Visibility      string     `json:"visibility" enum:"private,public"`
	Markdown        string     `json:"markdown" maxLength:"100000"`
	URL             string     `json:"url" maxLength:"4096"`
	Tags            []TagInput `json:"tags" minItems:"1" maxItems:"20"`
	PhotoIDs        []string   `json:"photo_ids" maxItems:"32"`
}
type PhotoView struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
type View struct {
	IndexRAG        bool        `json:"index_rag"`
	RAGState        string      `json:"rag_state"`
	SaveCopy        bool        `json:"save_copy"`
	CreateThumbnail bool        `json:"create_thumbnail"`
	CaptureState    string      `json:"capture_state"`
	CaptureWarning  string      `json:"capture_warning"`
	CopyURL         string      `json:"copy_url"`
	ThumbnailURL    string      `json:"thumbnail_url"`
	ID              string      `json:"id"`
	AuthorID        string      `json:"author_id"`
	Author          string      `json:"author"`
	AuthorName      string      `json:"author_name"`
	Title           string      `json:"title"`
	Type            string      `json:"type"`
	Visibility      string      `json:"visibility"`
	Status          string      `json:"status"`
	Markdown        string      `json:"markdown"`
	URL             string      `json:"url"`
	CreatedAt       time.Time   `json:"created_at"`
	Tags            []Tag       `json:"tags"`
	Photos          []PhotoView `json:"photos"`
}
type Page struct {
	Items []View `json:"items"`
	Total int    `json:"total"`
}

func active(ctx context.Context, tx *sql.Tx, owner string) error {
	var status string
	if err := tx.QueryRowContext(ctx, "SELECT status FROM users WHERE id=$1 FOR SHARE", owner).Scan(&status); err != nil {
		return err
	}
	if status != "active" {
		return ErrForbidden
	}
	return nil
}
func validate(w *Write) error {
	if w.Type != "link" && (w.SaveCopy || w.CreateThumbnail || w.IndexRAG) {
		return invalid("Копия и эскиз доступны только для ссылки")
	}
	w.Title = strings.TrimSpace(w.Title)
	if !validLabel(w.Title, 200) || len(w.Tags) < 1 || len(w.Tags) > 20 {
		return invalid("Укажите название и от 1 до 20 тегов")
	}
	if w.Visibility != "public" && w.Visibility != "private" {
		return invalid("Неверная видимость")
	}
	switch w.Type {
	case "note":
		if err := ValidateMarkdown(w.Markdown); err != nil {
			return err
		}
		if len(w.PhotoIDs) > 0 || w.URL != "" {
			return invalid("Лишние поля заметки")
		}
	case "link":
		if !safeURL(w.URL) || len(w.URL) > 4096 || w.Markdown != "" || len(w.PhotoIDs) > 0 {
			return invalid("Укажите HTTP(S) URL")
		}
	case "photo":
		if len(w.PhotoIDs) < 1 || len(w.PhotoIDs) > 32 || w.Markdown != "" || w.URL != "" {
			return invalid("Добавьте от 1 до 32 фото")
		}
	default:
		return invalid("Неверный тип")
	}
	for _, t := range w.Tags {
		if t.ID != "" {
			if _, err := uuid.Parse(t.ID); err != nil {
				return invalid("Неверный тег")
			}
		} else if !validLabel(strings.TrimSpace(t.Title), 80) {
			return invalid("Название тега: 1–80 символов")
		}
	}
	seen := map[string]bool{}
	for _, id := range w.PhotoIDs {
		if _, e := uuid.Parse(id); e != nil || seen[id] {
			return invalid("Неверные или повторные фото")
		}
		seen[id] = true
	}
	return nil
}
func (s *Service) Save(ctx context.Context, owner, id string, w Write) (View, error) {
	if err := validate(&w); err != nil {
		return View{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return View{}, err
	}
	defer tx.Rollback()
	if err = active(ctx, tx, owner); err != nil {
		return View{}, err
	}
	if id == "" {
		id = uuid.NewString()
		_, err = tx.ExecContext(ctx, "INSERT INTO posts(id,author_id,title,type,visibility,status) VALUES($1,$2,$3,$4,$5,'published')", id, owner, w.Title, w.Type, w.Visibility)
	} else {
		var author string
		err = tx.QueryRowContext(ctx, "SELECT author_id FROM posts WHERE id=$1 FOR UPDATE", id).Scan(&author)
		if errors.Is(err, sql.ErrNoRows) {
			return View{}, ErrNotFound
		}
		if err != nil {
			return View{}, err
		}
		if author != owner {
			return View{}, ErrForbidden
		}
		for _, table := range []string{"post_notes", "post_links", "post_photos", "post_tag_links"} {
			if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE post_id=$1", id); err != nil {
				return View{}, err
			}
		}
		_, err = tx.ExecContext(ctx, "UPDATE posts SET title=$2,type=$3,visibility=$4 WHERE id=$1", id, w.Title, w.Type, w.Visibility)
	}
	if err != nil {
		return View{}, err
	}
	// Editing a draft does not republish it. DELETE remains effective until a future drafts workflow.
	switch w.Type {
	case "note":
		_, err = tx.ExecContext(ctx, "INSERT INTO post_notes(post_id,text) VALUES($1,$2)", id, w.Markdown)
	case "link":
		_, err = tx.ExecContext(ctx, "INSERT INTO post_links(post_id,url) VALUES($1,$2)", id, w.URL)
	case "photo":
		for i, pid := range w.PhotoIDs {
			var got string
			err = tx.QueryRowContext(ctx, "UPDATE post_uploads SET post_id=$1 WHERE id=$2 AND owner_id=$3 AND (post_id IS NULL OR post_id=$1) RETURNING id", id, pid, owner).Scan(&got)
			if errors.Is(err, sql.ErrNoRows) {
				return View{}, invalid("Фото недоступно")
			}
			if err != nil {
				return View{}, err
			}
			_, err = tx.ExecContext(ctx, "INSERT INTO post_photos(id,post_id,url,position) VALUES($1,$2,$3,$4)", pid, id, "/api/v1/photos/"+pid, i)
			if err != nil {
				return View{}, err
			}
		}
	}
	if err != nil {
		return View{}, err
	}
	// Remove no-longer-used attachments, including after a change of type.
	if _, err = tx.ExecContext(ctx, "DELETE FROM post_uploads WHERE post_id=$1 AND id NOT IN (SELECT id FROM post_photos WHERE post_id=$1)", id); err != nil {
		return View{}, err
	}
	// Serialize tag creation so equal titles and transliteration collisions remain deterministic.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(726184901)"); err != nil {
		return View{}, err
	}
	for _, t := range w.Tags {
		tid := t.ID
		if tid != "" {
			var exists bool
			err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM post_tags WHERE id=$1)", tid).Scan(&exists)
			if err != nil {
				return View{}, err
			}
			if !exists {
				return View{}, invalid("Тег не существует")
			}
		} else {
			title := strings.TrimSpace(t.Title)
			err = tx.QueryRowContext(ctx, "SELECT id FROM post_tags WHERE lower(title)=lower($1) ORDER BY id LIMIT 1", title).Scan(&tid)
			if errors.Is(err, sql.ErrNoRows) {
				tid = uuid.NewString()
				slug := Slug(title)
				if slug == "" {
					slug = "tag"
				}
				base := slug
				for n := 1; ; n++ {
					var exists bool
					if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM post_tags WHERE name=$1)", slug).Scan(&exists); err != nil {
						return View{}, err
					}
					if !exists {
						break
					}
					slug = fmt.Sprintf("%s-%d", base, n+1)
				}
				_, err = tx.ExecContext(ctx, "INSERT INTO post_tags(id,name,title) VALUES($1,$2,$3)", tid, slug, title)
			}
			if err != nil {
				return View{}, err
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO post_tag_links(post_id,tag_id) VALUES($1,$2) ON CONFLICT DO NOTHING", id, tid); err != nil {
			return View{}, err
		}
	}
	if err = s.saveCapture(ctx, tx, id, w); err != nil {
		return View{}, err
	}
	if err = s.saveRAG(ctx, tx, id, owner, w); err != nil {
		return View{}, err
	}
	if err = tx.Commit(); err != nil {
		return View{}, err
	}
	return s.Get(ctx, owner, id)
}
func (s *Service) Draft(ctx context.Context, owner, id string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = active(ctx, tx, owner); err != nil {
		return err
	}
	var author string
	err = tx.QueryRowContext(ctx, "SELECT author_id FROM posts WHERE id=$1 FOR UPDATE", id).Scan(&author)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if author != owner {
		return ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, "UPDATE posts SET status='draft' WHERE id=$1", id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM post_rag WHERE post_id=$1", id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Get(ctx context.Context, viewer, id string) (View, error) {
	v := View{Tags: []Tag{}, Photos: []PhotoView{}}
	err := s.DB.QueryRowContext(ctx, `SELECT p.id,p.author_id,u.username,u.display_name,p.title,p.type,p.visibility,p.status,p.created_at,COALESCE(n.text,''),COALESCE(l.url,'') FROM posts p JOIN users u ON u.id=p.author_id LEFT JOIN post_notes n ON n.post_id=p.id LEFT JOIN post_links l ON l.post_id=p.id WHERE p.id=$1 AND (p.author_id::text=$2 OR (p.status='published' AND p.visibility='public'))`, id, viewer).Scan(&v.ID, &v.AuthorID, &v.Author, &v.AuthorName, &v.Title, &v.Type, &v.Visibility, &v.Status, &v.CreatedAt, &v.Markdown, &v.URL)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT t.id,t.name,t.title FROM post_tags t JOIN post_tag_links l ON l.tag_id=t.id WHERE l.post_id=$1 ORDER BY lower(t.title),t.id", id)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var t Tag
		if err = rows.Scan(&t.ID, &t.Name, &t.Title); err != nil {
			rows.Close()
			return v, err
		}
		v.Tags = append(v.Tags, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return v, err
	}
	rows, err = s.DB.QueryContext(ctx, "SELECT id,url FROM post_photos WHERE post_id=$1 ORDER BY position,id", id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var p PhotoView
		if err = rows.Scan(&p.ID, &p.URL); err != nil {
			return v, err
		}
		v.Photos = append(v.Photos, p)
	}
	if err = rows.Err(); err != nil {
		return v, err
	}
	rows.Close()
	err = s.captureView(ctx, &v)
	if err == nil {
		err = s.ragView(ctx, &v)
	}
	return v, err
}
func (s *Service) List(ctx context.Context, viewer, scope, kind, q, sort string, page, size int, included, excluded, author string) (Page, error) {
	out := Page{Items: []View{}}
	for _, list := range []string{included, excluded} {
		if list == "" {
			continue
		}
		ids := strings.Split(list, ",")
		if len(ids) > 20 {
			return out, ErrInvalid
		}
		for _, id := range ids {
			if _, err := uuid.Parse(id); err != nil || len(id) != 36 {
				return out, ErrInvalid
			}
		}
	}
	where := ` FROM posts p JOIN users u ON u.id=p.author_id WHERE ($7='' OR lower(u.username)=lower($7)) AND p.status='published' AND (($1='mine' AND p.author_id::text=$2) OR ($1='public' AND p.visibility='public')) AND ($3='' OR p.type=$3) AND ($4='' OR strpos(lower(p.title),lower($4))>0 OR strpos(lower(COALESCE((SELECT text FROM post_notes WHERE post_id=p.id),'')),lower($4))>0)
	AND NOT EXISTS (SELECT 1 FROM unnest(string_to_array($5, ',')::uuid[]) wanted(id) WHERE NOT EXISTS (SELECT 1 FROM post_tag_links l WHERE l.post_id=p.id AND l.tag_id=wanted.id))
	AND NOT EXISTS (SELECT 1 FROM post_tag_links l WHERE l.post_id=p.id AND l.tag_id=ANY(string_to_array($6, ',')::uuid[]))`
	args := []any{scope, viewer, kind, q, included, excluded, author}
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*)"+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	order := " DESC"
	if sort == "old" {
		order = " ASC"
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT p.id"+where+" ORDER BY p.created_at"+order+",p.id"+order+" LIMIT $8 OFFSET $9", append(args, size, (page-1)*size)...)
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	for _, id := range ids {
		v, e := s.Get(ctx, viewer, id)
		if errors.Is(e, ErrNotFound) {
			continue
		}
		if e != nil {
			return out, e
		}
		if v.Status == "published" {
			out.Items = append(out.Items, v)
		}
	}
	return out, nil
}
func (s *Service) Tags(ctx context.Context, q string) ([]Tag, error) {
	result := []Tag{}
	q = strings.TrimSpace(q)
	// Escape LIKE metacharacters: user input is a literal prefix.
	prefix := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(q)
	rows, err := s.DB.QueryContext(ctx, "SELECT id,name,title FROM post_tags WHERE lower(title) LIKE lower($1) || '%' OR lower(name) LIKE lower($1) || '%' ORDER BY (lower(title)=lower($2) OR lower(name)=lower($2)) DESC,lower(title),id LIMIT 12", prefix, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t Tag
		if err = rows.Scan(&t.ID, &t.Name, &t.Title); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
func (s *Service) Upload(ctx context.Context, owner string, data []byte) (PhotoView, error) {
	select {
	case s.photoSlots <- struct{}{}:
		defer func() { <-s.photoSlots }()
	case <-ctx.Done():
		return PhotoView{}, ctx.Err()
	}
	normalized, mime, err := NormalizePhoto(data)
	if err != nil {
		return PhotoView{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return PhotoView{}, err
	}
	defer tx.Rollback()
	// Owner lock serializes quota accounting and blocks disabled users.
	var status string
	if err = tx.QueryRowContext(ctx, "SELECT status FROM users WHERE id=$1 FOR UPDATE", owner).Scan(&status); err != nil {
		return PhotoView{}, err
	}
	if status != "active" {
		return PhotoView{}, ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM post_uploads WHERE owner_id=$1 AND post_id IS NULL AND created_at<now()-interval '24 hours'", owner); err != nil {
		return PhotoView{}, err
	}
	var used int64
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(sum(octet_length(content)),0) FROM post_uploads WHERE owner_id=$1", owner).Scan(&used); err != nil {
		return PhotoView{}, err
	}
	if used+int64(len(normalized)) > 256<<20 {
		return PhotoView{}, invalid("Лимит фотографий пользователя — 256 МБ")
	}
	id := uuid.NewString()
	if _, err = tx.ExecContext(ctx, "INSERT INTO post_uploads(id,owner_id,content,media_type) VALUES($1,$2,$3,$4)", id, owner, normalized, mime); err != nil {
		return PhotoView{}, err
	}
	if err = tx.Commit(); err != nil {
		return PhotoView{}, err
	}
	return PhotoView{id, "/api/v1/photos/" + id}, nil
}
func (s *Service) Photo(ctx context.Context, viewer, id string) ([]byte, string, error) {
	var data []byte
	var mime string
	err := s.DB.QueryRowContext(ctx, `SELECT a.content,a.media_type FROM post_uploads a LEFT JOIN posts p ON p.id=a.post_id WHERE a.id=$1 AND (a.owner_id::text=$2 OR (p.status='published' AND p.visibility='public'))`, id, viewer).Scan(&data, &mime)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return data, mime, err
}
