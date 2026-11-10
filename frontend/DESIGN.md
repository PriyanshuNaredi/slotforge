---
name: SlotForge
description: "The Living Departure Board — a print-timetable world of ink navy, warm paper, and deep vermilion for real-time appointment booking"
colors:
  paper: "#faf8f3"
  paper-raised: "#ffffff"
  ink: "#14263e"
  ink-soft: "#4c5f7a"
  navy: "#172b4d"
  navy-deep: "#0f1f38"
  board: "#13233d"
  board-ink: "#f2eee3"
  board-muted: "#9fb1c7"
  vermilion: "#bc3e1b"
  vermilion-deep: "#9c3115"
  coral: "#e2543a"
  coral-strong: "#a63a20"
  coral-wash: "#f9e2d9"
  coral-on-board: "#f0765a"
  rule: "#d9dfe8"
  rule-strong: "#b9c6d4"
  secondary: "#eef1f5"
  accent: "#f1ede4"
  destructive: "#b3362b"
  primary-foreground: "#fff8f2"
  success: "#2f5d46"
  success-wash: "#e3eee7"
  pending: "#8a5a12"
  pending-wash: "#f6ecd9"
  completed: "#41597a"
  completed-wash: "#e7edf4"
typography:
  display:
    fontFamily: "Besley, Georgia, serif"
    fontSize: "4.25rem"
    fontWeight: 600
    lineHeight: 1.05
    letterSpacing: "-0.025em"
  headline:
    fontFamily: "Besley, Georgia, serif"
    fontSize: "2.25rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.025em"
  title:
    fontFamily: "Besley, Georgia, serif"
    fontSize: "1.25rem"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.025em"
  standfirst:
    fontFamily: "Archivo, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 400
    lineHeight: 1.625
  body:
    fontFamily: "Archivo, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.625
  label:
    fontFamily: "Archivo, sans-serif"
    fontSize: "0.6875rem"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "0.14em"
  time:
    fontFamily: "Geist Mono, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
    fontFeature: "tabular-nums"
rounded:
  sm: "2px"
  md: "3px"
  lg: "4px"
  xl: "6px"
  2xl: "8px"
spacing:
  xs: "2px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "48px"
  section: "64px"
  section-lg: "96px"
components:
  button-default:
    backgroundColor: "{colors.vermilion}"
    textColor: "{colors.primary-foreground}"
    rounded: "{rounded.md}"
    height: "36px"
    padding: "0 16px"
  button-default-hover:
    backgroundColor: "{colors.vermilion-deep}"
  button-brand:
    backgroundColor: "{colors.coral-wash}"
    textColor: "{colors.coral-strong}"
    rounded: "{rounded.md}"
    height: "36px"
    padding: "0 16px"
  button-outline:
    backgroundColor: "{colors.paper-raised}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    height: "36px"
  button-destructive:
    backgroundColor: "{colors.destructive}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    height: "36px"
  stamp-confirmed:
    backgroundColor: "{colors.success-wash}"
    textColor: "{colors.success}"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  stamp-pending:
    backgroundColor: "{colors.pending-wash}"
    textColor: "{colors.pending}"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  stamp-completed:
    backgroundColor: "{colors.completed-wash}"
    textColor: "{colors.completed}"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  stamp-cancelled:
    backgroundColor: "{colors.secondary}"
    textColor: "{colors.ink-soft}"
    rounded: "{rounded.sm}"
    padding: "2px 8px"
  card:
    backgroundColor: "{colors.paper-raised}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "24px"
  input:
    backgroundColor: "{colors.paper-raised}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    height: "36px"
  board-panel:
    backgroundColor: "{colors.board}"
    textColor: "{colors.board-ink}"
    rounded: "{rounded.lg}"
---

# Design System: SlotForge

## Overview

**Creative North Star: "The Living Departure Board"**

SlotForge looks the way a station board reads: ruled, exact, current. Availability is set as a live departure list — deep-navy board panels with paper text, dotted leaders, and tabular mono times — bound into an editorial page of warm paper (#faf8f3), ink navy (#14263e), and blue-grey hairlines. Deep vermilion (#bc3e1b) is the only color that grants an action; coral (#e2543a) is a single spot color for markers, underlines, and large elements. Besley — the Clarendon face of 19th-century railway timetables and printed notices — carries headlines; Archivo (a grotesk with transport-signage lineage) carries the UI; Geist Mono carries every time and code. The world refuses the category-default arrangement of centered hero + three feature cards + logo band: sections close on hairline rules, lists are ruled rows rather than card grids, and the brand lives in precise details (tabular times, status stamps, dotted leaders) rather than decoration. Authenticated dashboards inherit the same ink/paper/rules vocabulary under operate-mode restraint.

**Key Characteristics:**

- Print-timetable palette: ink navy on crisp warm paper, one vermilion action color, one coral spot color, blue-grey hairlines.
- The navy departure board is the signature surface: ruled 32px texture, dotted leaders, tabular mono times, coral flag on the next open slot.
- Squared 2–4px corners everywhere; pills exist only for count bubbles and indicator dots.
- Statuses are squared uppercase stamps — wash ground, dark ink text — never colored pills.
- Depth is printed, not projected: hairline rules separate surfaces; shadows appear only under the board panel and dialogs.
- Every color is a named token from `frontend/src/app/globals.css` — no ad-hoc hexes.

## Colors

The palette is a two-ground system — warm paper for reading, ink navy for the board and chrome — with vermilion reserved for action, coral for annotation, and hairline blue-greys for structure.

### Primary

| Token | Value | Role | Contrast note |
| --- | --- | --- | --- |
| `vermilion` | `#bc3e1b` | Primary action ground (buttons, calendar primary/now-indicator, open-slot events) | Warm paper-white `#fff8f2` on it: 5.2:1 computed, 5.5:1 vs pure white (code-documented) |
| `vermilion-deep` | `#9c3115` | Primary button hover ground | Darker step of vermilion; hover only |
| `primary-foreground` | `#fff8f2` | Text on vermilion and coral-solid grounds (buttons, event blocks) | 5.2:1 on `vermilion` |

### Secondary — the coral spot family

| Token | Value | Role | Contrast note |
| --- | --- | --- | --- |
| `coral` | `#e2543a` | The single spot color: live dots, tab underline, focus rings, borders, selection tint, large elements only | 4.2:1 on `board` — large elements/decoration only, never small text |
| `coral-strong` | `#a63a20` | Small coral text on light grounds: links, row numbers, stamp text | 6.2:1 on paper (computed 6.1); 5:1 on `coral-wash` (computed 5.2) |
| `coral-wash` | `#f9e2d9` | Brand stamp ground, text selection background | Carries `coral-strong` at 5:1 |
| `coral-on-board` | `#f0765a` | Small coral text on the navy board (Live label, first row time, Book hover) | 5.5:1 on `board` (computed 5.6) — lightened because raw `coral` only reaches 4.2:1 there |

### Board grounds (the dark closed set)

| Token | Value | Role | Contrast note |
| --- | --- | --- | --- |
| `board` | `#13233d` | Departure-board panel ground | See board text tokens below |
| `board-ink` | `#f2eee3` | Paper text on the board (times, primary rows) | 13.6:1 on `board` |
| `board-muted` | `#9fb1c7` | Muted text on the board (provider names, durations, footers) | 7.2:1 on `board` |
| `board-rule` | `rgb(242 238 227 / 0.16)` | Hairlines and the 32px ruled texture on the board | Decorative only |
| `navy` | `#172b4d` | App-shell sidebar ground | `paper` on it: 13.3:1 |
| `navy-deep` | `#0f1f38` | Footer ground; board panel border tint | Darkest step of the ink family |

### Neutral

| Token | Value | Role | Contrast note |
| --- | --- | --- | --- |
| `paper` | `#faf8f3` | Page ground | `ink` on it: 14.4:1; `ink-soft`: 6.1:1 |
| `paper-raised` | `#ffffff` | Cards, inputs, popovers, calendar surface | `ink-soft` on it: 6.5:1 |
| `ink` | `#14263e` | Primary text | 14.4:1 on paper |
| `ink-soft` | `#4c5f7a` | Secondary text, descriptions, inactive labels | ~6:1 on paper/white (code-documented) |
| `rule` | `#d9dfe8` | Hairline borders and section rules | Decorative |
| `rule-strong` | `#b9c6d4` | Input borders, dotted leaders, stronger rules | Decorative |
| `secondary` / `muted` | `#eef1f5` | Muted fills, cancelled stamp ground | `ink-soft` on it: 5.8:1 |
| `accent` | `#f1ede4` | Hover wash on paper (ghost/outline hovers) | Warm neutral tint |
| `destructive` | `#b3362b` | Destructive actions and no-show events | White text on it: 6.0:1 |

### Sanctioned status hues (approved extension of the world)

The status wash system was reviewed and approved as a legitimate extension of the stamp language: each status is a light wash ground carrying its own dark ink — the same print logic as the paper world, never a saturated colored pill. These tokens are sanctioned for future surfaces.

| Token | Value | Role | Contrast note |
| --- | --- | --- | --- |
| `success` / `success-wash` | `#2f5d46` / `#e3eee7` | Confirmed bookings: stamp text/ground, solid confirmed events | 6.4:1 on its wash (code-documented ≥4.5:1) |
| `pending` / `pending-wash` | `#8a5a12` / `#f6ecd9` | Pending bookings: stamp text/ground | 5.0:1 on its wash |
| `completed` / `completed-wash` | `#41597a` / `#e7edf4` | Completed bookings: stamp text/ground, solid completed events | 6.1:1 on its wash |
| Cancelled (no dedicated hue) | `secondary` + `ink-soft` | Cancelled bookings render neutral — muted ground, soft ink | 5.8:1 |

Solid calendar event blocks use per-event color/contrast pairs from `frontend/src/lib/calendar-events.ts`, all checked ≥4.5:1: open slot `#bc3e1b`/`#fff8f2` (5.2), confirmed `#2f5d46`/`#f2eee3` (6.5), pending `#9a6110`/`#fff8f2` (4.9), completed `#41597a`/`#f2eee3` (6.2), cancelled `#b6c2cb`/`#1f3049` (7.3), no-show `#b3362b`/`#fff8f2` (5.7), closed slot `#9fb2bd`/`#1f3049` (6.1).

**The Token Rule.** Every color on screen is a named token from this palette — no ad-hoc hexes.
**The One Spot Rule.** Coral is the single spot color; vermilion alone carries primary actions. They never swap jobs.
**The Warm-Red Rule.** Small coral text is never raw `coral` on a light ground — it is `coral-strong` (6.2:1); on the board it is `coral-on-board` (5.5:1). Raw `coral` is for large elements and decoration only.

## Typography

**Display Font:** Besley (Clarendon serif — the face of railway timetables and printed notices), loaded via `next/font` as `--font-besley`
**Body/UI Font:** Archivo (grotesk with transport-signage lineage), as `--font-sans`
**Mono Font:** Geist Mono, as `--font-mono`, always with tabular numerals for times and codes

**Character:** A timetable pairing — Besley gives headlines the authority of a printed notice; Archivo keeps dense UI copy legible and unromantic; Geist Mono makes every time value align to the column like a real departure board.

### Hierarchy

- **Display** (Besley 600, 3rem → sm 3.75rem → lg 4.25rem, line-height 1.05, tracking −0.025em): page hero headline only ("Time, kept honest.").
- **Headline** (Besley 600, 1.875rem → sm 2.25rem, tracking −0.025em): section titles.
- **Title** (Besley 600, 1.25rem; dialog titles 1.125rem): row headings and dialog titles.
- **Standfirst** (Archivo 400, 1.125rem, line-height 1.625, `ink-soft`, max-width ~lg): the one-line supporting sentence under a display headline.
- **Body** (Archivo 400, 0.875rem, line-height 1.625): UI copy, descriptions, feature bodies; longform constrained to max-w-xl/max-w-lg (≈60–70ch).
- **Label** (Archivo 600, 11px, letter-spacing 0.14em, uppercase): kickers, board header, footer column heads — coral-strong on paper, board-muted on the board, paper/50 on navy-deep.
- **Time** (Geist Mono 400, 0.875rem, `tabular-nums`): times, ranges, durations, numbered steps, codes.

**The Tabular Time Rule.** Every time, duration, and code is Geist Mono with tabular numerals — never proportional digits, never a serif or sans numeral.
**The Serif-Says-Important Rule.** Besley marks structural headlines only; UI copy, labels, and body text stay Archivo.

## Layout

A single-column editorial page inside a max-w-6xl (72rem) container with 16px gutters (sm: 24px). Marketing sections stack vertically, each closing on a 1px hairline rule; vertical rhythm is py-16 (64px), rising to py-24 (96px) on large screens. The hero is a two-column grid at lg (1.02fr / 0.98fr, gap 56px) — headline and CTAs in the reading path, the live navy board demonstrating the mechanism beside them.

The authenticated app uses one shell: a fixed 240px navy sidebar on desktop (logo row h-14, nav rows, footer row), and on mobile a paper top bar (h-14, sticky) plus a paper bottom nav (h-16, sticky, padded by `env(safe-area-inset-bottom)` for the iOS home indicator); main content clears the bottom nav with pb-24. Operate-mode density: board rows sit at py-[7px] on the 32px ruled texture; paper table rows at py-4 (16px) with divide-y hairlines.

**The Ruled Section Rule.** Hairline rules (1px `#d9dfe8` on paper, paper/15 on navy, board-rule on the board) close every viewport and separate every section and row — whitespace alone never does the separating.

## Elevation & Depth

Flat by default. Surfaces separate by hairline rules and tonal steps (paper → paper-raised; paper → navy → board), never by resting shadows. Exactly two shadows exist in the system, both structural: the departure-board panel lifts off the page with `0 16px 40px -24px rgb(15 31 56 / 0.5)`, and dialogs use Tailwind's `shadow-lg` over a 40% black overlay. Focus is expressed as coral rings (3px at 30–40% opacity on buttons/inputs, inset 2px on nav rows) or 2px coral outlines on marketing and board links. Motion is brief and color-led: 150ms ease-out enter / ease-in exit, dialogs zoom from 95% over 200ms; transitions are color-only. `prefers-reduced-motion` clamps every animation and transition to 0.01ms and disables smooth scrolling.

### Shadow Vocabulary

- **Board lift** (`0 16px 40px -24px rgb(15 31 56 / 0.5)`): the hero departure-board panel only — the one object that is "mounted" on the page.
- **Dialog** (Tailwind `shadow-lg` + `bg-black/40` overlay): modal surfaces.

**The Ruled, Not Raised Rule.** Depth is printed, not projected: surfaces separate by hairlines at rest; shadows appear only under the board and dialogs, never as a hover or card treatment.

## Shapes

Corners are print-squared. The radius scale is 2px (sm: stamps, logo tile, board Book links), 3px (md: buttons, inputs, nav rows), 4px (lg: cards, panels, calendar scrollgrid, board panel), 6px (xl), 8px (2xl, large overlays). Borders are 1px hairlines (`rule` on paper, `rule-strong` for inputs and leaders, translucent paper on navy); stamps and the brand button add a 1px inset ring in their own hue at 25–30%. The recurring silhouettes are the dotted leader (2px dotted `rule-strong`, raised 4px to the baseline; 35% board-ink on the board) and the board's ruled texture (1px line every 32px at 16% board-ink). Scrollers are thin with a `rule-strong` thumb.

**The Squared Corners Rule.** Corners are print-squared (2–4px working range); pills are reserved for count bubbles and round indicator dots — anything that carries text is a squared stamp or rectangle.

## Components

### Buttons

- **Shape:** squared 3px radius; heights 36px (default), 32px (sm, 13px text), 40px (lg), 36px square (icon); focus-visible ring 3px coral at 40%.
- **Primary (`default`):** vermilion `#bc3e1b` ground, warm paper-white `#fff8f2` text; hover drops to vermilion-deep `#9c3115`; disabled 50% opacity.
- **Brand (stamp):** coral-wash `#f9e2d9` ground, coral-strong `#a63a20` text, 1px inset coral/30 ring; hover coral at 20%. The tinted secondary action.
- **Outline:** paper-raised ground, 1px rule-strong border, ink text; hover accent wash.
- **Secondary:** `#eef1f5` ground, ink text, hover 70%. **Ghost:** transparent, hover accent. **Link:** coral-strong, underline on hover (offset 4px). **Destructive:** `#b3362b` ground, white text.

### Status stamps

Squared 2px stamps (px-2 py-0.5), 11px semibold uppercase with wide tracking, 1px inset ring in the status hue at 25%, and a near-square 6px dot (1px radius). Pairs: Confirmed = success-wash/success; Pending = pending-wash/pending; Completed = completed-wash/completed; Cancelled = neutral secondary/ink-soft with rule-strong ring; No-show = destructive at 10% ground with destructive text. Dark ink on its own wash, never a colored pill.

**The Stamp Rule.** Statuses render as squared uppercase stamps — dark ink on its wash — never as rounded colored pills.

### Cards / Containers

4px radius, paper-raised `#ffffff` ground, 1px rule border, py-6/px-6 (24px) internals, 24px gap between slots. No shadow at rest. Hairline-divided list containers (the availability table) use the same recipe with divide-y `rule`.

### Tabs

A ruled rail, not a pill track: the list is a 36px-tall row with a 1px rule underneath; each trigger sits borderless with a 2px transparent underline; active state turns the underline coral and the text ink; inactive text is ink-soft, hover ink. No rounding anywhere.

### Dialogs

4px radius, card ground, 1px rule border, p-6 (24px), 40% black overlay, max-w-lg on sm+. Enter is a 150ms fade with 95% zoom (200ms total). The title is Besley semibold at 1.125rem — one of the few serif moments in operate mode. Close is a 60%-opacity X top-right.

### Inputs

3px radius, 36px tall, paper-raised ground, 1px rule-strong border, px-3. Focus-visible: coral border + 3px coral ring at 30%. Invalid (`aria-invalid`): destructive border + destructive ring at 20%. Text selection inside inputs uses coral-wash.

### Departure board (signature component)

The navy panel: 4px radius, `board` ground, 1px navy-deep/60 border, the one structural shadow. A header row (11px uppercase 0.14em board-muted label + coral Live dot) sits above rows laid on the 32px ruled texture; each row is mono tabular time (board-ink; the first/next slot flagged coral-on-board), provider name in board-muted, a dotted leader, duration in 12px board-muted, and a bordered Book link (board-ink/30 border, 2px radius) that hovers to coral-on-board. A board-rule hairline closes the panel above an 11px timezone footer. The paper `table` variant repeats the row anatomy on white with rule hairlines, the first time in coral-strong.

### Calendar skin

FullCalendar v7 classic theme restyled through `--fc-classic-*` in `.fc-wrapper`: paper-raised surface, rule/rule-strong borders, faint `rgb(20 38 62 / 0.04)` tints, vermilion for primary/now-indicator, coral at 8% for today and 16% for highlight, button outline coral/50. The active view button (`aria-pressed="true"`) is vermilion with `#fff8f2` text; calendar buttons get a 2px coral focus outline (offset 1px); the scrollgrid clips at 4px; event chips ellipsize via the `.sf-event` hook. Above the grid sits the legend: a hairline-underlined row of 8px (2px-radius) color squares with 12px ink-soft labels, in display order (Open slot, Confirmed, Pending, Completed, Cancelled, No-show).

### Navigation

The one shell both sides share: navy sidebar on desktop, paper bars on mobile. Active nav rows are paper-on-navy — the selected line of the board (paper ground, ink text, coral-strong icon); idle rows are paper/70, hover white/10. Mobile mirrors it on paper: active ink, idle ink-soft, coral-strong icons, vermilion unread dots, a vermilion rounded-full count bubble for unread notifications. Focus on nav is an inset 2px coral ring.

### Logo

A 28px ink tile (2px radius) holding three 4px-wide slots — two ruled in board-ink `#f2eee3`, the center one coral `#e2543a`: the open slot is the coral one. `tone="inverse"` flips to a paper tile with ink slots for navy surfaces. Wordmark "SlotForge" at 15px semibold, tracking tight.

## Voice

Benefit-first and precise. Headlines state the outcome ("Customers book in two clicks", "The calendar stays honest"); the hero standsfirst names the mechanism without hype. Copy never implies state the backend doesn't confirm: availability copy is sourced live ("straight from the booking engine"), footers state the timezone ("Times shown in your timezone"), and error states name the cause plus the remedy ("Live availability is unreachable right now. Try again shortly."). Empty states invite rather than apologize ("Be the first provider on SlotForge — publish your availability and it appears here instantly."). No testimonials, metrics, or logos are invented — proof is the product demonstrating itself.

**The Engine-Speaks Rule.** Interface copy states only what the booking engine confirms — no invented availability, metrics, or endorsements.

## Accessibility

WCAG AA is the floor; every text pair below is checked. Ratios are computed per WCAG relative luminance; the code's own documented figures are noted where they differ.

| Pair | Use | Documented | Computed |
| --- | --- | --- | --- |
| ink `#14263e` / paper | Body text on page | — | 14.4:1 |
| ink-soft `#4c5f7a` / paper, white | Secondary text | ~6:1 | 6.1–6.5:1 |
| vermilion `#bc3e1b` / `#fff8f2` | Primary button label | 5.5:1 (vs white) | 5.2:1 |
| coral-strong `#a63a20` / paper | Small coral text on light grounds | 6.2:1 | 6.1:1 |
| coral-strong / coral-wash | Brand stamp label | 5:1 | 5.2:1 |
| coral-on-board `#f0765a` / board | Small coral text on the board | 5.5:1 | 5.6:1 |
| coral `#e2543a` / board | Large coral on board only | 4.2:1 | 4.2:1 |
| board-ink / board, board-muted / board | Board text | — | 13.6:1 / 7.2:1 |
| paper / navy | Sidebar text | — | 13.3:1 |
| success/success-wash, pending/pending-wash, completed/completed-wash, ink-soft/muted | Status stamps | all ≥4.5:1 | 6.4 / 5.0 / 6.1 / 5.8 |
| Solid event pairs (calendar-events.ts) | Event block text | all ≥4.5:1 | 4.9–7.3:1 |

Also in force: visible focus everywhere (3px coral rings on controls, 2px coral outlines on links, inset rings in nav); keyboard-operable flows with `aria-current` on nav, `aria-pressed` on calendar views, `aria-live` states; `prefers-reduced-motion` clamps all animation/transition to 0.01ms; text selection is coral-wash with ink text (13.6:1); safe-area padding keeps the mobile nav above the home indicator.

## Do's and Don'ts

### Do:

- **Do** use named tokens from `globals.css` for every color; the two-ground system (paper for reading, navy for the board) is the whole stage.
- **Do** set every time, duration, and code in Geist Mono with `tabular-nums`.
- **Do** close sections and separate rows with hairline rules (1px `#d9dfe8`; paper/15 on navy; board-rule on the board).
- **Do** flag the next/first live item in coral — coral-on-board on the navy board (5.5:1), coral-strong on paper (6.1:1).
- **Do** render statuses as squared uppercase stamps with dark ink on their wash.
- **Do** keep primary actions vermilion with warm paper-white `#fff8f2` text, hover vermilion-deep.
- **Do** let the board demonstrate the product: live data, tabular times, dotted leaders, working inline actions.

### Don't:

- **Don't** introduce ad-hoc hexes or a second accent; coral and vermilion never swap jobs.
- **Don't** use raw `coral` `#e2543a` for small text on light grounds (4.2:1 on the board, below AA on paper) — use `coral-strong` or `coral-on-board`.
- **Don't** round corners beyond the 2–8px scale or use pills for anything that carries text; pills are for count bubbles and dots only.
- **Don't** add resting shadows to cards or rows — depth comes from rules; shadows belong to the board panel and dialogs alone.
- **Don't** rebuild the category-default landing pattern (centered hero + three feature cards + logo band); composition is ruled rows and boards.
- **Don't** imply state the backend doesn't confirm — no mock availability, invented metrics, or request-queue language in a real-time slot product.
