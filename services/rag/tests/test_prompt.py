import logging

from rag.prompt import SYSTEM_PROMPT, build_chat_messages, render_prompt


def test_prompt_includes_context_question_and_grounding_rule():
    messages = build_chat_messages(
        "Which embedding model is used by default?",
        [
            {
                "document_id": "doc-1",
                "chunk_index": 0,
                "content": "The default local model is all-MiniLM-L6-v2.",
            }
        ],
    )
    rendered = render_prompt(messages)

    assert messages[0]["content"] == SYSTEM_PROMPT
    assert "all-MiniLM-L6-v2" in rendered
    assert "Which embedding model is used by default?" in rendered
    assert "Answer only using the provided context" in rendered
    assert "Use only the context below" in rendered


def test_generate_grounded_answer_logs_prompt(caplog):
    """Logging around the LLM step; generation behaviour is covered in test_generate.py."""
    from rag.answer import generate_grounded_answer

    class Embedder:
        def embed_one(self, text: str) -> list[float]:
            return [0.1, 0.2]

    class Repository:
        def search_similar(self, query_vector: list[float], top_k: int) -> list[dict]:
            return [{"document_id": "doc-1", "chunk_index": 0, "content": "VECTOR_DIMENSION must be 384."}]

    class LLM:
        def chat(self, messages: list[dict[str, str]]) -> str:
            return "Use 384."

    with caplog.at_level(logging.INFO, logger="rag.answer"):
        generate_grounded_answer(
            question="What is the vector dimension?",
            top_k=2,
            embedder=Embedder(),
            repository=Repository(),
            llm=LLM(),
        )

    assert "query prompt" in caplog.text
    assert "What is the vector dimension?" in caplog.text
