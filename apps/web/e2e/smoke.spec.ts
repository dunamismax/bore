import { expect, test } from "@playwright/test";

const timestamp = "2026-03-31T14:00:00.000Z";

const waitingSession = {
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

const readySession = {
  ...waitingSession,
  status: "ready",
  updatedAt: "2026-03-31T14:01:00.000Z",
  participants: [
    ...waitingSession.participants,
    {
      role: "receiver",
      status: "joined",
      displayName: "Sawyer",
      joinedAt: "2026-03-31T14:01:00.000Z",
    },
  ],
  events: [
    ...waitingSession.events,
    {
      id: "f48d2ff0-b276-4b63-8cea-3e95cf5d2dcb",
      type: "receiver_joined",
      actorRole: "receiver",
      timestamp: "2026-03-31T14:01:00.000Z",
      payload: {
        displayName: "Sawyer",
      },
    },
  ],
};

const operatorSummary = {
  generatedAt: timestamp,
  counts: {
    total: 1,
    waitingReceiver: 0,
    ready: 1,
    completed: 0,
    failed: 0,
    expired: 0,
    cancelled: 0,
  },
  sessions: [
    {
      id: waitingSession.id,
      code: waitingSession.code,
      status: "ready",
      createdAt: waitingSession.createdAt,
      updatedAt: readySession.updatedAt,
      expiresAt: waitingSession.expiresAt,
      fileName: waitingSession.file.name,
      fileSizeBytes: waitingSession.file.sizeBytes,
      senderJoinedAt: timestamp,
      receiverJoinedAt: "2026-03-31T14:01:00.000Z",
      lastEventType: "receiver_joined",
    },
  ],
};

function json(body: unknown, status = 200) {
  return {
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  };
}

test("send shell creates a typed session", async ({ page }) => {
  await page.route("**/api/sessions", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }

    expect(route.request().postDataJSON()).toEqual({
      file: {
        name: "report.pdf",
        sizeBytes: 58213,
        mimeType: "application/pdf",
      },
      senderDisplayName: "Stephen",
      expiresInMinutes: 15,
    });

    await route.fulfill(json(waitingSession, 201));
  });

  await page.goto("/send");

  await page.locator('input[name="fileName"]').fill("report.pdf");
  await page.locator('input[name="sizeBytes"]').fill("58213");
  await page.locator('input[name="mimeType"]').fill("application/pdf");
  await page.locator('input[name="senderDisplayName"]').fill("Stephen");
  await page.locator('input[name="expiresInMinutes"]').fill("15");
  await page.getByRole("button", { name: "Create session" }).click();

  await expect(
    page.getByRole("heading", { name: "ember-orbit-421" }),
  ).toBeVisible();
  await expect(page.getByText("58,213 bytes")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open receive shell" }),
  ).toHaveAttribute("href", "/receive/ember-orbit-421");
});

test("receive shell loads and joins a waiting session", async ({ page }) => {
  await page.route("**/api/sessions/ember-orbit-421/join", async (route) => {
    expect(route.request().method()).toBe("POST");
    expect(route.request().postDataJSON()).toEqual({
      displayName: "Sawyer",
    });

    await route.fulfill(json(readySession));
  });

  await page.route("**/api/sessions/ember-orbit-421", async (route) => {
    expect(route.request().method()).toBe("GET");
    await route.fulfill(json(waitingSession));
  });

  await page.goto("/receive/ember-orbit-421");

  await expect(page.getByText("waiting_receiver").first()).toBeVisible();
  await expect(page.getByText("report.pdf").first()).toBeVisible();

  await page.getByLabel("Receiver display name").fill("Sawyer");
  await page.getByRole("button", { name: "Join session" }).click();

  await expect(page.getByText("Receiver already attached")).toBeVisible();
  await expect(page.getByText("ready").first()).toBeVisible();
  await expect(page.getByText("Sawyer").first()).toBeVisible();
});

test("ops shell renders health and operator summary data", async ({ page }) => {
  await page.route("**/api/health", async (route) => {
    await route.fulfill(
      json({
        service: "bore-v2-api",
        status: "ok",
        version: "0.0.0-test",
        environment: "test",
        uptimeSeconds: 3,
        timestamp,
        readiness: "ready",
      }),
    );
  });

  await page.route("**/api/readiness", async (route) => {
    await route.fulfill(
      json({
        service: "bore-v2-api",
        status: "ready",
        version: "0.0.0-test",
        timestamp,
        checks: [
          {
            name: "config",
            status: "ready",
          },
          {
            name: "database",
            status: "ready",
            latencyMs: 12,
          },
        ],
      }),
    );
  });

  await page.route("**/api/ops/summary", async (route) => {
    await route.fulfill(json(operatorSummary));
  });

  await page.goto("/ops");

  await expect(page.getByText("Runtime snapshot")).toBeVisible();
  await expect(page.getByText("bore-v2-api")).toBeVisible();
  await expect(page.getByText("12 ms")).toBeVisible();
  await expect(page.getByText("Live session inventory")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "ember-orbit-421" }),
  ).toHaveAttribute("href", "/receive/ember-orbit-421");
  await expect(page.getByText("receiver_joined").first()).toBeVisible();
});
