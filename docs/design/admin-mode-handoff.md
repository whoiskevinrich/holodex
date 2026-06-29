# Design Handoff: Admin Mode toggle (F29)

**Spec**: [Admin Mode (F29)](../specs/admin-mode.md) · **Gate**: [ADR-030](../architecture/ADR-030-access-control-gating-seam.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025). Mirrors the skin picker in
[`+layout.svelte`](../../web/src/routes/+layout.svelte) and the `theme.svelte.ts` store pattern.

---

## Overview

A single header control, **rendered only for the owner**, that turns **Admin mode** on or off. ON =
owner-only controls and data are visible (today's behavior). OFF = a faithful **visitor view**: every
owner-only control *and* data surface is removed from the DOM, so the owner sees exactly what a logged-out
visitor sees — for QA across the three skins and for distraction-free browsing. It mirrors the dark-mode /
skin toggle the owner already knows: per-device `localStorage`, reactive, no reload, no privilege change.

This handoff covers **one new control** plus the **visual contract for how each owner-gated surface
appears/disappears**. It does not restyle any of the gated controls themselves — they already exist.

### Design-system fit (the `/design-system` check)

No new tokens and no new primitive. The control is built from chrome the header already uses:

- **Same shell as the skin picker** — `rounded-theme border border-rule`, sitting as its sibling in the
  right-hand `<nav>` group. It is a **binary switch**, not a multi-option segmented control, so it reads as
  one button (the skin picker's 3-way segmented shape stays reserved for the 3-way choice next to it — a
  meaningful "binary = switch, multi = segmented" distinction).
- **Active/primary treatment = `--accent`** — when Admin mode is ON, the button uses
  `bg-accent text-accent-ink` (the established active/primary semantic, ADR-021). OFF uses the muted
  outline resting style (`text-muted`, transparent fill) the other nav controls use. The accent fill
  doubles as the **persistent "you have powers on" indicator**.
- **Owner gating** — `activity.isOwner`, identical to the Trash link beside it.

Because every piece already exists, the audit output is: **reuse the picker's shell, the `bg-accent
text-accent-ink` active idiom, and `activity.isOwner` verbatim; introduce nothing new.**

---

## Layout & placement

In the header's right-hand `<nav>` (the "tools" group), place the toggle **between `ActivityIndicator`
and the skin picker** — grouping the two view-preference controls (Admin mode, then skin) at the far
right. Reference: [`+layout.svelte:181`](../../web/src/routes/+layout.svelte#L181).

```
… Keys  Status  [Trash]  | (ActivityIndicator)  [⦿ Admin]  [ ◐ skin segmented ]
                  ↑ hides when OFF            ↑ new control   ↑ existing
```

- Spacing: it's a child of the existing `nav.flex.items-center.gap-3`, so the `gap-3` rhythm applies —
  **no custom margins.**
- The control's internal padding matches the skin-picker buttons: `px-2 py-1`, `text-xs`.

---

## Design tokens used

| Token (utility) | Usage |
|---|---|
| `rounded-theme` | Button corner radius (skin-aware). |
| `border-rule` | Resting 1px border (OFF state). |
| `bg-accent` / `text-accent-ink` | Fill + label when Admin mode is **ON**. |
| `text-muted` | Label/icon when **OFF** (resting). |
| `text-ink` | Label/icon on hover when OFF. |
| `text-xs` | Label size (matches skin picker). |
| `transition` | Color transition on state/hover (matches skin picker). |

**No literals.** No `zinc-*`/`sky-*`/hex, no fixed `rounded-lg`/px radii. Icon is an inline SVG using
`currentColor` so it inherits the token color in every state and skin.

---

## The control — anatomy & states

**Element**: a single `<button>` acting as a switch. Icon + the always-visible text label **"Admin"**
(unlike the skin picker, which hides inactive labels — there's only one control here, so its label is
always shown for clarity).

**Icon**: an inline SVG eye/shield glyph at `h-3.5 w-3.5`, `currentColor`. Suggestion: an **eye** that
reads as "what's visible" — open eye when ON, eye-with-slash when OFF — or a shield/wrench if an
admin-tools metaphor is preferred. Match whatever ActivityIndicator already does for icons if it uses a set;
otherwise inline SVG. (Designer's call — pick one and use it in the P1 indicator too.)

| State | Visual | Notes |
|---|---|---|
| **ON (default)** | `bg-accent text-accent-ink`, no border (or `border-transparent`), open-eye icon | Admin elements visible. Accent fill = active/primary + persistent indicator. |
| **OFF (visitor view)** | transparent fill, `border border-rule`, `text-muted`, eye-slash icon | Resting outline like other nav items. |
| **Hover (OFF)** | `text-ink` (icon + label brighten) | Matches `hover:text-ink` used across the nav. |
| **Hover (ON)** | slight emphasis only — keep `bg-accent`; do **not** invert | It's already the active treatment; avoid a hover that reads as "off." |
| **Focus-visible** | token focus ring (`focus:border-accent` or the app's focus-ring utility), ≥2px, never removed | Keyboard users must see focus. |
| **Active/press** | standard button press (no custom) | — |
| **Disabled** | n/a | The control is either rendered (owner) or absent (non-owner); it is never disabled. |
| **Absent** | not in DOM | Non-owner, or owner status dropped/expired. |

---

## Interaction

| Trigger | Result |
|---|---|
| Click / `Enter` / `Space` | Flip `adminMode.enabled`; persist to `localStorage`; all owner-gated surfaces re-render instantly (no reload). `aria-checked` updates. |
| Hover (OFF) | Label/icon brighten to `text-ink`. |
| Navigate to an **owner-only route** while OFF (e.g. `/trash`) | **Auto-reveal (P0-6)**: `adminMode` flips to ON (persisted), the control shows its ON state, the page renders fully, and an `aria-live="polite"` region announces **"Admin mode on."** |
| Owner token cleared / expires | Control unmounts (no longer owner). Stored preference is retained for the next authenticated session. |

**How gated surfaces appear/disappear**: each owner-only control/datum is gated on the effective
`activity.isOwner && adminMode.enabled` and is **removed from the DOM** (not `opacity`/`visibility` hidden)
when OFF, so layouts **reflow naturally with no reserved gaps or placeholder skeletons**. Toggling is a
pure state change — no spinner, no loading state. The bulk show/hide is **instant by default**; an
optional reduced-motion-friendly fade is a P1 nice-to-have (see Motion).

---

## Per-skin QA (all three — load-bearing)

Render and eyeball the control **and a previewed page** in each skin, in **both** states:

- **Cinémathèque** — confirm the ON accent fill has enough contrast against the header; the eye icon reads
  at `h-3.5`.
- **Broadcast** — confirm `text-accent-ink` on `bg-accent` is legible (Broadcast's accent is brightest);
  no collision with the adjacent skin swatches.
- **Brutalist** — confirm `rounded-theme` resolves to the skin's sharper radius and the border weight
  matches neighboring controls; the OFF outline shouldn't look heavier than the skin picker's.

In **all three**, with Admin mode **OFF**, verify a representative gated page (e.g. `/media/[id]`) shows
**zero** owner-only controls/badges and the layout has no orphaned gaps — it must read as the public page.

---

## Responsive

| Breakpoint | Behavior |
|---|---|
| Desktop (≥ `sm`) | Icon + "Admin" label, as specified. |
| `< sm` | Follow the skin picker's precedent (`{#if active}<span class="hidden sm:inline">`): **drop the text label, keep the icon** so the header stays compact. The accent fill still conveys ON/OFF, so the icon-only control remains unambiguous. Keep the `aria-label="Admin mode"` for the accessible name. |

No layout reflow of the header beyond label hide; the control stays in the same nav slot.

---

## Edge cases

- **Non-owner**: control absent; app identical to today.
- **First authentication, no stored value**: defaults **ON** — gaining admin visibly does something.
- **Stored OFF, then authenticate**: starts in visitor view (preference honored); owner toggles ON when
  ready to curate.
- **Direct-URL to owner-only route while OFF**: auto-reveal (above) — never show an empty/forbidden page.
- **Rapid toggling**: pure reactive state; safe to flip repeatedly with no debounce.
- **localStorage unavailable/blocked**: fall back to in-memory default ON for the session (mirror
  `theme.svelte.ts`'s `typeof localStorage !== 'undefined'` guard); never throw.
- **International / long label**: label is the fixed string "Admin" — no truncation concern; if localized
  later, the `gap-3` nav and `sm:inline` hide absorb a longer word.

---

## Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Toggle button | State/hover change | Color/background transition (the existing `transition` utility) | ~150ms (Tailwind default) | default |
| Gated surfaces | Toggle OFF/ON | **None by default** — instant DOM add/remove | 0 | — |
| Gated surfaces (P1) | Toggle | Optional short fade/opacity, **gated behind `prefers-reduced-motion: no-preference`** | ≤150ms | ease-out |

Honor `prefers-reduced-motion`: the optional fade must not run when reduced motion is requested.

---

## Accessibility

- **Role**: `<button>` with `role="switch"` and `aria-checked={adminMode.enabled}` (ON = `true`). A switch
  is the correct semantic for a binary on/off mode.
- **Accessible name**: `aria-label="Admin mode"` (covers the icon-only `<sm` variant). The visible "Admin"
  label is the name on ≥`sm`.
- **Keyboard**: reachable via `Tab`; toggled with `Enter` **and** `Space`. Focus order in the nav:
  …`ActivityIndicator` → **Admin toggle** → skin-picker buttons. Self-toggling never moves focus (the
  control persists), so there's no focus-loss when surfaces hide.
- **State announcement**: changing `aria-checked` announces the new state. For the **auto-reveal** case
  (state changes from navigation, not from the control), add a visually-hidden `aria-live="polite"` region
  that announces **"Admin mode on."** so the change isn't silent.
- **Contrast**: ON state must meet AA — `text-accent-ink` on `bg-accent` is the system's designated
  on-accent pair; verify per skin (Broadcast is the tightest).
- **Don't rely on color alone**: the icon swap (open-eye ↔ eye-slash) and the `aria-checked` state both
  carry the on/off meaning alongside the accent fill.

---

## Implementation pointers (non-binding)

- New store `web/src/lib/adminMode.svelte.ts`, a near-copy of `theme.svelte.ts`: `enabled = $state(true)`,
  `init()` reads `localStorage['holodex-admin-mode']`, `toggle()`/`set(v)` persist. Default ON.
- Call `adminMode.init()` in the layout's mount beside `theme.init()`.
- Effective gate helper, e.g. `const showAdmin = $derived(activity.isOwner && adminMode.enabled)`, used at
  every site currently testing `activity.isOwner` for a **control or owner-only datum** (per the spec's
  P0-4 hide-set). Auth-token entry on `/status` stays on bare `isOwner` only where it's the path back in —
  reconcile with P0-6 during build.
- Auto-reveal: in the owner-only route's `+page`/`+layout` load or onMount, `if (activity.isOwner)
  adminMode.set(true)`.

> Paired QA checklist: [`admin-mode-qa-checklist.md`](admin-mode-qa-checklist.md) — numbered, grouped by
> verifier (Setup / Smoke / Agent / Human), all three skins.
