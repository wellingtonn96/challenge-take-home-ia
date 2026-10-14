# AI Knowledge Assistant (take-home)

RAG over a document corpus: **Python** retrieval + generation, **Go** public REST API, **PostgreSQL + pgvector**, optional **React** UI, and an **MCP** stdio tool (`query_knowledge_base`).

## Quick start (clean clone)

Prerequisites: Docker Compose v2, Python 3.11+ on the host (for `make ingest` / wait scripts), `make` (optional but recommended).

```bash
cp .env.example .env
docker compose up -d --build
make pull-model    # once: downloads OLLAMA_MODEL into the ollama container
make ingest        # loads data/sample/*.txt when the corpus is empty
```

Demo query (replace port if `API_PORT` ≠ 8080):

```bash
curl -s http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"question":"How does ingestion work?"}'
```

Or one shot: `make demo` (up + pull model + seed).

| Service | URL |
| --- | --- |
| API | `http://localhost:${API_PORT:-8080}` |
| RAG (internal) | `http://localhost:8000` |
| UI | `http://localhost:5173` |
| Ollama | `http://localhost:11434` |
| Postgres | `localhost:5432` |

First RAG start downloads the **embedding** model (`all-MiniLM-L6-v2`); allow several minutes on a cold machine.

API reference: [`docs/api.md`](docs/api.md).

## Architecture

```
                    ┌─────────────┐
  Browser / curl ──►│  Go API     │──metadata──► PostgreSQL + pgvector
  (React UI)        │  :8080      │                    ▲
                    └──────┬──────┘                    │
                           │ HTTP                        │ chunks + vectors
                           ▼                             │
                    ┌─────────────┐      embed/search    │
                    │  Python RAG │──────────────────────┘
                    │  :8000      │
                    └──────┬──────┘
                           │ /api/chat
                           ▼
                    ┌─────────────┐
                    │   Ollama    │
                    │  :11434     │
                    └─────────────┘

  Claude Desktop ──stdio──► python -m rag.mcp ──► run_query() (same as POST /query)
```

- **Go API** owns document metadata (`GET`/`DELETE /documents`) and proxies ingest/query to RAG.
- **Python RAG** chunks, embeds, retrieves top-k, calls Ollama for grounded answers.
- **MCP** imports `run_query()` in-process; Postgres + Ollama env match the RAG service ([`services/rag/README.md`](services/rag/README.md#mcp-query_knowledge_base)).

## Design choices & trade-offs

| Topic | Choice | Trade-off |
| --- | --- | --- |
| Metadata vs vectors | Same Postgres DB | Simpler ops and FK cascades; API and RAG share one pool config |
| Embeddings | Local MiniLM (384-d) | No API cost; CPU ingest slower than hosted embeddings |
| ANN index | HNSW + cosine | Fast search; index build cost on large corpora |
| Go ↔ Python | HTTP internal API | Clear boundary and independent tests; extra hop vs monolith |
| LLM | Ollama in Compose | Works offline; first `ollama pull` + cold start latency |
| Ingest orchestration | API inserts row, then RAG | `source_uri` preserved; rollback on RAG HTTP error, not on timeout |

**Vector dimension:** `VECTOR_DIMENSION`, embedding model, and `embeddings.embedding vector(n)` in [`db/schema.sql`](db/schema.sql) must stay aligned (default **384**).

## Limitations

- Sample seed is **plain text** only (`data/sample/*.txt`); no PDF pipeline in core scope.
- `make ingest` skips seeding if any document already exists (idempotent, not a full re-sync).
- Large cloud models need `OLLAMA_API_KEY` and high `OLLAMA_TIMEOUT_SECONDS`; default Compose path uses **local** `llama3.2`.
- MCP runs on the **host**; it does not start with `docker compose` (stdio adapter for Claude Desktop).
- No auth on REST endpoints (challenge scope).

## Development

Reset DB (re-applies `schema.sql` on empty volume):

```bash
docker compose down -v
docker compose up -d --build
```

Tests:

```bash
make test-rag
make test-api
```

### Run RAG on the host (optional)

```bash
cd services/rag && pip install -e ".[dev]"
uvicorn rag.app:app --reload --port 8000
```

Point `RAG_SERVICE_URL_DOCKER` at `http://host.docker.internal:8000` if only the API runs in Docker.

## Project docs

Site (GitHub Pages): <https://wellingtonn96.github.io/challenge-take-home-ia/> — serve a pasta `docs/` (landing em [`docs/index.html`](docs/index.html)).

- [`docs/overview.html`](docs/overview.html) — visão visual (marcos, diagramas, testes; abrir no navegador)
- [`docs/tutorial.html`](docs/tutorial.html) — tutorial prático passo a passo: como construir e entender o sistema do zero (abrir no navegador)
- [`docs/deep-dive.html`](docs/deep-dive.html) — código, conceitos e decisões (abrir no navegador)
- [`AGENTS.md`](AGENTS.md) — milestones and agent rules
- [`desafio.md`](desafio.md) — full challenge spec
- [`services/api/README.md`](services/api/README.md) — Go service notes
- [`frontend/README.md`](frontend/README.md) — React UI
