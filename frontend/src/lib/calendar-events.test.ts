// @vitest-environment node
import { describe, expect, it } from "vitest";

import {
  customerBookingEvents,
  eventStatusLabel,
  providerBookingEvents,
  providerSlotEvents,
} from "./calendar-events";
import type { AvailableSlot, Booking, ProviderBooking, Slot } from "./api/types";

function makeSlot(overrides: Partial<Slot> = {}): Slot {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    provider_id: crypto.randomUUID(),
    starts_at: "2026-11-10T09:00:00Z",
    ends_at: "2026-11-10T10:00:00Z",
    status: "open",
    created_at: "2026-11-01T00:00:00Z",
    updated_at: "2026-11-01T00:00:00Z",
    ...overrides,
  } as Slot;
}

function makeProviderBooking(overrides: Partial<ProviderBooking> = {}): ProviderBooking {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    slot_id: crypto.randomUUID(),
    customer_id: crypto.randomUUID(),
    customer_email: "customer@example.com",
    status: "confirmed",
    slot_starts_at: "2026-11-10T09:00:00Z",
    slot_ends_at: "2026-11-10T10:00:00Z",
    created_at: "2026-11-01T00:00:00Z",
    updated_at: "2026-11-01T00:00:00Z",
    ...overrides,
  } as ProviderBooking;
}

describe("providerSlotEvents", () => {
  it("marks open slots with the ev-open treatment", () => {
    const events = providerSlotEvents([makeSlot()]);
    expect(events[0]?.color).toBe("#F17C69");
    expect(events[0]?.title).toBe("Open slot");
  });

  it("marks closed slots with the ev-closed treatment", () => {
    const events = providerSlotEvents([makeSlot({ status: "closed" })]);
    expect(events[0]?.color).toBe("#9FB2BD");
    expect(events[0]?.title).toBe("Closed slot");
  });

  it("prefixes ids to disambiguate slots from bookings", () => {
    const events = providerSlotEvents([makeSlot({ id: "s1" })]);
    expect(events[0]?.id).toBe("slot:s1");
  });
});

describe("providerBookingEvents", () => {
  it("maps every supported status to a distinct treatment", () => {
    const statuses = ["confirmed", "completed", "pending", "cancelled", "no_show"] as const;
    const events = providerBookingEvents(
      statuses.map((status) => makeProviderBooking({ status })),
    );
    expect(events.map((event) => event.color)).toEqual([
      "#344B43", // confirmed
      "#172B4D", // completed
      "#D97706", // pending
      "#B6C2CB", // cancelled
      "#B3362B", // no_show
    ]);
  });

  it("shows the customer email as the title", () => {
    const events = providerBookingEvents([makeProviderBooking()]);
    expect(events[0]?.title).toBe("customer@example.com");
  });
});

describe("customerBookingEvents", () => {
  const hydrated: Record<string, AvailableSlot> = {
    s1: {
      id: "s1",
      provider_id: "p1",
      provider_name: "Dr. Demo",
      timezone: "America/New_York",
      starts_at: "2026-11-10T09:00:00-05:00",
      ends_at: "2026-11-10T10:00:00-05:00",
    },
  };

  function makeBooking(overrides: Partial<Booking> = {}): Booking {
    return {
      id: overrides.id ?? crypto.randomUUID(),
      slot_id: overrides.slot_id ?? "s1",
      customer_id: crypto.randomUUID(),
      status: "confirmed",
      created_at: "2026-11-01T00:00:00Z",
      updated_at: "2026-11-01T00:00:00Z",
      ...overrides,
    } as Booking;
  }

  it("builds events from hydrated slots with the provider name", () => {
    const events = customerBookingEvents([makeBooking()], hydrated);
    expect(events[0]?.title).toBe("Dr. Demo");
    expect(events[0]?.start).toBe("2026-11-10T09:00:00-05:00");
    expect(events[0]?.color).toBe("#344B43");
  });

  it("skips bookings whose slot details are no longer published", () => {
    const events = customerBookingEvents([makeBooking({ slot_id: "gone" })], hydrated);
    expect(events).toEqual([]);
  });

  it("labels statuses via the shared status metadata", () => {
    expect(eventStatusLabel("no_show")).toBe("No-show");
  });
});
