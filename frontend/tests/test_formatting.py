"""Tests for formatting helpers."""

import math

from app.formatting import format_bytes, format_duration, room_fill_percent


class TestFormatDuration:
    def test_zero(self) -> None:
        assert format_duration(0) == "0s"

    def test_negative(self) -> None:
        assert format_duration(-10) == "0s"

    def test_nan(self) -> None:
        assert format_duration(math.nan) == "0s"

    def test_seconds(self) -> None:
        assert format_duration(45) == "45s"

    def test_minutes_seconds(self) -> None:
        assert format_duration(125) == "2m 5s"

    def test_hours_minutes(self) -> None:
        assert format_duration(3661) == "1h 1m"

    def test_days_hours(self) -> None:
        assert format_duration(90061) == "1d 1h"

    def test_exact_day(self) -> None:
        assert format_duration(86400) == "1d"


class TestFormatBytes:
    def test_zero(self) -> None:
        assert format_bytes(0) == "0 B"

    def test_negative(self) -> None:
        assert format_bytes(-1) == "0 B"

    def test_bytes(self) -> None:
        assert format_bytes(512) == "512 B"

    def test_kilobytes(self) -> None:
        assert format_bytes(1536) == "1.50 KB"

    def test_megabytes(self) -> None:
        assert format_bytes(67108864) == "64.0 MB"

    def test_gigabytes(self) -> None:
        result = format_bytes(1073741824)
        assert "GB" in result


class TestRoomFillPercent:
    def test_zero_max(self) -> None:
        assert room_fill_percent(5, 0) == 0

    def test_half_full(self) -> None:
        assert room_fill_percent(50, 100) == 50

    def test_over_max(self) -> None:
        assert room_fill_percent(200, 100) == 100

    def test_empty(self) -> None:
        assert room_fill_percent(0, 100) == 0
