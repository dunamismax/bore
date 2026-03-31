import { describe, expect, test } from "bun:test";

import {
  apiErrorPayloadSchema,
  healthPayloadSchema,
  operatorSummaryPayloadSchema,
  readinessPayloadSchema,
  type SessionDetail,
  sessionDetailSchema,
} from "@bore/contracts";
import postgres from "postgres";

import { createApp } from "../src/app";
import type { AppConfig } from "../src/config";
import type { Logger } from "../src/logger";
import {
  SessionConflictError,
  SessionNotFoundError,
  type SessionService,
} from "../src/sessions";

const config: AppConfig = {
  environment: "test",
  host: "127.0.0.1",
  port: 3000,
  requestTimeoutMs: 5_000,
  idleTimeoutSeconds: 30,
  maxRequestBodyBytes: 65_536,
  rateLimitWindowMs: 60_000,
  rateLimitMaxRequests: 30,
  version: "0.0.0-test",
  publicOrigin: "http://localhost:8080",
  databaseUrl: "postgres://bore:bore@localhost:5432/bore_v2",
  databaseSsl: false,
};

const timestamp = "2026-03-31T14:00:00.000Z";

const silentLogger: Logger = {
  info() {},
  warn() {},
  error() {},
};

function makeSessionDetail(
  status: SessionDetail["status"] = "waiting_receiver",
) {
  return sessionDetailSchema.parse({
    id: "7b8d1d0c-1dcc-4b49-bf34-ff94b68207b8",
    code: "ember-orbit-421",
    status,
    createdAt: timestamp,
    updatedAt: timestamp,
    expiresAt: "2026-03-31T14:15:00.000Z",
    file: {
      name: "report.pdf",
      sizeBytes: 58213,
      mimeType: "application/pdf",
    },
    participants: [
      {
        role: "sender",
        status: "joined",
        displayName: "Stephen",
        joinedAt: timestamp,
      },
      ...(status === "ready"
        ? [
            {
              role: "receiver",
              status: "joined",
              displayName: "Sawyer",
              joinedAt: "2026-03-31T14:01:00.000Z",
            },
          ]
        : []),
    ],
    events: [
      {
        id: "83786bb7-cc1f-48f6-946e-f491820cfcc0",
        type: "session_created",
        actorRole: "sender",
        timestamp,
        payload: {
          expiresInMinutes: 15,
        },
      },
    ],
  });
}

function makeSessionService(): SessionService {
  return {
    async createSession() {
      return makeSessionDetail();
    },
    async joinSession() {
      return makeSessionDetail("ready");
    },
    async getSession(code) {
      if (code === "missing-echo-void") {
        return null;
      }

      return makeSessionDetail();
    },
    async getOperatorSummary() {
      return operatorSummaryPayloadSchema.parse({
        generatedAt: timestamp,
        counts: {
          total: 1,
          waitingReceiver: 1,
          ready: 0,
          completed: 0,
          failed: 0,
          expired: 0,
          cancelled: 0,
        },
        sessions: [
          {
            id: "7b8d1d0c-1dcc-4b49-bf34-ff94b68207b8",
            code: "ember-orbit-421",
            status: "waiting_receiver",
            createdAt: timestamp,
            updatedAt: timestamp,
            expiresAt: "2026-03-31T14:15:00.000Z",
            fileName: "report.pdf",
            fileSizeBytes: 58213,
            senderJoinedAt: timestamp,
            lastEventType: "file_registered",
          },
        ],
      });
    },
  };
}

describe("api app", () => {
  test("returns a typed health payload", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      startedAt: new Date(Date.now() - 2_000),
      probe: async () => 8,
      sessions: makeSessionService(),
      logger: silentLogger,
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
      sessions: makeSessionService(),
      logger: silentLogger,
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

  test("creates a typed session payload", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions", {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          file: {
            name: "report.pdf",
            sizeBytes: 58213,
            mimeType: "application/pdf",
          },
          senderDisplayName: "Stephen",
          expiresInMinutes: 15,
        }),
      }),
    );
    const payload = sessionDetailSchema.parse(await response.json());

    expect(response.status).toBe(201);
    expect(payload.code).toBe("ember-orbit-421");

    await sql.end({ timeout: 0 });
  });

  test("returns a validation error for malformed session create requests", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions", {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          file: {
            name: "",
            sizeBytes: -1,
          },
        }),
      }),
    );
    const payload = apiErrorPayloadSchema.parse(await response.json());

    expect(response.status).toBe(400);
    expect(payload.error.code).toBe("bad_request");
    expect(payload.error.issues?.length).toBeGreaterThan(0);

    await sql.end({ timeout: 0 });
  });

  test("joins a session and returns the ready payload", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions/ember-orbit-421/join", {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          displayName: "Sawyer",
        }),
      }),
    );
    const payload = sessionDetailSchema.parse(await response.json());

    expect(response.status).toBe(200);
    expect(payload.status).toBe("ready");

    await sql.end({ timeout: 0 });
  });

  test("returns a typed not found error for missing sessions", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const sessions = makeSessionService();
    sessions.getSession = async () => null;
    const app = createApp({
      config,
      sql,
      sessions,
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions/missing-echo-void"),
    );
    const payload = apiErrorPayloadSchema.parse(await response.json());

    expect(response.status).toBe(404);
    expect(payload.error.code).toBe("not_found");

    await sql.end({ timeout: 0 });
  });

  test("returns a conflict error when join is rejected", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const sessions = makeSessionService();
    sessions.joinSession = async () => {
      throw new SessionConflictError("receiver already joined this session");
    };
    const app = createApp({
      config,
      sql,
      sessions,
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions/ember-orbit-421/join", {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          displayName: "Sawyer",
        }),
      }),
    );
    const payload = apiErrorPayloadSchema.parse(await response.json());

    expect(response.status).toBe(409);
    expect(payload.error.code).toBe("conflict");

    await sql.end({ timeout: 0 });
  });

  test("returns a typed operator summary payload", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config,
      sql,
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/ops/summary"),
    );
    const payload = operatorSummaryPayloadSchema.parse(await response.json());

    expect(response.status).toBe(200);
    expect(payload.counts.total).toBe(1);

    await sql.end({ timeout: 0 });
  });

  test("returns a typed not found error from the service layer", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const sessions = makeSessionService();
    sessions.joinSession = async () => {
      throw new SessionNotFoundError("ember-orbit-421");
    };
    const app = createApp({
      config,
      sql,
      sessions,
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions/ember-orbit-421/join", {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({}),
      }),
    );
    const payload = apiErrorPayloadSchema.parse(await response.json());

    expect(response.status).toBe(404);
    expect(payload.error.code).toBe("not_found");

    await sql.end({ timeout: 0 });
  });

  test("returns a typed rate-limited error for repeated write requests", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config: {
        ...config,
        rateLimitMaxRequests: 1,
        rateLimitWindowMs: 60_000,
      },
      sql,
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const request = () =>
      app.handle(
        new Request("http://localhost/api/sessions", {
          method: "POST",
          headers: {
            "content-type": "application/json",
            "x-forwarded-for": "203.0.113.10",
          },
          body: JSON.stringify({
            file: {
              name: "report.pdf",
              sizeBytes: 58213,
            },
            expiresInMinutes: 15,
          }),
        }),
      );

    const firstResponse = await request();
    const secondResponse = await request();
    const payload = apiErrorPayloadSchema.parse(await secondResponse.json());

    expect(firstResponse.status).toBe(201);
    expect(secondResponse.status).toBe(429);
    expect(secondResponse.headers.get("retry-after")).toBe("60");
    expect(payload.error.code).toBe("rate_limited");

    await sql.end({ timeout: 0 });
  });

  test("returns a typed payload-too-large error for oversized JSON bodies", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config: {
        ...config,
        maxRequestBodyBytes: 64,
      },
      sql,
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/sessions", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "content-length": "256",
        },
        body: JSON.stringify({
          file: {
            name: "report.pdf",
            sizeBytes: 58213,
            mimeType: "application/pdf",
          },
          senderDisplayName: "Stephen",
          expiresInMinutes: 15,
        }),
      }),
    );
    const payload = apiErrorPayloadSchema.parse(await response.json());

    expect(response.status).toBe(413);
    expect(payload.error.code).toBe("payload_too_large");

    await sql.end({ timeout: 0 });
  });

  test("returns a typed timeout error when a handler exceeds the configured request timeout", async () => {
    const sql = postgres(config.databaseUrl, { max: 1, idle_timeout: 1 });
    const app = createApp({
      config: {
        ...config,
        requestTimeoutMs: 5,
      },
      sql,
      probe: async () => {
        await Bun.sleep(25);
        return 8;
      },
      sessions: makeSessionService(),
      logger: silentLogger,
    });

    const response = await app.handle(
      new Request("http://localhost/api/health"),
    );
    const payload = apiErrorPayloadSchema.parse(await response.json());

    expect(response.status).toBe(504);
    expect(payload.error.code).toBe("timeout");

    await sql.end({ timeout: 0 });
  });
});
