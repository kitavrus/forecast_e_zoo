"""API key dependency.

Принимает токен в одном из заголовков:
- ``X-API-Key: <token>`` — основной формат, используется source-adapter
  HTTP reader (см. internal/features/data_export/loader/http_source_reader.go).
- ``Authorization: Bearer <token>`` — формат channel-router/sender
  (см. internal/features/channels/auth/api_key.go). Без поддержки этого
  варианта e_zoo channel-router не может отправлять PO в mock-erp.
"""
from __future__ import annotations

import os

from fastapi import Header, HTTPException, status


def require_api_key(
    x_api_key: str | None = Header(default=None),
    authorization: str | None = Header(default=None),
) -> None:
    expected = os.getenv("MOCK_ERP_API_KEY", "test-api-key")
    token: str | None = x_api_key
    if token is None and authorization is not None:
        prefix = "Bearer "
        if authorization.startswith(prefix):
            token = authorization[len(prefix):]
    if token != expected:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid api key",
        )
