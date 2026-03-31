import {
  healthPayloadSchema,
  type ReadinessPayload,
  type ReadinessStatus,
  readinessPayloadSchema,
  serviceName,
} from "@bore/contracts";
import { Elysia } from "elysia";

import type { Sql } from "postgres";

import type { AppConfig } from "./config";
import { probeDatabase } from "./db";

type AppDependencies = {
  config: AppConfig;
  sql: Sql;
  startedAt?: Date;
  probe?: (sql: Sql) => Promise<number>;
};

function buildReadinessPayload(
  config: AppConfig,
  status: ReadinessStatus,
  latencyMs?: number,
  detail?: string,
) {
  return readinessPayloadSchema.parse({
    service: serviceName,
    status,
    version: config.version,
    timestamp: new Date().toISOString(),
    checks: [
      {
        name: "config",
        status: "ready",
      },
      {
        name: "database",
        status,
        detail,
        latencyMs,
      },
    ],
  });
}

async function getReadinessPayload(
  config: AppConfig,
  sql: Sql,
  probe: (sql: Sql) => Promise<number>,
): Promise<ReadinessPayload> {
  try {
    const latencyMs = await probe(sql);

    return buildReadinessPayload(config, "ready", latencyMs);
  } catch (error) {
    return buildReadinessPayload(
      config,
      "not_ready",
      undefined,
      error instanceof Error ? error.message : "unknown readiness failure",
    );
  }
}

export function createApp({
  config,
  sql,
  startedAt = new Date(),
  probe = probeDatabase,
}: AppDependencies) {
  return new Elysia()
    .get("/api/health", async () => {
      const readiness = await getReadinessPayload(config, sql, probe);

      return healthPayloadSchema.parse({
        service: serviceName,
        status: "ok",
        version: config.version,
        environment: config.environment,
        uptimeSeconds: Math.max(
          0,
          Math.round((Date.now() - startedAt.getTime()) / 1000),
        ),
        timestamp: new Date().toISOString(),
        readiness: readiness.status,
      });
    })
    .get("/api/readiness", async ({ set }) => {
      const payload = await getReadinessPayload(config, sql, probe);

      set.status = payload.status === "ready" ? 200 : 503;

      return payload;
    });
}
