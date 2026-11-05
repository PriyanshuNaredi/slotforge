// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ProviderProfileContent } from "./provider-profile";
import { ApiError } from "@/lib/api/client";
import type { PublicProvider } from "@/lib/api/types";

const getPublicProvider = vi.fn();

vi.mock("@/lib/api/endpoints", () => ({
  getPublicProvider: (...args: unknown[]) => getPublicProvider(...args),
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "11111111-1111-1111-1111-111111111111" }),
}));

function makeProvider(overrides: Partial<PublicProvider>): PublicProvider {
  return {
    id: "11111111-1111-1111-1111-111111111111",
    display_name: "Detail Provider",
    bio: "Ten years of slots",
    timezone: "Europe/Berlin",
    created_at: "2026-01-15T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  getPublicProvider.mockReset();
});

afterEach(() => {
  cleanup();
});

describe("ProviderProfileContent", () => {
  it("renders public profile fields and links to available slots", async () => {
    getPublicProvider.mockResolvedValue(makeProvider({}));

    render(<ProviderProfileContent />);

    expect(await screen.findByText("Detail Provider")).toBeDefined();
    expect(screen.getByText("Ten years of slots")).toBeDefined();
    expect(screen.getByText(/Europe\/Berlin/)).toBeDefined();
    expect(screen.getByText(/On SlotForge since/)).toBeDefined();

    // Navigation from profile to the existing slot discovery.
    const cta = screen.getByRole("link", { name: /see available slots/i });
    expect(cta.getAttribute("href")).toBe(
      "/dashboard/discover?provider=11111111-1111-1111-1111-111111111111",
    );
  });

  it("shows a bio placeholder when the provider has none", async () => {
    getPublicProvider.mockResolvedValue(makeProvider({ bio: null }));

    render(<ProviderProfileContent />);

    expect(
      await screen.findByText("This provider has not added a bio yet."),
    ).toBeDefined();
  });

  it("shows the not-found state for a 404", async () => {
    getPublicProvider.mockRejectedValue(
      new ApiError(404, "provider not found"),
    );

    render(<ProviderProfileContent />);

    expect(await screen.findByText("Provider not found")).toBeDefined();
    // No retry action for a normal empty result.
    expect(screen.queryByRole("button", { name: /try again/i })).toBeNull();
  });

  it("shows the error state with retry on a server failure", async () => {
    getPublicProvider.mockRejectedValue(new ApiError(500, "database down"));

    render(<ProviderProfileContent />);

    expect(
      await screen.findByText("Couldn't load this provider"),
    ).toBeDefined();
    expect(screen.getByText("database down")).toBeDefined();

    getPublicProvider.mockResolvedValue(makeProvider({}));

    await userEvent.click(screen.getByRole("button", { name: /try again/i }));
    expect(await screen.findByText("Detail Provider")).toBeDefined();
  });
});
