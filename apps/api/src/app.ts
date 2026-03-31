import {
  apiErrorPayloadSchema,
  createSessionRequestSchema,
  healthPayloadSchema,
  joinSessionRequestSchema,
  operatorSummaryPayloadSchema,
  type ReadinessPayload,
  type ReadinessStatus,
  readinessPayloadSchema,
  serviceName,
  sessionDetailSchema,
  sessionRouteParamsSchema,
} from "@bore/contracts";
import { Elysia } from "elysia";
import type { Sql } from "postgres";
import { ZodError } from "zod";

import type { AppConfig } from "./config";
import { probeDatabase } from "./db";
import {
  createDatabaseSessionService,
  SessionConflictError,
  SessionNotFoundError,
  type SessionService,
} from "./sessions";

type AppDependencies = {
  config: AppConfig;
  sql: Sql;
  startedAt?: Date;
  probe?: (sql: Sql) => Promise<number>;
  sessions?: SessionService;
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

function buildErrorPayload(
  code: "bad_request" | "not_found" | "conflict" | "internal_error",
  message: string,
  issues?: Array<{
    code: string;
    message: string;
    path?: Array<string | number>;
  }>,
) {
  return apiErrorPayloadSchema.parse({
    error: {
      code,
      message,
      issues,
    },
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
  sessions = createDatabaseSessionService(sql),
}: AppDependencies) {
  return new Elysia()
    .onError(({ error, set }) => {
      if (error instanceof ZodError) {
        set.status = 400;

        return buildErrorPayload(
          "bad_request",
          "request validation failed",
          error.issues.map((issue) => ({
            code: issue.code,
            message: issue.message,
            path: issue.path.map((segment) =>
              typeof segment === "number" ? segment : String(segment),
            ),
          })),
        );
      }

      if (error instanceof SessionNotFoundError) {
        set.status = 404;
        return buildErrorPayload("not_found", error.message);
      }

      if (error instanceof SessionConflictError) {
        set.status = 409;
        return buildErrorPayload("conflict", error.message);
      }

      console.error(error);
      set.status = 500;
      return buildErrorPayload("internal_error", "internal server error");
    })
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
    })
    .post("/api/sessions", async ({ body, set }) => {
      const input = createSessionRequestSchema.parse(body);
      const payload = await sessions.createSession(input);

      set.status = 201;

      return sessionDetailSchema.parse(payload);
    })
    .post("/api/sessions/:code/join", async ({ body, params }) => {
      const input = joinSessionRequestSchema.parse(body);
      const { code } = sessionRouteParamsSchema.parse(params);
      const payload = await sessions.joinSession(code, input);

      return sessionDetailSchema.parse(payload);
    })
    .get("/api/sessions/:code", async ({ params }) => {
      const { code } = sessionRouteParamsSchema.parse(params);
      const payload = await sessions.getSession(code);

      if (!payload) {
        throw new SessionNotFoundError(code);
      }

      return sessionDetailSchema.parse(payload);
    })
    .get("/api/ops/summary", async () => {
      const payload = await sessions.getOperatorSummary();

      return operatorSummaryPayloadSchema.parse(payload);
    });
}
