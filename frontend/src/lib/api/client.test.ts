// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError, apiGet, toErrorMessage } from "./client";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("apiGet error mapping", () => {
  it("maps backend error bodies to ApiError with status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(409, { error: "availability slot is already booked" })),
    );

    const error = await apiGet("/slots").catch((err) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(409);
    expect((error as ApiError).message).toBe("availability slot is already booked");
  });

  it("falls back to a generic message on non-JSON errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("gateway timeout", { status: 502 })),
    );

    const error = await apiGet("/slots").catch((err) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).message).toContain("502");
  });
});

describe("401 refresh flow", () => {
  it("retries once after a successful refresh", async () => {
    const fetchMock = vi
      .fn()
      // First API call: expired token.
      .mockResolvedValueOnce(jsonResponse(401, { error: "invalid or expired access token" }))
      // Refresh call succeeds.
      .mockResolvedValueOnce(jsonResponse(200, { user: { id: "u", role: "customer", email: null }, expires_in: 900 }))
      // Retried API call succeeds.
      .mockResolvedValueOnce(jsonResponse(200, { slots: [], limit: 20, offset: 0, total: 0 }));

    vi.stubGlobal("fetch", fetchMock);

    const result = await apiGet<{ slots: unknown[] }>("/slots");
    expect(result.slots).toEqual([]);

    const calls = fetchMock.mock.calls as unknown as [Request | string][];
    expect(calls[1][0]).toBe("/api/auth/refresh");
    expect(calls[2][0]).toBe("/api/v1/slots");
  });

  it("propagates 401 when refresh fails", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(401, { error: "invalid or expired access token" }))
      .mockResolvedValueOnce(jsonResponse(401, { error: "No active session" }));

    vi.stubGlobal("fetch", fetchMock);

    await expect(apiGet("/slots")).rejects.toMatchObject({
      status: 401,
      message: "invalid or expired access token",
    });
  });
});

describe("toErrorMessage", () => {
  it("prefers ApiError messages", () => {
    expect(toErrorMessage(new ApiError(400, "display_name must be set"))).toBe(
      "display_name must be set",
    );
  });

  it("explains network failures", () => {
    expect(toErrorMessage(new TypeError("fetch failed"))).toContain("Cannot reach");
  });
});
