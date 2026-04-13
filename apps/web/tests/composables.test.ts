import { describe, expect, test } from "bun:test";

import type {
  ApiErrorPayload,
  HealthPayload,
  OperatorSummaryPayload,
  ReadinessPayload,
  SessionDetail,
} from "@bore/contracts";
import { useCreateSessionForm } from "../src/composables/useCreateSessionForm";
import { useJoinSession } from "../src/composables/useJoinSession";
import { useOpsSummary } from "../src/composables/useOpsSummary";
import { ApiClientError, type BoreApiClient } from "../src/lib/api";

const timestamp = "2026-03-31T14:00:00.000Z";

function makeSessionDetail(status: SessionDetail["status"]): SessionDetail {
  return {
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
              role: "receiver" as const,
              status: "joined" as const,
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
  };
}

function makeHealthPayload(): HealthPayload {
  return {
    service: "bore-v2-api",
    status: "ok",
    version: "0.0.0-test",
    environment: "test",
    uptimeSeconds: 3,
    timestamp,
    readiness: "ready",
  };
}

function makeReadinessPayload(): ReadinessPayload {
  return {
    service: "bore-v2-api",
    status: "ready",
    version: "0.0.0-test",
    timestamp,
    checks: [
      {
        name: "config",
        status: "ready",
      },
    ],
  };
}

function makeOperatorSummary(): OperatorSummaryPayload {
  return {
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
  };
}

function makeApiErrorPayload(
  issues: NonNullable<ApiErrorPayload["error"]["issues"]>,
): ApiErrorPayload {
  return {
    error: {
      code: "bad_request",
      message: "request validation failed",
      issues,
    },
  };
}

function makeClient(overrides: Partial<BoreApiClient> = {}): BoreApiClient {
  return {
    getHealth: async () => makeHealthPayload(),
    getReadiness: async () => makeReadinessPayload(),
    createSession: async () => makeSessionDetail("waiting_receiver"),
    getSession: async () => makeSessionDetail("waiting_receiver"),
    joinSession: async () => makeSessionDetail("ready"),
    getOperatorSummary: async () => makeOperatorSummary(),
    ...overrides,
  };
}

describe("web composables", () => {
  test("blocks invalid create-session input before the network call", async () => {
    const composable = useCreateSessionForm(makeClient());

    composable.form.fileName = "";
    composable.form.sizeBytes = "-1";
    composable.form.expiresInMinutes = "0";

    const result = await composable.submit();

    expect(result).toBe(false);
    expect(composable.fieldErrors.value["file.name"]).toBeDefined();
    expect(composable.fieldErrors.value["file.sizeBytes"]).toBeDefined();
    expect(composable.fieldErrors.value.expiresInMinutes).toBeDefined();
  });

  test("creates a session and stores the typed payload", async () => {
    const composable = useCreateSessionForm(makeClient());

    composable.form.fileName = "report.pdf";
    composable.form.sizeBytes = "58213";
    composable.form.mimeType = "application/pdf";
    composable.form.senderDisplayName = "Stephen";
    composable.form.expiresInMinutes = "15";

    const result = await composable.submit();

    expect(result).toBe(true);
    expect(composable.createdSession.value?.code).toBe("ember-orbit-421");
  });

  test("maps typed api validation issues back onto the create form", async () => {
    const composable = useCreateSessionForm(
      makeClient({
        createSession: async () => {
          throw new ApiClientError(
            400,
            "request validation failed",
            makeApiErrorPayload([
              {
                code: "too_small",
                message: "file name is required",
                path: ["file", "name"],
              },
            ]),
          );
        },
      }),
    );

    composable.form.fileName = "report.pdf";
    composable.form.sizeBytes = "58213";
    composable.form.mimeType = "application/pdf";
    composable.form.expiresInMinutes = "15";

    const result = await composable.submit();

    expect(result).toBe(false);
    expect(composable.submitError.value).toBe("request validation failed");
    expect(composable.fieldErrors.value["file.name"]).toBe(
      "file name is required",
    );
  });

  test("loads and joins a waiting session", async () => {
    const composable = useJoinSession("ember-orbit-421", makeClient());

    const loaded = await composable.loadSession();

    expect(loaded).toBe(true);
    expect(composable.session.value?.status).toBe("waiting_receiver");
    expect(composable.canJoin.value).toBe(true);

    composable.form.displayName = "Sawyer";

    const joined = await composable.submitJoin();

    expect(joined).toBe(true);
    expect(composable.session.value?.status).toBe("ready");
    expect(composable.canJoin.value).toBe(false);
  });

  test("maps typed api validation issues back onto the join form", async () => {
    const composable = useJoinSession(
      "ember-orbit-421",
      makeClient({
        joinSession: async () => {
          throw new ApiClientError(
            400,
            "request validation failed",
            makeApiErrorPayload([
              {
                code: "too_small",
                message: "display name must be at least 2 characters",
                path: ["displayName"],
              },
            ]),
          );
        },
      }),
    );

    composable.form.displayName = "S";

    const result = await composable.submitJoin();

    expect(result).toBe(false);
    expect(composable.joinError.value).toBe("request validation failed");
    expect(composable.fieldErrors.value.displayName).toBe(
      "display name must be at least 2 characters",
    );
  });

  test("loads the operator summary through the typed client", async () => {
    const composable = useOpsSummary(makeClient());

    const result = await composable.refresh();

    expect(result).toBe(true);
    expect(composable.summary.value?.counts.total).toBe(1);
  });
});
