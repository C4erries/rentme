package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"rentme/internal/domain/user"
)

var (
	ErrTokenRequired   = errors.New("auth: token is required")
	ErrUserRequired    = errors.New("auth: user is required")
	ErrTTLInvalid      = errors.New("auth: ttl must be positive")
	ErrSessionNotFound = errors.New("auth: session not found")
)

type Token string

type Session struct {
	Token     Token
	UserID    user.ID
	Roles     []user.Role
	CreatedAt time.Time
	ExpiresAt time.Time
}

type CreateSessionParams struct {
	Token  Token
	UserID user.ID
	Roles  []user.Role
	TTL    time.Duration
	Now    time.Time
}

func NewSession(params CreateSessionParams) (*Session, error) {
	token := strings.TrimSpace(string(params.Token))
	if token == "" {
		return nil, ErrTokenRequired
	}
	if strings.TrimSpace(string(params.UserID)) == "" {
		return nil, ErrUserRequired
	}
	if params.TTL <= 0 {
		return nil, ErrTTLInvalid
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	return &Session{
		Token:     Token(token),
		UserID:    params.UserID,
		Roles:     append([]user.Role(nil), params.Roles...),
		CreatedAt: now,
		ExpiresAt: now.Add(params.TTL),
	}, nil
}

func (s *Session) Expired(at time.Time) bool {
	if at.IsZero() {
		at = time.Now()
	}
	return !s.ExpiresAt.After(at.UTC())
}

type SessionStore interface {
	Save(ctx context.Context, session *Session) error
	Get(ctx context.Context, token Token) (*Session, error)
	Delete(ctx context.Context, token Token) error
	DeleteByUser(ctx context.Context, userID user.ID) error
}
