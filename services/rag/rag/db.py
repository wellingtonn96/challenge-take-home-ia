from collections.abc import Generator
from contextlib import contextmanager
from typing import TYPE_CHECKING

from psycopg.rows import dict_row
from psycopg_pool import ConnectionPool

from rag.config import Settings, get_settings

if TYPE_CHECKING:
    from psycopg import Connection

_pool: ConnectionPool | None = None


def init_pool(settings: Settings | None = None) -> ConnectionPool:
    global _pool
    if _pool is not None:
        return _pool
    cfg = settings or get_settings()
    _pool = ConnectionPool(
        conninfo=cfg.database_url,
        min_size=1,
        max_size=10,
        kwargs={"row_factory": dict_row},
        open=True,
    )
    return _pool


def close_pool() -> None:
    global _pool
    if _pool is not None:
        _pool.close()
        _pool = None


def get_pool() -> ConnectionPool:
    if _pool is None:
        return init_pool()
    return _pool


@contextmanager
def connection() -> Generator["Connection", None, None]:
    pool = get_pool()
    with pool.connection() as conn:
        yield conn


def check_database() -> None:
    with connection() as conn:
        row = conn.execute("SELECT 1 AS ok").fetchone()
        if row is None or row["ok"] != 1:
            raise RuntimeError("database health check failed")
