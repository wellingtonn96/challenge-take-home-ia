from functools import lru_cache
from pathlib import Path

from pydantic import Field, model_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

_REPO_ROOT = Path(__file__).resolve().parents[3]
_SERVICE_ROOT = Path(__file__).resolve().parents[1]


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=(_SERVICE_ROOT / ".env", _REPO_ROOT / ".env"),
        env_file_encoding="utf-8",
        extra="ignore",
    )

    postgres_host: str = Field(default="localhost", validation_alias="POSTGRES_HOST")
    postgres_port: int = Field(default=5432, validation_alias="POSTGRES_PORT")
    postgres_user: str = Field(default="knowledge", validation_alias="POSTGRES_USER")
    postgres_password: str = Field(default="change_me", validation_alias="POSTGRES_PASSWORD")
    postgres_db: str = Field(default="knowledge_assistant", validation_alias="POSTGRES_DB")
    database_url: str | None = Field(default=None, validation_alias="DATABASE_URL")
    vector_dimension: int = Field(default=384, validation_alias="VECTOR_DIMENSION")
    embedding_model: str = Field(
        default="all-MiniLM-L6-v2",
        validation_alias="EMBEDDING_MODEL",
    )
    chunk_size: int = Field(default=500, validation_alias="CHUNK_SIZE")
    chunk_overlap: int = Field(default=80, validation_alias="CHUNK_OVERLAP")
    retrieval_top_k: int = Field(default=5, validation_alias="RETRIEVAL_TOP_K")
    rag_service_host: str = Field(default="0.0.0.0", validation_alias="RAG_SERVICE_HOST")
    rag_service_port: int = Field(default=8000, validation_alias="RAG_SERVICE_PORT")

    @model_validator(mode="after")
    def resolve_database_url(self) -> "Settings":
        if self.database_url and "${" not in self.database_url:
            return self
        self.database_url = (
            f"postgresql://{self.postgres_user}:{self.postgres_password}"
            f"@{self.postgres_host}:{self.postgres_port}/{self.postgres_db}"
        )
        return self


@lru_cache
def get_settings() -> Settings:
    return Settings()
