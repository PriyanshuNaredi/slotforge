import Link from "next/link";

import { cn } from "@/lib/utils";

/**
 * The mark: three slots on a board — two navy, one open. The open slot is
 * the pale one. The coral tile reads on both cream and navy surfaces, so the
 * mark itself needs no tone variant; `tone` only affects the wordmark.
 */
export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        "flex size-7 items-center justify-center rounded-md bg-primary text-primary-foreground",
        className,
      )}
      aria-hidden="true"
    >
      <svg viewBox="0 0 16 16" fill="none" className="size-4">
        <rect x="2.5" y="3" width="2.5" height="10" fill="currentColor" />
        <rect x="6.75" y="3" width="2.5" height="10" fill="#f6f0e5" />
        <rect x="11" y="3" width="2.5" height="10" fill="currentColor" />
      </svg>
    </span>
  );
}

export function Logo({
  href = "/",
  className,
  tone = "light",
}: {
  href?: string;
  className?: string;
  tone?: "light" | "inverse";
}) {
  return (
    <Link
      href={href}
      className={cn(
        "inline-flex items-center gap-2 font-semibold tracking-tight",
        tone === "inverse" ? "text-cream" : "text-ink",
        className,
      )}
    >
      <LogoMark />
      <span className="text-[15px]">SlotForge</span>
    </Link>
  );
}
