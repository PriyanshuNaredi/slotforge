// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { NotificationsContent } from "./notifications-content";
import type { Notification } from "@/lib/api/types";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

const listNotifications = vi.fn();
const markNotificationRead = vi.fn();
const markAllNotificationsRead = vi.fn();

vi.mock("@/lib/api/endpoints", () => ({
  listNotifications: (...args: unknown[]) => listNotifications(...args),
  markNotificationRead: (...args: unknown[]) => markNotificationRead(...args),
  markAllNotificationsRead: (...args: unknown[]) => markAllNotificationsRead(...args),
}));

function makeNotification(overrides: Partial<Notification>): Notification {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    user_id: "u1",
    type: "booking_confirmed",
    title: "Booking confirmed",
    message: "Your booking has been confirmed successfully.",
    delivery_status: "sent",
    attempts: 1,
    available_at: "2026-11-03T18:00:00Z",
    created_at: "2026-11-03T18:00:00Z",
    updated_at: "2026-11-03T18:00:00Z",
    ...overrides,
  } as Notification;
}

beforeEach(() => {
  listNotifications.mockReset();
  markNotificationRead.mockReset();
  markAllNotificationsRead.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("NotificationsContent", () => {
  it("renders notifications with read/unread state", async () => {
    listNotifications.mockResolvedValue({
      notifications: [
        makeNotification({ id: "n1", read_at: undefined }),
        makeNotification({ id: "n2", read_at: "2026-11-03T19:00:00Z", title: "Old news" }),
      ],
      limit: 20,
      offset: 0,
      total: 2,
    });

    render(<NotificationsContent title="Notifications" description="desc" />);

    expect(await screen.findByText("Booking confirmed")).toBeDefined();
    expect(screen.getByText("Old news")).toBeDefined();
    // Unread row offers its "Mark read" button; read row does not.
    // (aria-label: Mark "<title>" as read — distinct from "Mark all as read".)
    expect(screen.getAllByRole("button", { name: /^mark "/i })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Mark all as read" })).toBeDefined();
  });

  it("marks one notification as read and refreshes the badge event", async () => {
    markNotificationRead.mockResolvedValue(
      makeNotification({ id: "n1", read_at: "2026-11-03T19:00:00Z" }),
    );
    listNotifications
      .mockResolvedValueOnce({
        notifications: [makeNotification({ id: "n1" })],
        limit: 20,
        offset: 0,
        total: 1,
      })
      .mockResolvedValue({
        notifications: [makeNotification({ id: "n1", read_at: "2026-11-03T19:00:00Z" })],
        limit: 20,
        offset: 0,
        total: 1,
      });

    const user = userEvent.setup();
    const events: string[] = [];
    window.addEventListener("sf:notifications-changed", () => events.push("changed"));

    render(<NotificationsContent title="Notifications" description="desc" />);
    await screen.findByText("Booking confirmed");

    await user.click(screen.getByRole("button", { name: /^mark "/i }));

    await waitFor(() => {
      expect(markNotificationRead).toHaveBeenCalledWith("n1");
      expect(events).toContain("changed");
    });
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /^mark "/i })).toBeNull();
    });
  });

  it("marks all as read", async () => {
    // First render: two unread notifications. After the action: all read.
    listNotifications
      .mockResolvedValueOnce({
        notifications: [
          makeNotification({ id: "n1" }),
          makeNotification({ id: "n2", type: "new_booking", title: "New booking received" }),
        ],
        limit: 20,
        offset: 0,
        total: 2,
      })
      .mockResolvedValue({
        notifications: [
          makeNotification({ id: "n1", read_at: "2026-11-03T19:00:00Z" }),
          makeNotification({
            id: "n2",
            type: "new_booking",
            title: "New booking received",
            read_at: "2026-11-03T19:00:00Z",
          }),
        ],
        limit: 20,
        offset: 0,
        total: 2,
      });
    markAllNotificationsRead.mockResolvedValue({ updated_count: 2 });

    const user = userEvent.setup();
    render(<NotificationsContent title="Notifications" description="desc" />);
    await screen.findByText("New booking received");

    await user.click(screen.getByRole("button", { name: "Mark all as read" }));

    await waitFor(() => {
      expect(markAllNotificationsRead).toHaveBeenCalledOnce();
    });
  });

  it("shows the empty state when there are no notifications", async () => {
    listNotifications.mockResolvedValue({
      notifications: [],
      limit: 20,
      offset: 0,
      total: 0,
    });

    render(<NotificationsContent title="Notifications" description="desc" />);

    expect(await screen.findByText("You're all caught up")).toBeDefined();
  });

  it("shows the error state with a retry", async () => {
    listNotifications
      .mockRejectedValueOnce(new Error("could not retrieve notifications"))
      .mockResolvedValue({
        notifications: [makeNotification({ id: "n1" })],
        limit: 20,
        offset: 0,
        total: 1,
      });

    const user = userEvent.setup();
    render(<NotificationsContent title="Notifications" description="desc" />);

    expect(await screen.findByText("Couldn't load notifications")).toBeDefined();

    await user.click(screen.getByText("Try again"));
    expect(await screen.findByText("Booking confirmed")).toBeDefined();
  });
});
