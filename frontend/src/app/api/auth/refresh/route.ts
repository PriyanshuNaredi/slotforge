import { NextResponse } from "next/server";

import { brokerAuth, readRefreshCookie } from "@/lib/server/auth-cookies";

export async function POST() {
  const refreshToken = await readRefreshCookie();

  if (!refreshToken) {
    return NextResponse.json(
      { error: "No active session" },
      { status: 401 },
    );
  }

  const result = await brokerAuth("/auth/refresh", {
    refresh_token: refreshToken,
  });

  if (!result.ok) {
    return NextResponse.json({ error: result.error }, { status: result.status });
  }

  return NextResponse.json({
    user: result.payload!.user,
    expires_in: result.payload!.expires_in,
  });
}
