import { describe, expect, test } from "bun:test";

import {
  formatBytes,
  formatDuration,
  parseRelayStatus,
  roomFillPercent,
} from "../src/lib/status";

describe("parseRelayStatus", () => {
  test("accepts the current Go-owned status contract", () => {
    const parsed = parseRelayStatus({
      service: "bore-relay",
      status: "ok",
      uptimeSeconds: 3661,
      rooms: { total: 5, waiting: 2, active: 3 },
      limits: {
        maxRooms: 100,
        roomTTLSeconds: 300,
        reapIntervalSeconds: 60,
        maxMessageSizeBytes: 67_108_864,
      },
      transport: {
        signalExchanges: 42,
        signalingStarted: 50,
        roomsRelayed: 10,
        bytesRelayed: 1_048_576,
        framesRelayed: 200,
      },
    });

    expect(parsed.transport.signalingStarted).toBe(50);
    expect(parsed.limits.maxMessageSizeBytes).toBe(67_108_864);
  });

  test("rejects malformed payloads", () => {
    expect(() =>
      parseRelayStatus({
        service: "bore-relay",
        status: "ok",
        uptimeSeconds: 10,
        rooms: { total: 1, waiting: 1, active: 0 },
        limits: {
          maxRooms: 10,
          roomTTLSeconds: 300,
          reapIntervalSeconds: 60,
          maxMessageSizeBytes: 100,
        },
        transport: {
          signalExchanges: 1,
          signalingStarted: "bad",
          roomsRelayed: 0,
          bytesRelayed: 0,
          framesRelayed: 0,
        },
      }),
    ).toThrow("relay status.transport.signalingStarted");
  });
});

describe("formatters", () => {
  test("formatDuration matches the old frontend behavior", () => {
    expect(formatDuration(3661)).toBe("1h 1m");
  });

  test("formatBytes matches the old frontend behavior", () => {
    expect(formatBytes(1_048_576)).toBe("1.00 MB");
  });

  test("roomFillPercent caps at 100", () => {
    expect(roomFillPercent(12, 10)).toBe(100);
  });
});
