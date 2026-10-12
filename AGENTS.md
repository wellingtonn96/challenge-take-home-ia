# AGENTS.md — Knowledge Assistant Challenge

Guidance for humans and AI agents working in this repository.

## Product goal

Build an **AI-powered Knowledge Assistant**: RAG over a document corpus, REST API (Go), PostgreSQL + pgvector, and an MCP tool `query_knowledge_base`.

## Stack (planned)

| Layer | Technology | Location |
|-------|------------|----------|
| RAG pipeline | Python 3.11+ | `services/rag/` |
| REST API | Go 1.22+ | `services/api/` (`cmd/`, `internal/`) |
| Vector + metadata DB | PostgreSQL 16 + **pgvector** | `db/` (`schema.sql`) |
| Sample corpus | Plain text (later PDF optional) | `data/sample/` |
| MCP server | Python or Go (reuse RAG logic) | TBD under `services/` |
| Orchestration | Docker Compose | `docker-compose.yml` (DB only for E1) |

### Service boundaries

- **Go API** is the public HTTP surface (`/ingest`, `/query`, `/documents`, `/documents/:id`).
- **Python RAG** is an internal HTTP service; Go calls it — no duplicated retrieval/generation logic.
- **MCP** calls the same RAG entrypoints as the API (shared library or HTTP), not a second implementation.

### Vector dimension (E1)

| Setting | Value |
|---------|--------|
| `VECTOR_DIMENSION` in `.env` | **384** |
| Default embedding model | `all-MiniLM-L6-v2` (sentence-transformers) |
| SQL column | `embeddings.embedding vector(384)` in [`db/schema.sql`](db/schema.sql) |

All three must stay aligned. If you change the embedding model, update `VECTOR_DIMENSION`, alter the column type (or recreate `embeddings`), and **re-embed** the corpus.

ANN index: HNSW with `vector_cosine_ops` (cosine similarity, typical for normalized MiniLM vectors).

### Embeddings & LLM (defaults in `.env`)

- Embeddings: local model (e.g. `sentence-transformers`) or Ollama/OpenAI — dimension must match `VECTOR_DIMENSION` and `vector(n)` in schema.
- Generation: Ollama preferred; OpenAI acceptable if documented in README.

## Repository layout

```
services/rag/     # ingest, chunk, embed, retrieve, generate
services/api/     # Go REST API
db/               # schema.sql
data/sample/      # seed documents
```

## Database bootstrap (E1)

- Schema is applied on **first** Postgres start via `docker-entrypoint-initdb.d/01-schema.sql` (bind-mount from `db/schema.sql`).
- Init scripts do **not** run again on an existing volume. To reapply from scratch: `docker compose down -v` then `docker compose up -d`.

Verify tables:

```bash
docker compose exec db psql -U knowledge -d knowledge_assistant -c "\dt"
docker compose exec db psql -U knowledge -d knowledge_assistant -c "\dx"
```

## Engineering rules

1. **Single-command run** (end state): `docker compose up` after copying `.env.example` → `.env`.
2. **Errors**: explicit handling; structured HTTP errors in Go; no bare `except` in Python.
3. **Tests**: unit tests at boundaries (mock vector store/LLM in Python; mock RAG client in Go); at least one integration test on real test DB.
4. **Schema**: document metadata + embeddings with indexes for metadata queries and ANN search; comments on tables/columns.
5. **Secrets**: never commit `.env`; only `.env.example` with placeholders.
6. **Commits**: work on branch `challenge/<your-name>`, not `main`.
7. **Scope**: prefer small, reviewable changes; document trade-offs in README.

## Current phase

- **E1 (done):** Postgres + pgvector, `schema.sql` (`documents`, `chunks`, `embeddings`).
- **Next (E2+):** RAG service, Go API, full compose stack.

## Commands (to be added later)

- `make ingest` or compose seed — load `data/sample/` into the corpus.
- Tests: `pytest` in `services/rag/`, `go test ./...` in `services/api/`.

## References

- Challenge spec: `desafio.md`
- Upstream fork source: `https://github.com/tucows/interview-exercise-ai`
