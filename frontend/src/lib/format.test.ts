import { describe, expect, it } from "vitest";

import { formatDuration, isoToLocalInput, localInputToIso } from "./format";

describe("formatDuration", () => {
  it("renders minutes under an hour", () => {
    expect(
      formatDuration("2030-01-01T09:00:00Z", "2030-01-01T09:45:00Z"),
    ).toBe("45 min");
  });

  it("renders whole hours", () => {
    expect(
      formatDuration("2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z"),
    ).toBe("2 h");
  });

  it("renders mixed hours and minutes", () => {
    expect(
      formatDuration("2030-01-01T09:00:00Z", "2030-01-01T10:30:00Z"),
    ).toBe("1 h 30 min");
  });
});

describe("timezone round-trip", () => {
  it("converts local input to ISO and back in a fixed-offset zone", () => {
    const tz = "Asia/Kolkata"; // UTC+5:30, no DST
    const iso = localInputToIso("2030-06-15T09:30", tz);
    expect(iso).toBe("2030-06-15T04:00:00.000Z");
    expect(isoToLocalInput(iso, tz)).toBe("2030-06-15T09:30");
  });

  it("handles UTC", () => {
    const iso = localInputToIso("2030-01-01T12:00", "UTC");
    expect(iso).toBe("2030-01-01T12:00:00.000Z");
    expect(isoToLocalInput(iso, "UTC")).toBe("2030-01-01T12:00");
  });

  it("lands on the correct instant across DST (America/New_York)", () => {
    // 2030-03-10 is the DST cutover: 09:00 EST -> 13:00 UTC (EDT starts 07:00 local).
    const iso = localInputToIso("2030-03-10T09:00", "America/New_York");
    const expected = new Date("2030-03-10T09:00:00-04:00").toISOString();
    expect(iso).toBe(expected);
  });
});
