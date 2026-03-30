import { describe, expect, test } from "bun:test";
import type { RelayStatus } from "../src/lib/status.ts";
import { buildDashboardViewModel } from "../src/lib/view-model.ts";

const sampleStatus: RelayStatus = {
  service: "bore-relay",
  status: "ok",
  uptimeSeconds: 3_661,
  rooms: {
    total: 4,
    waiting: 1,
    active: 3,
  },
  limits: {
    maxRooms: 10,
    roomTTLSeconds: 300,
    reapIntervalSeconds: 30,
    maxMessageSizeBytes: 65_536,
  },
  transport: {
    signalExchanges: 12,
    signalingStarted: 14,
    roomsRelayed: 2,
    bytesRelayed: 4_096,
    framesRelayed: 64,
  },
};

describe("dashboard view model", () => {
  test("renders a live healthy snapshot", () => {
    const view = buildDashboardViewModel(
      {
        relayURL: "http://127.0.0.1:8080",
        refreshIntervalMs: 2_000,
        timeoutMs: 5_000,
        loading: false,
        lastError: null,
        lastAttemptAt: 1_000,
        lastSuccessAt: 5_000,
        status: sampleStatus,
      },
      8_000,
    );

    expect(view.alertVisible).toBe(false);
    expect(view.header).toContain("bore relay operator console");
    expect(view.rooms).toContain("1/10");
    expect(view.transport).toContain("direct inferred  10");
    expect(view.limits).toContain("room ttl      5m");
  });

  test("surfaces stale errors without dropping the last good snapshot", () => {
    const view = buildDashboardViewModel(
      {
        relayURL: "http://127.0.0.1:8080",
        refreshIntervalMs: 2_000,
        timeoutMs: 5_000,
        loading: false,
        lastError: "refresh failed: GET /status returned 502",
        lastAttemptAt: 7_000,
        lastSuccessAt: 5_000,
        status: sampleStatus,
      },
      9_000,
    );

    expect(view.alertVisible).toBe(true);
    expect(view.alertTitle).toBe("stale snapshot");
    expect(view.alertBody).toContain("refresh failed");
    expect(view.alertBody).toContain("showing last good /status snapshot");
    expect(view.overview).toContain("bore-relay");
  });

  test("shows empty-state guidance before the first successful fetch", () => {
    const view = buildDashboardViewModel(
      {
        relayURL: "http://127.0.0.1:8080",
        refreshIntervalMs: 2_000,
        timeoutMs: 5_000,
        loading: false,
        lastError: "refresh failed: connection refused",
        lastAttemptAt: 7_000,
        lastSuccessAt: null,
        status: null,
      },
      9_000,
    );

    expect(view.alertVisible).toBe(true);
    expect(view.alertTitle).toBe("relay unavailable");
    expect(view.overview).toContain("waiting for /status");
    expect(view.transport).toContain("unknown");
  });
});
