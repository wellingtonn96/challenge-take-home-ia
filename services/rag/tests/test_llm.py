import json

import httpx
import pytest

from rag.config import Settings
from rag.errors import LLMError
from rag.llm import OllamaClient


def _settings() -> Settings:
    return Settings(
        ollama_base_url="http://ollama.test:11434",
        ollama_model="llama3.2",
        ollama_timeout_seconds=5,
    )


def _client(handler) -> OllamaClient:
    transport = httpx.MockTransport(handler)
    http = httpx.Client(transport=transport, timeout=httpx.Timeout(5))
    return OllamaClient(_settings(), client=http)


def test_chat_sends_bearer_for_cloud_api_key():
    settings = Settings(
        ollama_base_url="https://ollama.com",
        ollama_model="gemma4:31b",
        ollama_api_key="secret-key",
        ollama_timeout_seconds=5,
    )

    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.host == "ollama.com"
        assert request.headers["authorization"] == "Bearer secret-key"
        return httpx.Response(200, json={"message": {"role": "assistant", "content": "ok"}})

    transport = httpx.MockTransport(handler)
    client = OllamaClient(settings, client=httpx.Client(transport=transport, timeout=5))
    assert client.chat([{"role": "user", "content": "hi"}]) == "ok"
    client.close()


def test_chat_returns_assistant_content():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/api/chat"
        body = json.loads(request.content)
        assert body["model"] == "llama3.2"
        assert body["stream"] is False
        assert body["messages"][0]["role"] == "user"
        return httpx.Response(
            200,
            json={"message": {"role": "assistant", "content": "  MiniLM is the default.  "}},
        )

    client = _client(handler)
    assert client.chat([{"role": "user", "content": "Which model?"}]) == "MiniLM is the default."
    client.close()


def test_chat_when_ollama_is_down():
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused")

    client = _client(handler)
    with pytest.raises(LLMError) as exc:
        client.chat([{"role": "user", "content": "hi"}])
    assert exc.value.code == "llm_unavailable"
    assert "unreachable" in str(exc.value)
    assert "http://ollama.test:11434" in str(exc.value)


def test_chat_timeout():
    def handler(_request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timed out")

    client = _client(handler)
    with pytest.raises(LLMError) as exc:
        client.chat([{"role": "user", "content": "hi"}])
    assert exc.value.code == "llm_timeout"
    assert "timed out" in str(exc.value).lower()


def test_chat_http_error_and_empty_answer():
    def http_error(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(404, text='{"error":"model not found"}')

    client = _client(http_error)
    with pytest.raises(LLMError) as exc:
        client.chat([{"role": "user", "content": "hi"}])
    assert exc.value.code == "llm_error"
    assert "404" in str(exc.value)

    def empty(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"message": {"role": "assistant", "content": "  "}})

    client = _client(empty)
    with pytest.raises(LLMError) as exc:
        client.chat([{"role": "user", "content": "hi"}])
    assert "empty answer" in str(exc.value)
