package obs

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log/slog"
)

type Middleware struct {
	Logger *slog.Logger
}

func (m Middleware) RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		ctx := context.WithValue(c.Request.Context(), requestIDKey{}, id)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set("X-Request-ID", id)
		c.Set("request_id", id)
		c.Next()
	}
}

func (m Middleware) LoggerMiddleware() gin.HandlerFunc {
	log := m.Logger
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if log == nil {
			return
		}
		log.Info("http", "method", c.Request.Method, "path", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start), "request_id", c.GetString("request_id"))
	}
}

type requestIDKey struct{}

func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
