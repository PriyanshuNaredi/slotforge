// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { StatusMenu } from "./status-menu";
import type { ProviderBooking } from "@/lib/api/types";

afterEach(() => {
  cleanup();
});

function makeBooking(status: string): ProviderBooking {
  return {
    id: "b1",
    slot_id: crypto.randomUUID(),
    customer_id: crypto.randomUUID(),
    customer_email: "customer@example.com",
    status,
    slot_starts_at: "2030-01-20T14:00:00Z",
    slot_ends_at: "2030-01-20T15:00:00Z",
    created_at: "2026-11-01T00:00:00Z",
    updated_at: "2026-11-01T00:00:00Z",
  };
}

async function openMenu(status: string) {
  const user = userEvent.setup();
  const onSelect = vi.fn();
  render(<StatusMenu booking={makeBooking(status)} onSelect={onSelect} />);
  await user.click(screen.getByRole("button", { name: "Update status" }));
  return { user, onSelect };
}

describe("StatusMenu (backend transition matrix)", () => {
  it("offers only Confirm and Cancel for pending bookings", async () => {
    const { onSelect } = await openMenu("pending");
    expect(screen.getByRole("menuitem", { name: "Confirmed" })).toBeDefined();
    expect(screen.getByRole("menuitem", { name: "Cancelled" })).toBeDefined();
    expect(screen.queryByRole("menuitem", { name: "Completed" })).toBeNull();
    expect(screen.queryByRole("menuitem", { name: "No-show" })).toBeNull();
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("offers Complete, No-show and Cancel for confirmed bookings", async () => {
    const { user, onSelect } = await openMenu("confirmed");
    expect(screen.queryByRole("menuitem", { name: "Confirmed" })).toBeNull();
    expect(screen.getByRole("menuitem", { name: "Completed" })).toBeDefined();
    expect(screen.getByRole("menuitem", { name: "No-show" })).toBeDefined();
    expect(screen.getByRole("menuitem", { name: "Cancelled" })).toBeDefined();

    await user.click(screen.getByRole("menuitem", { name: "No-show" }));
    expect(onSelect).toHaveBeenCalledWith("no_show");
  });

  it("renders no actions for terminal statuses", () => {
    for (const status of ["completed", "cancelled", "no_show"]) {
      const { container } = render(<StatusMenu booking={makeBooking(status)} onSelect={() => {}} />);
      expect(screen.queryByRole("button", { name: "Update status" })).toBeNull();
      expect(container.querySelector("button")).toBeNull();
      cleanup();
    }
  });
});
