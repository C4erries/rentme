package ginserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/dto"
	authsvc "rentme/internal/app/services/auth"
	domainuser "rentme/internal/domain/user"
)

type AuthHTTP interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Logout(c *gin.Context)
	Me(c *gin.Context)
}

type AuthHandler struct {
	Service *authsvc.Service
	Logger  *slog.Logger
}

type registerRequest struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	Password   string `json:"password"`
	WantToHost bool   `json:"want_to_host"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h AuthHandler) Register(c *gin.Context) {
	if h.Service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	result, err := h.Service.Register(c.Request.Context(), authsvc.RegisterParams{
		Email:      req.Email,
		Name:       req.Name,
		Password:   req.Password,
		WantToHost: req.WantToHost,
	})
	if err != nil {
		h.respondAuthError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.NewAuthResponse(result.User, result.Token))
}

func (h AuthHandler) Login(c *gin.Context) {
	if h.Service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	result, err := h.Service.Login(c.Request.Context(), authsvc.LoginParams{
		Email:    strings.TrimSpace(req.Email),
		Password: req.Password,
	})
	if err != nil {
		h.respondAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.NewAuthResponse(result.User, result.Token))
}

func (h AuthHandler) Logout(c *gin.Context) {
	if h.Service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth service unavailable"})
		return
	}
	token := bearerTokenFromContext(c)
	if err := h.Service.Logout(c.Request.Context(), token); err != nil {
		if h.Logger != nil {
			h.Logger.Warn("logout failed", "error", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h AuthHandler) Me(c *gin.Context) {
	principal, ok := currentPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "auth required"})
		return
	}
	profile := dto.UserProfile{
		ID:        principal.ID,
		Email:     principal.Email,
		Name:      principal.Name,
		Roles:     append([]string(nil), principal.Roles...),
		CreatedAt: principal.CreatedAt,
		UpdatedAt: principal.UpdatedAt,
	}
	c.JSON(http.StatusOK, profile)
}

func (h AuthHandler) respondAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authsvc.ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, authsvc.ErrPasswordTooShort),
		errors.Is(err, domainuser.ErrEmailRequired),
		errors.Is(err, domainuser.ErrNameRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domainuser.ErrEmailAlreadyUsed):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		if h.Logger != nil {
			h.Logger.Error("auth operation failed", "error", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func bearerTokenFromContext(c *gin.Context) string {
	if principal, ok := currentPrincipal(c); ok && principal.Token != "" {
		return principal.Token
	}
	header := c.GetHeader("Authorization")
	return extractBearerToken(header)
}

var _ AuthHTTP = (*AuthHandler)(nil)
