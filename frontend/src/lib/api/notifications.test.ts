// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getUnreadCount,
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
} from "./endpoints";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("notification endpoints (backend contract)", () => {
  it("lists notifications with pagination params", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, {
        notifications: [
          {
            id: "n1",
            user_id: "u1",
            booking_id: "b1",
            type: "booking_confirmed",
            title: "Booking confirmed",
            message: "Your booking has been confirmed successfully.",
            delivery_status: "sent",
            attempts: 1,
            available_at: "2026-11-03T18:00:00Z",
            created_at: "2026-11-03T18:00:00Z",
            updated_at: "2026-11-03T18:00:00Z",
          },
        ],
        limit: 20,
        offset: 20,
        total: 41,
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await listNotifications(20, 20);

    expect(result.total).toBe(41);
    expect(result.notifications[0]?.type).toBe("booking_confirmed");
    expect(result.notifications[0]?.read_at).toBeUndefined();
    const [url] = fetchMock.mock.calls[0] as unknown as [string];
    expect(url).toBe("/api/v1/notifications?limit=20&offset=20");
  });

  it("fetches the unread count", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, { unread_count: 3 }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await getUnreadCount();

    expect(result.unread_count).toBe(3);
    const [url] = fetchMock.mock.calls[0] as unknown as [string];
    expect(url).toBe("/api/v1/notifications/unread-count");
  });

  it("marks one notification as read via PATCH", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse(200, {
        id: "n1",
        user_id: "u1",
        type: "booking_confirmed",
        title: "Booking confirmed",
        message: "done",
        delivery_status: "sent",
        attempts: 1,
        available_at: "2026-11-03T18:00:00Z",
        read_at: "2026-11-03T19:00:00Z",
        created_at: "2026-11-03T18:00:00Z",
        updated_at: "2026-11-03T19:00:00Z",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await markNotificationRead("n1");

    expect(result.read_at).toBe("2026-11-03T19:00:00Z");
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/notifications/n1/read");
    expect(init.method).toBe("PATCH");
  });

  it("marks all as read and reports the updated count", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, { updated_count: 7 }));
    vi.stubGlobal("fetch", fetchMock);

    const result = await markAllNotificationsRead();

    expect(result.updated_count).toBe(7);
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("/api/v1/notifications/read-all");
    expect(init.method).toBe("PATCH");
  });

  it("surfaces backend errors as ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse(400, { error: "invalid limit" })),
    );

    await expect(listNotifications(500, 0)).rejects.toMatchObject({
      status: 400,
      message: "invalid limit",
    });
  });
});
