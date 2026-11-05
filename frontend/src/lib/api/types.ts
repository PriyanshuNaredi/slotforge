/**
 * API DTOs — mirrors the Go backend JSON contracts exactly.
 * Source of truth: internal/auth/model.go, internal/bookings/*, internal/providers/*.
 */

export type Role = "customer" | "provider";

export interface User {
  id: string;
  email: string;
  role: Role;
  is_active: boolean;
  created_at: string;
}

/** Response from /auth/register, /auth/login, /auth/refresh. */
export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  token_type: "Bearer";
  expires_in: number;
  user: User;
}

export interface RegisterInput {
  email: string;
  password: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

// ---------------------------------------------------------------------------
// Slots (public discovery + provider availability)
// ---------------------------------------------------------------------------

export type SlotStatus = "open" | "closed";

/** Public slot as returned by GET /slots (includes provider info). */
export interface AvailableSlot {
  id: string;
  provider_id: string;
  provider_name: string;
  timezone: string;
  starts_at: string;
  ends_at: string;
}

export interface SlotSearchResult {
  slots: AvailableSlot[];
  limit: number;
  offset: number;
  total: number;
}

export interface SlotSearchParams {
  provider_id?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

/** Slot as owned/managed by a provider. */
export interface Slot {
  id: string;
  provider_id: string;
  starts_at: string;
  ends_at: string;
  status: SlotStatus;
  created_at: string;
  updated_at: string;
}

export interface CreateSlotInput {
  starts_at: string;
  ends_at: string;
}

// ---------------------------------------------------------------------------
// Bookings (customer)
// ---------------------------------------------------------------------------

export type BookingStatus =
  | "pending"
  | "confirmed"
  | "cancelled"
  | "completed"
  | "no_show";

export interface Booking {
  id: string;
  slot_id: string;
  customer_id: string;
  status: BookingStatus;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateBookingInput {
  slot_id: string;
  notes?: string;
}

export interface BookingListResult {
  bookings: Booking[];
  limit: number;
  offset: number;
  total: number;
}

export interface BookingListParams {
  status?: BookingStatus;
  limit?: number;
  offset?: number;
}

// ---------------------------------------------------------------------------
// Provider profile
// ---------------------------------------------------------------------------

export interface ProviderProfile {
  user_id: string;
  display_name: string;
  bio?: string;
  timezone: string;
  created_at: string;
  updated_at: string;
}

export interface CreateProfileInput {
  display_name: string;
  bio?: string;
  timezone?: string;
}

// ---------------------------------------------------------------------------
// Public provider discovery (unauthenticated API surface)
// ---------------------------------------------------------------------------

/** Public provider as returned by GET /providers — no user account fields. */
export interface PublicProvider {
  id: string;
  display_name: string;
  bio?: string | null;
  timezone: string;
  created_at: string;
}

export interface ProviderListParams {
  search?: string;
  available_from?: string;
  available_to?: string;
  limit?: number;
  offset?: number;
}

export interface ProviderListResult {
  items: PublicProvider[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Bookings (provider view)
// ---------------------------------------------------------------------------

export interface ProviderBooking {
  id: string;
  slot_id: string;
  customer_id: string;
  customer_email: string;
  status: string;
  notes?: string;
  slot_starts_at: string;
  slot_ends_at: string;
  created_at: string;
  updated_at: string;
}

export interface ProviderBookingListResult {
  bookings: ProviderBooking[];
  limit: number;
  offset: number;
  total: number;
}

export interface ProviderBookingListParams {
  status?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

/** Provider may transition a booking to any of these (never back to pending). */
export type ProviderBookingStatus = "confirmed" | "cancelled" | "completed" | "no_show";

// ---------------------------------------------------------------------------
// Notifications — models exist in the backend (migrations/004) but no HTTP
// routes are wired yet. Types are kept aligned with internal/notifications.
// ---------------------------------------------------------------------------

export type NotificationDeliveryStatus =
  | "pending"
  | "processing"
  | "sent"
  | "failed";

export interface Notification {
  id: string;
  user_id: string;
  booking_id?: string;
  type: string;
  title: string;
  message: string;
  read_at?: string;
  delivery_status: NotificationDeliveryStatus;
  attempts: number;
  available_at: string;
  sent_at?: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface NotificationListResult {
  notifications: Notification[];
  limit: number;
  offset: number;
  total: number;
}

export interface UnreadCountResult {
  unread_count: number;
}

export interface MarkAllReadResult {
  updated_count: number;
}
