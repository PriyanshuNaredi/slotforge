import { NextResponse, type NextRequest } from "next/server";

import { decodeJwtPayload } from "@/lib/decode-jwt";

const ACCESS_COOKIE = "sf_access";

function homeForRole(role: string | undefined): string {
  return role === "provider" ? "/provider" : "/dashboard";
}

export default function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const accessToken = request.cookies.get(ACCESS_COOKIE)?.value;
  const role = decodeJwtPayload(accessToken)?.role as string | undefined;

  // 1. API proxy requests: attach the bearer token from the HttpOnly cookie.
  if (pathname.startsWith("/api/v1")) {
    if (!accessToken) return NextResponse.next();
    const headers = new Headers(request.headers);
    headers.set("Authorization", `Bearer ${accessToken}`);
    return NextResponse.next({ request: { headers } });
  }

  // 2. Auth pages: signed-in users go straight to their dashboard.
  if (pathname === "/login" || pathname === "/register") {
    if (accessToken) {
      return NextResponse.redirect(new URL(homeForRole(role), request.url));
    }
    return NextResponse.next();
  }

  // 3. Protected areas: require a session, then gate by role.
  const requiredRole = pathname.startsWith("/provider") ? "provider" : "customer";

  if (!accessToken) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("next", pathname + search);
    return NextResponse.redirect(loginUrl);
  }

  if (role !== requiredRole) {
    return NextResponse.redirect(new URL(homeForRole(role), request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: [
    "/api/v1/:path*",
    "/dashboard/:path*",
    "/provider/:path*",
    "/login",
    "/register",
  ],
};
