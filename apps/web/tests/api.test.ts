import { describe, expect, test } from "bun:test";

import {
  type CreateSessionRequest,
  type SessionDetail,
  serviceName,
} from "@bore/contracts";

import { ApiClientError, createBoreApiClient } from "../src/lib/api";

const timestamp = "2026-03-31T14:00:00.000Z";

function makeSessionDetail(): SessionDetail {
  return {
    id: "7b8d1d0c-1dcc-4b49-bf34-ff94b68207b8",
    code: "ember-orbit-421",
    status: "waiting_receiver",
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

describe("typed web api client", () => {
  test("parses health responses through the shared contract", async () => {
    const client = createBoreApiClient({
      baseUrl: "http://localhost:8080",
      fetch: async (input) => {
        expect(String(input)).toBe("http://localhost:8080/api/health");

        return new Response(
          JSON.stringify({
            service: serviceName,
            status: "ok",
            version: "0.0.0-test",
            environment: "test",
            uptimeSeconds: 2,
            timestamp,
            readiness: "ready",
          }),
        );
      },
    });

    const payload = await client.getHealth();

    expect(payload.service).toBe(serviceName);
    expect(payload.readiness).toBe("ready");
  });

  test("posts typed session create payloads", async () => {
    const input: CreateSessionRequest = {
      file: {
        name: "report.pdf",
        sizeBytes: 58213,
        mimeType: "application/pdf",
      },
      senderDisplayName: "Stephen",
      expiresInMinutes: 15,
    };

    const client = createBoreApiClient({
      fetch: async (_, init) => {
        expect(init?.method).toBe("POST");
        expect(JSON.parse(String(init?.body))).toEqual(input);

        return new Response(JSON.stringify(makeSessionDetail()), {
          status: 201,
        });
      },
    });

    const payload = await client.createSession(input);

    expect(payload.code).toBe("ember-orbit-421");
  });

  test("surfaces typed api errors", async () => {
    const client = createBoreApiClient({
      fetch: async () =>
        new Response(
          JSON.stringify({
            error: {
              code: "conflict",
              message: "receiver already joined this session",
            },
          }),
          {
            status: 409,
          },
        ),
    });

    try {
      await client.getSession("ember-orbit-421");
      throw new Error("expected getSession to throw");
    } catch (error) {
      expect(error).toMatchObject({
        name: ApiClientError.name,
        status: 409,
        message: "receiver already joined this session",
      });
    }
  });
});
