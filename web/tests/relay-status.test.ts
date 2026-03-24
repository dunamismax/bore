import { describe, expect, it } from "vitest";

import { relayStatusSchema } from "../src/lib/relay-status";

describe("relayStatusSchema", () => {
  it("parses a valid status response with transport stats", () => {
    const payload = {
      service: "bore-relay",
      status: "ok",
      uptimeSeconds: 3600,
      rooms: { total: 5, waiting: 2, active: 3 },
      limits: {
        maxRooms: 100,
        roomTTLSeconds: 300,
        reapIntervalSeconds: 60,
        maxMessageSizeBytes: 67108864,
      },
      transport: {
        signalExchanges: 42,
        roomsRelayed: 10,
        bytesRelayed: 1048576,
        framesRelayed: 500,
      },
    };

    const result = relayStatusSchema.parse(payload);
    expect(result.transport.signalExchanges).toBe(42);
    expect(result.transport.roomsRelayed).toBe(10);
    expect(result.transport.bytesRelayed).toBe(1048576);
    expect(result.transport.framesRelayed).toBe(500);
  });

  it("rejects a response missing transport stats", () => {
    const payload = {
      service: "bore-relay",
      status: "ok",
      uptimeSeconds: 3600,
      rooms: { total: 0, waiting: 0, active: 0 },
      limits: {
        maxRooms: 100,
        roomTTLSeconds: 300,
        reapIntervalSeconds: 60,
        maxMessageSizeBytes: 67108864,
      },
    };

    expect(() => relayStatusSchema.parse(payload)).toThrow();
  });
});
