"""E6.4 — integration: real Postgres, minimal ingest, query with LLM stub."""

from uuid import uuid4

import pytest

from rag.answer import generate_grounded_answer
from rag.config import get_settings
from rag.db import check_database, close_pool, init_pool
from rag.repository import DocumentRepository


class _DeterministicEmbedder:
    """Fixed 384-d vectors so ingest and query share the same geometry (no ML model)."""

    model_name = "test-deterministic"

    def __init__(self, dimension: int = 384) -> None:
        self._dimension = dimension

    def embed(self, texts: list[str]) -> list[list[float]]:
        return [self.embed_one(text) for text in texts]

    def embed_one(self, text: str) -> list[float]:
        vec = [0.0] * self._dimension
        vec[0] = 1.0
        return vec


class _StubLLM:
    def chat(self, messages: list[dict[str, str]]) -> str:
        assert "MiniLM" in messages[1]["content"]
        return "The corpus mentions MiniLM as the default model."


@pytest.fixture(scope="module")
def postgres_ready():
    settings = get_settings()
    init_pool(settings)
    try:
        check_database()
    except Exception as exc:
        close_pool()
        pytest.skip(f"postgres not available: {exc}")
    yield settings
    close_pool()


def test_ingest_and_query_against_real_postgres(postgres_ready):
    settings = postgres_ready
    embedder = _DeterministicEmbedder(settings.vector_dimension)
    repo = DocumentRepository(embedder)
    document_id = uuid4()

    try:
        chunks = repo.ingest_document(
            document_id=document_id,
            text="The default local embedding model is all-MiniLM-L6-v2.",
            title="integration-fixture",
            chunk_size=settings.chunk_size,
            chunk_overlap=settings.chunk_overlap,
        )
        assert chunks >= 1

        answer, hits = generate_grounded_answer(
            question="Which embedding model is default?",
            top_k=3,
            embedder=embedder,
            repository=repo,
            llm=_StubLLM(),
        )
        assert "MiniLM" in answer
        assert len(hits) >= 1
        assert any("MiniLM" in hit["content"] for hit in hits)
    finally:
        from rag.db import connection

        with connection() as conn:
            conn.execute("DELETE FROM documents WHERE id = %s", (document_id,))
