/**
 * Seeds the Playwright E2E provider user (user row only — the API/UI creates
 * the profile). Usage:
 *   node scripts/e2e-seed.mjs "<DATABASE_URL of slotforge_e2e>"
 * Prints PW_PROVIDER_EMAIL / PW_PROVIDER_PASS exports; never prints the hash.
 */
import { randomUUID } from "node:crypto";
import { pathToFileURL } from "node:url";
import { fileURLToPath } from "node:url";

const databaseUrl = process.argv[2];
if (!databaseUrl) {
  console.error("usage: node scripts/e2e-seed.mjs <database-url>");
  process.exit(1);
}

const FE = fileURLToPath(new URL("../node_modules/", import.meta.url));
const pgMod = await import(pathToFileURL(`${FE}pg/lib/index.js`).href);
const Client = pgMod.default.Client;
const bcryptjs = (await import(pathToFileURL(`${FE}bcryptjs/index.js`).href)).default;

const u = new URL(databaseUrl);
const client = new Client({
  host: u.hostname,
  port: u.port || 5432,
  user: decodeURIComponent(u.username),
  password: decodeURIComponent(u.password),
  database: u.pathname.slice(1),
  connectionTimeoutMillis: 10000,
});
await client.connect();

const email = `pw-provider-${randomUUID().slice(0, 8)}@example.com`;
const pass = `PwProvider-${randomUUID()}!`;
const hash = bcryptjs.hashSync(pass, 10);

await client.query(
  `INSERT INTO users (email, password_hash, role) VALUES ($1, $2, 'provider')`,
  [email, hash],
);
await client.end();

console.log(`PW_PROVIDER_EMAIL=${email}`);
console.log(`PW_PROVIDER_PASS=${pass}`);
