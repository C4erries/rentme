package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	domainauth "rentme/internal/domain/auth"
	domainuser "rentme/internal/domain/user"
)

var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrPasswordTooShort   = errors.New("auth: password must be at least 8 characters")
	ErrUserBlocked        = errors.New("auth: user blocked")
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenGenerator interface {
	NewToken() (string, error)
}

type Service struct {
	Users      domainuser.Repository
	Sessions   domainauth.SessionStore
	Passwords  PasswordHasher
	Tokens     TokenGenerator
	SessionTTL time.Duration
	Logger     *slog.Logger
}

type RegisterParams struct {
	Email      string
	Name       string
	Password   string
	WantToHost bool
}

type LoginParams struct {
	Email    string
	Password string
}

type AuthResult struct {
	User  *domainuser.User
	Token string
}

type ResolveResult struct {
	User    *domainuser.User
	Session *domainauth.Session
}

func (s *Service) Register(ctx context.Context, params RegisterParams) (*AuthResult, error) {
	if err := s.ensureDependencies(); err != nil {
		return nil, err
	}
	email := strings.TrimSpace(strings.ToLower(params.Email))
	name := strings.TrimSpace(params.Name)
	if email == "" {
		return nil, domainuser.ErrEmailRequired
	}
	if name == "" {
		return nil, domainuser.ErrNameRequired
	}
	if err := s.validatePassword(params.Password); err != nil {
		return nil, err
	}
	hash, err := s.Passwords.Hash(params.Password)
	if err != nil {
		return nil, err
	}
	roles := []domainuser.Role{domainuser.RoleGuest}
	if params.WantToHost {
		roles = append(roles, domainuser.RoleHost)
	}
	user, err := domainuser.NewUser(domainuser.CreateParams{
		ID:           domainuser.ID(uuid.NewString()),
		Email:        email,
		Name:         name,
		PasswordHash: hash,
		Roles:        roles,
		CreatedAt:    time.Now(),
	})
	if err != nil {
		return nil, err
	}
	if err := s.Users.Save(ctx, user); err != nil {
		return nil, err
	}
	token, err := s.issueSession(ctx, user)
	if err != nil {
		return nil, err
	}
	if s.Logger != nil {
		s.Logger.Info("user registered", "user_id", user.ID, "email", user.Email, "roles", user.Roles)
	}
	return &AuthResult{User: user, Token: token}, nil
}

func (s *Service) Login(ctx context.Context, params LoginParams) (*AuthResult, error) {
	if err := s.ensureDependencies(); err != nil {
		return nil, err
	}
	email := strings.TrimSpace(strings.ToLower(params.Email))
	if email == "" {
		return nil, ErrInvalidCredentials
	}
	user, err := s.Users.ByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domainuser.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.Blocked {
		return nil, ErrUserBlocked
	}
	if err := s.Passwords.Compare(user.PasswordHash, params.Password); err != nil {
		return nil, ErrInvalidCredentials
	}
	token, err := s.issueSession(ctx, user)
	if err != nil {
		return nil, err
	}
	if s.Logger != nil {
		s.Logger.Info("user authenticated", "user_id", user.ID)
	}
	return &AuthResult{User: user, Token: token}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if err := s.ensureDependencies(); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if err := s.Sessions.Delete(ctx, domainauth.Token(token)); err != nil {
		return err
	}
	if s.Logger != nil {
		s.Logger.Info("session terminated")
	}
	return nil
}

func (s *Service) ResolveToken(ctx context.Context, token string) (*ResolveResult, error) {
	if err := s.ensureDependencies(); err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, domainauth.ErrTokenRequired
	}
	session, err := s.Sessions.Get(ctx, domainauth.Token(token))
	if err != nil {
		return nil, err
	}
	user, err := s.Users.ByID(ctx, session.UserID)
	if err != nil {
		_ = s.Sessions.Delete(ctx, session.Token)
		if errors.Is(err, domainuser.ErrNotFound) {
			return nil, domainauth.ErrSessionNotFound
		}
		return nil, err
	}
	if user.Blocked {
		_ = s.Sessions.DeleteByUser(ctx, user.ID)
		return nil, ErrUserBlocked
	}
	return &ResolveResult{User: user, Session: session}, nil
}

func (s *Service) issueSession(ctx context.Context, user *domainuser.User) (string, error) {
	token, err := s.Tokens.NewToken()
	if err != nil {
		return "", err
	}
	session, err := domainauth.NewSession(domainauth.CreateSessionParams{
		Token:  domainauth.Token(token),
		UserID: user.ID,
		Roles:  append([]domainuser.Role(nil), user.Roles...),
		TTL:    s.sessionTTL(),
		Now:    time.Now(),
	})
	if err != nil {
		return "", err
	}
	if err := s.Sessions.Save(ctx, session); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) sessionTTL() time.Duration {
	if s.SessionTTL > 0 {
		return s.SessionTTL
	}
	return 24 * time.Hour
}

func (s *Service) validatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return ErrPasswordTooShort
	}
	return nil
}

func (s *Service) ensureDependencies() error {
	switch {
	case s.Users == nil:
		return errors.New("auth: user repository required")
	case s.Sessions == nil:
		return errors.New("auth: session store required")
	case s.Passwords == nil:
		return errors.New("auth: password hasher required")
	case s.Tokens == nil:
		return errors.New("auth: token generator required")
	default:
		return nil
	}
}
