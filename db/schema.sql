-- Knowledge Assistant — PostgreSQL schema (metadata + pgvector)
-- Embedding dimension 384 matches VECTOR_DIMENSION and all-MiniLM-L6-v2 in .env.example

CREATE EXTENSION IF NOT EXISTS vector;

-- ---------------------------------------------------------------------------
-- documents: corpus metadata (Go API list/delete; RAG ingest creates rows)
-- ---------------------------------------------------------------------------
CREATE TABLE documents (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT NOT NULL,
    source_uri    TEXT,
    content_type  TEXT NOT NULL DEFAULT 'text/plain',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE documents IS 'Document metadata. The Go API inserts and deletes rows; RAG ingest upserts the title and writes chunks. Deleting a row cascades chunks and embeddings.';
COMMENT ON COLUMN documents.id IS 'Stable document identifier (UUID).';
COMMENT ON COLUMN documents.title IS 'Human-readable title or filename.';
COMMENT ON COLUMN documents.source_uri IS 'Optional origin path, URL, or upload reference.';
COMMENT ON COLUMN documents.content_type IS 'MIME type of the ingested content.';
COMMENT ON COLUMN documents.created_at IS 'When the document was first ingested.';
COMMENT ON COLUMN documents.updated_at IS 'Last metadata or content update.';

CREATE INDEX idx_documents_created_at ON documents (created_at DESC);

-- ---------------------------------------------------------------------------
-- chunks: text segments for retrieval (one document, many chunks)
-- ---------------------------------------------------------------------------
CREATE TABLE chunks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id   UUID NOT NULL,
    chunk_index   INTEGER NOT NULL,
    content       TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_chunks_document_index UNIQUE (document_id, chunk_index),
    CONSTRAINT fk_chunks_document
        FOREIGN KEY (document_id)
        REFERENCES documents (id)
        ON DELETE CASCADE,
    CONSTRAINT chk_chunks_index_non_negative CHECK (chunk_index >= 0)
);

COMMENT ON TABLE chunks IS 'Chunked document text used as retrieval units.';
COMMENT ON COLUMN chunks.document_id IS 'Parent document; deleting document cascades chunks.';
COMMENT ON COLUMN chunks.chunk_index IS 'Order of chunk within the document (0-based).';
COMMENT ON COLUMN chunks.content IS 'Plain text chunk body passed to the LLM as context.';

CREATE INDEX idx_chunks_document_id_chunk_index ON chunks (document_id, chunk_index);

-- ---------------------------------------------------------------------------
-- embeddings: one vector per chunk (semantic search via pgvector)
-- Dimension fixed at 384 — change model only with ALTER + full re-embed.
-- ---------------------------------------------------------------------------
CREATE TABLE embeddings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id      UUID NOT NULL,
    embedding     vector(384) NOT NULL,
    model_name    TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_embeddings_chunk_id UNIQUE (chunk_id),
    CONSTRAINT fk_embeddings_chunk
        FOREIGN KEY (chunk_id)
        REFERENCES chunks (id)
        ON DELETE CASCADE
);

COMMENT ON TABLE embeddings IS 'Vector representation of chunks for ANN similarity search.';
COMMENT ON COLUMN embeddings.chunk_id IS 'One embedding row per chunk.';
COMMENT ON COLUMN embeddings.embedding IS '384-dim vector (all-MiniLM-L6-v2); must match VECTOR_DIMENSION.';
COMMENT ON COLUMN embeddings.model_name IS 'Embedding model id used to produce this vector.';

-- Cosine distance: typical for sentence-transformers normalized embeddings
CREATE INDEX idx_embeddings_embedding_hnsw
    ON embeddings
    USING hnsw (embedding vector_cosine_ops);
