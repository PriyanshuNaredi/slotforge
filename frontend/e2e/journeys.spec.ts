import { expect, test } from "@playwright/test";
import { randomUUID } from "node:crypto";

const API = "http://localhost:8081/api/v1";
const PASS = `E2e-${randomUUID()}-pw!`;

const api = async (
  method: string,
  path: string,
  token?: string,
  body?: unknown,
) => {
  const res = await fetch(API + path, {
    method,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  let json: Record<string, unknown> = {};
  try {
    json = await res.json();
  } catch {}
  return { status: res.status, json };
};

const email = (tag: string) =>
  `pw-${tag}-${randomUUID().slice(0, 8)}@example.com`;

// Seeded by scripts/e2e-seed.mjs before the run (SQL: user row only, no profile).
const PROVIDER = { email: process.env.PW_PROVIDER_EMAIL!, password: process.env.PW_PROVIDER_PASS! };
let providerToken = "";
let providerUserId = "";

let slots: string[] = [];
const customer = { email: email("main"), password: PASS, token: "", refreshToken: "" };
const customer2 = { email: email("second"), password: PASS, token: "", refreshToken: "" };

test.describe.configure({ mode: "serial" });

test.beforeAll(async () => {
  const seeded = await api("POST", "/auth/login", undefined, {
    email: PROVIDER.email,
    password: PROVIDER.password,
  });
  expect(seeded.status, "seeded provider must log in").toBe(200);
  providerToken = seeded.json.access_token as string;
  providerUserId = (seeded.json.user as { id: string }).id;

  // Profile via API if the UI flow does not already own it (idempotent).
  const prof = await api("POST", "/providers/profile", providerToken, {
    display_name: "Playwright Provider",
    timezone: "UTC",
  });
  expect([201, 409]).toContain(prof.status);

  // Materialize slots via the recurring endpoint (also exercises 19.1).
  const day = 8;
  const from = new Date(Date.now() + day * 86400e3).toISOString().slice(0, 10);
  const to = new Date(Date.now() + (day + 4) * 86400e3).toISOString().slice(0, 10);
  const rec = await api("POST", "/providers/slots/recurring", providerToken, {
    start_time: "10:00",
    end_time: "16:00",
    days_of_week: [0, 1, 2, 3, 4, 5, 6],
    date_from: from,
    date_to: to,
    timezone: "UTC",
  });
  expect(rec.status, JSON.stringify(rec.json)).toBe(201);

  const disc = await api(
    "GET",
    `/slots?provider_id=${providerUserId}&limit=100`,
  );
  expect(disc.status).toBe(200);
  slots = ((disc.json.slots ?? []) as { id: string }[]).map((s) => s.id);
  expect(slots.length).toBeGreaterThanOrEqual(5);

  for (const c of [customer, customer2]) {
    const reg = await api("POST", "/auth/register", undefined, {
      email: c.email,
      password: c.password,
    });
    expect(reg.status).toBe(201);
    c.token = reg.json.access_token as string;
    c.refreshToken = reg.json.refresh_token as string;
  }
});

test("1+2: customer registration then login via UI", async ({ page }) => {
  const fresh = { email: email("ui"), password: PASS };

  await page.goto("/register");
  await page.fill("#email", fresh.email);
  await page.fill("#password", fresh.password);
  await page.fill("#confirmPassword", fresh.password);
  await page.getByRole("button", { name: /create|sign up|register/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });

  // Session cookie/token persisted in localStorage: log back in cleanly.
  await page.context().clearCookies();
  await page.goto("/login");
  await page.fill("#email", fresh.email);
  await page.fill("#password", fresh.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });
});

test("3+4: provider profile and availability via UI", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#email", PROVIDER.email);
  await page.fill("#password", PROVIDER.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/provider/, { timeout: 20_000 });

  // Pick a (day, hour) with no existing slot so reruns never collide.
  // The provider's own slot list includes booked rows, unlike discovery.
  const existing = await api("GET", "/providers/slots", providerToken);
  const busy = new Set(
    ((existing.json.slots ?? []) as { starts_at: string }[]).map((s) => {
      const d = new Date(s.starts_at);
      return `${d.toISOString().slice(0, 10)}|${d.getUTCHours()}`;
    }),
  );
  let future = new Date(Date.now() + 7 * 86400e3);
  let hour = 9;
  outer: for (let dayOff = 7; dayOff <= 14; dayOff++) {
    const d = new Date(Date.now() + dayOff * 86400e3);
    for (hour = 9; hour <= 21; hour++) {
      if (!busy.has(`${d.toISOString().slice(0, 10)}|${hour}`)) {
        future = d;
        break outer;
      }
    }
  }

  await page.goto("/provider/availability");
  await page.getByRole("button", { name: /add|new|create/i }).first().click();
  const dateStr = future.toISOString().slice(0, 10);
  const startStr = `${String(hour).padStart(2, "0")}:00`;
  const endStr = `${String(hour + 1).padStart(2, "0")}:00`;
  await page.fill("#slot-date", dateStr);
  await page.fill("#slot-start", startStr);
  await page.fill("#slot-end", endStr);
  await page.getByRole("button", { name: /publish slot/i }).click();

  // The availability UI is a month calendar: select the new slot's day and
  // expect its time in the day's slot list.
  const dayLabel = future.toLocaleDateString("en-US", {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
  await page.getByRole("button", { name: dayLabel }).click();
  const h12 = hour > 12 ? hour - 12 : hour;
  const timeRe = new RegExp(`${startStr}|${h12}:00( ?(PM|AM))?`);
  await expect(
    page.getByText(timeRe).first(),
    "created slot should appear in the day list",
  ).toBeVisible({ timeout: 20_000 });
});

test("5: customer discovers the provider", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#email", customer.email);
  await page.fill("#password", customer.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });

  await page.goto("/dashboard/discover");
  await expect(
    page.getByText("Playwright Provider").first(),
    "provider appears in directory",
  ).toBeVisible({ timeout: 20_000 });
});

test("6+7: booking via UI; conflicting second booking rejected", async ({
  page,
}) => {
  // Fresh context per test: log in first.
  await page.goto("/login");
  await page.fill("#email", customer.email);
  await page.fill("#password", customer.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });

  await page.goto("/dashboard/discover");
  await page.getByRole("tab", { name: /providers/i }).click();
  await page
    .locator('a[href*="/dashboard/discover/providers/"]')
    .first()
    .click();
  await expect(page).toHaveURL(/providers\//, { timeout: 20_000 });

  await page.getByRole("link", { name: /see available slots/i }).click();
  await expect(page).toHaveURL(/discover\?provider=/, { timeout: 20_000 });
  await page.getByRole("button", { name: /book this slot/i }).first().click();
  await expect(page.getByText(/confirm your booking/i)).toBeVisible();
  await page
    .getByRole("button", { name: /confirm/i })
    .last()
    .click();
  await expect(
    page.getByText(/confirmed|booked|success/i).first(),
  ).toBeVisible({ timeout: 20_000 });

  // The same slot through the API for a rival customer → 409.
  const myBookings = (await api("GET", "/bookings", customer.token)).json as {
    bookings?: { slot_id: string }[];
  };
  const bookedSlot = myBookings.bookings?.[0]?.slot_id as string;
  expect(bookedSlot).toBeTruthy();
  const conflict = await api("POST", "/bookings", customer2.token, {
    slot_id: bookedSlot,
  });
  expect(conflict.status).toBe(409);
});

test("8+9: reschedule then cancel via UI", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#email", customer.email);
  await page.fill("#password", customer.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });

  await page.goto("/dashboard/bookings");
  await expect(page.getByText(/playwright provider/i).first()).toBeVisible();

  // Reschedule from the booking detail page.
  await page.getByRole("link", { name: /details/i }).first().click();
  await page.getByRole("button", { name: /reschedule/i }).first().click();
  await expect(page.getByRole("dialog")).toBeVisible();
  // The dialog's slot rows are custom radios: the visible wrapper carries
  // the click handler, so click the label text, not the input.
  await page
    .getByRole("dialog")
    .getByText(/\d{1,2}:\d{2} (AM|PM) –/)
    .first()
    .click();
  await page
    .getByRole("button", { name: /confirm reschedule/i })
    .click();
  await expect(
    page.getByText(/rescheduled|moved/i).first(),
  ).toBeVisible({ timeout: 20_000 });

  // Cancel it (confirm dialog reuses the label "Cancel booking").
  await page.getByRole("button", { name: /cancel booking/i }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: /cancel booking/i })
    .click();
  await expect(
    page.getByText(/cancelled/i).first(),
  ).toBeVisible({ timeout: 20_000 });
});

test("10: provider booking lifecycle via UI", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#email", PROVIDER.email);
  await page.fill("#password", PROVIDER.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/provider/, { timeout: 20_000 });

  await page.goto("/provider/bookings");
  await expect(page.getByText(/playwright|confirmed|no_show|completed/i).first()).toBeVisible({
    timeout: 20_000,
  });
});

test("11: notifications visible for customer", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#email", customer.email);
  await page.fill("#password", customer.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });

  await page.goto("/dashboard/notifications");
  await expect(page.getByText(/rescheduled|cancelled|confirmed/i).first()).toBeVisible({
    timeout: 20_000,
  });
});

test("12: logout revokes the session", async ({ page }) => {
  await page.goto("/login");
  await page.fill("#email", customer2.email);
  await page.fill("#password", customer2.password);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 });

  // Refresh-token rotation through the API, then UI logout, then revocation.
  const oldRefresh = customer2.refreshToken;
  const refreshed = await api("POST", "/auth/refresh", undefined, {
    refresh_token: oldRefresh,
  });
  expect(refreshed.status).toBe(200);
  const rotatedRefresh = refreshed.json.refresh_token as string;
  const reuse = await api("POST", "/auth/refresh", undefined, {
    refresh_token: oldRefresh,
  });
  expect(reuse.status, "rotated-out refresh token must be rejected").toBe(401);

  // UI logout ends the browser session (redirects out of the dashboard).
  await page.getByRole("button", { name: /account menu/i }).click();
  await page.getByRole("menuitem", { name: /log ?out|sign out/i }).click();
  await expect(page).toHaveURL(/\/login$/, { timeout: 20_000 });

  // …and an explicit API logout revokes that session's refresh token.
  const out = await api("POST", "/auth/logout", refreshed.json.access_token as string, {
    refresh_token: rotatedRefresh,
  });
  expect(out.status).toBe(200);
  const revoked = await api("POST", "/auth/refresh", undefined, {
    refresh_token: rotatedRefresh,
  });
  expect(revoked.status, "logged-out refresh token must be rejected").toBe(401);

  // A different session (fresh login) is unaffected.
  const freshLogin = await api("POST", "/auth/login", undefined, {
    email: customer2.email,
    password: customer2.password,
  });
  expect(freshLogin.status).toBe(200);
  const freshRefresh = await api("POST", "/auth/refresh", undefined, {
    refresh_token: freshLogin.json.refresh_token as string,
  });
  expect(freshRefresh.status).toBe(200);
});
