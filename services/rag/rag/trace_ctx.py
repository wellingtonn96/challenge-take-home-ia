"""Per-request trace id for dev flow debugging (propagated via X-Trace-Id)."""

from __future__ import annotations

import uuid
from contextvars import ContextVar, Token

TRACE_HEADER = "X-Trace-Id"

_trace_id: ContextVar[str] = ContextVar("trace_id", default="")


def get_trace_id() -> str:
    return _trace_id.get()


def set_trace_id(trace_id: str) -> Token[str]:
    return _trace_id.set(trace_id or str(uuid.uuid4()))


def reset_trace_id(token: Token[str]) -> None:
    _trace_id.reset(token)
