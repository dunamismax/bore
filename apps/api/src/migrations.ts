import { readdir, readFile } from "node:fs/promises";

import type { Sql } from "postgres";

const migrationsDir = new URL("../../../db/migrations/", import.meta.url);

type MigrationRow = {
  filename: string;
};

export async function listMigrationFilenames() {
  const entries = await readdir(migrationsDir, { withFileTypes: true });

  return entries
    .filter((entry) => entry.isFile() && entry.name.endsWith(".sql"))
    .map((entry) => entry.name)
    .sort((left, right) => left.localeCompare(right));
}

async function ensureSchemaMigrationsTable(sql: Sql) {
  const rows = await sql<{ exists: string | null }[]>`
    select to_regclass('public.schema_migrations') as exists
  `;

  if (rows[0]?.exists) {
    return;
  }

  await sql`
    create table schema_migrations (
      filename text primary key,
      applied_at timestamptz not null default now()
    )
  `;
}

export async function runMigrations(sql: Sql) {
  await ensureSchemaMigrationsTable(sql);

  const [filenames, appliedRows] = await Promise.all([
    listMigrationFilenames(),
    sql<MigrationRow[]>`
      select filename
      from schema_migrations
      order by filename asc
    `,
  ]);

  const applied = new Set(appliedRows.map((row) => row.filename));
  const executed: string[] = [];

  for (const filename of filenames) {
    if (applied.has(filename)) {
      continue;
    }

    const sqlText = await readFile(new URL(filename, migrationsDir), "utf8");

    await sql.begin(async (tx) => {
      await tx.unsafe(sqlText);
      await tx`
        insert into schema_migrations (filename)
        values (${filename})
      `;
    });

    executed.push(filename);
  }

  return executed;
}

export async function resetDatabase(sql: Sql) {
  await sql`drop schema if exists public cascade`;
  await sql`create schema public`;
}
