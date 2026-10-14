import logging
from collections.abc import Sequence
from typing import Protocol

from rag.errors import NoRelevantChunksError
from rag.prompt import build_chat_messages, render_prompt

logger = logging.getLogger(__name__)


class EmbedsQuery(Protocol):
    def embed_one(self, text: str) -> list[float]: ...


class SearchesChunks(Protocol):
    def search_similar(self, query_vector: list[float], top_k: int) -> list[dict]: ...


class GeneratesAnswer(Protocol):
    def chat(self, messages: list[dict[str, str]]) -> str: ...


def generate_grounded_answer(
    *,
    question: str,
    top_k: int,
    embedder: EmbedsQuery,
    repository: SearchesChunks,
    llm: GeneratesAnswer,
) -> tuple[str, list[dict]]:
    """Retrieve top-k chunks and ask the LLM to answer from that context only."""
    logger.info("embedding question", extra={"step": "embed"})
    query_vector = embedder.embed_one(question)
    hits: Sequence[dict] = repository.search_similar(query_vector, top_k)
    logger.info("vector search finished hits=%s top_k=%s", len(hits), top_k, extra={"step": "search"})
    if not hits:
        raise NoRelevantChunksError()

    messages = build_chat_messages(question, list(hits))
    logger.info("query prompt\n%s", render_prompt(messages), extra={"step": "prompt"})
    logger.info("calling LLM", extra={"step": "llm"})
    answer = llm.chat(messages)
    logger.info("LLM response received chars=%s", len(answer), extra={"step": "llm.ok"})
    return answer, list(hits)
