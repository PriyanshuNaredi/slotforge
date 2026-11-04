/**
 * Centralized API client. All requests go to the same-origin Next.js proxy
 * (/api/v1 -> backend via rewrites), which also injects the Authorization
 * header from the HttpOnly cookie (middleware.ts).
 *
 * On 401 the client performs a single-flight refresh through the
 * /api/auth/refresh route handler, then retries the original request once.
 */

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** True when the error is a network failure (backend unreachable). */
export function isNetworkError(error: unknown): boolean {
  return error instanceof TypeError;
}

export function toErrorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof TypeError)
    return "Cannot reach the server. Please try again.";
  if (error instanceof Error) return error.message;
  return "Something went wrong. Please try again.";
}

type ApiFetchInit = Omit<RequestInit, "body"> & { body?: unknown };

async function parseError(response: Response): Promise<ApiError> {
  try {
    const data = (await response.json()) as { error?: string };
    return new ApiError(
      response.status,
      data.error ?? `Request failed (${response.status})`,
    );
  } catch {
    return new ApiError(response.status, `Request failed (${response.status})`);
  }
}

// Single-flight refresh: concurrent 401s share one refresh call.
let refreshPromise: Promise<boolean> | null = null;

export async function refreshSession(): Promise<boolean> {
  refreshPromise ??= (async () => {
    try {
      const response = await fetch("/api/auth/refresh", {
        method: "POST",
        credentials: "same-origin",
      });
      return response.ok;
    } catch {
      return false;
    }
  })();

  const promise = refreshPromise;
  // Release the lock once settled so later 401s can start a fresh refresh.
  void promise.finally(() => {
    if (refreshPromise === promise) refreshPromise = null;
  });

  return promise;
}

async function request<T>(path: string, init: ApiFetchInit, retry: boolean): Promise<T> {
  const isFormData = init.body instanceof FormData;
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    credentials: "same-origin",
    headers: {
      ...(isFormData ? {} : { "Content-Type": "application/json" }),
      ...init.headers,
    },
    body: init.body === undefined ? undefined : isFormData
      ? (init.body as FormData)
      : JSON.stringify(init.body),
  });

  if (response.status === 401 && retry && !path.startsWith("/auth/")) {
    const refreshed = await refreshSession();
    if (refreshed) return request<T>(path, init, false);
  }

  if (!response.ok) throw await parseError(response);

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export function apiGet<T>(path: string): Promise<T> {
  return request<T>(path, { method: "GET" }, true);
}

export function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, { method: "POST", body }, true);
}

export function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, { method: "PATCH", body }, true);
}
