import logging
from typing import Protocol
from uuid import UUID

from pgvector import Vector
from pgvector.psycopg import register_vector

from rag.chunking import chunk_text
from rag.db import connection


class _EmbedsText(Protocol):
    model_name: str

    def embed(self, texts: list[str]) -> list[list[float]]: ...

logger = logging.getLogger(__name__)


class DocumentRepository:
    def __init__(self, embedder: _EmbedsText) -> None:
        self._embedder = embedder

    def ingest_document(
        self,
        document_id: UUID,
        text: str,
        title: str,
        chunk_size: int,
        chunk_overlap: int,
    ) -> int:
        pieces = chunk_text(text, chunk_size, chunk_overlap)
        if not pieces:
            raise ValueError("document text produced no chunks after chunking")
        logger.info("chunked document chunks=%s", len(pieces), extra={"step": "chunk"})

        vectors = self._embedder.embed(pieces)
        logger.info("embedded chunks model=%s", self._embedder.model_name, extra={"step": "embed.batch"})
        model_name = self._embedder.model_name

        with connection() as conn:
            register_vector(conn)
            with conn.transaction():
                conn.execute(
                    """
                    INSERT INTO documents (id, title, content_type)
                    VALUES (%s, %s, 'text/plain')
                    ON CONFLICT (id) DO UPDATE
                    SET title = EXCLUDED.title,
                        updated_at = now()
                    """,
                    (document_id, title),
                )
                conn.execute(
                    "DELETE FROM chunks WHERE document_id = %s",
                    (document_id,),
                )

                for index, (content, vector) in enumerate(zip(pieces, vectors, strict=True)):
                    row = conn.execute(
                        """
                        INSERT INTO chunks (document_id, chunk_index, content)
                        VALUES (%s, %s, %s)
                        RETURNING id
                        """,
                        (document_id, index, content),
                    ).fetchone()
                    if row is None:
                        raise RuntimeError("failed to insert chunk")
                    chunk_id = row["id"]
                    conn.execute(
                        """
                        INSERT INTO embeddings (chunk_id, embedding, model_name)
                        VALUES (%s, %s, %s)
                        """,
                        (chunk_id, Vector(vector), model_name),
                    )

        return len(pieces)

    def search_similar(
        self,
        query_vector: list[float],
        top_k: int,
    ) -> list[dict]:
        with connection() as conn:
            register_vector(conn)
            rows = conn.execute(
                """
                SELECT
                    c.id AS chunk_id,
                    c.document_id,
                    c.chunk_index,
                    c.content,
                    1 - (e.embedding <=> %s) AS score
                FROM embeddings e
                INNER JOIN chunks c ON c.id = e.chunk_id
                ORDER BY e.embedding <=> %s
                LIMIT %s
                """,
                (Vector(query_vector), Vector(query_vector), top_k),
            ).fetchall()

        return [
            {
                "chunk_id": row["chunk_id"],
                "document_id": row["document_id"],
                "chunk_index": row["chunk_index"],
                "content": row["content"],
                "score": float(row["score"]),
            }
            for row in rows
        ]
