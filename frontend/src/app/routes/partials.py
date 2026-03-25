"""htmx partial routes for live-updating page fragments."""

from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse
from fastapi.templating import Jinja2Templates

from app.formatting import format_bytes, format_duration, room_fill_percent
from app.relay import fetch_status

router = APIRouter(prefix="/partials")
templates = Jinja2Templates(directory="src/app/templates")
templates.env.globals["format_duration"] = format_duration
templates.env.globals["format_bytes"] = format_bytes
templates.env.globals["room_fill_percent"] = room_fill_percent


@router.get("/relay-status", response_class=HTMLResponse)
async def relay_status_partial(request: Request) -> HTMLResponse:
    """Return the relay status fragment for htmx polling."""
    ctx: dict[str, object] = {}
    try:
        ctx["relay"] = await fetch_status()
        ctx["error"] = None
    except Exception as exc:
        ctx["relay"] = None
        ctx["error"] = str(exc)
    return templates.TemplateResponse(request, "partials/relay_status.html", ctx)
