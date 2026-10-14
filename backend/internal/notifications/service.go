package notifications

import (
	"context"
	"database/sql"
	"time"
)

// Event is an internal service event. A nil recipient broadcasts to existing users.
type Event struct {
	RecipientID             *string
	Title, Text, Importance string
	PostID                  *string
	Key                     *string
}

// PublishTx makes delivery atomic with the business event; Key deduplicates retries.
func PublishTx(ctx context.Context, tx *sql.Tx, e Event) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO notifications(recipient_id,title,text,importance,post_id,event_key) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(event_key) DO NOTHING`, e.RecipientID, e.Title, e.Text, e.Importance, e.PostID, e.Key)
	return err
}

type Service struct{ DB *sql.DB }
type Notification struct {
	ID         string    `json:"id" format:"uuid"`
	CreatedAt  time.Time `json:"created_at"`
	Title      string    `json:"title"`
	Text       string    `json:"text"`
	Importance string    `json:"importance" enum:"normal,important,critical"`
	PostID     *string   `json:"post_id" format:"uuid"`
	Read       bool      `json:"read"`
}
type NotificationPage struct {
	Items []Notification `json:"items"`
	Total int            `json:"total"`
}

const eligible = ` FROM notifications n JOIN users u ON u.id=$1 LEFT JOIN notification_reads r ON r.notification_id=n.id AND r.user_id=u.id WHERE n.created_at>u.created_at AND (n.recipient_id IS NULL OR n.recipient_id=u.id)`
const columns = `n.id,n.created_at,n.title,n.text,n.importance,n.post_id,(r.user_id IS NOT NULL)`

func scan(row interface{ Scan(...any) error }) (n Notification, err error) {
	err = row.Scan(&n.ID, &n.CreatedAt, &n.Title, &n.Text, &n.Importance, &n.PostID, &n.Read)
	return
}
func (s *Service) List(ctx context.Context, user string, page, size int) (NotificationPage, error) {
	out := NotificationPage{Items: []Notification{}}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*)`+eligible, user).Scan(&out.Total); err != nil {
		return out, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+columns+eligible+` ORDER BY n.created_at DESC,n.id DESC LIMIT $2 OFFSET $3`, user, size, (page-1)*size)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		n, e := scan(rows)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, n)
	}
	return out, rows.Err()
}
func (s *Service) Unread(ctx context.Context, user string) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*)`+eligible+` AND r.user_id IS NULL`, user).Scan(&count)
	return count, err
}
func (s *Service) Get(ctx context.Context, user, id string) (Notification, error) {
	return scan(s.DB.QueryRowContext(ctx, `SELECT `+columns+eligible+` AND n.id=$2`, user, id))
}
func (s *Service) Read(ctx context.Context, user, id string) error {
	// RETURNING also returns an already-read row, without distinguishing inaccessible IDs.
	var found string
	return s.DB.QueryRowContext(ctx, `INSERT INTO notification_reads(notification_id,user_id) SELECT n.id,u.id`+eligible+` AND n.id=$2 ON CONFLICT(notification_id,user_id) DO UPDATE SET read_at=notification_reads.read_at RETURNING notification_id`, user, id).Scan(&found)
}
