package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tucows/interview-exercise-ai/services/api/internal/apierror"
	"github.com/tucows/interview-exercise-ai/services/api/internal/documents"
	"github.com/tucows/interview-exercise-ai/services/api/internal/ragclient"
	"github.com/tucows/interview-exercise-ai/services/api/internal/trace"
)

type handler struct {
	log  *slog.Logger
	repo documents.Repository
	rag  ragclient.Client
}

// New routes the public API. Every error response is JSON {"error","message"}.
func New(log *slog.Logger, repo documents.Repository, rag ragclient.Client, corsOrigins []string) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	h := &handler{log: log, repo: repo, rag: rag}
	mux := http.NewServeMux()
	mux.Handle("GET /health", h.wrap(h.health))
	mux.Handle("GET /logs", h.wrap(h.logs))
	mux.Handle("GET /documents", h.wrap(h.listDocuments))
	mux.Handle("DELETE /documents/{id}", h.wrap(h.deleteDocument))
	mux.Handle("POST /ingest", h.wrap(h.ingest))
	mux.Handle("POST /query", h.wrap(h.query))
	return withCORS(corsOrigins, h.middleware(mux))
}

func (h *handler) wrap(fn func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			var apiErr *apierror.Error
			if !errors.As(err, &apiErr) {
				h.log.Error("unhandled error", "err", err, "path", r.URL.Path)
				apiErr = apierror.Internal("internal error")
			}
			writeAPIError(w, apiErr)
		}
	})
}

func (h *handler) middleware(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := strings.TrimSpace(r.Header.Get(trace.Header))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		ctx := trace.WithID(r.Context(), traceID)
		r = r.WithContext(ctx)

		start := time.Now()
		cw := newCaptureWriter()
		cw.header.Set(trace.Header, traceID)
		h.flow(ctx, slog.LevelInfo, "start", "request received",
			"method", r.Method,
			"path", r.URL.Path,
		)
		defer func() {
			if rec := recover(); rec != nil {
				h.log.Error("panic", "recover", fmt.Sprint(rec), "path", r.URL.Path)
				cw.reset()
				writeAPIError(cw, apierror.Internal("internal error"))
			}
			if cw.status == http.StatusMethodNotAllowed {
				allow := cw.Header().Get("Allow")
				cw.reset()
				if allow != "" {
					cw.Header().Set("Allow", allow)
				}
				writeAPIError(cw, apierror.MethodNotAllowed())
			} else if cw.status == http.StatusNotFound && strings.TrimSpace(cw.buf.String()) == "404 page not found" {
				cw.reset()
				writeAPIError(cw, apierror.NotFound("route not found"))
			}
			cw.flush(w)
			h.flow(ctx, slog.LevelInfo, "done", "request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", cw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}()

		// ServeHTTP records wildcard matches on the request. Calling the
		// handler from Handler() directly leaves PathValue empty.
		mux.ServeHTTP(cw, r)
	})
}

type captureWriter struct {
	header http.Header
	status int
	wrote  bool
	buf    bytes.Buffer
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{header: make(http.Header), status: http.StatusOK}
}

func (c *captureWriter) Header() http.Header { return c.header }

func (c *captureWriter) WriteHeader(status int) {
	if c.wrote {
		return
	}
	c.status = status
	c.wrote = true
}

func (c *captureWriter) Write(p []byte) (int, error) {
	if !c.wrote {
		c.WriteHeader(http.StatusOK)
	}
	return c.buf.Write(p)
}

func (c *captureWriter) reset() {
	c.header = make(http.Header)
	c.status = http.StatusOK
	c.wrote = false
	c.buf.Reset()
}

func (c *captureWriter) flush(dst http.ResponseWriter) {
	for key, values := range c.header {
		for _, value := range values {
			dst.Header().Add(key, value)
		}
	}
	if !c.wrote {
		c.status = http.StatusOK
	}
	dst.WriteHeader(c.status)
	if c.buf.Len() > 0 {
		_, _ = dst.Write(c.buf.Bytes())
	}
}
