package obslog

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const defaultCapacity = 400

// Entry is one recent log line kept in memory for the dev UI.
type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	TraceID string    `json:"trace_id,omitempty"`
	Step    string    `json:"step,omitempty"`
}

// Ring stores the last N log records from this process.
type Ring struct {
	mu       sync.RWMutex
	capacity int
	entries  []Entry
}

var defaultRing = NewRing(defaultCapacity)

func DefaultRing() *Ring {
	return defaultRing
}

func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = defaultCapacity
	}
	return &Ring{capacity: capacity}
}

func (r *Ring) Add(level slog.Level, message string, traceID, step string) {
	line := strings.TrimSpace(message)
	if line == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, Entry{
		Time:    time.Now().UTC(),
		Level:   level.String(),
		Message: line,
		TraceID: strings.TrimSpace(traceID),
		Step:    strings.TrimSpace(step),
	})
	if len(r.entries) > r.capacity {
		r.entries = r.entries[len(r.entries)-r.capacity:]
	}
}

func (r *Ring) Snapshot(since time.Time, limit int) []Entry {
	if limit <= 0 || limit > r.capacity {
		limit = r.capacity
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Entry, 0, limit)
	for _, e := range r.entries {
		if !since.IsZero() && !e.Time.After(since) {
			continue
		}
		out = append(out, e)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

// TeeHandler writes to stdout (inner) and mirrors into the ring buffer.
type TeeHandler struct {
	inner slog.Handler
	ring  *Ring
}

func NewTeeHandler(inner slog.Handler, ring *Ring) *TeeHandler {
	if ring == nil {
		ring = defaultRing
	}
	return &TeeHandler{inner: inner, ring: ring}
}

func (h *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *TeeHandler) Handle(ctx context.Context, record slog.Record) error {
	var traceID, step string
	var parts []string
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "trace_id":
			traceID = attr.Value.String()
		case "step":
			step = attr.Value.String()
		default:
			parts = append(parts, attr.Key+"="+attr.Value.String())
		}
		return true
	})
	msg := record.Message
	if len(parts) > 0 {
		msg = msg + " " + strings.Join(parts, " ")
	}
	h.ring.Add(record.Level, msg, traceID, step)
	return h.inner.Handle(ctx, record)
}

func (h *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TeeHandler{inner: h.inner.WithAttrs(attrs), ring: h.ring}
}

func (h *TeeHandler) WithGroup(name string) slog.Handler {
	return &TeeHandler{inner: h.inner.WithGroup(name), ring: h.ring}
}
