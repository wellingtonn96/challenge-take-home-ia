# RAG service (Python)

Internal HTTP service: chunk, embed, store in pgvector, retrieve top-k, generate a grounded answer with Ollama. The MCP stdio server lives in the same package and **imports `run_query()`** (the same function `POST /query` uses) — no second retrieval implementation.

## Setup

From repo root (uses root `.env`):

```bash
cd services/rag
python -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -e ".[dev]"
```

`[dev]` includes the MCP SDK used by tests and by `python -m rag.mcp`. To install only the MCP extra: `pip install -e ".[mcp]"`.

Ensure PostgreSQL is up: `docker compose up -d` from repo root.

Generation uses Ollama chat (`OLLAMA_BASE_URL`, `OLLAMA_MODEL`). **Cloud (default in `.env.example`):** set `OLLAMA_BASE_URL=https://ollama.com` and `OLLAMA_API_KEY` from [ollama.com/settings/keys](https://ollama.com/settings/keys). **Local:** `OLLAMA_BASE_URL=http://localhost:11434` and `ollama pull <model>`.

## Run

Settings load the repo-root `.env` from any working directory.

```bash
cd services/rag
uvicorn rag.app:app --host 0.0.0.0 --port 8000 --reload
```

## Tests

```bash
pytest
```

| File | Scope |
| --- | --- |
| `tests/test_retrieval.py` | E6.1 — fake vector store, no LLM on empty hits |
| `tests/test_generate.py` | E6.2 — LLM mock, grounded prompt |
| `tests/test_integration.py` | E6.4 — real Postgres (`docker compose up -d db`), stub embedder + LLM |

Go handler tests (E6.3): `services/api/internal/httpapi/handlers_test.go` — see `services/api/README.md`.

## Smoke (sample corpus)

```bash
DOC_ID=$(python -c "import uuid; print(uuid.uuid4())")
curl -s -X POST http://localhost:8000/ingest \
  -H "Content-Type: application/json" \
  -d "{\"document_id\":\"$DOC_ID\",\"text\":$(python -c "import json; print(json.dumps(open('../../data/sample/product-faq.txt').read()))")}"

curl -s -X POST http://localhost:8000/query \
  -H "Content-Type: application/json" \
  -d '{"question":"What is the default embedding model?"}'
```

`POST /query` returns `{ "answer", "sources" }`. The prompt (context + question + “only use the context”) is logged at INFO as `query prompt`. Ollama is non-streaming; the request waits until the model finishes or `OLLAMA_TIMEOUT_SECONDS` (default 300). The first call on CPU also loads the model into memory, so a cold `llama3.2` request can take a few minutes; a later call is much faster.

| Situation | Status | `detail.error` |
|-----------|--------|----------------|
| No chunks (empty corpus or no match) | 404 | `no_relevant_chunks` |
| Ollama down / unreachable | 503 | `llm_unavailable` |
| Ollama timeout | 504 | `llm_timeout` |
| Ollama HTTP or empty answer | 502 | `llm_error` |

Empty retrieval does not call the LLM, so the model cannot invent an answer when the corpus has nothing to ground it.

## MCP (`query_knowledge_base`)

Transport: **stdio** (Claude Desktop spawns the process). Mode: **in-process import** of `run_query()`, not an HTTP call to uvicorn. Postgres and Ollama must be reachable; the RAG HTTP server does **not** need to be running.

The HTTP source field is `content`. The challenge tool schema uses `chunk`, so the MCP layer maps `content` → `chunk` and stringifies `document_id`.

Lazy init: listing tools does not load MiniLM. The embedding model and DB pool start on the first tool call so Claude Desktop's initialize handshake stays fast.

### Claude Desktop

1. Install the RAG package into the venv (`pip install -e ".[mcp]"` or `".[dev]"`).
2. Ingest at least one document (see Smoke above, or the Go API).
3. Merge the JSON below into the Claude Desktop config, then restart Claude Desktop.

Config file:

| OS | Path |
| --- | --- |
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |

Replace the `command` path with the **absolute** path to this service's venv Python. `rag.config` also loads the repo-root `.env` by absolute path, so you can omit `env` if that file is already set; copying the vars here makes Claude Desktop self-contained.

**macOS / Linux** (Ollama Cloud — match `.env.example`)

```json
{
  "mcpServers": {
    "knowledge-assistant": {
      "command": "/absolute/path/to/challenge-take-home-ia/services/rag/.venv/bin/python",
      "args": ["-m", "rag.mcp"],
      "env": {
        "PYTHONUNBUFFERED": "1",
        "POSTGRES_HOST": "localhost",
        "POSTGRES_PORT": "5432",
        "POSTGRES_USER": "knowledge",
        "POSTGRES_PASSWORD": "change_me",
        "POSTGRES_DB": "knowledge_assistant",
        "EMBEDDING_MODEL": "all-MiniLM-L6-v2",
        "VECTOR_DIMENSION": "384",
        "OLLAMA_BASE_URL": "https://ollama.com",
        "OLLAMA_MODEL": "gemma4:31b-cloud",
        "OLLAMA_API_KEY": "your_ollama_cloud_key"
      }
    }
  }
}
```

**Windows**

```json
{
  "mcpServers": {
    "knowledge-assistant": {
      "command": "C:\\absolute\\path\\to\\challenge-take-home-ia\\services\\rag\\.venv\\Scripts\\python.exe",
      "args": ["-m", "rag.mcp"],
      "env": {
        "PYTHONUNBUFFERED": "1",
        "POSTGRES_HOST": "localhost",
        "POSTGRES_PORT": "5432",
        "POSTGRES_USER": "knowledge",
        "POSTGRES_PASSWORD": "change_me",
        "POSTGRES_DB": "knowledge_assistant",
        "EMBEDDING_MODEL": "all-MiniLM-L6-v2",
        "VECTOR_DIMENSION": "384",
        "OLLAMA_BASE_URL": "https://ollama.com",
        "OLLAMA_MODEL": "gemma4:31b-cloud",
        "OLLAMA_API_KEY": "your_ollama_cloud_key"
      }
    }
  }
}
```

Local Ollama instead: set `OLLAMA_BASE_URL` to `http://localhost:11434`, set `OLLAMA_MODEL` to a pulled tag, and omit `OLLAMA_API_KEY`.

Equivalent entrypoint after install: `knowledge-mcp` (same venv `Scripts` / `bin` directory).

Logs go to **stderr** only so they do not corrupt the JSON-RPC stream on stdout.

### Tool schema

```
Tool: query_knowledge_base
Input:  { "question": string }
Output: { "answer": string, "sources": [{ "document_id", "chunk", "score" }] }
```

### Inspector (optional)

From `services/rag` with the venv active:

```bash
npx @modelcontextprotocol/inspector python -m rag.mcp
```
