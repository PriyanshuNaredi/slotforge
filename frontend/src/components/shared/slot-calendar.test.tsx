// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { SlotCalendar } from "./slot-calendar";
import { selectionToSlotPrefill } from "@/lib/format";
import type { EventInput } from "@fullcalendar/react";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function tomorrowAt(hour: number, minute = 0): string {
  const date = new Date();
  date.setDate(date.getDate() + 1);
  date.setHours(hour, minute, 0, 0);
  // Local-time ISO with offset (FullCalendar handles offsets natively).
  const pad = (value: number) => String(value).padStart(2, "0");
  const offsetMinutes = -date.getTimezoneOffset();
  const sign = offsetMinutes >= 0 ? "+" : "-";
  const absOffset = Math.abs(offsetMinutes);
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(hour)}:${pad(minute)}:00${sign}${pad(Math.floor(absOffset / 60))}:${pad(absOffset % 60)}`;
}

const events: EventInput[] = [
  {
    id: "slot:1",
    title: "Open slot",
    start: tomorrowAt(9),
    end: tomorrowAt(10),
    backgroundColor: "#FBE3DE",
    extendedProps: { kind: "slot" },
  },
];

describe("SlotCalendar", () => {
  it("renders events in the month grid", async () => {
    render(<SlotCalendar events={events} emptyTitle="Nothing" emptyDescription="desc" />);
    await waitFor(
      () => {
        expect(screen.getAllByText("Open slot").length).toBeGreaterThan(0);
      },
      { timeout: 3000 },
    );
  });

  it("switches between month, week, day and agenda views", async () => {
    const views: string[] = [];
    render(
      <SlotCalendar
        events={events}
        emptyTitle="Nothing"
        emptyDescription="desc"
        onDatesSet={(info) => views.push(info.view)}
      />,
    );

    await waitFor(() => {
      expect(views).toContain("dayGridMonth");
    });

    await userEvent.setup().click(await screen.findByRole("tab", { name: "Week view" }));
    await waitFor(() => {
      expect(views[views.length - 1]).toBe("timeGridWeek");
    });

    await userEvent.setup().click(await screen.findByRole("tab", { name: "Day view" }));
    await waitFor(() => {
      expect(views[views.length - 1]).toBe("timeGridDay");
    });

    await userEvent.setup().click(await screen.findByRole("tab", { name: "List view" }));
    await waitFor(() => {
      expect(views[views.length - 1]).toBe("listWeek");
    });

    await userEvent.setup().click(await screen.findByRole("tab", { name: "Month view" }));
    await waitFor(() => {
      expect(views[views.length - 1]).toBe("dayGridMonth");
    });
  });

  it("navigates months with prev/next and returns with today", async () => {
    const user = userEvent.setup();
    render(<SlotCalendar events={events} emptyTitle="Nothing" emptyDescription="desc" />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 2 })).toBeDefined();
    });
    const title = () => screen.getByRole("heading", { level: 2 }).textContent ?? "";
    const titleBefore = title();

    await user.click(screen.getByRole("button", { name: "Next Month" }));
    await waitFor(() => {
      expect(title()).not.toBe(titleBefore);
    });

    await user.click(screen.getByRole("button", { name: "This Month" }));
    await waitFor(() => {
      expect(title()).toBe(titleBefore);
    });
  });

  it("fires onEventClick with the event id when an event is clicked", async () => {
    const onEventClick = vi.fn();
    const user = userEvent.setup();
    render(
      <SlotCalendar
        events={events}
        emptyTitle="Nothing"
        emptyDescription="desc"
        onEventClick={onEventClick}
      />,
    );

    // The legend row also says "Open slot" but is not an event — event chips
    // render inside calendar table cells, a tick later. Keep retrying until
    // the real chip exists.
    let eventEl: HTMLElement | undefined;
    await waitFor(() => {
      // The legend row also says "Open slot" but is not an event. The classic
      // theme hashes every class name, so scope by structure: the event chip
      // never lives inside an <li>.
      eventEl = screen
        .getAllByText("Open slot")
        .find((el) => el.closest("li") === null);
      expect(eventEl).toBeDefined();
    });
    await user.click(eventEl!);

    await waitFor(() => {
      expect(onEventClick).toHaveBeenCalledWith("slot:1", "slot");
    });
  });

  it("converts a grid selection into slot-prefill times via selectionToSlotPrefill", () => {
    // Same-day selection keeps both times in the provider's timezone.
    // (2030-03-15 is after the DST cutover — EDT, UTC-4.)
    expect(
      selectionToSlotPrefill("2030-03-15T09:30:00-04:00", "2030-03-15T11:00:00-04:00", "America/New_York"),
    ).toEqual({ date: "2030-03-15", startTime: "09:30", endTime: "11:00" });

    // Multi-day selection clamps the end time to a sensible default.
    expect(
      selectionToSlotPrefill("2030-03-15T09:00:00-04:00", "2030-03-17T14:00:00-04:00", "America/New_York"),
    ).toEqual({ date: "2030-03-15", startTime: "09:00", endTime: "10:00" });
  });

  it("defaults to the agenda list on mobile", async () => {
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query === "(max-width: 767px)",
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }));

    const views: string[] = [];
    render(
      <SlotCalendar
        events={events}
        emptyTitle="Nothing"
        emptyDescription="desc"
        onDatesSet={(info) => views.push(info.view)}
      />,
    );

    await waitFor(() => {
      expect(views).toContain("listWeek");
    });
  });

  it("renders times in the requested timezone", async () => {
    // Tomorrow at 09:00 UTC, displayed in America/New_York (UTC-5 in winter).
    const start = new Date();
    start.setDate(start.getDate() + 1);
    start.setUTCHours(9, 0, 0, 0);
    const end = new Date(start.getTime() + 3_600_000);
    const startIso = start.toISOString();
    const endIso = end.toISOString();

    // The expected wall-clock label, computed independently of this machine.
    // Month-grid chips render compact times like "4a" / "12p".
    const labeled = new Intl.DateTimeFormat("en-US", {
      timeZone: "America/New_York",
      hour: "numeric",
      hour12: true,
    }).format(start);
    const hourMatch = labeled.match(new RegExp("^(\\d+)\\s?(AM|PM)$"));
    const expectedHour =
      hourMatch![1]! + (hourMatch![2]!.toLowerCase() === "am" ? "a" : "p");

    const { container } = render(
      <SlotCalendar
        events={[
          {
            id: "slot:tz",
            title: "Timezone probe",
            start: startIso,
            end: endIso,
            backgroundColor: "#FBE3DE",
          },
        ]}
        emptyTitle="Nothing"
        emptyDescription="desc"
        timeZone="America/New_York"
        desktopInitialView="dayGridMonth"
      />,
    );

    await waitFor(
      () => {
        expect(container.textContent).toContain(expectedHour.split(" ")[0]);
      },
      { timeout: 3000 },
    );
  });

  it("shows the empty state when there are no events", () => {
    render(
      <SlotCalendar
        events={[]}
        emptyTitle="Nothing scheduled"
        emptyDescription="Book something first."
      />,
    );
    expect(screen.getByText("Nothing scheduled")).toBeDefined();
  });

  it("shows the error state and retries", async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(
      <SlotCalendar
        events={[]}
        error="backend unreachable"
        onRetry={onRetry}
        emptyTitle="Nothing"
        emptyDescription="desc"
      />,
    );

    expect(screen.getByText("Couldn't load the calendar")).toBeDefined();
    await user.click(screen.getByText("Try again"));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("renders a skeleton while loading with no events", () => {
    const { container } = render(
      <SlotCalendar events={[]} loading emptyTitle="Nothing" emptyDescription="desc" />,
    );
    expect(container.querySelector(".animate-pulse")).not.toBeNull();
  });
});
