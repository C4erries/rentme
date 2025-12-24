package ginserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/dto"
	domainauth "rentme/internal/domain/auth"
	domainuser "rentme/internal/domain/user"
	"rentme/internal/infra/pricing"
)

type AdminHTTP interface {
	ListUsers(c *gin.Context)
	MLMetrics(c *gin.Context)
	BlockUser(c *gin.Context)
	UnblockUser(c *gin.Context)
}

type AdminHandler struct {
	Users    domainuser.Repository
	Sessions domainauth.SessionStore
	Metrics  *pricing.MetricsClient
	Logger   *slog.Logger
}

func (h AdminHandler) ListUsers(c *gin.Context) {
	principal, ok := requireRole(c, "admin")
	if !ok || principal.ID == "" {
		return
	}
	if h.Users == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user repository unavailable"})
		return
	}

	limit := parseIntWithDefault(c.Query("limit"), 50)
	offset := parseIntWithDefault(c.Query("offset"), 0)
	users, total, err := h.Users.List(c.Request.Context(), domainuser.ListParams{
		Query:  c.Query("query"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("list users failed", "error", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot list users"})
		return
	}

	resp := dto.UserList{
		Items: make([]dto.UserProfile, 0, len(users)),
		Total: total,
	}
	for _, user := range users {
		resp.Items = append(resp.Items, dto.MapUserProfile(user))
	}
	c.JSON(http.StatusOK, resp)
}

func (h AdminHandler) BlockUser(c *gin.Context) {
	if _, ok := requireRole(c, "admin"); !ok {
		return
	}
	user, err := h.loadUserByID(c)
	if err != nil {
		return
	}
	if user.Blocked {
		c.JSON(http.StatusOK, dto.MapUserProfile(user))
		return
	}
	user.SetBlocked(true, time.Now())
	if err := h.Users.Save(c.Request.Context(), user); err != nil {
		if h.Logger != nil {
			h.Logger.Error("user block failed", "user_id", user.ID, "error", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot update user"})
		return
	}
	if h.Sessions != nil {
		_ = h.Sessions.DeleteByUser(c.Request.Context(), user.ID)
	}
	if h.Logger != nil {
		h.Logger.Info("user blocked", "user_id", user.ID, "email", user.Email)
	}
	c.JSON(http.StatusOK, dto.MapUserProfile(user))
}

func (h AdminHandler) UnblockUser(c *gin.Context) {
	if _, ok := requireRole(c, "admin"); !ok {
		return
	}
	user, err := h.loadUserByID(c)
	if err != nil {
		return
	}
	if !user.Blocked {
		c.JSON(http.StatusOK, dto.MapUserProfile(user))
		return
	}
	user.SetBlocked(false, time.Now())
	if err := h.Users.Save(c.Request.Context(), user); err != nil {
		if h.Logger != nil {
			h.Logger.Error("user unblock failed", "user_id", user.ID, "error", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot update user"})
		return
	}
	if h.Logger != nil {
		h.Logger.Info("user unblocked", "user_id", user.ID, "email", user.Email)
	}
	c.JSON(http.StatusOK, dto.MapUserProfile(user))
}

func (h AdminHandler) MLMetrics(c *gin.Context) {
	if _, ok := requireRole(c, "admin"); !ok {
		return
	}
	if h.Metrics == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ml metrics unavailable"})
		return
	}
	result, err := h.Metrics.Fetch(c.Request.Context())
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("ml metrics fetch failed", "error", err)
		}
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h AdminHandler) loadUserByID(c *gin.Context) (*domainuser.User, error) {
	if h.Users == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "user repository unavailable"})
		return nil, errors.New("user repository unavailable")
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user id is required"})
		return nil, errors.New("user id is required")
	}
	user, err := h.Users.ByID(c.Request.Context(), domainuser.ID(id))
	if err != nil {
		if errors.Is(err, domainuser.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return nil, err
		}
		if h.Logger != nil {
			h.Logger.Error("load user failed", "user_id", id, "error", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot load user"})
		return nil, err
	}
	return user, nil
}

var _ AdminHTTP = (*AdminHandler)(nil)
