import { createApp } from "./app";
import { parseConfig } from "./config";
import { createDatabaseClient } from "./db";
import { createJsonLogger } from "./logger";
import { runMigrations } from "./migrations";

const config = parseConfig(process.env);
const sql = createDatabaseClient(config);
const logger = createJsonLogger();
const app = createApp({ config, sql, logger });

async function shutdown(signal: string) {
  logger.info("api_shutdown", { signal });
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
    idleTimeout: config.idleTimeoutSeconds,
    maxRequestBodySize: config.maxRequestBodyBytes,
  },
  ({ hostname, port }) => {
    logger.info("api_started", {
      url: `http://${hostname}:${port}`,
      environment: config.environment,
      requestTimeoutMs: config.requestTimeoutMs,
      idleTimeoutSeconds: config.idleTimeoutSeconds,
      maxRequestBodyBytes: config.maxRequestBodyBytes,
      rateLimitWindowMs: config.rateLimitWindowMs,
      rateLimitMaxRequests: config.rateLimitMaxRequests,
      version: config.version,
    });
  },
);
