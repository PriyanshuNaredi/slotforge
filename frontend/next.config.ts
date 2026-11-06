import path from "node:path";
import type { NextConfig } from "next";

/**
 * The Go API has no CORS middleware, so all client traffic is proxied
 * through the same origin. The API origin is server-side only.
 */
const apiOrigin = process.env.API_PROXY_ORIGIN ?? "http://127.0.0.1:8080";

// The rewrite destination below is baked in at build time. A production build
// without API_PROXY_ORIGIN silently targets localhost and 500s on every API
// request, so flag it loudly where the deploy logs will show it.
if (process.env.NODE_ENV === "production" && !process.env.API_PROXY_ORIGIN) {
  console.warn(
    "[next.config] API_PROXY_ORIGIN is not set — API rewrites will target http://127.0.0.1:8080 and fail in production.",
  );
}

const nextConfig: NextConfig = {
  // Keep Turbopack rooted here — a stray lockfile above the repo confuses it.
  turbopack: {
    root: path.join(import.meta.dirname),
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: `${apiOrigin}/api/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
