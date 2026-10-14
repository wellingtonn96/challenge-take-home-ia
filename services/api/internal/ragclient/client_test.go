package ragclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHTTPClientIngestAndQuery(t *testing.T) {
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	chunkID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ingest":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"document_id":"` + docID.String() + `","chunks_created":2}`))
		case "/query":
			_, _ = w.Write([]byte(`{"answer":"MiniLM","sources":[{"chunk_id":"` + chunkID.String() + `","document_id":"` + docID.String() + `","chunk_index":0,"content":"all-MiniLM-L6-v2","score":0.91}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(server.URL, time.Second)
	ingested, err := client.Ingest(context.Background(), IngestRequest{
		DocumentID: docID,
		Text:       "hello",
		Title:      "FAQ",
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if ingested.ChunksCreated != 2 || ingested.DocumentID != docID {
		t.Fatalf("ingest = %+v", ingested)
	}

	queried, err := client.Query(context.Background(), QueryRequest{Question: "model?"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if queried.Answer != "MiniLM" || len(queried.Sources) != 1 || queried.Sources[0].Score != 0.91 {
		t.Fatalf("query = %+v", queried)
	}
}

func TestHTTPClientParsesFastAPIDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":{"error":"no_relevant_chunks","message":"empty corpus"}}`))
	}))
	defer server.Close()

	_, err := New(server.URL, time.Second).Query(context.Background(), QueryRequest{Question: "x"})
	var upstream *UpstreamError
	if !errors.As(err, &upstream) {
		t.Fatalf("err = %v", err)
	}
	if upstream.StatusCode != http.StatusNotFound || upstream.Code != "no_relevant_chunks" || upstream.Message != "empty corpus" {
		t.Fatalf("upstream = %+v", upstream)
	}
	if !ShouldRollback(err) {
		t.Fatal("http error should roll metadata back")
	}
}

func TestShouldRollbackSkipsTimeout(t *testing.T) {
	if ShouldRollback(context.DeadlineExceeded) {
		t.Fatal("timeout must not delete metadata that Python may still commit")
	}
}
