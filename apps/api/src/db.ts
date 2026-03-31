import postgres, { type Sql } from "postgres";

import type { AppConfig } from "./config";

export function createDatabaseClient(config: AppConfig) {
  return postgres(config.databaseUrl, {
    connect_timeout: 5,
    idle_timeout: 20,
    max: 4,
    ssl: config.databaseSsl ? "require" : false,
  });
}

export async function probeDatabase(sql: Sql) {
  const startedAt = performance.now();

  await sql`select 1`;

  return Math.round(performance.now() - startedAt);
}
