package dto

import (
	"time"

	domainuser "rentme/internal/domain/user"
)

type UserProfile struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuthResponse struct {
	User  UserProfile `json:"user"`
	Token string      `json:"token"`
}

func MapUserProfile(user *domainuser.User) UserProfile {
	if user == nil {
		return UserProfile{}
	}
	roles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, string(role))
	}
	return UserProfile{
		ID:        string(user.ID),
		Email:     user.Email,
		Name:      user.Name,
		Roles:     roles,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func NewAuthResponse(user *domainuser.User, token string) AuthResponse {
	return AuthResponse{
		User:  MapUserProfile(user),
		Token: token,
	}
}
