# Security Review — SlotForge

Date: 2026-11-07 · Covers Phases 18.4/20.4. Static review + targeted fixes +
verified behavior (production E2E + local integration tests).

## Verified controls

| Control | Status |
|---|---|
| Password policy | min 12 chars, max 72 **bytes** (bcrypt limit enforced before hashing); enforced server-side in `auth.Service.Register` |
| Password storage | bcrypt, default cost (10) |
| JWT | HS256, signing method pinned on parse, `exp` required, issuer `slotforge`, short access TTL (15 m default) |
| Refresh tokens | 256-bit random, SHA-256 hashed at rest, single-use rotation (old token dead immediately), TTL 7 d |
| Logout | Revokes the presented refresh token in the same transaction as issuance-side bookkeeping; idempotent |
| Session isolation | Other sessions unaffected by one session's logout (E2E-verified) |
| Role enforcement | `RequireRoles` middleware; provider/customer routes separated (E2E-verified 403s) |
| Ownership scoping | Foreign-customer actions return 404 (indistinguishable from missing) — E2E-verified |
| SQL injection | All queries parameterized; no string-built SQL found in `git grep` sweep |
| Sensitive data in responses | `PasswordHash` tagged `json:"-"`; public provider endpoints expose only profile fields |
| Sensitive data in logs | Passwords/tokens/emails never logged; access logs carry `user_id` only |
| Security headers | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, HSTS (added Phase 18) |
| Brute force | Per-account login throttle: 10 failures / 5 min → 429 (added Phase 18) |
| CSRF | API is Bearer-token based (no auth cookies) → classic CSRF not applicable; frontend stores tokens client-side (XSS is the relevant threat — see limitations) |
| Input validation | gin binding validators on all inputs; zero-UUIDs rejected 400; malformed UUIDs 400 |
| Dependency posture | `npm audit`: 8 moderate, all in the dev-only eslint toolchain (minimatch). Go deps current (jwt v5.3.1, pgx v5.12.0, gin v1.12.0) |
| Secret scanning | `git grep` patterns (AWS keys, private keys, connection strings, `re_` tokens): clean in tracked files |
| Migration consistency | Local dev + isolated E2E DB both at 6/6 with matching tables; prod at 5 (006 pending approval) |

## Fixes made during this review

1. **Login brute-force throttle** (`internal/auth/service.go`) — previously
   absent; unlimited password guessing was possible.
2. **Security headers** (`internal/logging/security_headers.go`) — previously
   absent.
3. **DB query/idle-transaction timeouts** (`internal/database/postgres.go`) —
   bounds lock-holding sessions (15 s / 30 s).
4. **Readiness schema+migration checks** (`internal/health/handler.go`) — a
   half-migrated database no longer reports ready.

## Known limitations (accepted, documented)

- No server-side rate limiting on non-auth endpoints (public discovery could
  be scraped aggressively; Render's own limits are the only backstop).
- Client-side token storage (frontend) means XSS would expose sessions; the
  frontend uses React with no `dangerouslySetInnerHTML` in the audited paths,
  but a dedicated frontend XSS audit was not in scope.
- Account-lockout DoS: an attacker can deliberately throttle a victim's
  login (10 fails/5 min window). Accepted trade-off of per-account
  throttling; IPs are untrustworthy behind Render's proxy.
- No TOTP/MFA.
- Refresh-token reuse does not trigger family-wide revocation (rotation
  still invalidates the reused token immediately).
