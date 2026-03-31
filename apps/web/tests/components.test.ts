import { describe, expect, test } from "bun:test";
import {
  existsSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { compileScript, compileTemplate, parse } from "@vue/compiler-sfc";
import { createSSRApp } from "vue";
import { renderToString } from "vue/server-renderer";

type ImportReplacements = Record<string, string>;

const webRoot = resolve(import.meta.dir, "..");
const componentsRoot = resolve(webRoot, "src/components");

function normalizeHtml(html: string) {
  return html.replace(/\s+/g, " ").trim();
}

function resolveImportSpecifier(fromFile: string, specifier: string) {
  if (!specifier.startsWith(".")) {
    return specifier;
  }

  const base = resolve(dirname(fromFile), specifier);
  const candidates = [
    base,
    `${base}.ts`,
    `${base}.js`,
    `${base}.mjs`,
    `${base}.vue`,
    join(base, "index.ts"),
    join(base, "index.js"),
  ];

  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      return pathToFileURL(candidate).href;
    }
  }

  return pathToFileURL(base).href;
}

function rewriteImportSpecifiers(
  code: string,
  fromFile: string,
  replacements: ImportReplacements,
) {
  return code.replace(
    /(from\s+["'])([^"']+)(["'])/g,
    (_match, prefix, specifier, suffix) => {
      const mapped =
        replacements[specifier] ?? resolveImportSpecifier(fromFile, specifier);
      return `${prefix}${mapped}${suffix}`;
    },
  );
}

async function compileVueSfc(
  componentFile: string,
  replacements: ImportReplacements = {},
  tempDir: string,
) {
  const filename = resolve(componentFile);
  const source = readFileSync(filename, "utf8");
  const { descriptor } = parse(source, { filename });
  const id = `bore-web-test-${Buffer.from(filename).toString("base64url")}`;
  const script = compileScript(descriptor, {
    id,
    genDefaultAs: "__sfc__",
  });
  const template = compileTemplate({
    id,
    filename,
    source: descriptor.template?.content ?? "",
    compilerOptions: {
      bindingMetadata: script.bindings,
    },
  });

  if (template.errors.length > 0) {
    throw new Error(
      `failed to compile ${filename}: ${template.errors.join(", ")}`,
    );
  }

  const compiled = [
    script.content,
    template.code.replace("export function render", "function render"),
    "__sfc__.render = render;",
    "export default __sfc__;",
    "",
  ].join("\n");

  const tempFile = join(tempDir, "component.ts");
  writeFileSync(
    tempFile,
    rewriteImportSpecifiers(compiled, filename, replacements),
  );

  return (await import(pathToFileURL(tempFile).href)).default;
}

function writeMockModule(tempDir: string, source: string) {
  const tempFile = join(
    tempDir,
    `mock-${Math.random().toString(36).slice(2)}.ts`,
  );
  writeFileSync(tempFile, source);
  return pathToFileURL(tempFile).href;
}

async function renderComponent(
  componentName: string,
  mockModules: Record<string, string> = {},
  props: Record<string, unknown> = {},
) {
  const tempDir = mkdtempSync(join(webRoot, ".bun-test-sfc-"));

  try {
    const replacements = Object.fromEntries(
      Object.entries(mockModules).map(([specifier, source]) => [
        specifier,
        writeMockModule(tempDir, source),
      ]),
    );
    const component = await compileVueSfc(
      resolve(componentsRoot, componentName),
      replacements,
      tempDir,
    );

    return normalizeHtml(await renderToString(createSSRApp(component, props)));
  } finally {
    rmSync(tempDir, { force: true, recursive: true });
  }
}

describe("web Vue SFC components", () => {
  test("CreateSessionForm renders validation feedback and the created-session summary", async () => {
    const html = await renderComponent("CreateSessionForm.vue", {
      "../composables/useCreateSessionForm": `
        import { reactive, ref } from "vue";

        export function useCreateSessionForm() {
          return {
            form: reactive({
              fileName: "report.pdf",
              sizeBytes: "58213",
              mimeType: "application/pdf",
              checksumSha256: "",
              senderDisplayName: "Stephen",
              expiresInMinutes: "15",
            }),
            createdSession: ref({
              code: "ember-orbit-421",
              status: "waiting_receiver",
              file: {
                name: "report.pdf",
                sizeBytes: 58213,
              },
              events: [{ id: "evt_1" }],
            }),
            fieldErrors: ref({
              "file.name": "file name is required",
            }),
            submitError: ref("request validation failed"),
            submitting: ref(false),
            submit: async () => true,
          };
        }
      `,
    });

    expect(html).toContain("Create a transfer session");
    expect(html).toContain("file name is required");
    expect(html).toContain("request validation failed");
    expect(html).toContain("Session created");
    expect(html).toContain("ember-orbit-421");
    expect(html).toContain("58,213 bytes");
    expect(html).toContain("/receive/ember-orbit-421");
  });

  test("JoinSessionPanel renders the waiting-session join shell", async () => {
    const html = await renderComponent(
      "JoinSessionPanel.vue",
      {
        "../composables/useJoinSession": `
        import { reactive, ref } from "vue";

        export function useJoinSession() {
          return {
            canJoin: ref(true),
            fieldErrors: ref({
              displayName: "display name must be at least 2 characters",
            }),
            form: reactive({
              displayName: "S",
            }),
            joinError: ref("request validation failed"),
            joining: ref(false),
            loadError: ref(null),
            loadingSession: ref(false),
            loadSession: async () => true,
            session: ref({
              code: "ember-orbit-421",
              status: "waiting_receiver",
              expiresAt: "2026-03-31T14:15:00.000Z",
              file: {
                name: "report.pdf",
                sizeBytes: 58213,
              },
              participants: [
                {
                  role: "sender",
                  displayName: "Stephen",
                  status: "joined",
                  joinedAt: "2026-03-31T14:00:00.000Z",
                },
              ],
              events: [
                {
                  id: "evt_1",
                  type: "session_created",
                  actorRole: "sender",
                  timestamp: "2026-03-31T14:00:00.000Z",
                },
              ],
            }),
            submitJoin: async () => true,
          };
        }
      `,
      },
      {
        code: "ember-orbit-421",
      },
    );

    expect(html).toContain("Join code:");
    expect(html).toContain("ember-orbit-421");
    expect(html).toContain("Attach as the receiver");
    expect(html).toContain("Stephen");
    expect(html).toContain("session_created");
    expect(html).toContain("display name must be at least 2 characters");
    expect(html).toContain("request validation failed");
  });

  test("JoinSessionPanel renders the post-join state when the receiver is already attached", async () => {
    const html = await renderComponent(
      "JoinSessionPanel.vue",
      {
        "../composables/useJoinSession": `
        import { reactive, ref } from "vue";

        export function useJoinSession() {
          return {
            canJoin: ref(false),
            fieldErrors: ref({}),
            form: reactive({
              displayName: "Sawyer",
            }),
            joinError: ref(null),
            joining: ref(false),
            loadError: ref(null),
            loadingSession: ref(false),
            loadSession: async () => true,
            session: ref({
              code: "ember-orbit-421",
              status: "ready",
              expiresAt: "2026-03-31T14:15:00.000Z",
              file: {
                name: "report.pdf",
                sizeBytes: 58213,
              },
              participants: [
                {
                  role: "sender",
                  displayName: "Stephen",
                  status: "joined",
                  joinedAt: "2026-03-31T14:00:00.000Z",
                },
                {
                  role: "receiver",
                  displayName: "Sawyer",
                  status: "joined",
                  joinedAt: "2026-03-31T14:01:00.000Z",
                },
              ],
              events: [
                {
                  id: "evt_2",
                  type: "participant_joined",
                  actorRole: "receiver",
                  timestamp: "2026-03-31T14:01:00.000Z",
                },
              ],
            }),
            submitJoin: async () => true,
          };
        }
      `,
      },
      {
        code: "ember-orbit-421",
      },
    );

    expect(html).toContain("Receiver already attached");
    expect(html).toContain("Sawyer");
    expect(html).toContain("participant_joined");
    expect(html).not.toContain("Attach as the receiver");
  });

  test("OpsSummaryPanel renders the typed operator summary inventory", async () => {
    const html = await renderComponent("OpsSummaryPanel.vue", {
      "../composables/useOpsSummary": `
        import { ref } from "vue";

        export function useOpsSummary() {
          return {
            error: ref(null),
            loading: ref(false),
            refresh: async () => true,
            summary: ref({
              generatedAt: "2026-03-31T14:00:00.000Z",
              counts: {
                total: 3,
                waitingReceiver: 1,
                ready: 1,
                completed: 1,
                failed: 0,
                expired: 0,
                cancelled: 0,
              },
              sessions: [
                {
                  id: "session_1",
                  code: "ember-orbit-421",
                  status: "waiting_receiver",
                  createdAt: "2026-03-31T13:55:00.000Z",
                  updatedAt: "2026-03-31T14:00:00.000Z",
                  expiresAt: "2026-03-31T14:15:00.000Z",
                  fileName: "report.pdf",
                  fileSizeBytes: 58213,
                  senderJoinedAt: "2026-03-31T13:56:00.000Z",
                  lastEventType: "file_registered",
                },
              ],
            }),
          };
        }
      `,
    });

    expect(html).toContain("Live session inventory");
    expect(html).toContain("report.pdf");
    expect(html).toContain("58,213 bytes");
    expect(html).toContain("file_registered");
    expect(html).toContain("/receive/ember-orbit-421");
    expect(html).toContain(">3<");
  });

  test("HealthSnapshot renders the initial loading state before client refresh completes", async () => {
    const html = await renderComponent("HealthSnapshot.vue");

    expect(html).toContain("Runtime snapshot");
    expect(html).toContain("Loading the v2 API health checks through Caddy.");
  });
});
