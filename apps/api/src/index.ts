import { createApp } from "./app";
import { parseConfig } from "./config";
import { createDatabaseClient } from "./db";
import { runMigrations } from "./migrations";

const config = parseConfig(process.env);
const sql = createDatabaseClient(config);
const app = createApp({ config, sql });

async function shutdown(signal: string) {
  console.log(`shutting down api after ${signal}`);
  await sql.end({ timeout: 5 });
  process.exit(0);
}

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.on(signal, () => {
    void shutdown(signal);
  });
}

await runMigrations(sql);

app.listen(
  {
    hostname: config.host,
    port: config.port,
  },
  ({ hostname, port }) => {
    console.log(
      `bore v2 api listening on http://${hostname}:${port} for ${config.environment}`,
    );
  },
);
