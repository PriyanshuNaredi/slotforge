import { NextResponse } from "next/server";

import type { Role } from "@/lib/api/types";
import { decodeJwtPayload } from "@/lib/decode-jwt";
import { readAccessCookie, readUserCookie } from "@/lib/server/auth-cookies";

/**
 * Returns the current session user: role from the (unverified, display-only)
 * access-cookie decode, email from the sf_user display cookie. The backend
 * verifies the token on every API call — this endpoint only drives UI.
 */
export async function GET() {
  const token = await readAccessCookie();
  const payload = decodeJwtPayload(token);

  if (!token || !payload?.sub || !payload.role) {
    return NextResponse.json({ user: null });
  }

  const display = await readUserCookie();

  return NextResponse.json({
    user: {
      id: payload.sub,
      role: payload.role as Role,
      email: display?.email ?? null,
    },
    expiresAt: payload.exp ? payload.exp * 1000 : null,
  });
}
