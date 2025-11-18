package obs

import (
	"log/slog"
	"os"
)

// NewLogger configures slog logger with JSON handler in production.
func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	var handler slog.Handler
	handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	if env == "dev" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler)
}
