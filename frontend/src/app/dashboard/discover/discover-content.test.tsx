// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DiscoverContent } from "./discover-content";
import type { AvailableSlot, PublicProvider } from "@/lib/api/types";

let searchParams = new URLSearchParams();

const searchAvailableSlots = vi.fn();
const getAvailableSlot = vi.fn();
const getPublicProvider = vi.fn();
const replace = vi.fn((url: string) => {
  searchParams = new URLSearchParams(url.split("?")[1] ?? "");
});

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  useRouter: () => ({ replace }),
}));

vi.mock("@/lib/api/endpoints", () => ({
  searchAvailableSlots: (...args: unknown[]) => searchAvailableSlots(...args),
  getAvailableSlot: (...args: unknown[]) => getAvailableSlot(...args),
  getPublicProvider: (...args: unknown[]) => getPublicProvider(...args),
  searchProviders: vi.fn().mockResolvedValue({
    items: [],
    total: 0,
    limit: 9,
    offset: 0,
  }),
}));

function makeSlot(overrides: Partial<AvailableSlot> = {}): AvailableSlot {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    provider_id: overrides.provider_id ?? "p1",
    provider_name: overrides.provider_name ?? "Slot Provider",
    timezone: "UTC",
    starts_at: "2026-11-10T10:00:00Z",
    ends_at: "2026-11-10T10:30:00Z",
    ...overrides,
  } as AvailableSlot;
}

beforeEach(() => {
  searchParams = new URLSearchParams();
  searchAvailableSlots.mockReset();
  getAvailableSlot.mockReset();
  getPublicProvider.mockReset();
  searchAvailableSlots.mockResolvedValue({
    slots: [makeSlot()],
    total: 1,
    limit: 12,
    offset: 0,
  });
  getAvailableSlot.mockResolvedValue(makeSlot({ id: "deep" }));
  getPublicProvider.mockResolvedValue({
    id: "p1",
    display_name: "Banner Provider",
    bio: null,
    timezone: "UTC",
    created_at: "2026-11-01T00:00:00Z",
  } satisfies PublicProvider);
});

afterEach(() => {
  cleanup();
});

describe("DiscoverContent", () => {
  it("renders the slots tab by default and opens the slot deep-link dialog", async () => {
    searchParams = new URLSearchParams("slot=deep");

    render(<DiscoverContent />);

    // Existing slot discovery still renders (grid + booking dialog both
    // show the provider name).
    expect((await screen.findAllByText("Slot Provider")).length).toBeGreaterThan(0);

    // ?slot=<id> opens the existing booking dialog.
    expect(await screen.findByRole("dialog")).toBeDefined();
    expect(getAvailableSlot).toHaveBeenCalledWith("deep");
  });

  it("filters slots by ?provider= and shows the provider banner", async () => {
    searchParams = new URLSearchParams("provider=p1");

    render(<DiscoverContent />);

    // "Availability for <name>" spans multiple text nodes.
    expect(await screen.findByText(/Banner Provider/)).toBeDefined();
    expect(screen.getByText(/Availability for/)).toBeDefined();
    expect(searchAvailableSlots).toHaveBeenCalledWith(
      expect.objectContaining({ provider_id: "p1" }),
    );
  });

  it("switches tabs through the URL and loads the directory", async () => {
    searchParams = new URLSearchParams("tab=providers");

    const view = render(<DiscoverContent />);

    // The providers tab mounts the directory; slots stay unfetched.
    expect(await screen.findByLabelText("Search providers by name")).toBeDefined();
    expect(searchAvailableSlots).not.toHaveBeenCalled();

    // Tab clicks update the URL; the visible tab is derived from it.
    await userEvent.click(screen.getByRole("tab", { name: "Available slots" }));
    expect(replace).toHaveBeenCalledWith(
      "/dashboard/discover?tab=slots",
      expect.anything(),
    );

    // Simulate the navigation re-render: slots flow is intact.
    view.rerender(<DiscoverContent />);
    expect((await screen.findAllByText("Slot Provider")).length).toBeGreaterThan(0);
    expect(searchAvailableSlots).toHaveBeenCalled();
  });
});
