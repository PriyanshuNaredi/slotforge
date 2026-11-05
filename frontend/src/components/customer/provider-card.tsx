"use client";

import Link from "next/link";
import { GlobeIcon, UserRoundIcon } from "lucide-react";

import type { PublicProvider } from "@/lib/api/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export function ProviderCard({ provider }: { provider: PublicProvider }) {
  return (
    <Card className="flex h-full flex-col justify-between transition-shadow hover:shadow-md">
      <CardHeader className="pb-3">
        <div className="flex items-start gap-2">
          <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-brand-soft text-brand-strong">
            <UserRoundIcon className="size-4" aria-hidden="true" />
          </span>
          <CardTitle className="truncate text-sm font-semibold">
            {provider.display_name}
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col justify-between gap-3">
        <p className="line-clamp-3 text-sm text-muted-foreground">
          {provider.bio?.trim() || "This provider has not added a bio yet."}
        </p>
        <div className="flex items-center justify-between gap-2">
          <p className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
            <GlobeIcon className="size-3.5 shrink-0" aria-hidden="true" />
            <span className="truncate">{provider.timezone}</span>
          </p>
          <Link
            href={`/dashboard/discover/providers/${provider.id}`}
            className="shrink-0 text-xs font-medium text-brand-strong underline-offset-4 hover:underline"
          >
            View profile
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}
