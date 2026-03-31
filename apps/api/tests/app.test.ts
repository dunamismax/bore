import { describe, expect, test } from "bun:test";

import { healthPayloadSchema, readinessPayloadSchema } from "@bore/contracts";
import postgres from "postgres";

import { createApp } from "../src/app";
import type { AppConfig } from "../src/config";

const config: AppConfig = {
  environment: "test",
  host: "127.0.0.1",
  port: 3000,
  version: "0.0.0-test",
  publicOrigin: "http://localhost:8080",
  databaseUrl: "postgres://bore:bore@localhost:5432/bore_v2",
  databaseSsl: false,
};

describe("api app", () => {
  test("returns a typed health payload", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      startedAt: new Date(Date.now() - 2_000),
      probe: async () => 8,
    });

    const response = await app.handle(
      new Request("http://localhost/api/health"),
    );
    const payload = healthPayloadSchema.parse(await response.json());

    expect(response.status).toBe(200);
    expect(payload.status).toBe("ok");
    expect(payload.readiness).toBe("ready");

    await sql.end({ timeout: 0 });
  });

  test("returns 503 readiness when the database probe fails", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      probe: async () => {
        throw new Error("database unavailable");
      },
    });

    const response = await app.handle(
      new Request("http://localhost/api/readiness"),
    );
    const payload = readinessPayloadSchema.parse(await response.json());

    expect(response.status).toBe(503);
    expect(payload.status).toBe("not_ready");
    expect(
      payload.checks.find((check) => check.name === "database")?.detail,
    ).toBe("database unavailable");

    await sql.end({ timeout: 0 });
  });
});
