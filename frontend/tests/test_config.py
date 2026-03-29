"""Tests for frontend configuration validation."""

import pytest
from pydantic import ValidationError

from app.config import Settings


class TestSettings:
    def test_normalizes_trailing_slash(self) -> None:
        settings = Settings(relay_url="https://relay.example.com:8443/")
        assert settings.relay_url == "https://relay.example.com:8443"

    @pytest.mark.parametrize(
        "relay_url",
        [
            "https://relay.example.com/status",
            "https://relay.example.com?via=proxy",
            "https://relay.example.com#fragment",
            "ftp://relay.example.com",
        ],
    )
    def test_rejects_non_origin_relay_urls(self, relay_url: str) -> None:
        with pytest.raises(ValidationError):
            Settings(relay_url=relay_url)
