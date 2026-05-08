"""Async client for mock-erp HTTP API.

Provides graceful-fallback helpers used by `/m0` and any module page that
needs to compare its DB state against the upstream source-of-truth.

Все функции возвращают безопасные значения (0 / None / пустой список) при
ошибках сети — `/m0` НЕ должен падать, если mock-erp недоступен.
"""
from __future__ import annotations

import logging
from typing import Any

import httpx

logger = logging.getLogger("dashboards.mock_erp")


class MockErpClient:
    """Thin wrapper around mock-erp REST API."""

    def __init__(self, base_url: str, api_key: str, timeout: float = 5.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout

    @property
    def _headers(self) -> dict[str, str]:
        return {"X-API-Key": self.api_key}

    async def get_total_count(self, entity: str) -> int | None:
        """Return total rows for entity from X-Total-Count header.

        Returns:
            int — actual count from header.
            None — if mock-erp unreachable / header missing.
        """
        url = f"{self.base_url}/api/v1/{entity}"
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                r = await client.get(url, headers=self._headers, params={"limit": 1})
        except Exception as exc:  # noqa: BLE001
            logger.warning("mock-erp count(%s) failed: %s", entity, exc)
            return None

        total = r.headers.get("X-Total-Count")
        if total is not None:
            try:
                return int(total)
            except ValueError:
                logger.warning("mock-erp count(%s): invalid X-Total-Count=%r", entity, total)
                return None

        # Fallback: derive from JSON body length (best-effort).
        if r.status_code == 200:
            try:
                body = r.json()
                if isinstance(body, list):
                    return len(body)
                if isinstance(body, dict) and "items" in body:
                    items = body["items"]
                    if isinstance(items, list):
                        return len(items)
            except Exception as exc:  # noqa: BLE001
                logger.warning("mock-erp count(%s): json parse failed: %s", entity, exc)
        return None

    async def get_total_counts(self, entities: list[str]) -> dict[str, int | None]:
        """Fetch counts for many entities sequentially (simple — N <= 16)."""
        result: dict[str, int | None] = {}
        async with httpx.AsyncClient(timeout=self.timeout) as client:
            for entity in entities:
                url = f"{self.base_url}/api/v1/{entity}"
                try:
                    r = await client.get(url, headers=self._headers, params={"limit": 1})
                except Exception as exc:  # noqa: BLE001
                    logger.warning("mock-erp count(%s) failed: %s", entity, exc)
                    result[entity] = None
                    continue
                total = r.headers.get("X-Total-Count")
                if total is not None:
                    try:
                        result[entity] = int(total)
                        continue
                    except ValueError:
                        result[entity] = None
                        continue
                if r.status_code == 200:
                    try:
                        body = r.json()
                        if isinstance(body, list):
                            result[entity] = len(body)
                            continue
                        if isinstance(body, dict) and isinstance(body.get("items"), list):
                            result[entity] = len(body["items"])
                            continue
                    except Exception:  # noqa: BLE001
                        pass
                result[entity] = None
        return result

    async def get_received_orders_count(self) -> int | None:
        """Return X-Total-Count for /api/v1/orders/received."""
        url = f"{self.base_url}/api/v1/orders/received"
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                r = await client.get(url, headers=self._headers, params={"limit": 1})
        except Exception as exc:  # noqa: BLE001
            logger.warning("mock-erp received-orders count failed: %s", exc)
            return None
        total = r.headers.get("X-Total-Count")
        if total is not None:
            try:
                return int(total)
            except ValueError:
                return None
        if r.status_code == 200:
            try:
                body = r.json()
                if isinstance(body, list):
                    return len(body)
                if isinstance(body, dict) and isinstance(body.get("items"), list):
                    return len(body["items"])
            except Exception:  # noqa: BLE001
                return None
        return None

    async def get_received_orders_sample(self, limit: int = 10) -> list[dict[str, Any]]:
        """Return first N received POs (for sample table on /m0)."""
        url = f"{self.base_url}/api/v1/orders/received"
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                r = await client.get(url, headers=self._headers, params={"limit": limit})
        except Exception as exc:  # noqa: BLE001
            logger.warning("mock-erp received-orders sample failed: %s", exc)
            return []
        if r.status_code != 200:
            logger.warning("mock-erp received-orders sample: status=%s", r.status_code)
            return []
        try:
            body = r.json()
        except Exception as exc:  # noqa: BLE001
            logger.warning("mock-erp received-orders sample: json failed: %s", exc)
            return []
        if isinstance(body, list):
            return body[:limit]
        if isinstance(body, dict) and isinstance(body.get("items"), list):
            return body["items"][:limit]
        return []
