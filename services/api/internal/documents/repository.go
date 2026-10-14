package documents

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("document not found")
	ErrConflict = errors.New("document already exists")
)

// Document is the metadata row owned by the API.
// Chunk text and vectors live in the RAG service's tables.
type Document struct {
	ID          uuid.UUID
	Title       string
	SourceURI   string
	ContentType string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListItem is the public document summary.
type ListItem struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository is the metadata store. Handlers depend on this interface.
type Repository interface {
	Ping(ctx context.Context) error
	Insert(ctx context.Context, doc Document) error
	Get(ctx context.Context, id uuid.UUID) (Document, error)
	List(ctx context.Context) ([]ListItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// Insert stores API-owned metadata. The RAG service later upserts the same id;
// its ON CONFLICT clause updates title only, so source_uri set here is kept.
func (r *PostgresRepository) Insert(ctx context.Context, doc Document) error {
	var sourceURI any
	if doc.SourceURI != "" {
		sourceURI = doc.SourceURI
	}
	contentType := doc.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO documents (id, title, source_uri, content_type)
		VALUES ($1, $2, $3, $4)
	`, doc.ID.String(), doc.Title, sourceURI, contentType)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, id uuid.UUID) (Document, error) {
	var (
		doc    Document
		source *string
		idText string
	)
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, title, source_uri, content_type, created_at, updated_at
		FROM documents
		WHERE id = $1
	`, id.String()).Scan(&idText, &doc.Title, &source, &doc.ContentType, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, fmt.Errorf("get document: %w", err)
	}
	parsed, err := uuid.Parse(idText)
	if err != nil {
		return Document{}, fmt.Errorf("parse document id: %w", err)
	}
	doc.ID = parsed
	if source != nil {
		doc.SourceURI = *source
	}
	return doc, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]ListItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, title, created_at
		FROM documents
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	items := make([]ListItem, 0)
	for rows.Next() {
		var (
			item   ListItem
			idText string
		)
		if err := rows.Scan(&idText, &item.Title, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		parsed, err := uuid.Parse(idText)
		if err != nil {
			return nil, fmt.Errorf("parse document id: %w", err)
		}
		item.ID = parsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return items, nil
}

// Delete removes the document row. chunks and embeddings use ON DELETE CASCADE,
// so retrieval data goes away without a second copy of the vector schema in Go.
func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, id.String())
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
