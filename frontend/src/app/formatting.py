"""Formatting helpers for display values."""

import math


def format_duration(seconds: int | float) -> str:
    """Format seconds into a human-readable duration like '2d 3h' or '45m 12s'."""
    if not math.isfinite(seconds) or seconds <= 0:
        return "0s"

    remaining = int(seconds)
    units = [("d", 86400), ("h", 3600), ("m", 60), ("s", 1)]
    parts: list[str] = []

    for label, size in units:
        if len(parts) == 2:
            break
        amount, remaining = divmod(remaining, size)
        if amount > 0:
            parts.append(f"{amount}{label}")

    return " ".join(parts) if parts else "0s"


def format_bytes(num_bytes: int | float) -> str:
    """Format bytes into a human-readable size like '1.5 MB'."""
    if not math.isfinite(num_bytes) or num_bytes <= 0:
        return "0 B"

    units = ["B", "KB", "MB", "GB", "TB"]
    value = float(num_bytes)
    unit = units[0]

    for u in units:
        unit = u
        if value < 1024 or u == units[-1]:
            break
        value /= 1024

    if value >= 100:
        return f"{value:.0f} {unit}"
    if value >= 10:
        return f"{value:.1f} {unit}"
    return f"{value:.2f} {unit}"


def room_fill_percent(value: int, max_rooms: int) -> int:
    """Calculate room fill percentage."""
    if max_rooms <= 0:
        return 0
    return min(100, round(value / max_rooms * 100))
