import { describe, expect, test } from "bun:test";

import {
  apiErrorPayloadSchema,
  coordinationEnvelopeSchema,
  healthPayloadSchema,
  operatorSummaryPayloadSchema,
  readinessPayloadSchema,
  sessionDetailSchema,
} from "../src/index";

describe("contracts", () => {
  test("parses a valid health payload", () => {
    const payload = healthPayloadSchema.parse({
      service: "bore-v2-api",
      status: "ok",
      version: "0.0.0-test",
      environment: "development",
      uptimeSeconds: 12,
      timestamp: new Date().toISOString(),
      readiness: "ready",
    });

    expect(payload.service).toBe("bore-v2-api");
  });

  test("rejects readiness payloads without checks", () => {
    expect(() =>
      readinessPayloadSchema.parse({
        service: "bore-v2-api",
        status: "ready",
        version: "0.0.0-test",
        timestamp: new Date().toISOString(),
        checks: [],
      }),
    ).toThrow();
  });

  test("parses a valid session detail payload", () => {
    const timestamp = new Date().toISOString();
    const payload = sessionDetailSchema.parse({
      id: "4a218497-6214-49a1-b7bf-85bf52ec2fbe",
      code: "ember-orbit-421",
      status: "waiting_receiver",
      createdAt: timestamp,
      updatedAt: timestamp,
      expiresAt: timestamp,
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
      ],
      events: [
        {
          id: "df8ffbde-cf34-4f8d-a73d-c6c13fda245d",
          type: "session_created",
          actorRole: "sender",
          timestamp,
          payload: {
            expiresInMinutes: 15,
          },
        },
      ],
    });

    expect(payload.file.name).toBe("report.pdf");
  });

  test("parses a valid coordination snapshot envelope", () => {
    const timestamp = new Date().toISOString();
    const payload = coordinationEnvelopeSchema.parse({
      type: "session_snapshot",
      session: {
        id: "4a218497-6214-49a1-b7bf-85bf52ec2fbe",
        code: "ember-orbit-421",
        status: "waiting_receiver",
        createdAt: timestamp,
        updatedAt: timestamp,
        expiresAt: timestamp,
        file: {
          name: "report.pdf",
          sizeBytes: 58213,
        },
        participants: [
          {
            role: "sender",
            status: "joined",
            joinedAt: timestamp,
          },
        ],
        events: [],
      },
    });

    expect(payload.type).toBe("session_snapshot");
  });

  test("parses a valid coordination event envelope", () => {
    const timestamp = new Date().toISOString();
    const payload = coordinationEnvelopeSchema.parse({
      type: "session_event",
      sessionCode: "ember-orbit-421",
      status: "ready",
      event: {
        id: "df8ffbde-cf34-4f8d-a73d-c6c13fda245d",
        type: "receiver_joined",
        actorRole: "receiver",
        timestamp,
        payload: {
          displayName: "Sawyer",
        },
      },
    });

    expect(payload.type).toBe("session_event");
    if (payload.type !== "session_event") {
      throw new Error("expected a session_event envelope");
    }
    expect(payload.event.type).toBe("receiver_joined");
  });

  test("rejects malformed coordination envelopes", () => {
    expect(() =>
      coordinationEnvelopeSchema.parse({
        type: "session_event",
        sessionCode: "not a rendezvous code",
        status: "ready",
        event: {
          id: "df8ffbde-cf34-4f8d-a73d-c6c13fda245d",
          type: "receiver_joined",
          timestamp: new Date().toISOString(),
          payload: {},
        },
      }),
    ).toThrow();
  });

  test("parses a valid operator summary payload", () => {
    const timestamp = new Date().toISOString();
    const payload = operatorSummaryPayloadSchema.parse({
      generatedAt: timestamp,
      counts: {
        total: 1,
        waitingReceiver: 1,
        ready: 0,
        transferring: 0,
        completed: 0,
        failed: 0,
        expired: 0,
        cancelled: 0,
      },
      sessions: [
        {
          id: "4a218497-6214-49a1-b7bf-85bf52ec2fbe",
          code: "ember-orbit-421",
          status: "waiting_receiver",
          createdAt: timestamp,
          updatedAt: timestamp,
          expiresAt: timestamp,
          fileName: "report.pdf",
          fileSizeBytes: 58213,
          senderJoinedAt: timestamp,
          lastEventType: "file_registered",
        },
      ],
    });

    expect(payload.counts.total).toBe(1);
  });

  test("parses a typed error payload", () => {
    const payload = apiErrorPayloadSchema.parse({
      error: {
        code: "rate_limited",
        message: "invalid request",
        issues: [
          {
            code: "invalid_type",
            message: "Expected object",
            path: ["file"],
          },
        ],
      },
    });

    expect(payload.error.code).toBe("rate_limited");
  });
});
