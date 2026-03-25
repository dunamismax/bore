"""Bore frontend — FastAPI application entry point."""

from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse
from fastapi.staticfiles import StaticFiles
from fastapi.templating import Jinja2Templates

from app.routes.pages import router as pages_router
from app.routes.partials import router as partials_router

app = FastAPI(title="bore", docs_url=None, redoc_url=None)

app.mount("/static", StaticFiles(directory="src/app/static"), name="static")
app.include_router(pages_router)
app.include_router(partials_router)

_404_templates = Jinja2Templates(directory="src/app/templates")


@app.exception_handler(404)
async def not_found_handler(request: Request, _exc: Exception) -> HTMLResponse:
    """Custom 404 page."""
    return _404_templates.TemplateResponse(request, "404.html", status_code=404)
