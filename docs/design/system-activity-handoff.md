# Design Handoff: System Activity — "Under the Hood" (F21)

**Status**: Draft (developer handoff)
**Date**: 2026-06-14
**Spec**: [`docs/specs/system-activity.md`](../specs/system-activity.md) (F21)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [`theming.md`](theming.md) — **tokens only, QA all three skins**

This handoff covers the two user-facing surfaces in F21: the **activity page** (F21.4)
and the **header indicator** (F21.5), plus the **controls** (F21.6) and the
**owner-gated** rendering (F21.7). It resolves Open Question 1 (route + placement).

---

## Decisions (resolve Open Q1)

- **Route:** `/status` (top-level, peer of `/keys`). Nav label **"Status"**, added to
  the header `<nav>` after "Keys". Rationale: short and memorable; the page is
  owner-facing but is *primarily* a status view, so `/status` reads truer than `/admin`.
- **Header indicator:** a compact **pill in the header nav**, placed immediately left of
  the skin switcher. It is **present only when work is active** (collapses to nothing
  when idle) — keeps the chrome quiet, satisfies "tell the system is busy at a glance."

---

## Shared data layer (one source of truth)

Both surfaces read from a single client store so they never disagree and so SSE (F21.8)
can later swap in behind the same interface (spec Open Q6).

- **New:** `web/src/lib/activity.svelte.ts` — a `$state`-backed store exposing
  `activity` (latest read-model), `loading`, `error`, plus `start()/stop()` polling
  control. Polls `GET /api/v1/admin/activity` every **3s** while
  `document.visibilityState === 'visible'`; pauses when hidden; resumes on focus.
  Transport-agnostic: the polling impl is internal so F21.8 replaces it with an
  `EventSource` without touching consumers.
- **New api.ts methods:** `activity()`, `activityHistory(days = 30)`, `rescan()`,
  `reloadConfig()` — following the existing `get<T>` / POST pattern in
  [`web/src/lib/api.ts`](../../web/src/lib/api.ts).
- **New types.ts:** `Activity` (with `scan`, `thumbnails`, `library`, `system`,
  `capabilities`) and `JobRun`, mirroring the F21.1 / F21.3 payloads.

---

## Surface 1 — Header indicator (`ActivityIndicator.svelte`)

Mounted once in [`+layout.svelte`](../../web/src/routes/+layout.svelte) inside the
`<nav>`, before the skin-switcher `<div>`.

**Visibility rule:** render only when `scan.state === 'running'` **or**
`thumbnails.queue_depth > 0`. Otherwise render nothing.

**Markup (tokens only):**
```svelte
<a href="/status"
   role="status" aria-live="polite"
   class="flex items-center gap-1.5 rounded-theme border border-rule bg-surface-2 px-2 py-1 text-xs text-muted hover:text-ink">
  <span class="activity-dot h-2 w-2 rounded-full bg-accent" aria-hidden="true"></span>
  <span>{label}</span>
</a>
```

**Label logic** (concise, never both):
| Condition | Label |
|---|---|
| scan running | `Indexing…` |
| scan idle, queue > 0 | `{n} thumbnails` |

**Animation:** the dot pulses via a new `@keyframes activity-pulse` in `app.css`
(opacity/scale), attached to `.activity-dot`. It **must** be wrapped in
`@media (prefers-reduced-motion: no-preference)` exactly like the existing
`.video-grid` / `thumb-sweep` rules; with reduced motion the dot is a static
`bg-accent` circle. The dot is `bg-accent`, so it inherits each skin's accent
automatically — **no per-skin markup**. Optional cinémathèque glow goes in `app.css`
under `[data-theme='cinematheque'] .activity-dot` only.

**A11y:** the anchor is an `aria-live="polite"` `role="status"` region so screen
readers announce "Indexing…" when work starts and silence when it clears.

---

## Surface 2 — Activity page (`/status/+page.svelte`)

Page shell matches the established pattern (`/keys`): a centered section with a
`skin-title` heading and a muted subtitle. Use `AsyncState` for the
loading/error choreography (`loadingText="Loading activity…"`).

```
<section class="mx-auto max-w-5xl space-y-6">   ← 5xl (card grid is wider than /keys)
  <header> h1.skin-title "System Activity" + p.text-muted subtitle </header>

  [ controls-unauthenticated banner — conditional, see below ]

  <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">  ← status cards
    Scan · Thumbnails · Library · System
  </div>

  [ Controls row — conditional on capabilities.owner ]

  <section> Job history (last 30 days) </section>
</section>
```

### Status cards (`StatusCard.svelte`, generic)
Each card: `rounded-theme border border-rule bg-surface p-4 space-y-2`.
- Card label: `text-xs uppercase tracking-wide text-muted`.
- Primary value: `text-2xl font-semibold tabular-nums text-ink`.
- Secondary lines: `text-sm text-muted`.

| Card | Primary | Secondary / detail |
|---|---|---|
| **Scan** | `Running` (accent) / `Idle` | trigger; elapsed (running) or "last run 3m ago"; added/updated/removed/**errors** chips; "next ~in 4m" |
| **Thumbnails** | `queue_depth` | `high`/`normal` split; `in_flight`; `workers` |
| **Library** | `videos_active` | `+N inactive`; people; tags |
| **System** | `Ready` / `Not ready` | uptime; version; `media path: set/missing` |

- **Running scan** uses the accent treatment: the Scan card's primary value
  `text-accent` + a small reuse of the `.activity-dot`. Idle is `text-ink`.
- **Errors > 0** on the last run render as a chip with an accent ring
  (`border border-accent text-ink`) — visible, not alarming-red (we have no
  semantic "danger" token; accent is the attention color in all three skins).

### Controls (F21.6) — rendered only when `capabilities.owner` is true (F21.7)
A row of buttons under the cards:
- **Rescan library** — primary: `rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink`.
- **Reload config** — secondary: `rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2`.

**Confirm pattern (no native `confirm()` — it can't be themed):** click → the button
swaps in place to a two-option inline confirm ("Rescan? **Yes** / Cancel") for ~3s.
`Yes` fires the POST. This keeps keyboard focus in-flow and styles per skin.

**Result feedback:** a transient inline toast line under the row
(`text-sm text-muted`):
- rescan `202 started:true` → "Scan started."
- rescan `started:false` → "A scan is already running." *(informational, not an error)*
- reload → "Config reloaded — {fields} fields."

### controls-unauthenticated banner (security cond. 1, F21.7)
When `system.controls_unauthenticated === true`, show a banner above the cards:
`rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink` —
"⚠ Admin controls are reachable without a token on a non-loopback bind. Set
`ADMIN_TOKEN` to require authentication." This makes the fail-loud signal visible
in-product, per the spec/ADR-030 condition.

### Job history (last 30 days, F21.3) — `JobHistory.svelte`
Reuse the `/keys` **table** idiom (`w-full text-left text-sm`, `border-b border-rule`,
`text-xs uppercase tracking-wide text-muted` head, `tabular-nums` cells):

| When | Trigger | Duration | Added | Updated | Removed | Errors | Status |
|---|---|---|---|---|---|---|---|

- Newest first. `status === 'error'` row marks the Status cell with an accent-ring
  chip; `error_message` (if present) shows on a second, muted sub-row.
- **Empty state:** `No scans recorded yet.` (`py-16 text-center text-sm text-muted`),
  matching `/keys`.
- **Mobile:** wrap the table in `overflow-x-auto`; do not restyle into cards (keeps it
  one component, and history is a power-user view).

---

## Interaction & states checklist

| State | Treatment |
|---|---|
| Loading | `AsyncState` "Loading activity…" |
| Error (API unreachable) | `AsyncState` error box (`border-accent`) — never blank |
| Empty history | centered muted line |
| Scan running | accent value + pulsing dot, elapsed timer ticks each poll |
| Idle | neutral `text-ink`, header indicator absent |
| Non-owner / no token | controls hidden; cards + history still shown (read-only) |
| Reduced motion | dot static; no pulse |

---

## Responsive

- Cards: `grid gap-4 sm:grid-cols-2 lg:grid-cols-4` (1 col < 640px).
- Section width `max-w-5xl`; page padding inherited from `<main class="px-6 py-6">`.
- History table horizontally scrolls under ~640px (`overflow-x-auto`).
- Header indicator: label text may hide under `sm` (icon-dot only) using the same
  `hidden md:inline` trick the skin switcher already uses.

---

## Three-skin QA (required before merge — CLAUDE.md)

Render `/status` and trigger a running scan in **all three** skins:

- **Cinémathèque** (gold `#e8a33d`, radius 2px): pulsing dot glow reads on the warm
  surface; verify the accent-ring error chip is legible.
- **Broadcast** (cyan `#36e0d0`, radius 0): note `.skin-title::after` adds an accent
  underline to the heading — confirm it doesn't collide with the subtitle.
- **Brutalist** (lime `#d6ff3f`, radius 0): high-chroma accent — confirm the primary
  "Running" text and the `bg-accent` rescan button keep AA contrast (`accent-ink` is
  near-black `#0a0a0a`, so the button is fine; check `text-accent` on `bg-surface`).

Token self-check (must be empty for the new components):
`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`.
All color/radius/font via tokens; the pulse keyframes + any skin flourish live in
`app.css` under the hook class, never in component markup.

---

## Build sequence (suggested)

1. `types.ts` + `api.ts` additions (no UI yet).
2. `activity.svelte.ts` store (polling, visibility-aware).
3. `ActivityIndicator.svelte` → mount in `+layout.svelte`; add "Status" nav link.
4. `/status/+page.svelte` + `StatusCard.svelte` (cards + AsyncState).
5. Controls + confirm + unauthenticated banner (gated on `capabilities.owner`).
6. `JobHistory.svelte`.
7. `app.css`: `@keyframes activity-pulse` (reduced-motion gated) + optional per-skin
   `.activity-dot` flourish.
8. Three-skin QA + token self-check.

> Backend prerequisites (separate PRs): the `GET /admin/activity` read-model (F21.1,
> ADR-028) and the `requireOwner` gate + `capabilities` signal (F21.7, ADR-030) must
> exist first; until then the page can develop against a stubbed payload.
