"""MCP stdio server: one tool that imports the same RAG query path as POST /query.

Claude Desktop (and any MCP client) spawns this process. Retrieval and
generation stay in run_query(); this module is only the transport + the
challenge tool schema (sources[].chunk instead of HTTP content).
"""

from __future__ import annotations

import logging
import os
from typing import Annotated

from mcp.server.fastmcp import FastMCP
from pydantic import BaseModel, Field

# Keep tokenizer/HF chatter off stdout (JSON-RPC lives there).
os.environ.setdefault("HF_HUB_DISABLE_PROGRESS_BARS", "1")
os.environ.setdefault("TOKENIZERS_PARALLELISM", "false")
os.environ.setdefault("TRANSFORMERS_VERBOSITY", "error")

logger = logging.getLogger("rag.mcp")

mcp = FastMCP("knowledge-assistant")


class KnowledgeBaseSource(BaseModel):
    document_id: str
    chunk: str
    score: float


class KnowledgeBaseResult(BaseModel):
    answer: str
    sources: list[KnowledgeBaseSource]


def query_knowledge_base(
    question: Annotated[
        str,
        Field(min_length=1, description="Natural-language question to ask the corpus"),
    ],
) -> KnowledgeBaseResult:
    """Answer a question from ingested documents (RAG).

    Returns a grounded answer plus the chunks that were retrieved
    (document_id, chunk text, similarity score).
    """
    cleaned = question.strip()
    if not cleaned:
        raise ValueError("question must not be empty")

    # Import here so `python -m rag.mcp` (initialize/list_tools) does not load
    # torch / MiniLM. Retrieval still goes through the same run_query().
    from rag.app import init_runtime, run_query

    init_runtime(register_atexit=True)
    result = run_query(cleaned)

    return KnowledgeBaseResult(
        answer=result.answer,
        sources=[
            KnowledgeBaseSource(
                document_id=str(source.document_id),
                chunk=source.content,
                score=source.score,
            )
            for source in result.sources
        ],
    )


mcp.tool(
    name="query_knowledge_base",
    description=(
        "Query the knowledge base with a natural-language question. "
        "Returns a grounded answer and source chunks "
        "(document_id, chunk, score)."
    ),
)(query_knowledge_base)


def main() -> None:
    logger.info("mcp stdio server starting")
    mcp.run(transport="stdio")


if __name__ == "__main__":
    main()
