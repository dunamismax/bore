import { describe, expect, it } from "vitest";

import { formatBytes, formatDuration } from "../src/lib/format";

describe("formatDuration", () => {
  it("returns zero seconds for empty values", () => {
    expect(formatDuration(0)).toBe("0s");
  });

  it("returns up to two units", () => {
    expect(formatDuration(3661)).toBe("1h 1m");
    expect(formatDuration(59)).toBe("59s");
  });
});

describe("formatBytes", () => {
  it("formats sizes using binary units", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1536)).toBe("1.50 KB");
    expect(formatBytes(64 << 20)).toBe("64.0 MB");
  });
});
