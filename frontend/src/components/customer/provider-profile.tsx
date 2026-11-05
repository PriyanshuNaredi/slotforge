"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import {
  ArrowLeftIcon,
  CalendarCheckIcon,
  FileQuestionIcon,
  GlobeIcon,
  ServerCrashIcon,
  UserRoundIcon,
} from "lucide-react";

import { getPublicProvider } from "@/lib/api/endpoints";
import { ApiError } from "@/lib/api/client";
import type { PublicProvider } from "@/lib/api/types";
import { timeZoneAbbr } from "@/lib/format";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/use-async-data";

type ProfileResult =
  | { provider: PublicProvider; notFound: false }
  | { provider: null; notFound: true };

export function ProviderProfileContent() {
  const params = useParams<{ id: string }>();
  const providerId = params.id ?? null;

  const profile = useAsyncData(providerId, async (): Promise<ProfileResult> => {
    try {
      return { provider: await getPublicProvider(providerId!), notFound: false };
    } catch (error) {
      // An unknown or deactivated provider is a normal empty state, not
      // an error banner.
      if (error instanceof ApiError && error.status === 404) {
        return { provider: null, notFound: true };
      }
      throw error;
    }
  });

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <Button variant="ghost" size="sm" asChild>
        <Link href="/dashboard/discover?tab=providers">
          <ArrowLeftIcon aria-hidden="true" />
          Back to providers
        </Link>
      </Button>

      {profile.error ? (
        <ErrorState
          icon={ServerCrashIcon}
          title="Couldn't load this provider"
          description={profile.error}
          action={
            <Button variant="outline" size="sm" onClick={profile.reload}>
              Try again
            </Button>
          }
        />
      ) : profile.loading ? (
        <div className="rounded-xl border bg-card p-6">
          <div className="flex items-center gap-3">
            <Skeleton className="size-12 rounded-full" />
            <Skeleton className="h-5 w-40" />
          </div>
          <Skeleton className="mt-6 h-3 w-full" />
          <Skeleton className="mt-2 h-3 w-3/4" />
          <Skeleton className="mt-8 h-9 w-44" />
        </div>
      ) : profile.data?.notFound ? (
        <EmptyState
          icon={FileQuestionIcon}
          title="Provider not found"
          description="This provider doesn't exist or is no longer active on SlotForge."
          action={
            <Button variant="outline" size="sm" asChild>
              <Link href="/dashboard/discover?tab=providers">Browse providers</Link>
            </Button>
          }
        />
      ) : profile.data?.provider ? (
        <Card>
          <CardHeader className="pb-4">
            <div className="flex items-center gap-3">
              <span className="flex size-12 items-center justify-center rounded-full bg-brand-soft text-brand-strong">
                <UserRoundIcon className="size-6" aria-hidden="true" />
              </span>
              <div className="min-w-0">
                <h1 className="truncate text-lg font-semibold">
                  {profile.data.provider.display_name}
                </h1>
                <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <GlobeIcon className="size-3.5 shrink-0" aria-hidden="true" />
                  {profile.data.provider.timezone} ({timeZoneAbbr(profile.data.provider.timezone)})
                </p>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-6">
            <div>
              <h2 className="text-sm font-medium">About</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {profile.data.provider.bio?.trim() ||
                  "This provider has not added a bio yet."}
              </p>
            </div>

            <Separator />

            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-xs text-muted-foreground">
                On SlotForge since{" "}
                {new Date(profile.data.provider.created_at).toLocaleDateString()}
              </p>
              <Button asChild>
                <Link href={`/dashboard/discover?provider=${profile.data.provider.id}`}>
                  <CalendarCheckIcon aria-hidden="true" />
                  See available slots
                </Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
