"""Application configuration via environment variables."""

from pydantic import AnyHttpUrl, TypeAdapter, field_validator
from pydantic_settings import BaseSettings

_http_url = TypeAdapter(AnyHttpUrl)


class Settings(BaseSettings):
    """Bore frontend configuration.

    All values can be overridden with environment variables prefixed BORE_.
    """

    relay_url: str = "http://127.0.0.1:8080"
    host: str = "127.0.0.1"
    port: int = 3000
    reload: bool = False

    model_config = {"env_prefix": "BORE_"}

    @field_validator("relay_url")
    @classmethod
    def validate_relay_url(cls, value: str) -> str:
        """Require a bare relay origin so endpoint composition stays predictable."""
        url = _http_url.validate_python(value)
        if (url.path or "") not in ("", "/"):
            raise ValueError("relay_url must be a bare relay origin without a path")
        if url.query not in (None, ""):
            raise ValueError("relay_url must not include a query string")
        if url.fragment not in (None, ""):
            raise ValueError("relay_url must not include a fragment")
        return str(url).rstrip("/")


settings = Settings()
