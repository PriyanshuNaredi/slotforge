import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

interface StateProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: React.ReactNode;
  className?: string;
}

/** Compact, purposeful states — a ruled note, not a large empty container. */
export function EmptyState({ icon: Icon, title, description, action, className }: StateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-start justify-center gap-1 rounded-lg border border-rule bg-card/60 px-5 py-7 text-left sm:px-6",
        className,
      )}
    >
      <div className="flex items-center gap-2.5">
        <Icon className="size-4 text-coral-strong" aria-hidden="true" />
        <h3 className="text-sm font-semibold">{title}</h3>
      </div>
      <p className="max-w-md text-sm text-ink-soft">{description}</p>
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}

export function ErrorState({ icon: Icon, title, description, action, className }: StateProps) {
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-start justify-center gap-1 rounded-lg border border-destructive/30 bg-destructive/5 px-5 py-7 text-left sm:px-6",
        className,
      )}
    >
      <div className="flex items-center gap-2.5">
        <Icon className="size-4 text-destructive" aria-hidden="true" />
        <h3 className="text-sm font-semibold">{title}</h3>
      </div>
      <p className="max-w-md text-sm text-ink-soft">{description}</p>
      {action ? <div className="mt-3">{action}</div> : null}
    </div>
  );
}
