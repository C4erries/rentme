package ginserver

import (
	"log/slog"
	"net/http"

	gin "github.com/gin-gonic/gin"

	"rentme/internal/app/dto"
	domainuser "rentme/internal/domain/user"
	"rentme/internal/infra/pricing"
)

type AdminHTTP interface {
	ListUsers(c *gin.Context)
	MLMetrics(c *gin.Context)
}

type AdminHandler struct {
	Users   domainuser.Repository
	Metrics *pricing.MetricsClient
	Logger  *slog.Logger
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
		c.JSON(http.StatusBadGateway, gin.H{"error": "ml metrics unavailable"})
		return
	}
	c.JSON(http.StatusOK, result)
}

var _ AdminHTTP = (*AdminHandler)(nil)
