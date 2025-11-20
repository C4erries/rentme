package ginserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/services/auth"
	domainauth "rentme/internal/domain/auth"
	domainuser "rentme/internal/domain/user"
)

const principalContextKey = "rentme.principal"

type principal struct {
	ID        string
	Email     string
	Name      string
	Roles     []string
	Token     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (p principal) HasRole(role string) bool {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return false
	}
	for _, r := range p.Roles {
		if strings.ToLower(r) == role {
			return true
		}
	}
	return false
}

type AuthMiddleware struct {
	Service *auth.Service
	Logger  *slog.Logger
}

func (m AuthMiddleware) Handle(c *gin.Context) {
	token := extractBearerToken(c.GetHeader("Authorization"))
	if token == "" || m.Service == nil {
		c.Next()
		return
	}
	resolved, err := m.Service.ResolveToken(c.Request.Context(), token)
	if err != nil {
		if !errors.Is(err, domainauth.ErrSessionNotFound) && m.Logger != nil {
			m.Logger.Debug("token validation failed", "error", err)
		}
		c.Next()
		return
	}
	user := resolved.User
	setPrincipal(c, principal{
		ID:        string(user.ID),
		Email:     user.Email,
		Name:      user.Name,
		Roles:     mapRoles(user.Roles),
		Token:     token,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	})
	c.Next()
}

func mapRoles(roles []domainuser.Role) []string {
	result := make([]string, 0, len(roles))
	for _, r := range roles {
		result = append(result, string(r))
	}
	return result
}

func setPrincipal(c *gin.Context, p principal) {
	c.Set(principalContextKey, p)
}

func currentPrincipal(c *gin.Context) (principal, bool) {
	val, exists := c.Get(principalContextKey)
	if !exists {
		return principal{}, false
	}
	p, ok := val.(principal)
	return p, ok
}

func requireRole(c *gin.Context, role string) (principal, bool) {
	p, ok := currentPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "auth required"})
		return principal{}, false
	}
	if role != "" && !p.HasRole(role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		return principal{}, false
	}
	return p, true
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	token := strings.TrimSpace(header[7:])
	return token
}
