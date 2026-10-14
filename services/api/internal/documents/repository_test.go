package documents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tucows/interview-exercise-ai/services/api/internal/config"
)

func TestPostgresRepository_CRUDAndCascade(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL())
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	repo := NewPostgresRepository(pool)
	id := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM documents WHERE id = $1`, id.String())
	})

	err = repo.Insert(ctx, Document{
		ID:          id,
		Title:       "repo test",
		SourceURI:   "file://repo-test.txt",
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "repo test" || got.SourceURI != "file://repo-test.txt" {
		t.Fatalf("get = %+v", got)
	}

	var chunkID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chunks (document_id, chunk_index, content)
		VALUES ($1, 0, 'cascade-me')
		RETURNING id::text
	`, id.String()).Scan(&chunkID); err != nil {
		t.Fatalf("insert chunk: %v", err)
	}
	zeros := make([]string, 384)
	for i := range zeros {
		zeros[i] = "0"
	}
	vector := "[" + strings.Join(zeros, ",") + "]"
	if _, err := pool.Exec(ctx, `
		INSERT INTO embeddings (chunk_id, embedding, model_name)
		VALUES ($1, $2::vector, 'test')
	`, chunkID, vector); err != nil {
		t.Fatalf("insert embedding: %v", err)
	}

	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range items {
		if item.ID == id && item.Title == "repo test" && !item.CreatedAt.IsZero() {
			found = true
		}
	}

	if !found {
		t.Fatal("inserted document missing from list")
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, id); err != ErrNotFound {
		t.Fatalf("get after delete = %v", err)
	}

	var chunks, embeddings int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM chunks WHERE document_id = $1`, id.String()).Scan(&chunks); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM embeddings WHERE chunk_id = $1`, chunkID).Scan(&embeddings); err != nil {
		t.Fatal(err)
	}
	if chunks != 0 || embeddings != 0 {
		t.Fatalf("cascade left chunks=%d embeddings=%d", chunks, embeddings)
	}
	if err := repo.Delete(ctx, id); err != ErrNotFound {
		t.Fatalf("second delete = %v", err)
	}
}
