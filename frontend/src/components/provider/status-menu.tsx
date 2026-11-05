"use client";

import {
  CheckCircle2Icon,
  CircleXIcon,
  UserXIcon,
  type LucideIcon,
} from "lucide-react";

import type { ProviderBooking } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type ProviderStatus = "confirmed" | "cancelled" | "completed" | "no_show";

interface StatusMenuProps {
  booking: ProviderBooking;
  onSelect: (status: ProviderStatus) => void;
}

/**
 * Status actions for one booking, mirroring the backend transition matrix
 * (canTransitionBookingStatus): pending -> confirmed|cancelled,
 * confirmed -> completed|no_show|cancelled. Terminal statuses render nothing.
 */
export function StatusMenu({ booking, onSelect }: StatusMenuProps) {
  const actions: { status: ProviderStatus; label: string; icon: LucideIcon; destructive?: boolean }[] =
    booking.status === "pending"
      ? [
          { status: "confirmed", label: "Confirmed", icon: CheckCircle2Icon },
          { status: "cancelled", label: "Cancelled", icon: CircleXIcon, destructive: true },
        ]
      : booking.status === "confirmed"
        ? [
            { status: "completed", label: "Completed", icon: CheckCircle2Icon },
            { status: "no_show", label: "No-show", icon: UserXIcon },
            { status: "cancelled", label: "Cancelled", icon: CircleXIcon, destructive: true },
          ]
        : [];

  if (actions.length === 0) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          Update status
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Set status</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {actions.map((action) => (
          <DropdownMenuItem
            key={action.status}
            variant={action.destructive ? "destructive" : "default"}
            onClick={() => onSelect(action.status)}
          >
            <action.icon aria-hidden="true" />
            {action.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
