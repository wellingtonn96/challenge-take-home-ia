from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException

from rag.config import get_settings
from rag.db import check_database, close_pool, init_pool
from rag.embedder import Embedder
from rag.repository import DocumentRepository
from rag.schemas import (
    HealthResponse,
    IngestRequest,
    IngestResponse,
    QueryRequest,
    QueryResponse,
    SourceChunk,
)


@asynccontextmanager
async def lifespan(_app: FastAPI):
    settings = get_settings()
    init_pool(settings)
    _app.state.embedder = Embedder(settings)
    _app.state.repository = DocumentRepository(_app.state.embedder)
    _app.state.settings = settings
    yield
    close_pool()


app = FastAPI(title="Knowledge Assistant RAG", lifespan=lifespan)


@app.get("/health", response_model=HealthResponse)
def health() -> HealthResponse:
    try:
        check_database()
    except Exception as exc:
        raise HTTPException(
            status_code=503,
            detail={"error": "database_unavailable", "message": str(exc)},
        ) from exc
    return HealthResponse(status="ok", database="ok")


@app.post("/ingest", response_model=IngestResponse)
def ingest(body: IngestRequest) -> IngestResponse:
    settings = app.state.settings
    repository: DocumentRepository = app.state.repository
    title = body.title or str(body.document_id)

    try:
        chunks_created = repository.ingest_document(
            document_id=body.document_id,
            text=body.text,
            title=title,
            chunk_size=settings.chunk_size,
            chunk_overlap=settings.chunk_overlap,
        )
    except ValueError as exc:
        raise HTTPException(
            status_code=400,
            detail={"error": "invalid_request", "message": str(exc)},
        ) from exc
    except Exception as exc:
        raise HTTPException(
            status_code=500,
            detail={"error": "ingest_failed", "message": str(exc)},
        ) from exc

    return IngestResponse(document_id=body.document_id, chunks_created=chunks_created)


@app.post("/query", response_model=QueryResponse)
def query(body: QueryRequest) -> QueryResponse:
    settings = app.state.settings
    embedder: Embedder = app.state.embedder
    repository: DocumentRepository = app.state.repository
    top_k = body.top_k or settings.retrieval_top_k

    try:
        query_vector = embedder.embed_one(body.question)
        hits = repository.search_similar(query_vector, top_k)
    except Exception as exc:
        raise HTTPException(
            status_code=500,
            detail={"error": "query_failed", "message": str(exc)},
        ) from exc

    sources = [SourceChunk(**hit) for hit in hits]
    return QueryResponse(answer="TODO", sources=sources)
