package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/tucows/interview-exercise-ai/services/api/internal/obslog"
)

func New(levelName, environment string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var inner slog.Handler
	if strings.EqualFold(environment, "production") {
		inner = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		inner = slog.NewTextHandler(os.Stdout, opts)
	}
	handler := obslog.NewTeeHandler(inner, obslog.DefaultRing())
	return slog.New(handler)
}
