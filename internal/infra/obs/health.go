package obs

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandlers exposes endpoints for liveness and readiness checks.
type HealthHandlers struct {
	Ready func() error
}

func (h HealthHandlers) Livez(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (h HealthHandlers) Readyz(c *gin.Context) {
	if h.Ready != nil {
		if err := h.Ready(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
	}
	c.Status(http.StatusOK)
}
