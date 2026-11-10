"use client";

import Link from "next/link";
import { ArrowRightIcon, GlobeIcon } from "lucide-react";

import type { PublicProvider } from "@/lib/api/types";
import { Card, CardContent } from "@/components/ui/card";

/** Directory entry — a ruled catalog row in card form, no icon tiles. */
export function ProviderCard({ provider }: { provider: PublicProvider }) {
  return (
    <Card className="flex h-full flex-col justify-between transition-colors hover:border-rule-strong">
      <CardContent>
        <h3 className="font-display text-lg font-semibold tracking-tight">
          {provider.display_name}
        </h3>
        <p className="mt-2 line-clamp-3 text-sm leading-relaxed text-ink-soft">
          {provider.bio?.trim() || "This provider has not added a bio yet."}
        </p>
      </CardContent>
      <CardContent className="mt-auto flex items-center justify-between gap-2 border-t border-rule pt-3">
        <p className="flex min-w-0 items-center gap-1.5 text-xs text-ink-soft">
          <GlobeIcon className="size-3.5 shrink-0" aria-hidden="true" />
          <span className="truncate">{provider.timezone}</span>
        </p>
        <Link
          href={`/dashboard/discover/providers/${provider.id}`}
          className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-coral-strong underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-coral"
        >
          View profile
          <ArrowRightIcon className="size-3.5" aria-hidden="true" />
        </Link>
      </CardContent>
    </Card>
  );
}
