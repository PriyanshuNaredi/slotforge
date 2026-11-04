"use client";

import { useEffect, useRef, useState } from "react";

import { getAvailableSlot } from "@/lib/api/endpoints";
import type { AvailableSlot } from "@/lib/api/types";

export type SlotDetailMap = Record<string, AvailableSlot | null>;

/**
 * Hydrate bookings with their slot details. Bookings carry only slot_id;
 * slot lookup 404s once a slot is closed or in the past — those map to
 * null and the UI degrades to status-only display.
 *
 * The slot cache is cumulative across keys (data is immutable per slot id),
 * so consumers render per-row fallbacks for ids not yet hydrated.
 */
export function useSlotDetails(slotIds: string[]): {
  slots: SlotDetailMap;
} {
  const key = [...new Set(slotIds)].sort().join(",");

  const [slots, setSlots] = useState<SlotDetailMap>({});
  const cache = useRef<SlotDetailMap>({});

  useEffect(() => {
    if (key === "") return;

    const missing = key.split(",").filter((id) => !(id in cache.current));
    if (missing.length === 0) return;

    let cancelled = false;

    Promise.all(
      missing.map((id) =>
        getAvailableSlot(id).then(
          (slot) => {
            cache.current[id] = slot;
          },
          () => {
            cache.current[id] = null;
          },
        ),
      ),
    ).then(() => {
      if (!cancelled) setSlots({ ...cache.current });
    });

    return () => {
      cancelled = true;
    };
  }, [key]);

  return { slots };
}
