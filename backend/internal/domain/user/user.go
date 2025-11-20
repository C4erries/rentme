package user

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrIDRequired          = errors.New("user: id is required")
	ErrEmailRequired       = errors.New("user: email is required")
	ErrPasswordHashMissing = errors.New("user: password hash is required")
	ErrNameRequired        = errors.New("user: name is required")
	ErrInvalidRole         = errors.New("user: invalid role")
	ErrEmailAlreadyUsed    = errors.New("user: email already used")
	ErrNotFound            = errors.New("user: not found")
)

type ID string

type Role string

const (
	RoleGuest Role = "guest"
	RoleHost  Role = "host"
)

// ReservedRoles lists roles reserved for internal usage.
var ReservedRoles = []Role{RoleGuest, RoleHost}

type User struct {
	ID           ID
	Email        string
	Name         string
	PasswordHash string
	Roles        []Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Repository interface {
	ByID(ctx context.Context, id ID) (*User, error)
	ByEmail(ctx context.Context, email string) (*User, error)
	Save(ctx context.Context, user *User) error
}

type CreateParams struct {
	ID           ID
	Email        string
	Name         string
	PasswordHash string
	Roles        []Role
	CreatedAt    time.Time
}

func NewUser(params CreateParams) (*User, error) {
	id := strings.TrimSpace(string(params.ID))
	if id == "" {
		return nil, ErrIDRequired
	}
	email := normalizeEmail(params.Email)
	if email == "" {
		return nil, ErrEmailRequired
	}
	if strings.TrimSpace(params.PasswordHash) == "" {
		return nil, ErrPasswordHashMissing
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return nil, ErrNameRequired
	}

	now := params.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	roles, err := normalizeRoles(params.Roles)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		roles = []Role{RoleGuest}
	}

	return &User{
		ID:           ID(id),
		Email:        email,
		Name:         name,
		PasswordHash: params.PasswordHash,
		Roles:        roles,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (u *User) UpdateName(name string, now time.Time) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrNameRequired
	}
	u.Name = trimmed
	u.touch(now)
	return nil
}

func (u *User) SetPasswordHash(hash string, now time.Time) error {
	if strings.TrimSpace(hash) == "" {
		return ErrPasswordHashMissing
	}
	u.PasswordHash = hash
	u.touch(now)
	return nil
}

func (u *User) AssignRoles(roles []Role, now time.Time) error {
	norm, err := normalizeRoles(roles)
	if err != nil {
		return err
	}
	if len(norm) == 0 {
		norm = []Role{RoleGuest}
	}
	u.Roles = norm
	u.touch(now)
	return nil
}

func (u *User) EnsureRole(role Role, now time.Time) error {
	role = normalizeRole(role)
	if role == "" {
		return ErrInvalidRole
	}
	if u.HasRole(role) {
		return nil
	}
	u.Roles = append(u.Roles, role)
	u.touch(now)
	return nil
}

func (u *User) HasRole(role Role) bool {
	role = normalizeRole(role)
	if role == "" {
		return false
	}
	for _, current := range u.Roles {
		if normalizeRole(current) == role {
			return true
		}
	}
	return false
}

func (u *User) touch(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	u.UpdatedAt = now.UTC()
}

func normalizeRoles(roles []Role) ([]Role, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	seen := make(map[Role]struct{}, len(roles))
	normalized := make([]Role, 0, len(roles))
	for _, role := range roles {
		normalizedRole := normalizeRole(role)
		if normalizedRole == "" {
			return nil, ErrInvalidRole
		}
		if _, ok := seen[normalizedRole]; ok {
			continue
		}
		seen[normalizedRole] = struct{}{}
		normalized = append(normalized, normalizedRole)
	}
	return normalized, nil
}

func normalizeRole(role Role) Role {
	switch strings.ToLower(strings.TrimSpace(string(role))) {
	case "guest":
		return RoleGuest
	case "host":
		return RoleHost
	default:
		return Role(strings.ToLower(strings.TrimSpace(string(role))))
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
