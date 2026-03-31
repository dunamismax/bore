import { describe, expect, test } from "bun:test";

import {
  healthPayloadSchema,
  readinessPayloadSchema,
  serviceName,
} from "../src/index";

describe("contracts", () => {
  test("parses a valid health payload", () => {
    const payload = healthPayloadSchema.parse({
      service: serviceName,
      status: "ok",
      version: "0.0.0-phase1",
      environment: "development",
      uptimeSeconds: 12,
      timestamp: "2026-03-31T00:00:00.000Z",
      readiness: "ready",
    });

    expect(payload.service).toBe(serviceName);
  });

  test("rejects readiness payloads without checks", () => {
    expect(() =>
      readinessPayloadSchema.parse({
        service: serviceName,
        status: "ready",
        version: "0.0.0-phase1",
        timestamp: "2026-03-31T00:00:00.000Z",
        checks: [],
      }),
    ).toThrow();
  });
});
