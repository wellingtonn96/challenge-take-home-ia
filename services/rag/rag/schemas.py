from uuid import UUID

from pydantic import BaseModel, Field


class HealthResponse(BaseModel):
    status: str
    database: str


class IngestRequest(BaseModel):
    document_id: UUID
    text: str = Field(min_length=1)
    title: str | None = None


class IngestResponse(BaseModel):
    document_id: UUID
    chunks_created: int


class QueryRequest(BaseModel):
    question: str = Field(min_length=1)
    top_k: int | None = Field(default=None, ge=1, le=50)


class SourceChunk(BaseModel):
    chunk_id: UUID
    document_id: UUID
    chunk_index: int
    content: str
    score: float


class QueryResponse(BaseModel):
    answer: str
    sources: list[SourceChunk]


class LogEntryOut(BaseModel):
    time: str
    level: str
    message: str
    trace_id: str | None = None
    step: str | None = None


class LogsResponse(BaseModel):
    service: str
    entries: list[LogEntryOut]
