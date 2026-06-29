# Design Handoff: Owner tooling hub + nav split (F35)

**Spec**: [Owner tooling hub (F35)](../specs/owner-tooling-hub.md) · **Gate**: [ADR-030](../architecture/ADR-030-access-control-gating-seam.md)
**Builds on**: [Admin Mode (F29)](../specs/admin-mode.md) + [its handoff](admin-mode-handoff.md) — same header, same toggle (here relabeled).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025). Reuses the chrome in
[`+layout.svelte`](../../web/src/routes/+layout.svelte) (skin picker, Preview toggle, `ActivityIndicator`).

---

## Overview

Two surfaces, one change of information architecture:

1. **Reworked header** — the content nav drops to **Media · People · Tags**; the three owner tools (Metadata
   keys, System Activity, Trash) leave the bar and are reached from a single **gear "Owner"** entry in the
   right-hand owner-chrome cluster. The F29 view toggle is **relabeled Preview / Owner view** — the word
   "Admin" leaves the UI, killing the two-controls-named-Admin collision.
2. **New `/owner` hub** — a tabbed shell (Status · Metadata keys · Trash) that holds the three relocated
   pages as nested routes. It is the home that future owner tooling joins as new tabs, so the header never
   grows again.

This handoff covers the **gear entry**, the **toggle relabel**, the **content-nav reduction**, and the
**hub shell + tab states**. It does **not** restyle the inner content of the three pages — they move under
`/owner/*` essentially unchanged.

### Design-system fit (the `/design-system` check)

No new tokens, no new primitive. Everything is assembled from chrome the header and pages already use:

- **Gear entry = the existing icon-control idiom.** Same `rounded-theme border border-rule` resting shell
  as the Preview toggle and skin-picker buttons, same `text-muted hover:text-ink`, same `px-2 py-1
  text-xs`. It sits in the same right-hand `<nav>` group, after the content-nav separator.
- **Active state = `text-accent` only.** When an `/owner` route is active, the gear indicates it with
  **`text-accent`** (the sanctioned active/primary semantic) — **not** a second solid `bg-accent` fill. The
  one solid-accent fill on screen stays reserved for the Preview toggle's ON state and for a page's single
  primary action.
- **Tabs reuse the skin-picker's active idiom.** The active tab is **`bg-surface-2 text-ink`** — exactly
  how the skin picker marks its active segment ([`+layout.svelte:258-261`](../../web/src/routes/+layout.svelte#L258)) —
  so the tab row needs no new "selected" treatment and does **not** consume the accent.
- **`skin-title` for the hub heading**, `font-display` via that hook, like every other page `<h1>`
  (e.g. [`keys/+page.svelte:22`](../../web/src/routes/keys/+page.svelte#L22)).
- **Owner gating** reuses `activity.effectiveOwner` (`isOwner && adminMode.enabled`), identical to today's
  Trash link and the F29 hide-set.

Audit output: **reuse the picker shell, `text-accent` active, `bg-surface-2 text-ink` tab active,
`skin-title`, and `effectiveOwner` verbatim; introduce nothing new.**

---

## Surface 1 — Header rework

### Layout & placement

Current bar (owner): `Holodex · search · [Media People Tags | Keys Status Trash] · (Activity) [Admin] [skins]`.
Target bar (owner): the **Keys/Status/Trash group is removed**; a **gear** joins the chrome cluster.

```
Holodex   [ search…………… ]   Media  People  Tags  | (Activity)  [👁 Owner view]  [⚙ Owner]  [ ◐ skins ]
                                └─ content nav ─┘  └──────────────── owner chrome ────────────────┘
                                                     ↑ all gated on effectiveOwner
```

- The content nav keeps its `nav.flex.items-center.gap-3` rhythm; just three links now.
- The owner-chrome cluster keeps the existing `border-l border-rule pl-3` separator that today wraps the
  "library tools" span — **reuse that separator** to divide content nav from chrome (it no longer wraps
  Keys/Status/Trash; it now opens the Activity/Preview/Owner/skins group).
- Order within chrome: `ActivityIndicator` → **Preview/Owner-view toggle** → **Owner gear** → skin picker.
  Rationale: the two *view-state* controls (Preview, then the owner-tools gear) sit together, with the skin
  picker (the other "view" control) at the far right as today.

### The gear entry — anatomy & states

**Element**: an `<a href="/owner">` styled as an icon button (it is navigation, not a toggle). Icon +
optional "Owner" text label.

**Icon**: inline SVG gear/settings glyph at `h-3.5 w-3.5`, `currentColor` (matches the toggle's `h-3.5`).
Use a cog; do **not** reuse the eye (that's Preview's metaphor).

| State | Visual | Notes |
|---|---|---|
| **Resting** | `border border-rule`, `text-muted`, gear icon + "Owner" label (≥`sm`) | Same outline as Preview-OFF / skin buttons. |
| **Hover** | `text-ink` | Matches `hover:text-ink` across the nav. |
| **Active** (on an `/owner` route) | `text-accent`, gear icon + label; border may stay `border-rule` or go `border-accent` | Indicates "you are in Owner" via accent **text**, not a fill. `aria-current="page"`. |
| **Focus-visible** | token focus ring (`focus:border-accent` / app focus utility), ≥2px, never removed | Keyboard users must see focus. |
| **Absent** | not in DOM | Non-owner, or Preview ON (visitor view) — gated on `effectiveOwner`. |

### The relabeled toggle (was "Admin mode", F29 P0-7)

**No structural change** — same `role="switch"`, same `bg-accent text-accent-ink` ON / muted-outline OFF,
same eye/eye-slash icon swap, same placement. **Only the strings change:**

| Property | Was (F29) | Now (F35) |
|---|---|---|
| Visible label (≥`sm`) | `Admin` | `Owner view` |
| `aria-label` | `Admin mode` | `Owner view` |
| `title` (ON) | `Admin mode on — switch to visitor view` | `Owner view — switch to visitor preview` |
| `title` (OFF) | `Visitor view — switch to admin mode` | `Previewing as visitor — switch to owner view` |
| `aria-live` announcement (auto-reveal) | `Admin mode on.` | `Owner view on.` |

> Semantics unchanged: ON = `aria-checked="true"` = owner view (controls visible); OFF = visitor preview.
> The internal `adminMode` store/key are **untouched** (spec Non-Goals) — this is a string-only relabel.

### Design tokens used (header)

| Token (utility) | Usage |
|---|---|
| `rounded-theme` | Gear button corner radius (skin-aware). |
| `border-rule` | Gear resting border; the content/chrome separator (`border-l`). |
| `text-muted` → `text-ink` | Gear resting → hover label/icon. |
| `text-accent` | Gear **active** state (on an `/owner` route). |
| `text-xs`, `px-2 py-1`, `gap-3` | Size/padding/rhythm — match the toggle & skin buttons. |
| `bg-accent` / `text-accent-ink` | **Unchanged** — Preview toggle ON only. |

**No literals.** Gear SVG uses `currentColor`. No `zinc-*`/`sky-*`/hex/named font/fixed radius.

---

## Surface 2 — `/owner` hub + tabs

### Layout

`/owner` renders a centered column matching the existing pages' `mx-auto max-w-4xl` (Keys) / page width —
**use `max-w-4xl`** so the hub aligns with the Metadata-keys table it contains. Structure:

```
┌ /owner ───────────────────────────────────────────────┐
│  Owner            (skin-title h1, font-display)        │  ← heading + one-line subtitle
│  Owner tools — visible only in your view.              │
│                                                        │
│  [ Status ] [ Metadata keys ] [ Trash ]   (tab row)    │  ← border-b border-rule under the row
│  ──────────────────────────────────────────────────   │
│                                                        │
│  «active nested route renders here»                    │  ← /owner/status | /owner/keys | /owner/trash
└────────────────────────────────────────────────────────┘
```

- **Heading**: `h1.skin-title.text-2xl.font-semibold.text-ink` "Owner" (matches Keys' `text-2xl` h1), with a
  `text-sm text-muted` subtitle "Owner tools — visible only in your view."
- **Tab row**: a horizontal list of links, `border-b border-rule` beneath, `gap-2`/`gap-3` between tabs.
- **Content slot**: the nested route's existing page markup, rendered via the `/owner` layout's
  `{@render children()}` (SvelteKit nested layout). The nested pages keep their own internal headings/tables
  as-is.

### Tab — anatomy & states

**Element**: each tab is an `<a href="/owner/{status|keys|trash}">` with `aria-current="page"` when active.

| State | Visual (token) | Notes |
|---|---|---|
| **Active** | `bg-surface-2 text-ink rounded-theme px-3 py-1.5` | Skin-picker active idiom. **No accent fill.** `aria-current="page"`. |
| **Inactive** | `text-muted hover:text-ink px-3 py-1.5` | Quiet, like inactive skin segments / nav links. |
| **Focus-visible** | token focus ring, ≥2px | Keyboard. |
| **Future tab** (P2 placeholder, if shown) | `text-muted border border-dashed border-rule rounded-theme`, non-interactive | Only if a "coming soon" affordance is desired; **omit in v1** unless a tab is genuinely stubbed. |

> **Do not** accent-fill the active tab. The page may host one primary action (e.g. Status's "Rescan" =
> `bg-accent text-accent-ink`); that is the single solid accent per view (CDS/ADR-021 restraint).

### Design tokens used (hub)

| Token (utility) | Usage |
|---|---|
| `skin-title`, `font-display`, `text-ink` | Hub `<h1>`. |
| `text-muted` | Subtitle; inactive tabs. |
| `bg-surface-2`, `text-ink` | **Active** tab. |
| `border-rule` | Tab-row underline (`border-b`); future-tab dashed border. |
| `rounded-theme`, `px-3 py-1.5`, `text-sm` | Tab shape/size. |
| `bg-accent` / `text-accent-ink` | Reserved for a page's single primary action only. |

---

## Interaction

| Trigger | Result |
|---|---|
| Click **Owner gear** | Navigate to `/owner` (default tab = **Status**). |
| Click a **tab** | Navigate to that nested route (`/owner/keys` …); active styling + `aria-current` move; no full-app reload (SvelteKit client nav within the group). |
| **Preview toggle** (relabeled) click / `Enter` / `Space` | Flip `adminMode.enabled`; persist; owner chrome (incl. the gear) and all gated surfaces re-render instantly; `aria-checked` updates. |
| Navigate to **any `/owner` route while in Preview** (visitor view) | **Auto-reveal at the group gate (P0-6)**: `adminMode` flips to owner view (persisted), the gear + hub render, `aria-live` announces **"Owner view on."** Fires **once at the `/owner` layout**, not per child. |
| Open old `/status` `/keys` `/trash` | Redirect to `/owner/*` equivalent (subject to the same gate/auto-reveal). |
| Non-owner opens `/owner*` | Redirect home (no owner data rendered); gear absent in DOM. |

---

## Per-skin QA (all three — load-bearing)

Render the **header** and the **hub** in each skin:

- **Cinémathèque** — gear `text-accent` (warm gold) reads against the header; the active tab's `surface-2`
  is distinguishable from the page `bg`; `skin-title` uses Fraunces.
- **Broadcast** — gear active accent (bright cyan) doesn't vibrate against the skin swatches; the VT323
  `skin-title` uppercases "Owner"; the active tab's `surface-2` vs `surface` separation is visible (the two
  are close in this skin — verify the active tab still reads as selected).
- **Brutalist** — `rounded-theme` → square; the tab-row `border-b` and gear border match the skin's heavier
  rule weight; lime `text-accent` active gear is legible; mono `skin-title`.

In **all three**: with **Preview ON (visitor view)**, the gear and any `/owner` content are **absent** and
the bar is exactly Media/People/Tags + search + activity + skins — the clean visitor surface.

---

## Responsive

| Breakpoint | Header behavior | Hub behavior |
|---|---|---|
| Desktop (≥ `sm`) | Gear shows icon + "Owner" label, like the skin picker's active label. | Tabs in a single row. |
| `< sm` | Gear **drops the label, keeps the icon** (`hidden sm:inline` on the label span) — mirrors the skin-picker / Preview precedent. `aria-label="Owner tools"` keeps the name. | Tab row may wrap; tabs stay `px-3 py-1.5`, no horizontal scroll. Heading/subtitle stack. |

The three icons in the chrome cluster (activity · eye · gear) sit adjacent below `sm`; keep their
`aria-label`/`title` so each is distinguishable to AT and on hover (the recognition risk flagged in review).

---

## Edge cases

- **Non-owner**: gear absent; `/owner*` redirects home; bar is the public surface.
- **Preview ON, direct `/owner` URL**: auto-reveal (above) — never a dead/empty hub.
- **Old bookmark** (`/status` etc.): redirects into `/owner/*`; F29 docs/tests referencing `/trash` still
  resolve via the redirect.
- **Unknown `/owner/*` child**: fall through to the hub default (Status) or a themed not-found within the
  shell — pick one; don't render a bare error.
- **Empty/loading inner pages**: unchanged — each page keeps its own loading/empty/error states (e.g. Keys'
  "No extended metadata captured yet."). The hub shell itself has no data dependency, so the tab row renders
  immediately even while a tab's content loads.
- **localStorage blocked**: Preview falls back to in-memory owner-view default (mirrors `theme.svelte.ts`);
  never throws.
- **Long localized labels**: "Owner" / tab labels are short; `gap-3` nav + `sm:inline` hide absorb growth.

---

## Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Gear / tabs | Hover / active | Color transition (`transition` utility) | ~150ms (Tailwind default) | default |
| Tab content | Tab switch | **None by default** — route swap, no cross-fade | 0 | — |
| Owner chrome | Preview flip | Instant DOM add/remove (per F29); optional reduced-motion-safe fade is P1 | 0 / ≤150ms | ease-out |

Honor `prefers-reduced-motion` for any optional fade.

---

## Accessibility

- **Gear**: an `<a>` with `aria-label="Owner tools"`, `aria-current="page"` when on an `/owner` route.
  Keyboard-reachable in the nav focus order: `ActivityIndicator` → Preview toggle → **Owner gear** →
  skin-picker buttons.
- **Tabs**: links with `aria-current="page"` on the active tab. If implemented as an ARIA tablist, use
  `role="tablist"`/`role="tab"`/`aria-selected` + **roving tabindex** (arrow-key nav, single tab-stop) per
  the project's keyboard-list convention — but since each tab is a real route, simple `<a>` + `aria-current`
  is acceptable and preferred (browser handles focus on navigation). **Pick one**; if tablist, wire roving
  tabindex and Left/Right arrows.
- **Focus on navigation**: after a tab/gear click, focus follows normal SvelteKit nav; ensure the hub
  heading is reachable (a focusable `<h1 tabindex="-1">` or the framework's announce) so screen-reader users
  learn where they landed.
- **Preview relabel**: `aria-checked` still carries on/off; the **auto-reveal** path announces "Owner view
  on." via the existing visually-hidden `aria-live="polite"` region (string updated from F29).
- **Don't rely on color alone**: the gear's active state pairs `text-accent` with `aria-current`; the active
  tab pairs `bg-surface-2` with `aria-current`/`aria-selected` — never accent/contrast alone.
- **Contrast**: `text-accent` on the header `bg` and `text-ink` on `bg-surface-2` must meet AA in all three
  skins (Broadcast tightest).

---

## Implementation pointers (non-binding)

- **Routes**: create `web/src/routes/owner/+layout.svelte` (the hub shell: heading + tab row +
  `{@render children()}`) and `owner/+page.svelte` (redirect to `owner/status` or render the Status tab as
  index). Move `status`, `keys`, `trash` page files under `owner/`. Add redirects from the old top-level
  routes (`+page.ts` `redirect(308, '/owner/status')` etc.).
- **Group gate + auto-reveal**: in `owner/+layout` load/onMount — `if (!activity.isOwner) redirect home`;
  `if (activity.isOwner) adminMode.set(true)` (auto-reveal, fired **once here**, removed from the individual
  pages so it isn't duplicated). Reconcile with `/status`'s token-unlock path (the way back in) per spec.
- **Header**: in [`+layout.svelte`](../../web/src/routes/+layout.svelte) remove the Keys/Status/Trash
  `<span class="grp">`; add the gear `<a href="/owner">` into the chrome cluster gated on
  `activity.effectiveOwner`; relabel the toggle strings (P0-7 table above). Active state via
  `$page.url.pathname.startsWith('/owner')`.
- **Tab active**: `aria-current={active ? 'page' : undefined}` + the `bg-surface-2 text-ink` class toggle,
  reusing the exact pattern at the skin-picker buttons.
- **Token guard** stays empty: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`.

> Paired QA checklist: [`owner-tooling-hub-qa-checklist.md`](owner-tooling-hub-qa-checklist.md) — numbered,
> grouped by verifier (Setup / Smoke / Agent / Human), all three skins.
