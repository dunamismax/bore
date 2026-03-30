import { describe, expect, test } from "bun:test";
import {
  formatBytes,
  formatDurationSeconds,
  formatPercent,
  makeGauge,
} from "../src/lib/format.ts";

describe("format helpers", () => {
  test("formats durations compactly", () => {
    expect(formatDurationSeconds(0)).toBe("0s");
    expect(formatDurationSeconds(65)).toBe("1m 5s");
    expect(formatDurationSeconds(3_661)).toBe("1h 1m");
  });

  test("formats bytes with binary units", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1_024)).toBe("1.0 KiB");
    expect(formatBytes(10 * 1_024 * 1_024)).toBe("10 MiB");
  });

  test("formats percentages and gauges safely", () => {
    expect(formatPercent(0, 0)).toBe("0%");
    expect(formatPercent(1, 4)).toBe("25%");
    expect(makeGauge(3, 6, 6)).toBe("###...");
    expect(makeGauge(1, 0, 4)).toBe("....");
  });
});
