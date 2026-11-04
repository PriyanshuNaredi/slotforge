/**
 * Decode a JWT payload without verification.
 * Used ONLY for UX (routing, display) — the backend verifies every token
 * on every request, so the browser is never trusted for authorization.
 */
export interface JwtPayload {
  sub?: string;
  role?: string;
  exp?: number;
  iss?: string;
}

export function decodeJwtPayload(token: string | undefined | null): JwtPayload | null {
  if (!token) return null;
  const parts = token.split(".");
  if (parts.length !== 3) return null;
  try {
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const json = atob(base64);
    return JSON.parse(json) as JwtPayload;
  } catch {
    return null;
  }
}
