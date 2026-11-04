import Link from "next/link";
import { CheckIcon } from "lucide-react";

import { Logo } from "@/components/shared/logo";

const points = [
  "Slots open and fill in real time — no phone tag.",
  "Bookings confirm instantly and lock the slot.",
  "Cancel anytime; the slot reopens for others.",
];

export default function AuthLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <div className="grid min-h-svh lg:grid-cols-[1fr_1.1fr]">
      {/* Brand panel */}
      <aside className="relative hidden flex-col justify-between overflow-hidden bg-primary p-10 text-primary-foreground lg:flex">
        <div
          className="absolute inset-0 opacity-[0.12]"
          style={{
            backgroundImage:
              "radial-gradient(circle at 15% 25%, oklch(0.72 0.15 45) 0%, transparent 40%), radial-gradient(circle at 85% 75%, oklch(0.72 0.15 45) 0%, transparent 40%)",
          }}
          aria-hidden="true"
        />
        <div className="relative">
          <Link href="/" className="inline-flex items-center gap-2 font-semibold">
            <span className="flex size-7 items-center justify-center rounded-md bg-primary-foreground text-primary">
              <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
                <rect x="1.5" y="2.5" width="13" height="12" rx="2.5" stroke="currentColor" strokeWidth="1.5" />
                <path d="M1.5 6h13" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                <rect x="4.5" y="8.5" width="4" height="3.5" rx="1" fill="currentColor" />
              </svg>
            </span>
            SlotForge
          </Link>
        </div>
        <div className="relative max-w-md">
          <h2 className="text-3xl font-semibold tracking-tight text-balance">
            The calendar that books itself.
          </h2>
          <ul className="mt-6 space-y-3">
            {points.map((point) => (
              <li key={point} className="flex items-start gap-2.5 text-sm text-primary-foreground/80">
                <CheckIcon className="mt-0.5 size-4 shrink-0 text-brand" aria-hidden="true" />
                {point}
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-xs text-primary-foreground/50">
          © {new Date().getFullYear()} SlotForge
        </p>
      </aside>

      {/* Form panel */}
      <main className="flex flex-col">
        <div className="flex items-center justify-between p-6 lg:hidden">
          <Logo />
        </div>
        <div className="flex flex-1 items-center justify-center px-4 pb-16 pt-4 sm:px-6">
          <div className="w-full max-w-sm">{children}</div>
        </div>
      </main>
    </div>
  );
}
