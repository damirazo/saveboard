package identity

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type PublicProfile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	PublicPosts int    `json:"public_posts"`
}

func (s *AuthService) PublicProfile(ctx context.Context, username string) (PublicProfile, error) {
	var profile PublicProfile
	err := s.db.QueryRowContext(ctx, `SELECT u.username,u.display_name,
 (SELECT count(*) FROM posts p WHERE p.author_id=u.id AND p.visibility='public' AND p.status='published')
 FROM users u WHERE lower(u.username)=lower($1) AND u.status='active' AND u.disabled_at IS NULL`, username).
		Scan(&profile.Username, &profile.DisplayName, &profile.PublicPosts)
	return profile, err
}

func (s *AuthService) UpdateDisplayName(ctx context.Context, userID uuid.UUID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrRegistrationInvalid
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET display_name=$2 WHERE id=$1`, userID, name)
	return err
}
