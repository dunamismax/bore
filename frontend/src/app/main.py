"""Bore frontend -- FastAPI application entry point."""

from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, Response
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates

from app.routes.pages import router as pages_router
from app.routes.partials import router as partials_router

FRONTEND_CONTENT_SECURITY_POLICY = (
    "default-src 'self'; "
    "connect-src 'self'; "
    "script-src 'self' https://unpkg.com; "
    "style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "
    "img-src 'self' data:; "
    "font-src 'self' data:; "
    "object-src 'none'; "
    "base-uri 'none'; "
    "frame-ancestors 'none'; "
    "form-action 'none'"
)

app = FastAPI(title="bore", docs_url=None, redoc_url=None)

app.mount("/static", StaticFiles(directory="src/app/static"), name="static")
app.include_router(pages_router)
app.include_router(partials_router)

_404_templates = Jinja2Templates(directory="src/app/templates")


@app.middleware("http")
async def set_security_headers(request: Request, call_next) -> Response:
    """Apply a small, explicit browser hardening policy."""
    response = await call_next(request)
    response.headers.setdefault("Content-Security-Policy", FRONTEND_CONTENT_SECURITY_POLICY)
    response.headers.setdefault("X-Content-Type-Options", "nosniff")
    response.headers.setdefault("X-Frame-Options", "DENY")
    response.headers.setdefault("Referrer-Policy", "no-referrer")
    if request.url.path == "/ops/relay" or request.url.path.startswith("/partials/"):
        response.headers.setdefault("Cache-Control", "no-store")
    return response


@app.exception_handler(404)
async def not_found_handler(request: Request, _exc: Exception) -> HTMLResponse:
    """Custom 404 page."""
    return _404_templates.TemplateResponse(request, "404.html", status_code=404)
