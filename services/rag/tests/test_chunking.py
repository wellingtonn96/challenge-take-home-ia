import pytest

from rag.chunking import chunk_text


def test_chunk_text_splits_with_overlap():
    text = "a" * 85
    chunks = chunk_text(text, chunk_size=40, chunk_overlap=10)
    assert len(chunks) == 3
    assert all(len(c) <= 40 for c in chunks)
    assert chunks[0] == "a" * 40
    assert chunks[1] == "a" * 40
    assert chunks[2] == "a" * 25


def test_chunk_text_empty_returns_empty_list():
    assert chunk_text("   ", chunk_size=100, chunk_overlap=10) == []


def test_chunk_text_overlap_must_be_smaller_than_size():
    with pytest.raises(ValueError, match="chunk_overlap"):
        chunk_text("hello", chunk_size=10, chunk_overlap=10)
