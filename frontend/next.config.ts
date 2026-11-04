import path from "node:path";
import type { NextConfig } from "next";

/**
 * The Go API has no CORS middleware, so all client traffic is proxied
 * through the same origin. The API origin is server-side only.
 */
const apiOrigin = process.env.API_PROXY_ORIGIN ?? "http://127.0.0.1:8080";

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
