import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const workspaces = ["packages/contracts", "apps/api", "apps/web"];
const requestedTask = process.argv[2];

const tasks = {
  lint: ["lint"],
  check: ["check"],
  test: ["test"],
  build: ["build"],
  verify: ["lint", "check", "test", "build"],
};

if (!requestedTask || !(requestedTask in tasks)) {
  console.error(
    `Unknown workspace task "${requestedTask ?? ""}". Expected one of: ${Object.keys(tasks).join(", ")}.`,
  );
  process.exit(1);
}

for (const task of tasks[requestedTask]) {
  for (const workspace of workspaces) {
    const cwd = resolve(rootDir, workspace);
    console.log(`\n> ${workspace}: bun run ${task}`);

    const result = Bun.spawnSync({
      cmd: ["bun", "run", task],
      cwd,
      stdout: "inherit",
      stderr: "inherit",
    });

    if (result.exitCode !== 0) {
      process.exit(result.exitCode ?? 1);
    }
  }
}
