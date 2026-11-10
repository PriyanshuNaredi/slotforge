/**
 * Apply migrations/*.sql to a PostgreSQL database, in filename order,
 * tracked in a schema_migrations table. Each file runs inside one
 * transaction (the schema DDL is transaction-safe; no CONCURRENTLY).
 *
 * Target URL resolution order:
 *   1. SLOTFORGE_DB_URL environment variable
 *   2. explicit command-line argument (legacy compatibility)
 *   3. .env DATABASE_URL fallback (local development only)
 *
 * With --production only the SLOTFORGE_DB_URL environment variable is
 * accepted: a missing variable is an error, and a command-line URL or
 * the .env fallback are refused.
 *
 * TLS policy (decided by this script, never by URL parameters):
 *   - every non-local hostname connects with TLS and full certificate
 *     verification (rejectUnauthorized: true);
 *   - local Docker development connects without TLS.
 * All sslmode/sslcert/sslkey/sslrootcert/sslnegotiation parameters are
 * stripped from the URL before pg parses it, so connection-string
 * parsing can never enable, disable, or weaken this configuration.
 * pg uses the PostgreSQL SSLRequest handshake (8-byte packet, code
 * 80877103, 'S' reply) before upgrading to TLS.
 *
 * The connection URL, password, and credentials are never printed; the
 * target is shown masked (username@host/database only) and error text
 * is scrubbed of the password before display.
 *
 * Usage:
 *   node scripts/apply-migrations.mjs --production
 *   node scripts/apply-migrations.mjs "<DATABASE_URL>"
 *   # or set DATABASE_URL in the environment (local development)
 */
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
import pg from "pg";

function loadEnvFallback() {
  try {
    const env = readFileSync(new URL("../../.env", import.meta.url), "utf8");
    const match = env.match(/^DATABASE_URL=(.*)$/m);
    if (match) return match[1].trim();
  } catch {
    // fall through
  }
  return null;
}

function resolveTarget() {
  const args = process.argv.slice(2);
  const production = args.includes("--production");
  const positional = args.filter((a) => a !== "--production");

  if (positional.length > 1) {
    console.error("Error: expected at most one URL argument.");
    process.exit(1);
  }

  if (production) {
    if (positional.length > 0) {
      console.error(
        "Error: --production does not accept a URL argument; " +
          "set SLOTFORGE_DB_URL instead."
      );
      process.exit(1);
    }
    const url = process.env.SLOTFORGE_DB_URL;
    if (!url) {
      console.error(
        "Error: --production requires SLOTFORGE_DB_URL to be set " +
          "(the .env fallback is disabled in production mode)."
      );
      process.exit(1);
    }
    return { url, production };
  }

  const url =
    process.env.SLOTFORGE_DB_URL ||
    positional[0] ||
    process.env.DATABASE_URL ||
    loadEnvFallback();

  if (!url) {
    console.error("Usage: node scripts/apply-migrations.mjs \"<DATABASE_URL>\"");
    process.exit(1);
  }

  return { url, production };
}

function isLocalHostname(hostname) {
  return ["localhost", "127.0.0.1", "::1"].includes(hostname);
}

// Removes every TLS-related parameter so pg's connection-string parsing
// can never override the TLS configuration decided by clientSslConfig.
function stripSslParams(urlString) {
  const parsed = new URL(urlString);
  for (const param of ["sslmode", "sslcert", "sslkey", "sslrootcert", "sslnegotiation"]) {
    parsed.searchParams.delete(param);
  }
  return parsed.toString();
}

// The exact `ssl` value handed to pg.Client. Non-local hosts always get
// TLS with full certificate verification; local development never does.
// The decision is independent of any sslmode parameter in the URL.
function clientSslConfig(urlString) {
  const parsed = new URL(urlString);
  if (isLocalHostname(parsed.hostname)) {
    return false;
  }
  return { rejectUnauthorized: true };
}

// Masked target: username@host/database only. Never the password, port,
// query string, or the raw URL.
function maskedTarget(urlString) {
  let parsed;
  try {
    parsed = new URL(urlString);
  } catch {
    console.error("Error: the connection URL is not valid.");
    process.exit(1);
  }
  return `${decodeURIComponent(parsed.username)}@${parsed.hostname}${parsed.pathname}`;
}

// Scrubs the password out of any error text before it reaches output.
function sanitize(text, password) {
  let out = String(text);
  if (password) out = out.split(password).join("***");
  return out;
}

async function run() {
  const { url: databaseUrl, production } = resolveTarget();

  const password = (() => {
    try {
      return decodeURIComponent(new URL(databaseUrl).password || "");
    } catch {
      return "";
    }
  })();

  const migrationsDir = path.join(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
    "..",
    "migrations",
  );

  const files = readdirSync(migrationsDir)
    .filter((name) => name.endsWith(".sql"))
    .sort();

  const ssl = clientSslConfig(databaseUrl);

  console.log(
    `Target: ${maskedTarget(databaseUrl)}${production ? " (production mode)" : ""}`
  );
  console.log(
    `TLS: ${ssl ? "required with certificate verification" : "disabled (local connection)"}`
  );
  console.log(`Migrations found: ${files.join(", ")}`);

  const client = new pg.Client({
    connectionString: stripSslParams(databaseUrl),
    ssl,
  });

  try {
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
          throw new Error(`${file} failed: ${sanitize(err.message, password)}`);
        }
      }

      console.log(ran === 0 ? "Database is up to date." : `Applied ${ran} migration(s).`);
    } finally {
      await client.end();
    }
  } catch (err) {
    console.error(`FAILED: ${sanitize(err && err.message, password)}`);
    process.exit(1);
  }
}

const invokedDirectly =
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href;

if (invokedDirectly) {
  await run();
}

// Exported for TLS-policy tests only; run() executes nothing on import.
export {
  isLocalHostname,
  stripSslParams,
  clientSslConfig,
  maskedTarget,
  resolveTarget,
  sanitize,
};
