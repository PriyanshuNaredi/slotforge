import { NextResponse } from "next/server";

import { clearSessionCookies } from "@/lib/server/auth-cookies";

export async function POST() {
  await clearSessionCookies();
  return NextResponse.json({ ok: true });
}
