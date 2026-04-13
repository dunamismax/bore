import {
  apiErrorPayloadSchema,
  createSessionRequestSchema,
  healthPayloadSchema,
  joinSessionRequestSchema,
  operatorSummaryPayloadSchema,
  type ParticipantRole,
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
import { createJsonLogger, type Logger } from "./logger";
import { InMemoryRateLimiter } from "./rate-limit";
import { createRelay, type RelayRoom } from "./relay";
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
  logger?: Logger;
  rateLimiter?: InMemoryRateLimiter;
  relay?: RelayRoom;
};

type RequestContext = {
  requestId: string;
  startedAt: number;
  method: string;
  path: string;
  clientIp: string;
};

class PayloadTooLargeError extends Error {
  readonly actualBytes: number;
  readonly limitBytes: number;

  constructor(limitBytes: number, actualBytes: number) {
    super(`request body exceeds ${limitBytes} bytes`);
    this.name = "PayloadTooLargeError";
    this.limitBytes = limitBytes;
    this.actualBytes = actualBytes;
  }
}

class RateLimitExceededError extends Error {
  readonly retryAfterSeconds: number;

  constructor(retryAfterSeconds: number) {
    super("rate limit exceeded for this client");
    this.name = "RateLimitExceededError";
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

class RequestTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(timeoutMs: number) {
    super(`request exceeded ${timeoutMs}ms timeout`);
    this.name = "RequestTimeoutError";
    this.timeoutMs = timeoutMs;
  }
}

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
  code:
    | "bad_request"
    | "not_found"
    | "conflict"
    | "payload_too_large"
    | "rate_limited"
    | "timeout"
    | "internal_error",
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

function buildRequestContext(request: Request): RequestContext {
  const forwardedFor = request.headers
    .get("x-forwarded-for")
    ?.split(",")[0]
    ?.trim();
  const realIp = request.headers.get("x-real-ip")?.trim();
  const url = new URL(request.url);

  return {
    requestId: crypto.randomUUID(),
    startedAt: performance.now(),
    method: request.method,
    path: url.pathname,
    clientIp: forwardedFor || realIp || "unknown",
  };
}

function getRequestContext(
  contexts: WeakMap<Request, RequestContext>,
  request: Request,
) {
  const existing = contexts.get(request);

  if (existing) {
    return existing;
  }

  const created = buildRequestContext(request);
  contexts.set(request, created);
  return created;
}

function resolveResponseStatus(response: unknown, status: unknown) {
  if (typeof status === "number") {
    return status;
  }

  if (response instanceof Response) {
    return response.status;
  }

  return 200;
}

function buildLogFields(
  context: RequestContext,
  status: number,
  extra: Record<string, string | number | boolean | null | undefined> = {},
) {
  return {
    requestId: context.requestId,
    method: context.method,
    path: context.path,
    clientIp: context.clientIp,
    status,
    durationMs: Math.round(performance.now() - context.startedAt),
    ...extra,
  };
}

function shouldRateLimit(request: Request) {
  const path = new URL(request.url).pathname;

  return request.method === "POST" && path.startsWith("/api/");
}

async function runWithTimeout<T>(timeoutMs: number, task: Promise<T>) {
  let timeoutId: ReturnType<typeof setTimeout> | undefined;

  try {
    return await Promise.race([
      task,
      new Promise<never>((_, reject) => {
        timeoutId = setTimeout(() => {
          reject(new RequestTimeoutError(timeoutMs));
        }, timeoutMs);
      }),
    ]);
  } finally {
    if (timeoutId) {
      clearTimeout(timeoutId);
    }
  }
}

export function createApp({
  config,
  sql,
  startedAt = new Date(),
  probe = probeDatabase,
  sessions = createDatabaseSessionService(sql),
  logger = createJsonLogger(),
  rateLimiter = new InMemoryRateLimiter({
    maxRequests: config.rateLimitMaxRequests,
    windowMs: config.rateLimitWindowMs,
  }),
  relay = createRelay(sessions, logger),
}: AppDependencies) {
  const requestContexts = new WeakMap<Request, RequestContext>();

  return new Elysia()
    .onRequest(({ request }) => {
      requestContexts.set(request, buildRequestContext(request));
    })
    .onBeforeHandle(({ request, set }) => {
      const context = getRequestContext(requestContexts, request);
      const contentLengthHeader = request.headers.get("content-length");
      const contentLength = contentLengthHeader
        ? Number.parseInt(contentLengthHeader, 10)
        : undefined;

      set.headers["x-request-id"] = context.requestId;

      if (
        typeof contentLength === "number" &&
        Number.isFinite(contentLength) &&
        contentLength > config.maxRequestBodyBytes
      ) {
        throw new PayloadTooLargeError(
          config.maxRequestBodyBytes,
          contentLength,
        );
      }

      if (!shouldRateLimit(request)) {
        return undefined;
      }

      const result = rateLimiter.check(context.clientIp);

      if (result.allowed) {
        return undefined;
      }

      const retryAfterSeconds = Math.max(
        1,
        Math.ceil((result.resetAt - Date.now()) / 1_000),
      );

      logger.warn(
        "http_request_rate_limited",
        buildLogFields(context, 429, {
          retryAfterSeconds,
          rateLimitWindowMs: config.rateLimitWindowMs,
          rateLimitMaxRequests: config.rateLimitMaxRequests,
        }),
      );

      throw new RateLimitExceededError(retryAfterSeconds);
    })
    .onAfterHandle(({ request, responseValue, set }) => {
      const context = getRequestContext(requestContexts, request);
      const status = resolveResponseStatus(responseValue, set.status);

      logger.info("http_request_completed", buildLogFields(context, status));
    })
    .onError(({ error, request, set }) => {
      const context = getRequestContext(requestContexts, request);

      set.headers["x-request-id"] = context.requestId;

      if (error instanceof ZodError) {
        set.status = 400;

        logger.warn(
          "http_request_failed",
          buildLogFields(context, 400, {
            errorName: error.name,
            errorMessage: "request validation failed",
          }),
        );

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

        logger.warn(
          "http_request_failed",
          buildLogFields(context, 404, {
            errorName: error.name,
            errorMessage: error.message,
          }),
        );

        return buildErrorPayload("not_found", error.message);
      }

      if (error instanceof SessionConflictError) {
        set.status = 409;

        logger.warn(
          "http_request_failed",
          buildLogFields(context, 409, {
            errorName: error.name,
            errorMessage: error.message,
          }),
        );

        return buildErrorPayload("conflict", error.message);
      }

      if (error instanceof PayloadTooLargeError) {
        set.status = 413;

        logger.warn(
          "http_request_failed",
          buildLogFields(context, 413, {
            errorName: error.name,
            errorMessage: error.message,
            bodyBytes: error.actualBytes,
            bodyLimitBytes: error.limitBytes,
          }),
        );

        return buildErrorPayload("payload_too_large", error.message, [
          {
            code: "body_too_large",
            message: `body was ${error.actualBytes} bytes, limit is ${error.limitBytes}`,
            path: [],
          },
        ]);
      }

      if (error instanceof RateLimitExceededError) {
        set.status = 429;
        set.headers["retry-after"] = String(error.retryAfterSeconds);

        logger.warn(
          "http_request_failed",
          buildLogFields(context, 429, {
            errorName: error.name,
            errorMessage: error.message,
            retryAfterSeconds: error.retryAfterSeconds,
          }),
        );

        return buildErrorPayload("rate_limited", error.message, [
          {
            code: "too_many_requests",
            message: `retry after ${error.retryAfterSeconds} seconds`,
            path: [],
          },
        ]);
      }

      if (error instanceof RequestTimeoutError) {
        set.status = 504;

        logger.error(
          "http_request_failed",
          buildLogFields(context, 504, {
            errorName: error.name,
            errorMessage: error.message,
            timeoutMs: error.timeoutMs,
          }),
        );

        return buildErrorPayload("timeout", error.message);
      }

      logger.error(
        "http_request_failed",
        buildLogFields(context, 500, {
          errorName: error instanceof Error ? error.name : "UnknownError",
          errorMessage:
            error instanceof Error ? error.message : "unknown internal error",
        }),
      );
      set.status = 500;
      return buildErrorPayload("internal_error", "internal server error");
    })
    .get("/api/health", async () => {
      const readiness = await runWithTimeout(
        config.requestTimeoutMs,
        getReadinessPayload(config, sql, probe),
      );

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
      const payload = await runWithTimeout(
        config.requestTimeoutMs,
        getReadinessPayload(config, sql, probe),
      );

      set.status = payload.status === "ready" ? 200 : 503;

      return payload;
    })
    .post("/api/sessions", async ({ body, set }) => {
      const input = createSessionRequestSchema.parse(body);
      const payload = await runWithTimeout(
        config.requestTimeoutMs,
        sessions.createSession(input),
      );

      set.status = 201;

      return sessionDetailSchema.parse(payload);
    })
    .post("/api/sessions/:code/join", async ({ body, params }) => {
      const input = joinSessionRequestSchema.parse(body);
      const { code } = sessionRouteParamsSchema.parse(params);
      const payload = await runWithTimeout(
        config.requestTimeoutMs,
        sessions.joinSession(code, input),
      );

      return sessionDetailSchema.parse(payload);
    })
    .get("/api/sessions/:code", async ({ params }) => {
      const { code } = sessionRouteParamsSchema.parse(params);
      const payload = await runWithTimeout(
        config.requestTimeoutMs,
        sessions.getSession(code),
      );

      if (!payload) {
        throw new SessionNotFoundError(code);
      }

      return sessionDetailSchema.parse(payload);
    })
    .get("/api/ops/summary", async () => {
      const payload = await runWithTimeout(
        config.requestTimeoutMs,
        sessions.getOperatorSummary(),
      );

      return operatorSummaryPayloadSchema.parse(payload);
    })
    .ws("/api/sessions/:code/ws", {
      params: sessionRouteParamsSchema,
      async open(ws) {
        const code = (ws.data as { params: { code: string } }).params.code;
        const url = new URL(
          ws.data.request?.url ?? `http://localhost/api/sessions/${code}/ws`,
        );
        const role = url.searchParams.get("role") as ParticipantRole | null;

        if (role !== "sender" && role !== "receiver") {
          ws.close(4400, "role query parameter must be sender or receiver");
          return;
        }

        const session = await sessions.getSession(code);

        if (!session) {
          ws.close(4404, "session not found");
          return;
        }

        const validStates = ["ready", "transferring"];

        if (!validStates.includes(session.status)) {
          ws.close(4409, `session is in state ${session.status}`);
          return;
        }

        ws.data = {
          ...ws.data,
          sessionCode: code,
          role,
        } as typeof ws.data;

        relay.handleOpen(ws as never);
      },
      message(ws, message) {
        relay.handleMessage(ws as never, message as string | ArrayBuffer);
      },
      close(ws) {
        relay.handleClose(ws as never);
      },
    });
}
