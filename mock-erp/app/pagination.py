"""Cursor pagination helper.

Cursor is base64(last_pk). For master entities last_pk is `id` (str).
For facts entities last_pk is `id` (int) plus a sentinel suffix to keep ordering
stable when a single date contains many rows.
"""
from __future__ import annotations

import base64


def encode_cursor(value: str | int | None) -> str:
    if value is None:
        return ""
    raw = str(value).encode("utf-8")
    return base64.urlsafe_b64encode(raw).decode("ascii")


def decode_cursor(cursor: str | None) -> str | None:
    if not cursor:
        return None
    try:
        raw = base64.urlsafe_b64decode(cursor.encode("ascii"))
        return raw.decode("utf-8")
    except (ValueError, UnicodeDecodeError):
        return None


DEFAULT_LIMIT = 10000
MAX_LIMIT = 50000


def clamp_limit(limit: int | None) -> int:
    if not limit or limit <= 0:
        return DEFAULT_LIMIT
    if limit > MAX_LIMIT:
        return MAX_LIMIT
    return limit
