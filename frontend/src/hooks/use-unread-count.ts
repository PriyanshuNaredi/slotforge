"use client";

import { useCallback, useEffect, useState } from "react";

import { getUnreadCount } from "@/lib/api/endpoints";

/**
 * Event fired by notification mutations so every mounted nav badge
 * refreshes its unread count without a polling loop.
 */
export const NOTIFICATIONS_CHANGED_EVENT = "sf:notifications-changed";

export function notifyNotificationsChanged(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(NOTIFICATIONS_CHANGED_EVENT));
  }
}

/** Unread notification count for the signed-in user (any role). */
export function useUnreadCount(): { count: number; refresh: () => void } {
  const [count, setCount] = useState(0);
  const [nonce, setNonce] = useState(0);

  const refresh = useCallback(() => setNonce((value) => value + 1), []);

  useEffect(() => {
    let cancelled = false;
    getUnreadCount().then(
      (result) => {
        if (!cancelled) setCount(result.unread_count);
      },
      () => {
        // Nav badges degrade to hidden on failure; the page itself surfaces errors.
        if (!cancelled) setCount(0);
      },
    );
    return () => {
      cancelled = true;
    };
  }, [nonce]);

  useEffect(() => {
    window.addEventListener(NOTIFICATIONS_CHANGED_EVENT, refresh);
    return () => window.removeEventListener(NOTIFICATIONS_CHANGED_EVENT, refresh);
  }, [refresh]);

  // Server-side events (e.g. provider status changes) don't fire the local
  // event, so poll quietly: cheap indexed count, slow cadence.
  useEffect(() => {
    const timer = window.setInterval(refresh, 60_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  return { count, refresh };
}
