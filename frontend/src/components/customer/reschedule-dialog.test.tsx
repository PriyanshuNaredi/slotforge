// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RescheduleDialog } from "./reschedule-dialog";
import { ApiError } from "@/lib/api/client";
import type { AvailableSlot, Booking } from "@/lib/api/types";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn(), refresh: vi.fn() }),
}));

const searchAvailableSlots = vi.fn();
const rescheduleBooking = vi.fn();

vi.mock("@/lib/api/endpoints", () => ({
  searchAvailableSlots: (...args: unknown[]) => searchAvailableSlots(...args),
  rescheduleBooking: (...args: unknown[]) => rescheduleBooking(...args),
}));

function makeBooking(): Booking {
  return {
    id: "b1",
    slot_id: "current-slot",
    customer_id: "c1",
    status: "confirmed",
    created_at: "2026-11-01T00:00:00Z",
    updated_at: "2026-11-01T00:00:00Z",
  };
}

function makeCurrentSlot(): AvailableSlot {
  return {
    id: "current-slot",
    provider_id: "p1",
    provider_name: "Dr. Demo",
    timezone: "America/New_York",
    starts_at: "2030-01-15T14:00:00Z",
    ends_at: "2030-01-15T15:00:00Z",
  };
}

function makeSlot(id: string, startsAt: string): AvailableSlot {
  return {
    id,
    provider_id: "p1",
    provider_name: "Dr. Demo",
    timezone: "America/New_York",
    starts_at: startsAt,
    ends_at: startsAt.replace("09:00:00", "10:00:00"),
  };
}

beforeEach(() => {
  searchAvailableSlots.mockReset();
  rescheduleBooking.mockReset();
  searchAvailableSlots.mockResolvedValue({
    slots: [makeSlot("new-1", "2030-01-20T14:00:00Z"), makeSlot("new-2", "2030-01-22T09:00:00Z")],
    limit: 8,
    offset: 0,
    total: 2,
  });
});

afterEach(() => {
  cleanup();
});

describe("RescheduleDialog", () => {
  it("lists future slots from the same provider, excluding the current slot", async () => {
    render(
      <RescheduleDialog
        open
        booking={makeBooking()}
        currentSlot={makeCurrentSlot()}
        onRescheduled={() => {}}
        onOpenChange={() => {}}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("radio", { name: /Jan 22/ })).toBeDefined();
    });
    expect(searchAvailableSlots).toHaveBeenCalledWith(
      expect.objectContaining({ provider_id: "p1" }),
    );
    // The current slot is filtered out of the replacement options.
    expect(screen.queryByRole("radio", { name: /Jan 15/ })).toBeNull();
  });

  it("shows current vs proposed time and reschedules on confirm", async () => {
    const onRescheduled = vi.fn();
    rescheduleBooking.mockResolvedValue({ id: "b1", status: "confirmed" });
    const user = userEvent.setup();

    render(
      <RescheduleDialog
        open
        booking={makeBooking()}
        currentSlot={makeCurrentSlot()}
        onRescheduled={onRescheduled}
        onOpenChange={() => {}}
      />,
    );

    const pick = await screen.findByRole("radio", { name: /Jan 20/ });
    await user.click(pick);

    // Comparison view: both times visible side by side.
    expect(screen.getByText("Current")).toBeDefined();
    expect(screen.getByText("New time")).toBeDefined();
    expect(screen.getByText("Confirm reschedule")).toBeDefined();

    await user.click(screen.getByText("Confirm reschedule"));

    await waitFor(() => {
      expect(rescheduleBooking).toHaveBeenCalledWith("b1", "new-1");
      expect(onRescheduled).toHaveBeenCalledOnce();
    });
  });

  it("returns to selection with a message on a 409 conflict", async () => {
    rescheduleBooking.mockRejectedValueOnce(
      new ApiError(409, "availability slot is already booked"),
    );
    // After the conflict the list is reloaded with the same (still listed) slots.
    const user = userEvent.setup();

    render(
      <RescheduleDialog
        open
        booking={makeBooking()}
        currentSlot={makeCurrentSlot()}
        onRescheduled={() => {}}
        onOpenChange={() => {}}
      />,
    );

    await user.click(await screen.findByRole("radio", { name: /Jan 20/ }));
    await user.click(screen.getByText("Confirm reschedule"));

    await waitFor(() => {
      expect(
        screen.getByText(/That slot was just taken or is no longer available/i),
      ).toBeDefined();
    });
    // Back on the selection step with refreshed options.
    expect(screen.getByRole("radio", { name: /Jan 20/ })).toBeDefined();
    expect(searchAvailableSlots).toHaveBeenCalledTimes(2);
  });

  it("renders no replacement options when the provider has no future slots", async () => {
    searchAvailableSlots.mockResolvedValue({ slots: [], limit: 8, offset: 0, total: 0 });

    render(
      <RescheduleDialog
        open
        booking={makeBooking()}
        currentSlot={makeCurrentSlot()}
        onRescheduled={() => {}}
        onOpenChange={() => {}}
      />,
    );

    expect(
      await screen.findByText(/No future availability from this provider right now/i),
    ).toBeDefined();
    expect(screen.queryByText("Confirm reschedule")).toBeNull();
  });
});
