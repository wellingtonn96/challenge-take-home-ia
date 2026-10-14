# REST API (Go)

Public HTTP surface. Base URL: `API_BASE_URL` from `.env` (default `http://localhost:8080`).

All error responses use:

```json
{ "error": "<code>", "message": "<human-readable detail>" }
```

## `POST /ingest`

Add a plain-text document to the corpus. The API stores metadata in PostgreSQL, then calls the internal RAG service to chunk, embed, and store vectors.

**Request**

```json
{
  "title": "Product FAQ",
  "text": "Full document body…",
  "source_uri": "file://data/sample/product-faq.txt",
  "content_type": "text/plain"
}
```

| Field | Required | Notes |
| --- | --- | --- |
| `title` | yes | Non-empty after trim |
| `text` | yes | Non-empty after trim |
| `source_uri` | no | Stored on the metadata row |
| `content_type` | no | Defaults to `text/plain` |

**Success — `201 Created`**

```json
{
  "document_id": "550e8400-e29b-41d4-a716-446655440000",
  "chunks_created": 12
}
```

**Common errors**

| Status | `error` | When |
| --- | --- | --- |
| `400` | `invalid_request` | Missing title/text, bad JSON |
| `502` | `rag_unavailable` / upstream code | RAG down or embed failure |
| `503` | `database_unavailable` | Postgres unreachable |

---

## `POST /query`

Natural-language question over the ingested corpus. Returns a grounded answer and the retrieved source chunks (same JSON as the Python RAG service).

**Request**

```json
{
  "question": "How does ingestion work?",
  "top_k": 5
}
```

| Field | Required | Notes |
| --- | --- | --- |
| `question` | yes | Non-empty after trim |
| `top_k` | no | Integer 1–50; server default from `RETRIEVAL_TOP_K` |

**Success — `200 OK`**

```json
{
  "answer": "…",
  "sources": [
    {
      "chunk_id": "…",
      "document_id": "…",
      "chunk_index": 0,
      "content": "…",
      "score": 0.82
    }
  ]
}
```

**Common errors**

| Status | `error` | When |
| --- | --- | --- |
| `400` | `invalid_request` | Missing question or invalid `top_k` |
| `404` | `no_relevant_chunks` | Empty corpus or no chunks above threshold |
| `502` | `llm_error` / `rag_unavailable` | LLM or RAG failure |
| `503` | `llm_unavailable` | Ollama unreachable |
| `504` | `llm_timeout` | Generation exceeded `OLLAMA_TIMEOUT_SECONDS` |

---

## `GET /documents`

List ingested documents (metadata only).

**Success — `200 OK`**

```json
{
  "documents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "title": "Product FAQ",
      "created_at": "2026-03-26T12:00:00Z"
    }
  ]
}
```

---

## `DELETE /documents/{id}`

Remove a document and its chunks/embeddings (`ON DELETE CASCADE`).

**Path parameter:** `id` — UUID.

**Success — `204 No Content`** (empty body)

**Common errors**

| Status | `error` | When |
| --- | --- | --- |
| `400` | `invalid_request` | `id` is not a valid UUID |
| `404` | `not_found` | Unknown document |

---

## Health (optional)

`GET /health` → `200 {"status":"ok","database":"ok"}` when Postgres is reachable.
