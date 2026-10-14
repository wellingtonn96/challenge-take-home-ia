from uuid import uuid4

import pytest
from fastapi import HTTPException

from rag.app import app, query
from rag.config import get_settings
from rag.errors import LLMError
from rag.schemas import QueryRequest


class _Embedder:
    def embed_one(self, text: str) -> list[float]:
        return [0.0, 1.0]


class _Repository:
    def __init__(self, hits: list[dict]) -> None:
        self.hits = hits

    def search_similar(self, query_vector: list[float], top_k: int) -> list[dict]:
        return self.hits[:top_k]


class _LLM:
    def __init__(self, answer: str = "The default model is all-MiniLM-L6-v2.") -> None:
        self.answer = answer
        self.messages: list[dict[str, str]] | None = None

    def chat(self, messages: list[dict[str, str]]) -> str:
        self.messages = messages
        return self.answer


def _wire(*, hits: list[dict], llm: _LLM | None = None) -> _LLM:
    model = llm or _LLM()
    app.state.settings = get_settings()
    app.state.embedder = _Embedder()
    app.state.repository = _Repository(hits)
    app.state.llm = model
    return model


def _hit() -> dict:
    return {
        "chunk_id": uuid4(),
        "document_id": uuid4(),
        "chunk_index": 0,
        "content": "The default local model is all-MiniLM-L6-v2.",
        "score": 0.88,
    }


def test_query_returns_answer_and_sources():
    hit = _hit()
    llm = _wire(hits=[hit])
    response = query(QueryRequest(question="Which embedding model is used by default?"))

    assert response.answer == "The default model is all-MiniLM-L6-v2."
    assert len(response.sources) == 1
    assert response.sources[0].chunk_id == hit["chunk_id"]
    assert response.sources[0].content == hit["content"]
    assert llm.messages is not None
    assert "all-MiniLM-L6-v2" in llm.messages[1]["content"]
    assert "only using the provided context" in llm.messages[0]["content"]


def test_query_empty_corpus_returns_404():
    _wire(hits=[])
    with pytest.raises(HTTPException) as exc:
        query(QueryRequest(question="Which embedding model is used by default?"))

    assert exc.value.status_code == 404
    assert exc.value.detail["error"] == "no_relevant_chunks"
    assert "Ingest" in exc.value.detail["message"]


def test_query_ollama_down_returns_503():
    class Down:
        def chat(self, messages: list[dict[str, str]]) -> str:
            raise LLMError(
                "llm_unavailable",
                "Ollama is unreachable at http://localhost:11434.",
            )

    app.state.settings = get_settings()
    app.state.embedder = _Embedder()
    app.state.repository = _Repository([_hit()])
    app.state.llm = Down()

    with pytest.raises(HTTPException) as exc:
        query(QueryRequest(question="Which embedding model is used by default?"))

    assert exc.value.status_code == 503
    assert exc.value.detail["error"] == "llm_unavailable"
    assert "unreachable" in exc.value.detail["message"]
