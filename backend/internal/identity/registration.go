package identity

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrRegistrationInvalid = errors.New("invalid registration")
var ErrAccountExists = errors.New("username or email is already registered")
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{2,63}$`)

func validateRegistration(username, email, password string) error {
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%w: Username должен содержать 3–64 символа: латиницу, цифры, _, . или -", ErrRegistrationInvalid)
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 254 || strings.ContainsAny(email, " \t\r\n") {
		return fmt.Errorf("%w: Укажите корректный email", ErrRegistrationInvalid)
	}
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 8 || len(password) > 72 || strings.ContainsRune(password, 0) {
		return fmt.Errorf("%w: Пароль должен содержать минимум 8 символов и не более 72 байт", ErrRegistrationInvalid)
	}
	return nil
}

// Register commits credentials and the initial session together. Email ownership is not inferred.
func (s *AuthService) Register(ctx context.Context, username, email, password string) (Tokens, error) {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validateRegistration(username, email, password); err != nil {
		return Tokens{}, err
	}
	select {
	case s.passwordSlots <- struct{}{}:
		defer func() { <-s.passwordSlots }()
	default:
		return Tokens{}, ErrBusy
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Tokens{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback()
	user := uuid.New()
	_, err = tx.ExecContext(ctx, `INSERT INTO users(id,username,display_name,email,password_hash,password_changed_at,status) VALUES($1,$2,$2,$3,$4,now(),'active')`, user, username, email, hash)
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return Tokens{}, ErrAccountExists
		}
		return Tokens{}, err
	}
	pair, err := s.newSession(ctx, tx, user)
	if err != nil {
		return Tokens{}, err
	}
	if err = tx.Commit(); err != nil {
		return Tokens{}, err
	}
	return pair, nil
}
