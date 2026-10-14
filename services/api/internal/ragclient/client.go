package ragclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tucows/interview-exercise-ai/services/api/internal/trace"
)

// Client is the internal Python RAG service. Handlers mock this in tests.
type Client interface {
	Ingest(ctx context.Context, req IngestRequest) (IngestResponse, error)
	Query(ctx context.Context, req QueryRequest) (QueryResponse, error)
}

type IngestRequest struct {
	DocumentID uuid.UUID `json:"document_id"`
	Text       string    `json:"text"`
	Title      string    `json:"title,omitempty"`
}

type IngestResponse struct {
	DocumentID    uuid.UUID `json:"document_id"`
	ChunksCreated int       `json:"chunks_created"`
}

type QueryRequest struct {
	Question string `json:"question"`
	TopK     *int   `json:"top_k,omitempty"`
}

// SourceChunk matches the Python query source object field for field.
type SourceChunk struct {
	ChunkID    uuid.UUID `json:"chunk_id"`
	DocumentID uuid.UUID `json:"document_id"`
	ChunkIndex int       `json:"chunk_index"`
	Content    string    `json:"content"`
	Score      float64   `json:"score"`
}

// QueryResponse is the Python POST /query body, returned unchanged by the API.
type QueryResponse struct {
	Answer  string        `json:"answer"`
	Sources []SourceChunk `json:"sources"`
}

// UpstreamError is a non-2xx response from the RAG service.
// Code and Message come from FastAPI's detail object when that shape is present.
type UpstreamError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("rag status %d: %s", e.StatusCode, e.Message)
}

// ShouldRollback reports whether the RAG call finished without committing chunks.
// A timeout is excluded: Python may still commit and recreate the document row.
func ShouldRollback(err error) bool {
	if err == nil {
		return false
	}
	var upstream *UpstreamError
	if errors.As(err, &upstream) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var timed interface{ Timeout() bool }
	if errors.As(err, &timed) && timed.Timeout() {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return true
}

type HTTPClient struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *HTTPClient) Ingest(ctx context.Context, req IngestRequest) (IngestResponse, error) {
	var out IngestResponse
	if err := c.post(ctx, "/ingest", req, &out); err != nil {
		return IngestResponse{}, err
	}
	return out, nil
}

func (c *HTTPClient) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	var out QueryResponse
	if err := c.post(ctx, "/query", req, &out); err != nil {
		return QueryResponse{}, err
	}
	if out.Sources == nil {
		out.Sources = []SourceChunk{}
	}
	return out, nil
}

func (c *HTTPClient) post(ctx context.Context, path string, payload any, dest any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode rag request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build rag request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if id := trace.ID(ctx); id != "" {
		httpReq.Header.Set(trace.Header, id)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call rag %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read rag %s: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseUpstream(resp.StatusCode, raw)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("decode rag %s: %w", path, err)
	}
	return nil
}

func parseUpstream(status int, body []byte) *UpstreamError {
	out := &UpstreamError{
		StatusCode: status,
		Code:       "rag_error",
		Message:    "knowledge service request failed",
	}
	var envelope struct {
		Detail  json.RawMessage `json:"detail"`
		Error   string          `json:"error"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		if msg := truncate(strings.TrimSpace(string(body))); msg != "" {
			out.Message = msg
		}
		return out
	}
	if envelope.Error != "" || envelope.Message != "" {
		if envelope.Error != "" {
			out.Code = envelope.Error
		}
		if envelope.Message != "" {
			out.Message = envelope.Message
		}
		return out
	}

	var obj struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(envelope.Detail, &obj) == nil && (obj.Error != "" || obj.Message != "") {
		if obj.Error != "" {
			out.Code = obj.Error
		}
		if obj.Message != "" {
			out.Message = obj.Message
		}
		return out
	}
	var text string
	if json.Unmarshal(envelope.Detail, &text) == nil && strings.TrimSpace(text) != "" {
		out.Message = truncate(text)
		return out
	}
	var items []struct {
		Msg string `json:"msg"`
	}
	if json.Unmarshal(envelope.Detail, &items) == nil && len(items) > 0 {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			if item.Msg != "" {
				parts = append(parts, item.Msg)
			}
		}
		if len(parts) > 0 {
			out.Code = "invalid_request"
			out.Message = truncate(strings.Join(parts, "; "))
		}
	}
	return out
}

func truncate(message string) string {
	const max = 300
	if len(message) <= max {
		return message
	}
	return message[:max]
}
