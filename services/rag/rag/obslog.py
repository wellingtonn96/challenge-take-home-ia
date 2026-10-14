"""In-memory log ring for the dev UI (not a replacement for real observability)."""

from __future__ import annotations

import logging
import threading
from collections import deque
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Deque


@dataclass(frozen=True)
class LogEntry:
    time: datetime
    level: str
    message: str
    trace_id: str = ""
    step: str = ""

    def to_dict(self) -> dict[str, str]:
        out = {
            "time": self.time.isoformat().replace("+00:00", "Z"),
            "level": self.level,
            "message": self.message,
        }
        if self.trace_id:
            out["trace_id"] = self.trace_id
        if self.step:
            out["step"] = self.step
        return out


class RingBuffer:
    def __init__(self, capacity: int = 400) -> None:
        self._capacity = max(1, capacity)
        self._entries: Deque[LogEntry] = deque(maxlen=self._capacity)
        self._lock = threading.Lock()

    def add(
        self,
        level: str,
        message: str,
        *,
        trace_id: str = "",
        step: str = "",
    ) -> None:
        line = (message or "").strip()
        if not line:
            return
        entry = LogEntry(
            time=datetime.now(timezone.utc),
            level=level.upper(),
            message=line,
            trace_id=(trace_id or "").strip(),
            step=(step or "").strip(),
        )
        with self._lock:
            self._entries.append(entry)

    def snapshot(self, since: datetime | None = None, limit: int = 200) -> list[LogEntry]:
        limit = max(1, min(limit, self._capacity))
        with self._lock:
            items = list(self._entries)
        if since is not None:
            items = [e for e in items if e.time > since]
        if len(items) > limit:
            items = items[-limit:]
        return items


_RING = RingBuffer()


def ring() -> RingBuffer:
    return _RING


class RingHandler(logging.Handler):
    def emit(self, record: logging.LogRecord) -> None:
        from rag.trace_ctx import get_trace_id

        try:
            message = self.format(record)
        except Exception:
            message = record.getMessage()
        step = getattr(record, "step", "") or ""
        _RING.add(
            record.levelname,
            message,
            trace_id=get_trace_id(),
            step=step,
        )
