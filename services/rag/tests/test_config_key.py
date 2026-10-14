from rag.config import Settings


def test_strips_quotes_from_api_key():
    s = Settings(
        ollama_api_key='"my-secret-key"',
        ollama_base_url="https://ollama.com",
    )
    assert s.ollama_api_key == "my-secret-key"


def test_empty_api_key_becomes_none():
    s = Settings(ollama_api_key="   ")
    assert s.ollama_api_key is None
