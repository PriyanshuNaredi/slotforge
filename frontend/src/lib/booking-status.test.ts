import { describe, expect, it } from "vitest";

import { customerCanCancel, providerCanTransition } from "./booking-status";

describe("provider transitions (mirror of backend rules)", () => {
  it("allows acting on pending and confirmed bookings", () => {
    expect(providerCanTransition("pending")).toBe(true);
    expect(providerCanTransition("confirmed")).toBe(true);
  });

  it("blocks transitions from terminal statuses", () => {
    for (const status of ["cancelled", "completed", "no_show"]) {
      expect(providerCanTransition(status)).toBe(false);
    }
  });
});

describe("customer cancellation (mirror of backend rules)", () => {
  it("only confirmed bookings can be cancelled", () => {
    expect(customerCanCancel("confirmed")).toBe(true);
    expect(customerCanCancel("pending")).toBe(false);
    expect(customerCanCancel("completed")).toBe(false);
    expect(customerCanCancel("cancelled")).toBe(false);
  });
});
