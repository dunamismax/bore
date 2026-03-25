"""HTTP client for the Go relay API."""

from typing import Any

import httpx

from app.config import settings


async def fetch_status() -> dict[str, Any]:
    """Fetch /status from the Go relay."""
    async with httpx.AsyncClient(timeout=5.0) as client:
        resp = await client.get(f"{settings.relay_url}/status")
        resp.raise_for_status()
        return resp.json()  # type: ignore[no-any-return]


async def fetch_healthz() -> dict[str, Any]:
    """Fetch /healthz from the Go relay."""
    async with httpx.AsyncClient(timeout=5.0) as client:
        resp = await client.get(f"{settings.relay_url}/healthz")
        resp.raise_for_status()
        return resp.json()  # type: ignore[no-any-return]


async def fetch_metrics() -> dict[str, Any]:
    """Fetch /metrics from the Go relay."""
    async with httpx.AsyncClient(timeout=5.0) as client:
        resp = await client.get(f"{settings.relay_url}/metrics")
        resp.raise_for_status()
        return resp.json()  # type: ignore[no-any-return]
