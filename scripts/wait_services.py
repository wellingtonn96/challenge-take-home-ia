#!/usr/bin/env python3
"""Block until the Go API and RAG HTTP health endpoints respond."""

from __future__ import annotations

import os
import sys
import time
import urllib.error
import urllib.request

DEFAULT_API = "http://localhost:8080"
DEFAULT_RAG = "http://localhost:8000"
TIMEOUT_S = 5
MAX_ATTEMPTS = 90
SLEEP_S = 5


def _ok(url: str) -> bool:
    try:
        with urllib.request.urlopen(url, timeout=TIMEOUT_S) as resp:
            return 200 <= resp.status < 300
    except (urllib.error.URLError, TimeoutError, OSError):
        return False


def main() -> int:
    api_base = os.environ.get("API_BASE_URL", DEFAULT_API).rstrip("/")
    rag_base = os.environ.get("RAG_SERVICE_URL", DEFAULT_RAG).rstrip("/")
    targets = [
        ("API", f"{api_base}/health"),
        ("RAG", f"{rag_base}/health"),
    ]
    for attempt in range(1, MAX_ATTEMPTS + 1):
        pending = [name for name, url in targets if not _ok(url)]
        if not pending:
            print("All services healthy.", file=sys.stderr)
            return 0
        print(
            f"Waiting for {', '.join(pending)} "
            f"(attempt {attempt}/{MAX_ATTEMPTS})…",
            file=sys.stderr,
        )
        time.sleep(SLEEP_S)
    print("Timed out waiting for services.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
