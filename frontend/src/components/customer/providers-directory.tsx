"use client";

import { useState } from "react";
import { ChevronLeftIcon, ChevronRightIcon, SearchIcon, UsersIcon } from "lucide-react";

import { searchProviders } from "@/lib/api/endpoints";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { ProviderCard } from "@/components/customer/provider-card";
import { useAsyncData } from "@/hooks/use-async-data";
import { useDebouncedValue } from "@/hooks/use-debounced-value";

const PAGE_SIZE = 9;

const windows = [
  { value: "any", label: "Any time" },
  { value: "7", label: "Available next 7 days" },
  { value: "14", label: "Available next 14 days" },
  { value: "30", label: "Available next 30 days" },
] as const;

type WindowValue = (typeof windows)[number]["value"];

/**
 * Derives the RFC3339 availability window sent to the backend. Returns
 * null only when the selected range is invalid (guarded before any API
 * call); preset windows are constructed from now, so they are valid by
 * design. Timestamps are UTC instants (toISOString), matching the slots
 * discovery semantics — no local-time conversion happens.
 */
function availabilityRange(
  value: WindowValue,
): { from: string; to: string } | null {
  if (value === "any") return { from: "", to: "" };

  const now = new Date();
  const to = new Date(now);
  to.setDate(to.getDate() + Number(value));

  const from = now.toISOString();
  const toIso = to.toISOString();

  return new Date(from) < new Date(toIso) ? { from, to: toIso } : null;
}

export function ProvidersDirectory() {
  const [searchInput, setSearchInput] = useState("");
  const search = useDebouncedValue(searchInput.trim(), 300);
  const [windowValue, setWindowValue] = useState<WindowValue>("any");
  const [page, setPage] = useState(0);

  // Validate the selected range before fetching; an invalid range skips
  // the request entirely (null key) and shows a validation message.
  const range = availabilityRange(windowValue);
  const rangeInvalid = range === null;

  const list = useAsyncData(
    rangeInvalid ? null : `${search}#${windowValue}#${page}`,
    () =>
      searchProviders({
        search: search || undefined,
        available_from: range!.from || undefined,
        available_to: range!.to || undefined,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
  );

  const providers = list.data?.items ?? [];
  const total = list.data?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <SearchIcon
            className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={searchInput}
            onChange={(event) => {
              setPage(0);
              setSearchInput(event.target.value);
            }}
            placeholder="Search providers by name"
            aria-label="Search providers by name"
            className="pl-9"
          />
        </div>
        <Select
          value={windowValue}
          onValueChange={(value) => {
            setPage(0);
            setWindowValue(value as WindowValue);
          }}
        >
          <SelectTrigger aria-label="Availability window" className="w-full sm:w-56">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {windows.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {windowValue !== "any" && !rangeInvalid ? (
        <Badge variant="secondary" aria-live="polite">
          Showing providers with availability in the next {windowValue} days
        </Badge>
      ) : null}

      {rangeInvalid ? (
        <ErrorState
          icon={UsersIcon}
          title="Invalid date range"
          description="The selected window must end after it starts."
        />
      ) : list.error ? (
        <ErrorState
          icon={UsersIcon}
          title="Couldn't load providers"
          description={list.error}
          action={
            <Button variant="outline" size="sm" onClick={list.reload}>
              Try again
            </Button>
          }
        />
      ) : list.loading && providers.length === 0 ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, index) => (
            <div key={index} className="rounded-lg border border-rule bg-card p-5">
              <Skeleton className="h-4 w-32" />
              <Skeleton className="mt-3 h-3 w-full" />
              <Skeleton className="mt-2 h-3 w-2/3" />
            </div>
          ))}
        </div>
      ) : !list.loading && providers.length === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title={search ? "No providers match your search" : "No providers yet"}
          description={
            search
              ? `Nothing found for "${search}". Try a different name, or widen the availability window.`
              : "Providers appear here once they publish their profile on SlotForge."
          }
        />
      ) : (
        <>
          <div
            className={`grid gap-4 sm:grid-cols-2 lg:grid-cols-3 ${list.loading ? "opacity-60 transition-opacity" : ""}`}
          >
            {providers.map((provider) => (
              <ProviderCard key={provider.id} provider={provider} />
            ))}
          </div>

          {pageCount > 1 ? (
            <div className="flex items-center justify-between pt-2">
              <p className="text-sm text-ink-soft" aria-live="polite">
                Page {page + 1} of {pageCount} · {total} providers
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 0}
                  onClick={() => setPage((value) => value - 1)}
                >
                  <ChevronLeftIcon aria-hidden="true" />
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= pageCount - 1}
                  onClick={() => setPage((value) => value + 1)}
                >
                  Next
                  <ChevronRightIcon aria-hidden="true" />
                </Button>
              </div>
            </div>
          ) : null}
        </>
      )}
    </div>
  );
}
