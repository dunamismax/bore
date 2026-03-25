"""Tests for FastAPI routes."""

from typing import Any
from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient

from app.main import app

MOCK_STATUS: dict[str, Any] = {
    "service": "bore-relay",
    "status": "ok",
    "uptimeSeconds": 3661,
    "rooms": {"total": 5, "waiting": 2, "active": 3},
    "limits": {
        "maxRooms": 100,
        "roomTTLSeconds": 300,
        "reapIntervalSeconds": 60,
        "maxMessageSizeBytes": 67108864,
    },
    "transport": {
        "signalExchanges": 42,
        "roomsRelayed": 10,
        "bytesRelayed": 1048576,
        "framesRelayed": 200,
    },
}


@pytest.fixture
def client() -> TestClient:
    return TestClient(app, raise_server_exceptions=False)


class TestHomePage:
    def test_returns_html(self, client: TestClient) -> None:
        resp = client.get("/")
        assert resp.status_code == 200
        assert "text/html" in resp.headers["content-type"]
        assert "bore" in resp.text

    def test_contains_product_content(self, client: TestClient) -> None:
        resp = client.get("/")
        assert "Peer-to-peer" in resp.text
        assert "Noise XXpsk0" in resp.text


class TestOpsRelayPage:
    def test_renders_with_relay_data(self, client: TestClient) -> None:
        with patch("app.routes.pages.fetch_status", new_callable=AsyncMock) as mock:
            mock.return_value = MOCK_STATUS
            resp = client.get("/ops/relay")
        assert resp.status_code == 200
        assert "bore-relay" in resp.text
        assert "Signal exchanges" in resp.text

    def test_renders_error_on_failure(self, client: TestClient) -> None:
        with patch("app.routes.pages.fetch_status", new_callable=AsyncMock) as mock:
            mock.side_effect = Exception("connection refused")
            resp = client.get("/ops/relay")
        assert resp.status_code == 200
        assert "connection refused" in resp.text


class TestRelayStatusPartial:
    def test_returns_fragment(self, client: TestClient) -> None:
        with patch("app.routes.partials.fetch_status", new_callable=AsyncMock) as mock:
            mock.return_value = MOCK_STATUS
            resp = client.get("/partials/relay-status")
        assert resp.status_code == 200
        assert "bore-relay" in resp.text
        # Partial should not contain full HTML structure
        assert "<!DOCTYPE" not in resp.text

    def test_returns_error_fragment(self, client: TestClient) -> None:
        with patch("app.routes.partials.fetch_status", new_callable=AsyncMock) as mock:
            mock.side_effect = Exception("timeout")
            resp = client.get("/partials/relay-status")
        assert resp.status_code == 200
        assert "timeout" in resp.text


class TestNotFound:
    def test_404_page(self, client: TestClient) -> None:
        resp = client.get("/nonexistent-route")
        assert resp.status_code == 404
        assert "404" in resp.text
