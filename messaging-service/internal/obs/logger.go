package obs

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

// NewLogger creates a slog logger with dev-friendly output by default.
func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	writer := os.Stdout
	if env == "dev" || env == "local" {
		handler := tint.NewHandler(writer, &tint.Options{
			Level:      level,
			TimeFormat: time.RFC3339,
			AddSource:  true,
		})
		return slog.New(handler)
	}
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	return slog.New(handler)
}
