"""Page routes serving Jinja2 templates."""

from fastapi import APIRouter, Request
from fastapi.responses import HTMLResponse
from fastapi.templating import Jinja2Templates

from app.formatting import format_bytes, format_duration, room_fill_percent
from app.relay import fetch_status

router = APIRouter()
templates = Jinja2Templates(directory="src/app/templates")

# Register template globals for formatting helpers.
templates.env.globals["format_duration"] = format_duration
templates.env.globals["format_bytes"] = format_bytes
templates.env.globals["room_fill_percent"] = room_fill_percent


@router.get("/", response_class=HTMLResponse)
async def home(request: Request) -> HTMLResponse:
    """Product homepage."""
    return templates.TemplateResponse(request, "home.html")


@router.get("/ops/relay", response_class=HTMLResponse)
async def ops_relay(request: Request) -> HTMLResponse:
    """Relay operator dashboard."""
    ctx: dict[str, object] = {}
    try:
        ctx["relay"] = await fetch_status()
        ctx["error"] = None
    except Exception as exc:
        ctx["relay"] = None
        ctx["error"] = str(exc)
    return templates.TemplateResponse(request, "ops_relay.html", ctx)
