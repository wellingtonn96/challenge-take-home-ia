import atexit
import logging
import os
import sys
from contextlib import asynccontextmanager
from datetime import datetime

import uuid

from fastapi import FastAPI, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware

from rag.answer import generate_grounded_answer
from rag.config import Settings, get_settings
from rag.db import check_database, close_pool, init_pool
from rag.embedder import Embedder
from rag.errors import RAGError
from rag.llm import OllamaClient
from rag.repository import DocumentRepository
from rag.obslog import RingHandler, ring
from rag.trace_ctx import TRACE_HEADER, get_trace_id, reset_trace_id, set_trace_id
from rag.schemas import (
    HealthResponse,
    IngestRequest,
    IngestResponse,
    LogsResponse,
    QueryRequest,
    QueryResponse,
    SourceChunk,
)

logger = logging.getLogger("rag")

_ERROR_STATUS = {
    "no_relevant_chunks": 404,
    "llm_unavailable": 503,
    "llm_timeout": 504,
    "llm_error": 502,
}


def configure_logging(level_name: str) -> None:
    level = getattr(logging, level_name.upper(), logging.INFO)
    rag_logger = logging.getLogger("rag")
    rag_logger.setLevel(level)
    formatter = logging.Formatter("%(asctime)s %(levelname)s %(name)s: %(message)s")
    if not rag_logger.handlers:
        # stderr only: MCP stdio uses stdout for JSON-RPC.
        stream = logging.StreamHandler(sys.stderr)
        stream.setLevel(level)
        stream.setFormatter(formatter)
        rag_logger.addHandler(stream)
        ring_handler = RingHandler()
        ring_handler.setLevel(level)
        ring_handler.setFormatter(formatter)
        rag_logger.addHandler(ring_handler)


def init_runtime(*, register_atexit: bool = False) -> None:
    """Wire embedder, repository, and LLM onto app.state.

    Shared by the FastAPI lifespan and the MCP stdio process so both call
    the same run_query() instead of duplicating retrieval.
    """
    if getattr(app.state, "repository", None) is not None:
        return
    settings = get_settings()
    configure_logging(settings.log_level)
    init_pool(settings)
    llm = OllamaClient(settings)
    try:
        app.state.settings = settings
        app.state.embedder = Embedder(settings)
        app.state.repository = DocumentRepository(app.state.embedder)
        app.state.llm = llm
    except Exception:
        llm.close()
        close_pool()
        raise
    if register_atexit:
        atexit.register(shutdown_runtime)


def shutdown_runtime() -> None:
    llm = getattr(app.state, "llm", None)
    if llm is not None:
        llm.close()
        app.state.llm = None
    close_pool()
    app.state.repository = None
    app.state.embedder = None


def run_query(question: str, top_k: int | None = None) -> QueryResponse:
    """Retrieve + generate. Used by POST /query and the MCP tool."""
    settings: Settings = app.state.settings
    k = top_k or settings.retrieval_top_k
    answer, hits = generate_grounded_answer(
        question=question,
        top_k=k,
        embedder=app.state.embedder,
        repository=app.state.repository,
        llm=app.state.llm,
    )
    return QueryResponse(answer=answer, sources=[SourceChunk(**hit) for hit in hits])


@asynccontextmanager
async def lifespan(_app: FastAPI):
    init_runtime()
    try:
        yield
    finally:
        shutdown_runtime()


app = FastAPI(title="Knowledge Assistant RAG", lifespan=lifespan)

_cors_raw = os.environ.get(
    "CORS_ALLOWED_ORIGINS",
    "http://localhost:5173,http://localhost:3000",
)
app.add_middleware(
    CORSMiddleware,
    allow_origins=[part.strip() for part in _cors_raw.split(",") if part.strip()],
    allow_credentials=False,
    allow_methods=["GET", "POST", "OPTIONS"],
    allow_headers=["Content-Type", "Accept", TRACE_HEADER],
)


@app.middleware("http")
async def trace_middleware(request: Request, call_next):
    trace_id = request.headers.get(TRACE_HEADER, "").strip()
    if not trace_id:
        trace_id = str(uuid.uuid4())
    token = set_trace_id(trace_id)
    try:
        response = await call_next(request)
        response.headers[TRACE_HEADER] = trace_id
        return response
    finally:
        reset_trace_id(token)


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


@app.get("/logs", response_model=LogsResponse)
def logs(since: str | None = None, limit: int = 200) -> LogsResponse:
    since_dt: datetime | None = None
    if since:
        normalized = since.replace("Z", "+00:00")
        since_dt = datetime.fromisoformat(normalized)
    limit = max(1, min(limit, 500))
    return LogsResponse(
        service="rag",
        entries=[entry.to_dict() for entry in ring().snapshot(since_dt, limit)],
    )


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

    logger.info(
        "ingest complete document_id=%s chunks_created=%s",
        body.document_id,
        chunks_created,
        extra={"step": "ingest.ok"},
    )
    return IngestResponse(document_id=body.document_id, chunks_created=chunks_created)


def _http_error(exc: RAGError) -> HTTPException:
    logger.warning("query rejected: %s", exc)
    return HTTPException(
        status_code=_ERROR_STATUS.get(exc.code, 500),
        detail={"error": exc.code, "message": str(exc)},
    )


@app.post("/query", response_model=QueryResponse)
def query(body: QueryRequest) -> QueryResponse:
    try:
        return run_query(body.question, body.top_k)
    except RAGError as exc:
        raise _http_error(exc) from exc
    except Exception as exc:
        raise HTTPException(
            status_code=500,
            detail={"error": "query_failed", "message": str(exc)},
        ) from exc
