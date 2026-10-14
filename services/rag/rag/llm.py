import json
import os

import httpx

from rag.config import Settings
from rag.errors import LLMError


def chat_url(base_url: str) -> str:
    """Local Ollama uses http://host:11434; cloud uses https://ollama.com (see Ollama API docs)."""
    base = base_url.rstrip("/")
    if base.endswith("/api"):
        return f"{base}/chat"
    return f"{base}/api/chat"


def resolve_api_key(settings: Settings) -> str | None:
    key = settings.ollama_api_key
    if key is None:
        raw = os.environ.get("OLLAMA_API_KEY")
        if raw is not None:
            key = raw.strip()
            if len(key) >= 2 and key[0] == key[-1] and key[0] in "\"'":
                key = key[1:-1].strip()
    return key or None


def uses_ollama_cloud(base_url: str) -> bool:
    host = base_url.rstrip("/").lower()
    return "ollama.com" in host


class OllamaClient:
    """Non-streaming Ollama /api/chat (local daemon or Ollama Cloud)."""

    def __init__(self, settings: Settings, *, client: httpx.Client | None = None) -> None:
        self._chat_url = chat_url(settings.ollama_base_url)
        self._model = settings.ollama_model
        self._timeout = settings.ollama_timeout_seconds
        self._api_key = resolve_api_key(settings)
        self._base_label = settings.ollama_base_url.rstrip("/")
        self._client = client or httpx.Client(timeout=httpx.Timeout(self._timeout))
        self._owns_client = client is None

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def _headers(self) -> dict[str, str]:
        headers = {"Content-Type": "application/json"}
        if self._api_key:
            headers["Authorization"] = f"Bearer {self._api_key}"
        return headers

    def chat(self, messages: list[dict[str, str]]) -> str:
        if uses_ollama_cloud(self._base_label) and not self._api_key:
            raise LLMError(
                "llm_error",
                "OLLAMA_API_KEY is missing. Set it in the repo root .env (not only "
                ".env.example), save the file, then restart the RAG process "
                "(uvicorn or docker compose restart rag).",
            )

        payload = {"model": self._model, "messages": messages, "stream": False}
        try:
            response = self._client.post(
                self._chat_url,
                json=payload,
                headers=self._headers(),
            )
        except httpx.TimeoutException as exc:
            raise LLMError(
                "llm_timeout",
                f"Ollama timed out after {self._timeout:g}s at {self._base_label}. "
                "Increase OLLAMA_TIMEOUT_SECONDS or try a smaller cloud model.",
            ) from exc
        except httpx.NetworkError as exc:
            raise LLMError(
                "llm_unavailable",
                f"Ollama is unreachable at {self._base_label}. "
                f"Check OLLAMA_BASE_URL and network access. ({exc})",
            ) from exc
        except httpx.HTTPError as exc:
            raise LLMError(
                "llm_unavailable",
                f"Ollama request to {self._base_label} failed. ({exc})",
            ) from exc

        if response.status_code == 401:
            raise LLMError(
                "llm_error",
                "Ollama Cloud rejected the API key (HTTP 401). Create a new key at "
                "ollama.com/settings/keys, update OLLAMA_API_KEY in the repo root "
                ".env, and restart the RAG service.",
            )

        if response.status_code >= 400:
            body = response.text.strip().replace("\n", " ")
            if len(body) > 300:
                body = body[:300] + "..."
            raise LLMError(
                "llm_error",
                f"Ollama returned HTTP {response.status_code} from {self._base_label}: {body}",
            )

        try:
            data = response.json()
        except json.JSONDecodeError as exc:
            raise LLMError(
                "llm_error",
                f"Ollama returned a non-JSON body from {self._base_label}.",
            ) from exc

        if not isinstance(data, dict):
            raise LLMError("llm_error", "Ollama returned an unexpected payload.")

        error = data.get("error")
        if error:
            raise LLMError("llm_error", f"Ollama error: {error}")

        message = data.get("message")
        content = message.get("content") if isinstance(message, dict) else None
        if not isinstance(content, str) or not content.strip():
            raise LLMError("llm_error", "Ollama returned an empty answer.")
        return content.strip()
