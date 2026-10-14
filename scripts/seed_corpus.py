#!/usr/bin/env python3
"""Load data/sample/* into the corpus via POST /ingest on the Go API."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SAMPLE_DIR = REPO_ROOT / "data" / "sample"
DEFAULT_API = "http://localhost:8080"


def _request(method: str, url: str, body: dict | None = None) -> bytes:
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    with urllib.request.urlopen(req, timeout=120) as resp:
        return resp.read()


def _list_documents(api_base: str) -> list[dict]:
    raw = _request("GET", f"{api_base}/documents")
    payload = json.loads(raw.decode("utf-8"))
    return list(payload.get("documents") or [])


def _ingest_file(api_base: str, path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    title = path.stem.replace("-", " ").replace("_", " ").strip().title()
    body = {
        "title": title,
        "text": text,
        "source_uri": f"file://data/sample/{path.name}",
        "content_type": "text/plain",
    }
    raw = _request("POST", f"{api_base}/ingest", body)
    return json.loads(raw.decode("utf-8"))


def main() -> int:
    api_base = os.environ.get("API_BASE_URL", DEFAULT_API).rstrip("/")
    if not SAMPLE_DIR.is_dir():
        print(f"Sample directory not found: {SAMPLE_DIR}", file=sys.stderr)
        return 1

    try:
        existing = _list_documents(api_base)
    except urllib.error.URLError as exc:
        print(f"Cannot reach API at {api_base}: {exc}", file=sys.stderr)
        return 1

    if existing:
        print(
            f"Corpus already has {len(existing)} document(s); skipping seed.",
            file=sys.stderr,
        )
        return 0

    files = sorted(SAMPLE_DIR.glob("*.txt"))
    if not files:
        print(f"No .txt files in {SAMPLE_DIR}", file=sys.stderr)
        return 1

    for path in files:
        try:
            result = _ingest_file(api_base, path)
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            print(f"Ingest failed for {path.name}: HTTP {exc.code} {detail}", file=sys.stderr)
            return 1
        except urllib.error.URLError as exc:
            print(f"Ingest failed for {path.name}: {exc}", file=sys.stderr)
            return 1
        print(
            f"Ingested {path.name} → document_id={result.get('document_id')} "
            f"chunks={result.get('chunks_created')}",
            file=sys.stderr,
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
