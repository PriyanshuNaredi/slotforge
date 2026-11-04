import { cookies } from "next/headers";
import type { AuthResponse } from "@/lib/api/types";

/**
 * HttpOnly cookie session storage.
 *
 * Tokens are set by Next.js route handlers and attached to proxied API
 * requests by middleware. The backend keeps issuing/verifying tokens —
 * this layer only controls transport security.
 */

export const ACCESS_COOKIE = "sf_access";
export const REFRESH_COOKIE = "sf_refresh";
/** Non-sensitive display info (email) — the JWT carries no email claim. */
export const USER_COOKIE = "sf_user";

const ACCESS_MAX_AGE = 15 * 60; // matches JWT_ACCESS_TTL (15m)
const REFRESH_MAX_AGE = 7 * 24 * 60 * 60; // matches REFRESH_TOKEN_TTL (168h)

interface CookieOptions {
  maxAge: number;
}

function cookieOptions({ maxAge }: CookieOptions) {
  return {
    httpOnly: true,
    sameSite: "lax" as const,
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge,
  };
}

export async function setSessionCookies(auth: AuthResponse): Promise<void> {
  const store = await cookies();
  store.set(ACCESS_COOKIE, auth.access_token, cookieOptions({ maxAge: ACCESS_MAX_AGE }));
  store.set(REFRESH_COOKIE, auth.refresh_token, cookieOptions({ maxAge: REFRESH_MAX_AGE }));
  // Display-only: readable by the client on purpose. Never a credential.
  store.set(
    USER_COOKIE,
    encodeURIComponent(JSON.stringify({ email: auth.user.email, role: auth.user.role })),
    { ...cookieOptions({ maxAge: REFRESH_MAX_AGE }), httpOnly: false },
  );
}

/**
 * Clears the session cookies in the browser only. The backend has no
 * logout/revoke endpoint, so the refresh token remains valid server-side
 * until its natural expiry — a lost device is not fully logged out.
 */
export async function clearSessionCookies(): Promise<void> {
  const store = await cookies();
  store.set(ACCESS_COOKIE, "", { ...cookieOptions({ maxAge: 0 }) });
  store.set(REFRESH_COOKIE, "", { ...cookieOptions({ maxAge: 0 }) });
  store.set(USER_COOKIE, "", { ...cookieOptions({ maxAge: 0 }), httpOnly: false });
}

export async function readUserCookie(): Promise<{ email: string; role: string } | null> {
  const store = await cookies();
  const raw = store.get(USER_COOKIE)?.value;
  if (!raw) return null;
  try {
    const parsed = JSON.parse(decodeURIComponent(raw)) as { email?: string; role?: string };
    if (!parsed.email) return null;
    return { email: parsed.email, role: parsed.role ?? "customer" };
  } catch {
    return null;
  }
}

export async function readAccessCookie(): Promise<string | undefined> {
  const store = await cookies();
  return store.get(ACCESS_COOKIE)?.value;
}

export async function readRefreshCookie(): Promise<string | undefined> {
  const store = await cookies();
  return store.get(REFRESH_COOKIE)?.value;
}

/** Forward a brokered auth call to the backend and persist its tokens. */
export async function brokerAuth(
  path: string,
  body: Record<string, string>,
): Promise<{ ok: boolean; status: number; payload?: AuthResponse; error?: string }> {
  const origin = process.env.API_PROXY_ORIGIN ?? "http://127.0.0.1:8080";
  try {
    const response = await fetch(`${origin}/api/v1${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      cache: "no-store",
    });
    const data = (await response.json().catch(() => ({}))) as
      | AuthResponse
      | { error?: string };

    if (!response.ok) {
      return {
        ok: false,
        status: response.status,
        error: (data as { error?: string }).error ?? "Request failed",
      };
    }

    const auth = data as AuthResponse;
    await setSessionCookies(auth);
    return { ok: true, status: response.status, payload: auth };
  } catch {
    return { ok: false, status: 502, error: "Cannot reach the authentication server" };
  }
}
