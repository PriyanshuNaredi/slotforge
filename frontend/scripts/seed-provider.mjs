/**
 * Seed a provider account (and optional sample slots) directly into the
 * SlotForge database. The backend registers customers only — provider roles
 * are granted in the database, exactly like the platform's integration tests do.
 *
 * Usage:
 *   node scripts/seed-provider.mjs --email provider@example.com --password "at-least-12-chars" --name "Dr. Example" [--slots 3]
 *
 * Env: DATABASE_URL (postgres://...) — defaults to the repo root .env value.
 */
import { readFileSync } from "node:fs";
import { randomUUID } from "node:crypto";
import bcrypt from "bcryptjs";
import pg from "pg";

function loadDatabaseUrl() {
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

function argValue(flag) {
  const index = process.argv.indexOf(flag);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

const email = argValue("--email");
const password = argValue("--password");
const name = argValue("--name") ?? "Dr. Example Provider";
const slotCount = Number(argValue("--slots") ?? "3");

if (!email || !password) {
  console.error("Usage: node scripts/seed-provider.mjs --email you@example.com --password 'min-12-chars' [--name 'Dr. Example'] [--slots 3]");
  process.exit(1);
}

if (password.length < 12) {
  console.error("Password must be at least 12 characters (backend rule).");
  process.exit(1);
}

const databaseUrl = loadDatabaseUrl();
if (!databaseUrl) {
  console.error("DATABASE_URL not set and not found in ../../.env");
  process.exit(1);
}

const client = new pg.Client({ connectionString: databaseUrl });
await client.connect();

try {
  const userId = randomUUID();

  await client.query(
    `INSERT INTO users (id, email, password_hash, role)
     VALUES ($1, $2, $3, 'provider')`,
    [userId, email.toLowerCase(), bcrypt.hashSync(password, 10)],
  );

  await client.query(
    `INSERT INTO provider_profiles (user_id, display_name, bio, timezone)
     VALUES ($1, $2, $3, $4)`,
    [userId, name, "Seeded demo provider", Intl.DateTimeFormat().resolvedOptions().timeZone],
  );

  // Sample slots: the next N business mornings, 1 hour each.
  let created = 0;
  const day = 24 * 60 * 60 * 1000;
  for (let offset = 1; created < slotCount; offset++) {
    const start = new Date(Date.now() + offset * day);
    start.setHours(9, 0, 0, 0);
    if (start.getDay() === 0 || start.getDay() === 6) continue;
    const end = new Date(start.getTime() + 60 * 60 * 1000);
    await client.query(
      `INSERT INTO availability_slots (id, provider_id, starts_at, ends_at, status)
       VALUES ($1, $2, $3, $4, 'open')`,
      [randomUUID(), userId, start.toISOString(), end.toISOString()],
    );
    created++;
  }

  console.log(`Provider ready: ${email}`);
  console.log(`  profile: ${name}`);
  console.log(`  open slots: ${created}`);
  console.log("Sign in at /login with this email and password.");
} finally {
  await client.end();
}
