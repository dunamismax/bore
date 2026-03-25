"""Application configuration via environment variables."""

from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Bore frontend configuration.

    All values can be overridden with environment variables prefixed BORE_.
    """

    relay_url: str = "http://127.0.0.1:8080"
    host: str = "127.0.0.1"
    port: int = 3000
    reload: bool = False

    model_config = {"env_prefix": "BORE_"}


settings = Settings()
