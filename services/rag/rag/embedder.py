from functools import lru_cache

import numpy as np
from sentence_transformers import SentenceTransformer

from rag.config import Settings, get_settings


@lru_cache
def _load_model(model_name: str) -> SentenceTransformer:
    return SentenceTransformer(model_name)


class Embedder:
    def __init__(self, settings: Settings | None = None) -> None:
        self._settings = settings or get_settings()
        self._model = _load_model(self._settings.embedding_model)

    @property
    def model_name(self) -> str:
        return self._settings.embedding_model

    @property
    def dimension(self) -> int:
        return self._settings.vector_dimension

    def embed(self, texts: list[str]) -> list[list[float]]:
        if not texts:
            return []
        vectors = self._model.encode(
            texts,
            normalize_embeddings=True,
            show_progress_bar=False,
        )
        array = np.asarray(vectors, dtype=np.float32)
        if array.ndim == 1:
            array = array.reshape(1, -1)
        if array.shape[1] != self.dimension:
            raise ValueError(
                f"embedding dimension {array.shape[1]} != "
                f"VECTOR_DIMENSION {self.dimension}"
            )
        return array.tolist()

    def embed_one(self, text: str) -> list[float]:
        return self.embed([text])[0]
