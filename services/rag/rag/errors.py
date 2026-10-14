class RAGError(Exception):
    """Domain failure with a stable machine code and a human message."""

    def __init__(self, code: str, message: str) -> None:
        self.code = code
        super().__init__(message)


class NoRelevantChunksError(RAGError):
    def __init__(self) -> None:
        super().__init__(
            "no_relevant_chunks",
            "No chunks found for this question. Ingest a document first, "
            "or the corpus has no matching text.",
        )


class LLMError(RAGError):
    """Ollama chat failed. code is llm_unavailable, llm_timeout, or llm_error."""
