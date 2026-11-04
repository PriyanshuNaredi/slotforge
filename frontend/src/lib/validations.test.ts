import { describe, expect, it } from "vitest";

import { createSlotSchema, loginSchema, registerSchema } from "./validations";

describe("registerSchema", () => {
  it("accepts a valid registration", () => {
    const result = registerSchema.safeParse({
      email: "user@example.com",
      password: "longenoughpass1",
      confirmPassword: "longenoughpass1",
    });
    expect(result.success).toBe(true);
  });

  it("rejects passwords under 12 characters (backend rule)", () => {
    const result = registerSchema.safeParse({
      email: "user@example.com",
      password: "shortpass1",
      confirmPassword: "shortpass1",
    });
    expect(result.success).toBe(false);
  });

  it("rejects mismatched confirmation", () => {
    const result = registerSchema.safeParse({
      email: "user@example.com",
      password: "longenoughpass1",
      confirmPassword: "differentpass12",
    });
    expect(result.success).toBe(false);
  });

  it("rejects invalid emails", () => {
    const result = loginSchema.safeParse({ email: "not-an-email", password: "x" });
    expect(result.success).toBe(false);
  });
});

describe("createSlotSchema", () => {
  it("accepts end after start", () => {
    const result = createSlotSchema.safeParse({
      date: "2030-01-15",
      startTime: "09:00",
      endTime: "10:00",
    });
    expect(result.success).toBe(true);
  });

  it("rejects end before start", () => {
    const result = createSlotSchema.safeParse({
      date: "2030-01-15",
      startTime: "10:00",
      endTime: "09:00",
    });
    expect(result.success).toBe(false);
  });

  it("rejects missing fields", () => {
    const result = createSlotSchema.safeParse({ date: "", startTime: "", endTime: "" });
    expect(result.success).toBe(false);
  });
});
