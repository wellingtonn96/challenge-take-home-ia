// E6.3 — unit HTTP handlers with stub Postgres repo and mock RAG client.
package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tucows/interview-exercise-ai/services/api/internal/documents"
	"github.com/tucows/interview-exercise-ai/services/api/internal/httpapi"
	"github.com/tucows/interview-exercise-ai/services/api/internal/ragclient"
)

func TestHealth(t *testing.T) {
	repo := &stubRepo{}
	rec := perform(t, httpapi.New(discardLog(), repo, &stubRAG{}, nil), http.MethodGet, "/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	decode(t, rec.Body.Bytes(), &body)
	if body["status"] != "ok" || body["database"] != "ok" {
		t.Fatalf("body = %#v", body)
	}

	repo.pingErr = context.DeadlineExceeded
	rec = perform(t, httpapi.New(discardLog(), repo, &stubRAG{}, nil), http.MethodGet, "/health", "")
	assertError(t, rec, http.StatusServiceUnavailable, "database_unavailable")
}

func TestDocumentsListAndDelete(t *testing.T) {
	repo := &stubRepo{now: time.Date(2026, 10, 12, 18, 0, 0, 0, time.UTC)}
	rag := &stubRAG{}
	srv := httpapi.New(discardLog(), repo, rag, nil)

	rec := perform(t, srv, http.MethodGet, "/documents", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"documents":[]}` {
		t.Fatalf("empty list = %d %s", rec.Code, rec.Body.String())
	}

	rec = performJSON(t, srv, http.MethodPost, "/ingest", `{"title":"FAQ","text":"The model is MiniLM.","source_uri":"file://faq.txt"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("ingest = %d %s", rec.Code, rec.Body.String())
	}
	var created ragclient.IngestResponse
	decode(t, rec.Body.Bytes(), &created)
	if created.ChunksCreated != 3 || rag.ingestCalls != 1 {
		t.Fatalf("created = %+v calls = %d", created, rag.ingestCalls)
	}
	if rag.lastIngest.Title != "FAQ" || rag.lastIngest.DocumentID != created.DocumentID {
		t.Fatalf("rag ingest = %+v", rag.lastIngest)
	}

	rec = perform(t, srv, http.MethodGet, "/documents", "")
	var list struct {
		Documents []documents.ListItem `json:"documents"`
	}
	decode(t, rec.Body.Bytes(), &list)
	if len(list.Documents) != 1 || list.Documents[0].Title != "FAQ" || list.Documents[0].ID != created.DocumentID {
		t.Fatalf("list = %+v", list.Documents)
	}
	if !strings.Contains(rec.Body.String(), `"created_at":"2026-10-12T18:00:00Z"`) {
		t.Fatalf("created_at missing: %s", rec.Body.String())
	}

	rec = perform(t, srv, http.MethodDelete, "/documents/"+created.DocumentID.String(), "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete = %d %q", rec.Code, rec.Body.String())
	}
	rec = perform(t, srv, http.MethodGet, "/documents", "")
	decode(t, rec.Body.Bytes(), &list)
	if len(list.Documents) != 0 {
		t.Fatalf("list after delete = %+v", list.Documents)
	}
}

func TestIngestValidationAndRollback(t *testing.T) {
	repo := &stubRepo{now: time.Now().UTC()}
	rag := &stubRAG{ingestErr: &ragclient.UpstreamError{StatusCode: 500, Code: "ingest_failed", Message: "embed failed"}}
	srv := httpapi.New(discardLog(), repo, rag, nil)

	rec := performJSON(t, srv, http.MethodPost, "/ingest", `{"title":"","text":"x"}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_request")
	if rag.ingestCalls != 0 || len(repo.inserted) != 0 {
		t.Fatal("invalid ingest must not call the store or RAG")
	}

	rec = performJSON(t, srv, http.MethodPost, "/ingest", `{"title":"FAQ","text":"hello"}`)
	assertError(t, rec, http.StatusBadGateway, "ingest_failed")
	if len(repo.deleted) != 1 {
		t.Fatalf("expected metadata rollback, deleted = %v", repo.deleted)
	}

	rag.ingestErr = context.DeadlineExceeded
	rec = performJSON(t, srv, http.MethodPost, "/ingest", `{"title":"FAQ","text":"hello"}`)
	assertError(t, rec, http.StatusBadGateway, "rag_unavailable")
	if len(repo.deleted) != 1 {
		t.Fatal("timeout must not delete metadata")
	}
}

func TestQueryMatchesPythonJSON(t *testing.T) {
	docID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	chunkID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	rag := &stubRAG{queryResp: ragclient.QueryResponse{
		Answer: "all-MiniLM-L6-v2",
		Sources: []ragclient.SourceChunk{{
			ChunkID:    chunkID,
			DocumentID: docID,
			ChunkIndex: 1,
			Content:    "default embedding model",
			Score:      0.5,
		}},
	}}
	srv := httpapi.New(discardLog(), &stubRepo{}, rag, nil)

	rec := performJSON(t, srv, http.MethodPost, "/query", `{"question":"  What is the model?  ","top_k":2}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if rag.lastQuery.Question != "What is the model?" || rag.lastQuery.TopK == nil || *rag.lastQuery.TopK != 2 {
		t.Fatalf("forwarded = %+v", rag.lastQuery)
	}
	var body ragclient.QueryResponse
	decode(t, rec.Body.Bytes(), &body)
	if body.Answer != "all-MiniLM-L6-v2" || len(body.Sources) != 1 || body.Sources[0].ChunkID != chunkID || body.Sources[0].Score != 0.5 {
		t.Fatalf("body = %+v", body)
	}

	rec = performJSON(t, srv, http.MethodPost, "/query", `{"question":""}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_request")

	rec = performJSON(t, srv, http.MethodPost, "/query", `{"question":"x","top_k":0}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_request")

	rag.queryErr = &ragclient.UpstreamError{StatusCode: 404, Code: "no_relevant_chunks", Message: "empty corpus"}
	rec = performJSON(t, srv, http.MethodPost, "/query", `{"question":"missing"}`)
	assertError(t, rec, http.StatusNotFound, "no_relevant_chunks")

	rag.queryErr = context.DeadlineExceeded
	rec = performJSON(t, srv, http.MethodPost, "/query", `{"question":"down"}`)
	assertError(t, rec, http.StatusBadGateway, "rag_unavailable")
}

func TestErrorEnvelope(t *testing.T) {
	srv := httpapi.New(discardLog(), &stubRepo{}, &stubRAG{}, nil)

	rec := perform(t, srv, http.MethodGet, "/missing", "")
	assertError(t, rec, http.StatusNotFound, "not_found")

	rec = perform(t, srv, http.MethodPost, "/health", "")
	assertError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if rec.Header().Get("Allow") == "" {
		t.Fatal("Allow header missing")
	}

	rec = perform(t, srv, http.MethodDelete, "/documents/not-a-uuid", "")
	assertError(t, rec, http.StatusBadRequest, "invalid_request")

	rec = perform(t, srv, http.MethodDelete, "/documents/"+uuid.NewString(), "")
	assertError(t, rec, http.StatusNotFound, "not_found")

	rec = perform(t, srv, http.MethodPost, "/query", `{"question":"x"}`)
	assertError(t, rec, http.StatusBadRequest, "invalid_request")

	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(`{"question":"`+strings.Repeat("a", 2<<20)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assertError(t, rec, http.StatusBadRequest, "invalid_request")
}

type stubRepo struct {
	now      time.Time
	pingErr  error
	inserted []documents.Document
	deleted  []uuid.UUID
}

func (s *stubRepo) Ping(context.Context) error { return s.pingErr }

func (s *stubRepo) Insert(_ context.Context, doc documents.Document) error {
	s.inserted = append(s.inserted, doc)
	return nil
}

func (s *stubRepo) Get(context.Context, uuid.UUID) (documents.Document, error) {
	return documents.Document{}, documents.ErrNotFound
}

func (s *stubRepo) List(context.Context) ([]documents.ListItem, error) {
	items := make([]documents.ListItem, 0)
	for _, doc := range s.inserted {
		if containsID(s.deleted, doc.ID) {
			continue
		}
		items = append(items, documents.ListItem{ID: doc.ID, Title: doc.Title, CreatedAt: s.now})
	}
	return items, nil
}

func (s *stubRepo) Delete(_ context.Context, id uuid.UUID) error {
	for _, doc := range s.inserted {
		if doc.ID == id && !containsID(s.deleted, id) {
			s.deleted = append(s.deleted, id)
			return nil
		}
	}
	return documents.ErrNotFound
}

type stubRAG struct {
	ingestErr   error
	queryErr    error
	ingestCalls int
	lastIngest  ragclient.IngestRequest
	lastQuery   ragclient.QueryRequest
	queryResp   ragclient.QueryResponse
}

func (s *stubRAG) Ingest(_ context.Context, req ragclient.IngestRequest) (ragclient.IngestResponse, error) {
	s.ingestCalls++
	s.lastIngest = req
	if s.ingestErr != nil {
		return ragclient.IngestResponse{}, s.ingestErr
	}
	return ragclient.IngestResponse{DocumentID: req.DocumentID, ChunksCreated: 3}, nil
}

func (s *stubRAG) Query(_ context.Context, req ragclient.QueryRequest) (ragclient.QueryResponse, error) {
	s.lastQuery = req
	if s.queryErr != nil {
		return ragclient.QueryResponse{}, s.queryErr
	}
	return s.queryResp, nil
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func perform(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func performJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, raw []byte, dest any) {
	t.Helper()
	if err := json.Unmarshal(raw, dest); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, status, rec.Body.String())
	}
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	decode(t, rec.Body.Bytes(), &body)
	if body.Error != code || body.Message == "" {
		t.Fatalf("error body = %+v", body)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type = %s", rec.Header().Get("Content-Type"))
	}
}
