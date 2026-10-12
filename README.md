# AI Knowledge Assistant (take-home challenge)

Production-minded RAG assistant: Python RAG service, Go REST API, PostgreSQL + pgvector, MCP tool. Work in progress by milestone (E0 → E5).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with Compose v2
- Git

## Setup (E1 — database only)

1. Copy environment file:

   ```bash
   cp .env.example .env
   ```

2. Start PostgreSQL with pgvector:

   ```bash
   docker compose up -d
   ```

3. Wait until the `db` service is healthy, then verify schema:

   ```bash
   docker compose exec db psql -U knowledge -d knowledge_assistant -c "\dt"
   docker compose exec db psql -U knowledge -d knowledge_assistant -c "\d+ embeddings"
   docker compose exec db psql -U knowledge -d knowledge_assistant -c "\dx"
   ```

   Expected: tables `documents`, `chunks`, `embeddings`; extension `vector`; column `embedding` type `vector(384)`.

### Vector dimension

Default embedding model: **all-MiniLM-L6-v2** → **384** dimensions. This matches `VECTOR_DIMENSION` in `.env` and `embeddings.embedding vector(384)` in [`db/schema.sql`](db/schema.sql). Changing the model requires updating all three and re-embedding documents.

### Reset database (dev)

Init SQL runs only on a fresh data volume:

```bash
docker compose down -v
docker compose up -d
```

## Project docs

- [`AGENTS.md`](AGENTS.md) — stack, boundaries, and agent rules
- [`desafio.md`](desafio.md) — full challenge requirements

## Architecture (planned)

```
Client → Go API → Python RAG → PostgreSQL (metadata + pgvector)
MCP tool ────────────────┘
```

Further sections (RAG, API, MCP, tests, full `docker compose` stack) will be added in later milestones.
