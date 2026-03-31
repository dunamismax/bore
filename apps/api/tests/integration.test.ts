import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  test,
} from "bun:test";

import {
  operatorSummaryPayloadSchema,
  sessionDetailSchema,
} from "@bore/contracts";
import postgres from "postgres";

import { createApp } from "../src/app";
import type { AppConfig } from "../src/config";
import { resetDatabase, runMigrations } from "../src/migrations";

const databaseUrl = process.env.BORE_V2_DATABASE_TEST_URL ?? "";
const integration = databaseUrl ? describe : describe.skip;

integration("api integration", () => {
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
    databaseUrl,
    databaseSsl: false,
  };

  const sql = postgres(databaseUrl, {
    connect_timeout: 5,
    idle_timeout: 1,
    max: 1,
  });
  const app = createApp({ config, sql });

  beforeAll(async () => {
    await resetDatabase(sql);
    await runMigrations(sql);
  });

  beforeEach(async () => {
    await resetDatabase(sql);
    await runMigrations(sql);
  });

  afterAll(async () => {
    await sql.end({ timeout: 0 });
  });

  test("creates, reads, joins, and summarizes a persisted session", async () => {
    const createResponse = await app.handle(
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
            checksumSha256:
              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          },
          senderDisplayName: "Stephen",
          expiresInMinutes: 15,
        }),
      }),
    );
    const created = sessionDetailSchema.parse(await createResponse.json());

    expect(createResponse.status).toBe(201);
    expect(created.status).toBe("waiting_receiver");
    expect(created.participants).toHaveLength(1);
    expect(created.events.map((event) => event.type)).toEqual([
      "session_created",
      "file_registered",
    ]);

    const readResponse = await app.handle(
      new Request(`http://localhost/api/sessions/${created.code}`),
    );
    const readBack = sessionDetailSchema.parse(await readResponse.json());

    expect(readResponse.status).toBe(200);
    expect(readBack.code).toBe(created.code);

    const joinResponse = await app.handle(
      new Request(`http://localhost/api/sessions/${created.code}/join`, {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          displayName: "Receiver",
        }),
      }),
    );
    const joined = sessionDetailSchema.parse(await joinResponse.json());

    expect(joinResponse.status).toBe(200);
    expect(joined.status).toBe("ready");
    expect(joined.participants).toHaveLength(2);
    expect(joined.events.at(-1)?.type).toBe("receiver_joined");

    const summaryResponse = await app.handle(
      new Request("http://localhost/api/ops/summary"),
    );
    const summary = operatorSummaryPayloadSchema.parse(
      await summaryResponse.json(),
    );

    expect(summaryResponse.status).toBe(200);
    expect(summary.counts.total).toBe(1);
    expect(summary.counts.ready).toBe(1);
    expect(summary.sessions[0]?.code).toBe(created.code);
  });

  test("rejects duplicate receiver joins", async () => {
    const createResponse = await app.handle(
      new Request("http://localhost/api/sessions", {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          file: {
            name: "report.pdf",
            sizeBytes: 58213,
          },
        }),
      }),
    );
    const created = sessionDetailSchema.parse(await createResponse.json());

    const firstJoin = await app.handle(
      new Request(`http://localhost/api/sessions/${created.code}/join`, {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          displayName: "Receiver",
        }),
      }),
    );

    expect(firstJoin.status).toBe(200);

    const duplicateJoin = await app.handle(
      new Request(`http://localhost/api/sessions/${created.code}/join`, {
        method: "POST",
        headers: {
          "content-type": "application/json",
        },
        body: JSON.stringify({
          displayName: "Another receiver",
        }),
      }),
    );
    const payload = await duplicateJoin.json();

    expect(duplicateJoin.status).toBe(409);
    expect(payload.error.code).toBe("conflict");
  });
});
