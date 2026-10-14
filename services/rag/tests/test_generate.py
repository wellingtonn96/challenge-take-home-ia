"""E6.2 — unit generate with LLM mock (retrieval faked)."""

from uuid import uuid4

from rag.answer import generate_grounded_answer
from rag.prompt import SYSTEM_PROMPT


class _StubEmbedder:
    def embed_one(self, text: str) -> list[float]:
        return [0.0, 1.0]


class _StubVectorStore:
    def search_similar(self, query_vector: list[float], top_k: int) -> list[dict]:
        return [
            {
                "chunk_id": uuid4(),
                "document_id": uuid4(),
                "chunk_index": 0,
                "content": "VECTOR_DIMENSION defaults to 384 for all-MiniLM-L6-v2.",
                "score": 0.88,
            }
        ]


class _MockLLM:
    def __init__(self, answer: str = "384 dimensions.") -> None:
        self.answer = answer
        self.messages: list[dict[str, str]] | None = None

    def chat(self, messages: list[dict[str, str]]) -> str:
        self.messages = messages
        return self.answer


def test_generate_calls_llm_with_grounded_context():
    llm = _MockLLM()
    answer, hits = generate_grounded_answer(
        question="What is the vector dimension?",
        top_k=2,
        embedder=_StubEmbedder(),
        repository=_StubVectorStore(),
        llm=llm,
    )

    assert answer == "384 dimensions."
    assert len(hits) == 1
    assert llm.messages is not None
    assert llm.messages[0]["content"] == SYSTEM_PROMPT
    user_content = llm.messages[1]["content"]
    assert "VECTOR_DIMENSION defaults to 384" in user_content
    assert "What is the vector dimension?" in user_content
    assert "Use only the context below" in user_content
