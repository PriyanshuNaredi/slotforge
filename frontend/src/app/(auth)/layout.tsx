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
      <aside className="relative hidden flex-col justify-between overflow-hidden bg-navy p-10 text-cream lg:flex">
        <div
          className="absolute inset-0 opacity-[0.12]"
          style={{
            backgroundImage:
              "radial-gradient(circle at 15% 25%, rgb(241 124 105 / 0.5) 0%, transparent 40%), radial-gradient(circle at 85% 75%, rgb(241 124 105 / 0.5) 0%, transparent 40%)",
          }}
          aria-hidden="true"
        />
        <div className="relative">
          <Logo href="/" tone="inverse" />
        </div>
        <div className="relative max-w-md">
          <h2 className="font-display text-4xl font-semibold tracking-tight text-balance">
            The calendar that books itself.
          </h2>
          <ul className="mt-8 space-y-3 border-t border-cream/15 pt-6">
            {points.map((point) => (
              <li key={point} className="flex items-start gap-3 text-sm text-cream/80">
                <CheckIcon className="mt-0.5 size-4 shrink-0 text-coral" aria-hidden="true" />
                {point}
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-xs text-cream/50">
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
