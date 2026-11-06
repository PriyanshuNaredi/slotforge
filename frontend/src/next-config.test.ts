import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * The /api/v1 rewrite is the only path from the browser to the Go API, and
 * its destination is resolved when next.config is loaded (i.e. at build time
 * in production). These tests pin the contract so a missing API_PROXY_ORIGIN
 * is caught locally instead of surfacing as 500s in production.
 */

async function loadRewrites() {
  vi.resetModules();
  const config = (await import("../next.config")).default;
  if (!config.rewrites) throw new Error("next.config must define rewrites");
  return config.rewrites();
}

describe("next.config rewrites", () => {
  const original = process.env.API_PROXY_ORIGIN;

  afterEach(() => {
    if (original === undefined) delete process.env.API_PROXY_ORIGIN;
    else process.env.API_PROXY_ORIGIN = original;
  });

  it("forwards /api/v1 to API_PROXY_ORIGIN, preserving the path", async () => {
    process.env.API_PROXY_ORIGIN = "https://slotforge-api.onrender.com";
    const rewrites = await loadRewrites();
    expect(rewrites).toEqual([
      {
        source: "/api/v1/:path*",
        destination: "https://slotforge-api.onrender.com/api/v1/:path*",
      },
    ]);
  });

  it("falls back to the local dev origin when unset", async () => {
    delete process.env.API_PROXY_ORIGIN;
    const rewrites = await loadRewrites();
    expect(rewrites).toEqual([
      {
        source: "/api/v1/:path*",
        destination: "http://127.0.0.1:8080/api/v1/:path*",
      },
    ]);
  });
});
