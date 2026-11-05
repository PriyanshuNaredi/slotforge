import { NextResponse } from "next/server";

import {
  clearSessionCookies,
  readAccessCookie,
  readRefreshCookie,
} from "@/lib/server/auth-cookies";

export async function POST() {
  const refreshToken = await readRefreshCookie();
  const accessToken = await readAccessCookie();

  // Revoke the refresh token server-side before clearing cookies.
  // Cookies are cleared regardless of the backend result, so the user is
  // always logged out of the browser even if the API is unreachable.
  if (refreshToken) {
    const origin = process.env.API_PROXY_ORIGIN ?? "http://127.0.0.1:8080";
    try {
      await fetch(`${origin}/api/v1/auth/logout`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
        },
        body: JSON.stringify({ refresh_token: refreshToken }),
        cache: "no-store",
      });
    } catch {
      // Fall through: cookies must still be cleared.
    }
  }

  await clearSessionCookies();
  return NextResponse.json({ ok: true });
}
