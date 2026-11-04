/**
 * Timezone-aware date formatting built on Intl.
 * Slot times are anchored to the provider's timezone (slots.timezone).
 */

const DATE_OPTS = { weekday: "short", month: "short", day: "numeric" } as const;
const FULL_DATE_OPTS = {
  weekday: "long",
  year: "numeric",
  month: "long",
  day: "numeric",
} as const;
const TIME_OPTS = { hour: "numeric", minute: "2-digit" } as const;

function format(date: Date, opts: Intl.DateTimeFormatOptions, timeZone?: string): string {
  try {
    return new Intl.DateTimeFormat("en-US", { ...opts, timeZone }).format(date);
  } catch {
    // Invalid timezone IANA name — fall back to the viewer's locale.
    return new Intl.DateTimeFormat("en-US", opts).format(date);
  }
}

export function formatDate(date: Date, timeZone?: string): string {
  return format(date, DATE_OPTS, timeZone);
}

export function formatFullDate(date: Date, timeZone?: string): string {
  return format(date, FULL_DATE_OPTS, timeZone);
}

export function formatTime(date: Date, timeZone?: string): string {
  return format(date, { ...TIME_OPTS }, timeZone);
}

/** "Wed, Nov 5 · 9:00 AM – 9:30 AM (CST)" style range label. */
export function formatSlotRange(startsAt: string, endsAt: string, timezone?: string): string {
  const start = new Date(startsAt);
  const end = new Date(endsAt);
  const sameDay =
    format(start, DATE_OPTS, timezone) === format(end, DATE_OPTS, timezone);
  const tz = timezone ? timeZoneAbbr(timezone) : "";
  const timePart = `${formatTime(start, timezone)} – ${sameDay ? "" : `${format(end, DATE_OPTS, timezone)}, `}${formatTime(end, timezone)}`;
  return tz ? `${format(start, DATE_OPTS, timezone)} · ${timePart} (${tz})` : `${format(start, DATE_OPTS, timezone)} · ${timePart}`;
}

export function formatDateTime(date: Date, timeZone?: string): string {
  return `${format(date, DATE_OPTS, timeZone)}, ${formatTime(date, timeZone)}`;
}

/** IANA timezone -> short label like "CST" / "GMT+5:30". */
export function timeZoneAbbr(timeZone: string, date = new Date()): string {
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone,
      timeZoneName: "short",
    }).formatToParts(date);
    return parts.find((p) => p.type === "timeZoneName")?.value ?? timeZone;
  } catch {
    return timeZone;
  }
}

/** Duration between two ISO timestamps as "30 min" / "1 h 30 min". */
export function formatDuration(startsAt: string, endsAt: string): string {
  const minutes = Math.round(
    (new Date(endsAt).getTime() - new Date(startsAt).getTime()) / 60_000,
  );
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours} h` : `${hours} h ${rest} min`;
}

/** "in 2 days" / "3 days ago" style relative time. */
export function formatRelative(date: Date): string {
  const diffMs = date.getTime() - Date.now();
  const diffDays = Math.round(diffMs / 86_400_000);
  const rtf = new Intl.RelativeTimeFormat("en-US", { numeric: "auto" });
  if (Math.abs(diffDays) < 1) {
    const diffMinutes = Math.round(diffMs / 60_000);
    if (Math.abs(diffMinutes) < 60) return rtf.format(diffMinutes, "minute");
    return rtf.format(Math.round(diffMinutes / 60), "hour");
  }
  return rtf.format(diffDays, "day");
}

/** ISO (UTC) -> value usable in <input type="datetime-local">, in a tz. */
export function isoToLocalInput(iso: string, timeZone?: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(new Date(iso));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? "00";
  return `${get("year")}-${get("month")}-${get("day")}T${get("hour") === "24" ? "00" : get("hour")}:${get("minute")}`;
}

/** Local input value -> ISO string interpreted in the given timezone. */
export function localInputToIso(value: string, timeZone: string): string {
  // Build the UTC timestamp for the wall-clock time in `timeZone`.
  const [datePart, timePart] = value.split("T");
  const [year, month, day] = datePart.split("-").map(Number);
  const [hour, minute] = timePart.split(":").map(Number);

  // Start with a guess: treat the wall clock as UTC, then correct by the
  // timezone offset at that instant (two passes handle DST edge cases).
  let ts = Date.UTC(year, month - 1, day, hour, minute);
  for (let i = 0; i < 2; i++) {
    const offset = tzOffsetMs(new Date(ts), timeZone);
    ts = Date.UTC(year, month - 1, day, hour, minute) - offset;
  }
  return new Date(ts).toISOString();
}

function tzOffsetMs(date: Date, timeZone: string): number {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value ?? 0);
  const asUtc = Date.UTC(
    get("year"),
    get("month") - 1,
    get("day"),
    get("hour") === 24 ? 0 : get("hour"),
    get("minute"),
    get("second"),
  );
  return asUtc - date.getTime();
}
