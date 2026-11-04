"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { toErrorMessage } from "@/lib/api/client";

interface AsyncState<T> {
  data: T | null;
  error: string | null;
  /** requestKey ("key#nonce") of the last settled request. */
  settled: string | null;
  /** key of the last settled request, for same-key stale reads. */
  settledKey: string | null;
}

export interface AsyncData<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

/**
 * Canonical client-side data fetching: one request per key, stale data kept
 * visible while reloading the same key, cleared when the key changes.
 *
 * Pass key=null to skip fetching (e.g. waiting for a param or a parent
 * request). The fetcher is read from a ref, so callers can inline it.
 */
export function useAsyncData<T>(
  key: string | null,
  fetcher: () => Promise<T>,
): AsyncData<T> {
  const [state, setState] = useState<AsyncState<T>>({
    data: null,
    error: null,
    settled: null,
    settledKey: null,
  });
  const [nonce, setNonce] = useState(0);

  const fetcherRef = useRef(fetcher);
  useEffect(() => {
    fetcherRef.current = fetcher;
  });

  const requestKey = `${key ?? "none"}#${nonce}`;
  const settled = state.settled === requestKey;

  useEffect(() => {
    if (key === null) return;
    let cancelled = false;
    fetcherRef.current().then(
      (data) => {
        if (!cancelled) {
          setState({ data, error: null, settled: requestKey, settledKey: key });
        }
      },
      (err) => {
        if (!cancelled) {
          setState({
            data: null,
            error: toErrorMessage(err),
            settled: requestKey,
            settledKey: key,
          });
        }
      },
    );
    return () => {
      cancelled = true;
    };
  }, [requestKey, key]);

  const reload = useCallback(() => setNonce((value) => value + 1), []);

  if (key === null) {
    return { data: null, loading: false, error: null, reload };
  }

  return {
    data: state.settledKey === key ? state.data : null,
    loading: !settled,
    error: settled ? state.error : null,
    reload,
  };
}
