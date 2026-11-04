/**
 * Apply migrations/*.sql to a PostgreSQL database, in filename order,
 * tracked in a schema_migrations table. Each file runs inside one
 * transaction (the schema DDL is transaction-safe; no CONCURRENTLY).
 *
 * Usage:
 *   node scripts/apply-migrations.mjs <DATABASE_URL>
 *   # or set DATABASE_URL in the environment
 */
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import pg from "pg";

function loadDatabaseUrl() {
  if (process.argv[2]) return process.argv[2];
  if (process.env.DATABASE_URL) return process.env.DATABASE_URL;
  try {
    const env = readFileSync(new URL("../../.env", import.meta.url), "utf8");
    const match = env.match(/^DATABASE_URL=(.*)$/m);
    if (match) return match[1].trim();
  } catch {
    // fall through
  }
  return null;
}

const databaseUrl = loadDatabaseUrl();
if (!databaseUrl) {
  console.error("Usage: node scripts/apply-migrations.mjs <DATABASE_URL>");
  process.exit(1);
}

const migrationsDir = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "migrations",
);

const files = readdirSync(migrationsDir)
  .filter((name) => name.endsWith(".sql"))
  .sort();

const client = new pg.Client({ connectionString: databaseUrl });
await client.connect();

try {
  await client.query(`
    CREATE TABLE IF NOT EXISTS public.schema_migrations (
      name text PRIMARY KEY,
      applied_at timestamptz NOT NULL DEFAULT now()
    )
  `);

  const { rows } = await client.query("SELECT name FROM public.schema_migrations");
  const applied = new Set(rows.map((row) => row.name));

  let ran = 0;
  for (const file of files) {
    if (applied.has(file)) {
      console.log(`= ${file} (already applied)`);
      continue;
    }
    // Goose-style files carry a "-- +goose Down" rollback section; apply Up only.
    const raw = readFileSync(path.join(migrationsDir, file), "utf8");
    const sql = raw.split("-- +goose Down")[0];
    await client.query("BEGIN");
    try {
      await client.query(sql);
      await client.query("INSERT INTO public.schema_migrations (name) VALUES ($1)", [file]);
      await client.query("COMMIT");
      ran++;
      console.log(`+ ${file}`);
    } catch (err) {
      await client.query("ROLLBACK");
      throw new Error(`${file} failed: ${err.message}`);
    }
  }

  console.log(ran === 0 ? "Database is up to date." : `Applied ${ran} migration(s).`);
} finally {
  await client.end();
}
