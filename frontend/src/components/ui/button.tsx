import * as React from "react";
import { Slot } from "radix-ui";

import { cn } from "@/lib/utils";

interface ButtonVariantProps {
  variant?: "default" | "brand" | "destructive" | "outline" | "secondary" | "ghost" | "link";
  size?: "default" | "sm" | "lg" | "icon";
}

const buttonVariants = ({
  variant = "default",
  size = "default",
}: ButtonVariantProps) => {
  return cn(
    "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/40 focus-visible:border-ring cursor-pointer",
    // Primary action: coral with navy text (5.3:1).
    variant === "default" &&
      "bg-primary text-primary-foreground shadow-xs hover:bg-primary/90",
    // Stamped tinted action: coral wash with strong coral text (5:1).
    variant === "brand" &&
      "bg-brand-soft text-brand-strong ring-1 ring-inset ring-coral/30 hover:bg-coral/20",
    variant === "destructive" &&
      "bg-destructive text-destructive-foreground hover:bg-destructive/90 focus-visible:ring-destructive/30",
    variant === "outline" &&
      "border border-rule-strong bg-card hover:bg-accent hover:text-accent-foreground",
    variant === "secondary" &&
      "bg-secondary text-secondary-foreground hover:bg-secondary/70",
    variant === "ghost" && "hover:bg-accent hover:text-accent-foreground",
    variant === "link" && "text-brand-strong underline-offset-4 hover:underline",
    size === "default" && "h-9 px-4 py-2 has-[>svg]:px-3",
    size === "sm" && "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5 text-[13px]",
    size === "lg" && "h-10 rounded-md px-6 has-[>svg]:px-4",
    size === "icon" && "size-9",
  );
};

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> & ButtonVariantProps & {
  asChild?: boolean;
}) {
  const Comp = asChild ? Slot.Root : "button";
  return (
    <Comp
      data-slot="button"
      className={cn(buttonVariants({ variant, size }), className)}
      {...props}
    />
  );
}

export { Button, buttonVariants };
