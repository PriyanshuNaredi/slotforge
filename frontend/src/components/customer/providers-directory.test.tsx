// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ProvidersDirectory } from "./providers-directory";
import type { PublicProvider } from "@/lib/api/types";

const searchProviders = vi.fn();

vi.mock("@/lib/api/endpoints", () => ({
  searchProviders: (...args: unknown[]) => searchProviders(...args),
}));

function makeProvider(overrides: Partial<PublicProvider>): PublicProvider {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    display_name: overrides.display_name ?? "Test Provider",
    bio: overrides.bio ?? "Slots all week",
    timezone: overrides.timezone ?? "UTC",
    created_at: overrides.created_at ?? "2026-11-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  // Radix Select needs pointer-capture APIs jsdom doesn't implement.
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
  window.HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
  window.HTMLElement.prototype.setPointerCapture = vi.fn();
  window.HTMLElement.prototype.releasePointerCapture = vi.fn();

  searchProviders.mockReset();
  searchProviders.mockResolvedValue({
    items: [],
    total: 0,
    limit: 9,
    offset: 0,
  });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("ProvidersDirectory", () => {
  it("renders providers and paginates with backend totals", async () => {
    searchProviders.mockResolvedValue({
      items: [
        makeProvider({ display_name: "Alpha Provider" }),
        makeProvider({ display_name: "Beta Provider" }),
      ],
      total: 11,
      limit: 9,
      offset: 0,
    });

    render(<ProvidersDirectory />);

    expect(await screen.findByText("Alpha Provider")).toBeDefined();
    expect(screen.getByText("Beta Provider")).toBeDefined();
    expect(screen.getByText(/Page 1 of 2 · 11 providers/)).toBeDefined();

    searchProviders.mockResolvedValue({
      items: [makeProvider({ display_name: "Gamma Provider" })],
      total: 11,
      limit: 9,
      offset: 9,
    });

    await userEvent.click(screen.getByRole("button", { name: /next/i }));

    await waitFor(() => {
      expect(screen.getByText("Gamma Provider")).toBeDefined();
    });
    expect(searchProviders).toHaveBeenLastCalledWith(
      expect.objectContaining({ limit: 9, offset: 9 }),
    );
  });

  it("debounces the search input before calling the API", async () => {
    vi.useFakeTimers();

    render(<ProvidersDirectory />);

    await vi.advanceTimersByTimeAsync(0);
    expect(searchProviders).toHaveBeenCalledTimes(1); // initial directory load

    // Simulate typing: each change resets the debounce timer.
    const input = screen.getByLabelText("Search providers by name");
    fireEvent.change(input, { target: { value: "d" } });
    fireEvent.change(input, { target: { value: "de" } });
    fireEvent.change(input, { target: { value: "den" } });

    // No request per keystroke while the debounce timer is pending.
    expect(searchProviders).toHaveBeenCalledTimes(1);

    // act flushes the debounced state update and the fetch effect.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(300);
    });

    expect(searchProviders).toHaveBeenCalledTimes(2);
    expect(searchProviders).toHaveBeenLastCalledWith(
      expect.objectContaining({ search: "den" }),
    );
  });

  it("shows the availability badge and sends RFC3339 window filters", async () => {
    searchProviders.mockResolvedValue({
      items: [makeProvider({ display_name: "Available Soon" })],
      total: 1,
      limit: 9,
      offset: 0,
    });

    render(<ProvidersDirectory />);

    await userEvent.click(
      screen.getByRole("combobox", { name: "Availability window" }),
    );
    await userEvent.click(screen.getByRole("option", { name: /next 7 days/i }));

    expect(await screen.findByText("Available Soon")).toBeDefined();
    expect(
      screen.getByText("Showing providers with availability in the next 7 days"),
    ).toBeDefined();

    const call = searchProviders.mock.lastCall?.[0] as {
      available_from: string;
      available_to: string;
    };

    expect(call.available_from).toBeDefined();
    expect(call.available_to).toBeDefined();
    // RFC3339 instants, window ending after it starts.
    expect(new Date(call.available_to).getTime()).toBeGreaterThan(
      new Date(call.available_from).getTime(),
    );
  });

  it("shows an error state with retry when the API fails", async () => {
    searchProviders.mockRejectedValue(new Error("Backend exploded"));

    render(<ProvidersDirectory />);

    expect(await screen.findByText("Couldn't load providers")).toBeDefined();
    expect(screen.getByText("Backend exploded")).toBeDefined();

    searchProviders.mockResolvedValue({
      items: [makeProvider({ display_name: "Recovered" })],
      total: 1,
      limit: 9,
      offset: 0,
    });

    await userEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(await screen.findByText("Recovered")).toBeDefined();
  });

  it("shows the empty state when nothing matches the search", async () => {
    searchProviders.mockResolvedValue({
      items: [],
      total: 0,
      limit: 9,
      offset: 0,
    });

    render(<ProvidersDirectory />);

    await userEvent.type(
      screen.getByLabelText("Search providers by name"),
      "nobody",
    );

    expect(await screen.findByText("No providers match your search")).toBeDefined();
  });
});
