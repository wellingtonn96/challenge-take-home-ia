# RAG service (Python)

Internal HTTP service: chunk, embed, store in pgvector, retrieve top-k.

## Setup

From repo root (uses root `.env`):

```bash
cd services/rag
python -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -e ".[dev]"
```

Ensure PostgreSQL is up: `docker compose up -d` from repo root.

## Run

```bash
cd services/rag
uvicorn rag.app:app --host 0.0.0.0 --port 8000 --reload
```

Load env from repo root (pydantic reads `.env` in cwd — run from root or copy symlink):

```bash
cd ../..
set -a && source .env && set +a
cd services/rag && uvicorn rag.app:app --reload
```

## Tests

```bash
pytest
```

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
