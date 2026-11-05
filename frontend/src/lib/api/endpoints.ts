import {
  apiGet,
  apiPatch,
  apiPost,
} from "./client";
import type {
  AvailableSlot,
  Booking,
  BookingListParams,
  BookingListResult,
  CreateBookingInput,
  CreateProfileInput,
  CreateSlotInput,
  MarkAllReadResult,
  Notification,
  NotificationListResult,
  UnreadCountResult,
  ProviderBooking,
  ProviderBookingListParams,
  ProviderBookingListResult,
  ProviderBookingStatus,
  ProviderListParams,
  ProviderListResult,
  ProviderProfile,
  PublicProvider,
  Slot,
  SlotSearchParams,
  SlotSearchResult,
} from "./types";

function toQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") search.set(key, String(value));
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

// ---------------------------------------------------------------------------
// Public availability discovery
// ---------------------------------------------------------------------------

export function searchAvailableSlots(params: SlotSearchParams): Promise<SlotSearchResult> {
  return apiGet<SlotSearchResult>(
    `/slots${toQuery({
      provider_id: params.provider_id,
      from: params.from,
      to: params.to,
      limit: params.limit,
      offset: params.offset,
    })}`,
  );
}

export function getAvailableSlot(slotId: string): Promise<AvailableSlot> {
  return apiGet<AvailableSlot>(`/slots/${slotId}`);
}

// ---------------------------------------------------------------------------
// Public provider discovery
// ---------------------------------------------------------------------------

export function searchProviders(params: ProviderListParams): Promise<ProviderListResult> {
  return apiGet<ProviderListResult>(
    `/providers${toQuery({
      search: params.search,
      available_from: params.available_from,
      available_to: params.available_to,
      limit: params.limit,
      offset: params.offset,
    })}`,
  );
}

export function getPublicProvider(providerId: string): Promise<PublicProvider> {
  return apiGet<PublicProvider>(`/providers/${providerId}`);
}

// ---------------------------------------------------------------------------
// Customer bookings
// ---------------------------------------------------------------------------

export function createBooking(input: CreateBookingInput): Promise<Booking> {
  return apiPost<Booking>("/bookings", input);
}

export function listCustomerBookings(params: BookingListParams): Promise<BookingListResult> {
  return apiGet<BookingListResult>(
    `/bookings${toQuery({
      status: params.status,
      limit: params.limit,
      offset: params.offset,
    })}`,
  );
}

export function getBooking(bookingId: string): Promise<Booking> {
  return apiGet<Booking>(`/bookings/${bookingId}`);
}

export function cancelBooking(bookingId: string): Promise<Booking> {
  return apiPatch<Booking>(`/bookings/${bookingId}/cancel`);
}

/** Move a confirmed booking to a new slot with the same provider. */
export function rescheduleBooking(bookingId: string, slotId: string): Promise<Booking> {
  return apiPatch<Booking>(`/bookings/${bookingId}/reschedule`, { slot_id: slotId });
}

// ---------------------------------------------------------------------------
// Provider profile
// ---------------------------------------------------------------------------

export function createProviderProfile(input: CreateProfileInput): Promise<ProviderProfile> {
  return apiPost<ProviderProfile>("/providers/profile", input);
}

export function getMyProviderProfile(): Promise<ProviderProfile> {
  return apiGet<ProviderProfile>("/providers/profile/me");
}

// ---------------------------------------------------------------------------
// Provider availability slots
// ---------------------------------------------------------------------------

export function createProviderSlot(input: CreateSlotInput): Promise<Slot> {
  return apiPost<Slot>("/providers/slots", input);
}

export function listProviderSlots(): Promise<{ slots: Slot[] }> {
  return apiGet<{ slots: Slot[] }>("/providers/slots");
}

export function closeProviderSlot(slotId: string): Promise<{ message: string }> {
  return apiPatch<{ message: string }>(`/providers/slots/${slotId}/close`);
}

// ---------------------------------------------------------------------------
// Provider bookings
// ---------------------------------------------------------------------------

export function listProviderBookings(
  params: ProviderBookingListParams,
): Promise<ProviderBookingListResult> {
  return apiGet<ProviderBookingListResult>(
    `/providers/bookings${toQuery({
      status: params.status,
      from: params.from,
      to: params.to,
      limit: params.limit,
      offset: params.offset,
    })}`,
  );
}

export function updateProviderBookingStatus(
  bookingId: string,
  status: ProviderBookingStatus,
): Promise<ProviderBooking> {
  return apiPatch<ProviderBooking>(`/providers/bookings/${bookingId}/status`, { status });
}

// ---------------------------------------------------------------------------
// Notifications (any authenticated role; backend filters by user id)
// ---------------------------------------------------------------------------

export function listNotifications(limit = 20, offset = 0): Promise<NotificationListResult> {
  return apiGet<NotificationListResult>(
    `/notifications${toQuery({ limit, offset })}`,
  );
}

export function getUnreadCount(): Promise<UnreadCountResult> {
  return apiGet<UnreadCountResult>("/notifications/unread-count");
}

export function markNotificationRead(notificationId: string): Promise<Notification> {
  return apiPatch<Notification>(`/notifications/${notificationId}/read`);
}

export function markAllNotificationsRead(): Promise<MarkAllReadResult> {
  return apiPatch<MarkAllReadResult>("/notifications/read-all");
}

// Re-export common types for convenience of UI modules.
export type {
  Booking,
  AvailableSlot,
  Slot,
  ProviderProfile,
  ProviderBooking,
  PublicProvider,
};
