# Go API

Public REST surface. It stores document metadata in Postgres and calls the Python RAG service for chunking, embeddings, and answers.

The image is built by Docker Compose. A Go toolchain on the host is not required.

## Run

From the repo root, with `.env` present:

```bash
docker compose up -d --build
```

This starts Postgres, Ollama, RAG, API, and the UI. The API is published on `API_PORT` (default `8080`). If that host port is already taken, change `API_PORT` in `.env` and run the command again. The process inside the container still listens on `8080`.

`POSTGRES_HOST` inside the container is `db`. `RAG_SERVICE_URL` defaults to `http://rag:8000` on the Compose network.

Seed sample documents from the repo root: `make ingest` (see root `README.md`).

HTTP contract: [`docs/api.md`](../../docs/api.md).

## Endpoints

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| `GET` | `/health` | | `200 {"status":"ok","database":"ok"}` |
| `GET` | `/documents` | | `200 {"documents":[{"id","title","created_at"}]}` |
| `POST` | `/ingest` | `{"title","text","source_uri?","content_type?"}` | `201` same shape as Python ingest |
| `POST` | `/query` | `{"question","top_k?"}` | `200` same JSON as Python `/query` |
| `DELETE` | `/documents/{id}` | | `204` |

Errors are always `{"error","message"}`.

| Situation | Status | `error` |
| --- | --- | --- |
| Invalid JSON, missing title/text/question, bad UUID, `top_k` out of 1..50 | 400 | `invalid_request` |
| Unknown route or document | 404 | `not_found` |
| RAG has no chunks | 404 | `no_relevant_chunks` |
| RAG down, timed out, or returned 5xx | 502 | `rag_unavailable` or the Python code |
| Postgres down | 503 | `database_unavailable` |

`POST /ingest` writes the metadata row, then calls Python. If Python answers with an HTTP error, the row is deleted. A timeout does not delete it: the RAG transaction may still commit. `DELETE` relies on `ON DELETE CASCADE` for `chunks` and `embeddings`.

## Tests

Unit tests mock Postgres and the RAG client. The repository test uses the Compose database and skips when it is unreachable.

```bash
docker compose up -d db
docker run --rm --network container:knowledge-assistant-db \
  -v "$PWD:/work" -w /work/services/api \
  golang:1.23-bookworm go test ./...
```

`--network container:knowledge-assistant-db` makes `localhost:5432` inside the test container the database published by that container, which matches `POSTGRES_HOST` in `.env`.
