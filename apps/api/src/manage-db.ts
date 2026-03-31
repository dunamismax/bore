import { parseConfig } from "./config";
import { createDatabaseClient } from "./db";
import { resetDatabase, runMigrations } from "./migrations";

const command = process.argv[2] ?? "migrate";
const config = parseConfig(process.env);
const sql = createDatabaseClient(config);

try {
  if (command === "migrate") {
    const executed = await runMigrations(sql);

    console.log(
      executed.length === 0
        ? "database already up to date"
        : `applied migrations: ${executed.join(", ")}`,
    );
  } else if (command === "reset") {
    await resetDatabase(sql);
    const executed = await runMigrations(sql);

    console.log(
      `database reset complete${
        executed.length === 0 ? "" : ` with migrations: ${executed.join(", ")}`
      }`,
    );
  } else {
    console.error(`unknown db command: ${command}`);
    process.exitCode = 1;
  }
} finally {
  await sql.end({ timeout: 5 });
}
