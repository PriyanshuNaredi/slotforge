import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

/**
 * Phase 20.3 accessibility checks. The visual design is frozen: these scans
 * catch functional defects only (labels, roles, keyboard, contrast basics).
 * Serious/critical violations fail; moderate ones are reported and reviewed.
 */

const PAGES = [
  { path: "/login", name: "login" },
  { path: "/register", name: "register" },
  { path: "/", name: "marketing" },
];

const AUTHED = process.env.PW_A11Y_EMAIL;
const PASS = process.env.PW_A11Y_PASS;

for (const p of PAGES) {
  test(`axe: ${p.name} page has no serious violations`, async ({ page }) => {
    await page.goto(p.path);
    // Dev-mode pages must fully settle (fonts, CSS) or axe misreads contrast.
    await page.waitForLoadState("networkidle");
    await page.evaluate(() => document.fonts.ready);
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
      .analyze();
    const serious = results.violations.filter((v) =>
      ["serious", "critical"].includes(v.impact ?? ""),
    );
    if (serious.length > 0) {
      console.log(
        `${p.name}: ${serious.map((v) => `${v.id}(${v.impact})×${v.nodes.length}`).join(", ")}`,
      );
    }
    expect(serious, `${p.name} serious a11y violations`).toEqual([]);
  });
}

test("login form: labels, keyboard order, Enter submits", async ({ page }) => {
  await page.goto("/login");

  // Labels are associated with inputs (getByLabel resolves).
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByLabel("Password")).toBeVisible();

  // Keyboard: focus lands on email first, Tab reaches password, then submit.
  await page.getByLabel("Email").focus();
  await page.keyboard.press("Tab");
  await expect(page.getByLabel("Password")).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: /log ?in|sign in/i })).toBeFocused();

  // Enter in a field submits the form (native form semantics).
  await page.getByLabel("Email").fill(AUTHED ?? "unknown@example.com");
  await page.getByLabel("Password").fill(PASS ?? "wrong-password");
  await page.getByLabel("Password").press("Enter");
  // Either navigation or a server error alert must appear; a dead form fails.
  await expect(
    page.getByRole("alert").or(page.locator("text=/invalid|incorrect/i")),
  ).toBeVisible({ timeout: 10_000 });
});

test("dialog semantics: Esc closes the slot dialog", async ({ page }) => {
  test.setTimeout(120_000);
  test.skip(!process.env.PW_PROVIDER_EMAIL, "provider credentials not seeded");
  await page.goto("/login");
  await page.fill("#email", process.env.PW_PROVIDER_EMAIL!);
  await page.fill("#password", process.env.PW_PROVIDER_PASS!);
  await page.getByRole("button", { name: /log ?in|sign in/i }).click();
  await expect(page).toHaveURL(/\/provider/);
  await page.goto("/provider/availability");
  await page
    .getByRole("button", { name: /new slot/i })
    .first()
    .click({ timeout: 45_000 });
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).not.toBeVisible();
});

test("focus indicators are visible on primary controls", async ({ page }) => {
  await page.goto("/login");
  const button = page.getByRole("button", { name: /log ?in|sign in/i });
  await button.focus();
  const styles = await button.evaluate((el) => {
    const s = getComputedStyle(el);
    return {
      outline: s.outlineStyle,
      outlineWidth: s.outlineWidth,
      boxShadow: s.boxShadow,
    };
  });
  const hasIndicator =
    styles.outline !== "none" ||
    (styles.boxShadow && styles.boxShadow !== "none");
  expect(
    hasIndicator,
    "primary button must show a visible focus indicator",
  ).toBeTruthy();
});
