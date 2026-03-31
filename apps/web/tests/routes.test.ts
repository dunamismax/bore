import { describe, expect, test } from "bun:test";

import { buildReceivePath, primaryRoutes } from "../src/lib/routes";

describe("web routes", () => {
  test("exposes the expected primary route structure", () => {
    expect(primaryRoutes.map((route) => route.href)).toEqual([
      "/",
      "/send",
      "/receive/demo-code",
      "/ops",
    ]);
  });

  test("encodes receive codes in route paths", () => {
    expect(buildReceivePath("outer crane")).toBe("/receive/outer%20crane");
  });
});
