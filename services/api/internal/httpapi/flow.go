package httpapi

import (
	"context"
	"log/slog"

	"github.com/tucows/interview-exercise-ai/services/api/internal/trace"
)

func (h *handler) flow(ctx context.Context, level slog.Level, step, msg string, attrs ...any) {
	args := make([]any, 0, 2+len(attrs))
	args = append(args, "trace_id", trace.ID(ctx), "step", step)
	args = append(args, attrs...)
	h.log.Log(ctx, level, msg, args...)
}
