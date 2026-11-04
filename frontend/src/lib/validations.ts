import { z } from "zod";

/**
 * Mirrors backend validation exactly:
 * - auth: email required, password 12..72 (RegisterInput binding)
 * - profile: display_name 2..150
 * - slots: end after start
 */

export const loginSchema = z.object({
  email: z.email("Enter a valid email address"),
  password: z.string().min(1, "Password is required"),
});

export const registerSchema = z
  .object({
    email: z.email("Enter a valid email address"),
    password: z
      .string()
      .min(12, "Password must be at least 12 characters")
      .max(72, "Password must not exceed 72 characters"),
    confirmPassword: z.string(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords do not match",
    path: ["confirmPassword"],
  });

export const profileSchema = z.object({
  display_name: z
    .string()
    .min(2, "Display name must be at least 2 characters")
    .max(150, "Display name must not exceed 150 characters"),
  bio: z
    .string()
    .max(2000, "Bio must not exceed 2000 characters")
    .optional()
    .or(z.literal("")),
  timezone: z.string().min(1, "Timezone is required"),
});

export const createSlotSchema = z
  .object({
    date: z.string().min(1, "Pick a date"),
    startTime: z.string().min(1, "Start time is required"),
    endTime: z.string().min(1, "End time is required"),
  })
  .refine((data) => data.endTime > data.startTime, {
    message: "End time must be after start time",
    path: ["endTime"],
  });

export const bookingNotesSchema = z.object({
  notes: z
    .string()
    .max(1000, "Notes must not exceed 1000 characters")
    .optional()
    .or(z.literal("")),
});

export type LoginValues = z.infer<typeof loginSchema>;
export type RegisterValues = z.infer<typeof registerSchema>;
export type ProfileValues = z.infer<typeof profileSchema>;
export type CreateSlotValues = z.infer<typeof createSlotSchema>;
