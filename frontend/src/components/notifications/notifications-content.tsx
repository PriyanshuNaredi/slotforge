"use client";

import { useState } from "react";
import {
  BellIcon,
  BellOffIcon,
  CheckCheckIcon,
  CheckIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  BellRingIcon,
} from "lucide-react";
import { toast } from "sonner";

import { listNotifications, markAllNotificationsRead, markNotificationRead } from "@/lib/api/endpoints";
import type { Notification } from "@/lib/api/types";
import { ApiError, toErrorMessage } from "@/lib/api/client";
import { formatRelative } from "@/lib/format";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState, ErrorState } from "@/components/shared/states";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAsyncData } from "@/hooks/use-async-data";
import { notifyNotificationsChanged } from "@/hooks/use-unread-count";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;

/**
 * Shared notifications feed for customers and providers — the backend
 * scopes every query to the authenticated user, so the component is
 * role-agnostic by design.
 */
export function NotificationsContent({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  const [page, setPage] = useState(0);

  const list = useAsyncData(String(page), () =>
    listNotifications(PAGE_SIZE, page * PAGE_SIZE),
  );

  const notifications = list.data?.notifications ?? [];
  const total = list.data?.total ?? 0;
  const unreadCount = notifications.filter((n) => !n.read_at).length;
  const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

  const [markingId, setMarkingId] = useState<string | null>(null);
  const [markingAll, setMarkingAll] = useState(false);

  async function markRead(notification: Notification) {
    setMarkingId(notification.id);
    try {
      await markNotificationRead(notification.id);
      // Server truth; the nav badge refreshes via the change event.
      list.reload();
      notifyNotificationsChanged();
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        toast.error("That notification no longer exists.");
        list.reload();
      } else {
        toast.error(toErrorMessage(err));
      }
    } finally {
      setMarkingId(null);
    }
  }

  async function markAll() {
    setMarkingAll(true);
    try {
      const result = await markAllNotificationsRead();
      if (result.updated_count > 0) {
        list.reload();
        toast.success(`Marked ${result.updated_count} notification${result.updated_count === 1 ? "" : "s"} as read`);
      }
      notifyNotificationsChanged();
    } catch (err) {
      toast.error(toErrorMessage(err));
    } finally {
      setMarkingAll(false);
    }
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageHeader
        title={title}
        description={description}
        actions={
          unreadCount > 0 ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void markAll()}
              disabled={markingAll}
            >
              <CheckCheckIcon aria-hidden="true" />
              Mark all as read
            </Button>
          ) : null
        }
      />

      {list.error ? (
        <ErrorState
          icon={BellOffIcon}
          title="Couldn't load notifications"
          description={list.error}
          action={
            <Button variant="outline" size="sm" onClick={list.reload}>
              Try again
            </Button>
          }
        />
      ) : list.loading && notifications.length === 0 ? (
        <div className="space-y-3">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      ) : !list.loading && notifications.length === 0 ? (
        <EmptyState
          icon={BellIcon}
          title="You're all caught up"
          description="Booking updates — confirmations, cancellations, completions — will appear here as they happen."
        />
      ) : (
        <>
          <ul
            className={`divide-y rounded-xl border bg-card ${
              list.loading ? "opacity-60 transition-opacity" : ""
            }`}
          >
            {notifications.map((notification) => (
              <NotificationRow
                key={notification.id}
                notification={notification}
                pending={markingId === notification.id}
                onMarkRead={() => void markRead(notification)}
              />
            ))}
          </ul>

          {pageCount > 1 ? (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground" aria-live="polite">
                Page {page + 1} of {pageCount} · {total} notifications
              </p>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page === 0}
                  onClick={() => setPage(page - 1)}
                >
                  <ChevronLeftIcon aria-hidden="true" />
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={page >= pageCount - 1}
                  onClick={() => setPage(page + 1)}
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

function NotificationRow({
  notification,
  pending,
  onMarkRead,
}: {
  notification: Notification;
  pending: boolean;
  onMarkRead: () => void;
}) {
  const unread = !notification.read_at;

  return (
    <li
      className={cn(
        "flex items-start gap-3 px-4 py-4 sm:px-5",
        unread && "bg-brand-soft/40",
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full",
          unread ? "bg-brand text-brand-foreground" : "bg-muted text-muted-foreground",
        )}
      >
        {unread ? (
          <BellRingIcon className="size-4" />
        ) : (
          <BellIcon className="size-4" />
        )}
      </span>

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className={cn("text-sm", unread ? "font-semibold" : "font-medium")}>
            {notification.title}
          </p>
          {unread ? (
            <span
              className="inline-flex items-center rounded-full bg-brand px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-brand-foreground"
            >
              New
            </span>
          ) : null}
        </div>
        <p className="mt-0.5 text-sm text-muted-foreground">{notification.message}</p>
        <p className="mt-1 text-xs text-muted-foreground/80">
          {formatRelative(new Date(notification.created_at))} ·{" "}
          {new Date(notification.created_at).toLocaleString("en-US", {
            month: "short",
            day: "numeric",
            hour: "numeric",
            minute: "2-digit",
          })}
        </p>
      </div>

      {unread ? (
        <Button
          variant="ghost"
          size="sm"
          className="shrink-0"
          onClick={onMarkRead}
          disabled={pending}
          aria-label={`Mark "${notification.title}" as read`}
        >
          <CheckIcon aria-hidden="true" />
          Mark read
        </Button>
      ) : null}
    </li>
  );
}
