import { NextResponse } from "next/server";

import { brokerAuth } from "@/lib/server/auth-cookies";

export async function POST(request: Request) {
  let body: { email?: string; password?: string };
  try {
    body = (await request.json()) as { email?: string; password?: string };
  } catch {
    return NextResponse.json({ error: "Invalid request body" }, { status: 400 });
  }

  if (!body.email || !body.password) {
    return NextResponse.json(
      { error: "Email and password are required" },
      { status: 400 },
    );
  }

  const result = await brokerAuth("/auth/login", {
    email: body.email,
    password: body.password,
  });

  if (!result.ok) {
    return NextResponse.json(
      { error: result.error },
      { status: result.status },
    );
  }

  return NextResponse.json({
    user: result.payload!.user,
    expires_in: result.payload!.expires_in,
  });
}
