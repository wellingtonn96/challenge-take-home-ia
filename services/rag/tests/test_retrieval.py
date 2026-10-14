"""E6.1 — unit retrieval with a fake vector store (no Postgres, no LLM)."""

from uuid import uuid4

import pytest

from rag.answer import generate_grounded_answer
from rag.errors import NoRelevantChunksError


class _FakeEmbedder:
    def __init__(self, vector: list[float] | None = None) -> None:
        self.vector = vector or [1.0, 0.0]
        self.last_question: str | None = None

    def embed_one(self, text: str) -> list[float]:
        self.last_question = text
        return self.vector


class _FakeVectorStore:
    def __init__(self, hits: list[dict]) -> None:
        self.hits = hits
        self.last_query_vector: list[float] | None = None
        self.last_top_k: int | None = None

    def search_similar(self, query_vector: list[float], top_k: int) -> list[dict]:
        self.last_query_vector = query_vector
        self.last_top_k = top_k
        return self.hits[:top_k]


class _SpyLLM:
    def __init__(self) -> None:
        self.called = False

    def chat(self, messages: list[dict[str, str]]) -> str:
        self.called = True
        return "should not run"


def test_retrieval_passes_embedding_and_top_k_to_vector_store():
    hit = {
        "chunk_id": uuid4(),
        "document_id": uuid4(),
        "chunk_index": 0,
        "content": "MiniLM is the default embedding model.",
        "score": 0.91,
    }
    embedder = _FakeEmbedder([0.5, 0.5])
    store = _FakeVectorStore([hit])
    llm = _SpyLLM()

    answer, hits = generate_grounded_answer(
        question="Which model is used?",
        top_k=3,
        embedder=embedder,
        repository=store,
        llm=llm,
    )

    assert embedder.last_question == "Which model is used?"
    assert store.last_query_vector == [0.5, 0.5]
    assert store.last_top_k == 3
    assert hits == [hit]
    assert answer == "should not run"
    assert llm.called


def test_empty_vector_store_raises_without_calling_llm():
    llm = _SpyLLM()
    with pytest.raises(NoRelevantChunksError):
        generate_grounded_answer(
            question="anything",
            top_k=5,
            embedder=_FakeEmbedder(),
            repository=_FakeVectorStore([]),
            llm=llm,
        )
    assert not llm.called
