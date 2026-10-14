import json
from uuid import uuid4

import pytest

from rag.errors import NoRelevantChunksError
from rag.mcp import KnowledgeBaseResult, mcp, query_knowledge_base
from rag.schemas import QueryResponse, SourceChunk


def _patch_query(monkeypatch, run_query) -> None:
    import rag.app as app

    monkeypatch.setattr(app, "init_runtime", lambda **_kwargs: None)
    monkeypatch.setattr(app, "run_query", run_query)


def _result(monkeypatch, response: QueryResponse) -> KnowledgeBaseResult:
    _patch_query(monkeypatch, lambda question, top_k=None: response)
    return query_knowledge_base("Which embedding model is used by default?")


def test_mcp_tool_input_output_json_schema_matches_spec():
    tool = mcp._tool_manager.get_tool("query_knowledge_base")
    assert tool is not None
    assert tool.parameters["type"] == "object"
    assert tool.parameters["required"] == ["question"]
    assert tool.parameters["properties"]["question"]["type"] == "string"
    output = tool.output_schema
    assert set(output["properties"]) == {"answer", "sources"}
    source = output["$defs"]["KnowledgeBaseSource"]["properties"]
    assert set(source) == {"document_id", "chunk", "score"}


def test_query_knowledge_base_output_matches_spec_schema(monkeypatch):
    doc_id = uuid4()
    response = QueryResponse(
        answer="The default model is all-MiniLM-L6-v2.",
        sources=[
            SourceChunk(
                chunk_id=uuid4(),
                document_id=doc_id,
                chunk_index=0,
                content="The default local model is all-MiniLM-L6-v2.",
                score=0.91,
            )
        ],
    )

    payload = _result(monkeypatch, response).model_dump(mode="json")
    encoded = json.dumps(payload)
    parsed = json.loads(encoded)

    assert parsed == {
        "answer": "The default model is all-MiniLM-L6-v2.",
        "sources": [
            {
                "document_id": str(doc_id),
                "chunk": "The default local model is all-MiniLM-L6-v2.",
                "score": 0.91,
            }
        ],
    }
    assert "content" not in parsed["sources"][0]
    schema = KnowledgeBaseResult.model_json_schema()
    assert set(schema["properties"]) == {"answer", "sources"}
    defs = schema.get("$defs") or schema.get("definitions") or {}
    source_schema = defs["KnowledgeBaseSource"]["properties"]
    assert set(source_schema) == {"document_id", "chunk", "score"}


def test_query_knowledge_base_reuses_run_query(monkeypatch):
    calls: list[str] = []

    def fake_run_query(question: str, top_k: int | None = None) -> QueryResponse:
        calls.append(question)
        return QueryResponse(answer="ok", sources=[])

    _patch_query(monkeypatch, fake_run_query)

    result = query_knowledge_base("  What is pgvector?  ")

    assert calls == ["What is pgvector?"]
    assert result.answer == "ok"
    assert result.sources == []


def test_query_knowledge_base_rejects_empty_question():
    with pytest.raises(ValueError, match="empty"):
        query_knowledge_base("   ")


def test_query_knowledge_base_propagates_rag_error(monkeypatch):
    def boom(question: str, top_k: int | None = None) -> QueryResponse:
        raise NoRelevantChunksError()

    _patch_query(monkeypatch, boom)

    with pytest.raises(NoRelevantChunksError):
        query_knowledge_base("Anything")
