# Spec: Owner tooling hub + visitor/owner nav split (F35)

**Status**: Draft
**Phase**: Post–Phase 2 polish (nav information architecture over the owner gate)
**Date**: 2026-06-29
**Owner**: Project owner

**Depends on**: the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)) — `activity.isOwner` / `effectiveOwner`;
the Admin-mode toggle and its store ([Admin Mode F29](admin-mode.md), `web/src/lib/adminMode.svelte.ts`);
the theming/token system ([ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).
**Related surfaces (folded into the hub)**: System Activity `/status` ([F21](system-activity.md)),
Metadata keys `/keys`, Trash `/trash` ([F24](delete-media.md)).
**Supersedes a follow-up of**: [F29](admin-mode.md) explicitly parked "a consolidated **Admin page**
(folding `/status`, `/trash`, `/keys` into one tabbed page)" in `TASKS.md` — **this spec is that work**,
with one deliberate change: the area is named **Owner** (`/owner`), not "Admin", and F29's "Admin mode"
toggle is renamed **Preview / Owner view** (see Resolved Decisions #3).
**Design handoff**: `docs/design/owner-tooling-hub-handoff.md` (to be written — `/design-handoff`).
**Triggers before merge**: `/design-handoff` (UX surface), `/security-review` (access/owner-gating),
`/testing-strategy` (route gating + nav change), `/simplify`.

---

## Problem Statement

The global header (`web/src/routes/+layout.svelte`) carries ~11 interactive targets in a single flat row,
and the owner's operational tooling — **Metadata keys** (`/keys`), **System Activity** (`/status`), and
**Trash** (`/trash`) — sits as peer text links styled **identically** to the primary content nav
(Media · People · Tags). Two problems follow. **(1) A tier confusion:** "how I operate the library" looks
exactly like "where I browse," so the bar reads as one undifferentiated list and feels crowded. **(2) A
visibility leak:** `/keys` and `/status` render their nav links for **everyone** — only `/trash` is
owner-gated — so a visitor to the library currently sees owner operational links that aren't theirs.
Compounding both, the word **"Admin"** is already overloaded: the F29 view toggle is labeled "Admin", and
the obvious "collapse admin tooling" instinct would add a *second* "Admin" (the tooling entry point) inches
away, meaning something entirely different (a destination vs. a view state).

## Goals

1. **One owner-tooling home.** Metadata keys, System Activity, and Trash live in a single **Owner area at
   `/owner`** (a hub with nested tabbed routes), reached from one entry point in the header — so the content
   nav is just Media · People · Tags.
2. **Owner-only, end to end.** Every owner-tooling surface — the entry point, the `/owner` hub, and each
   nested route — is hidden from visitors and gated server-side. The current `/keys` and `/status` link
   leak is closed.
3. **A bar that reads in tiers.** The header resolves into three visual altitudes — brand · content nav ·
   owner chrome — so operational tooling no longer masquerades as primary navigation.
4. **Kill the "Admin" × 2 collision.** Exactly one meaning per label: the view toggle becomes **Preview /
   Owner view**; the tooling area is **Owner**. The word "Admin" disappears from the user-facing surface.
5. **Grow without touching the nav again.** Future owner surfaces (enrichment runs, config, plugin
   management, writeback history) become new tabs under `/owner`, never new header links.
6. **Keep all three skins clean.** The reworked bar and the `/owner` hub render correctly in Cinémathèque,
   Broadcast, and Brutalist using semantic tokens only.

## Non-Goals

- **Not changing what the three pages do.** `/keys`, `/status`, and `/trash` keep their current behavior,
  data, and actions; they are **relocated** under `/owner/*` and reachable as tabs, not redesigned.
  *(Why: this is an information-architecture change, not a feature change — bundling a redesign in would
  bloat scope and risk.)*
- **Not renaming the internal `adminMode` store now.** Only **user-facing strings** change to
  Owner/Preview. The `adminMode` store, its `localStorage` key, and F29's code identifiers keep their names
  this change (Resolved Decisions #6). *(Why: a rename touches every owner-gated surface plus a
  localStorage migration — disproportionate to a nav change; tracked as tech-debt.)*
- **Not touching search or the skin picker.** Both stay exactly where and how they are. *(Why: out of the
  problem's scope; they're not owner tooling.)*
- **Not a new server permission model.** Reuses the existing `requireOwner` gate and `effectiveOwner`
  client gate; no new roles, capabilities, or auth flow. *(Why: ADR-030's seam already covers this; the
  change is which routes sit behind it.)*
- **Not multi-user / per-account.** Single-owner model unchanged. *(Why: no accounts today.)*

## User Stories

**As the library owner:**
- I want my operational tools (Status, Metadata keys, Trash) in one place I reach from the header, so the
  top bar isn't cluttered and I'm not hunting across peer links.
- I want a single **Owner** entry in the header's chrome cluster (next to Preview and the skin picker), so
  owner tooling visually reads as *my* controls, distinct from Media/People/Tags.
- I want to switch between Status, Metadata keys, and Trash as **tabs** within `/owner`, so moving between
  my tools doesn't bounce me back to the top of the app.
- I want new owner tools to show up as new tabs here later, so the header never grows again.

**As a visitor (or the owner previewing the visitor view):**
- I want to see only Media, People, Tags, and search — no Owner entry, no owner routes — so the library
  reads as a clean public catalog and nothing owner-only leaks.

**Edge / boundary cases:**
- When **not** owner, the Owner entry is absent and `/owner*` routes are not reachable (server 401s, client
  redirects).
- When the owner is in **Preview** (visitor view) and opens an `/owner` route directly by URL, the app
  **auto-reveals** owner view once, at the `/owner` gate, so the page is usable (Resolved Decisions #4).
- A bookmark to the old `/status`, `/keys`, or `/trash` path should not dead-end (see P0-5 redirects).

## Requirements

### Must-Have (P0)

**P0-1 — `/owner` hub with nested tabbed routes.**
A new route group: `/owner` (the hub landing) with children `/owner/status`, `/owner/keys`,
`/owner/trash`. The hub renders a tabbed shell — a `skin-title` "Owner" (or "Manage library") heading and a
tab row (Status · Metadata keys · Trash) — with the active child's page rendered below. The three existing
pages move under this group; their `<script>`/content move essentially unchanged.
- Given I am owner and visit `/owner`, then I see the hub with tabs and a default tab selected (Status).
- Given I click a tab, then the corresponding nested route renders without a full reload and the URL
  reflects it (`/owner/keys` etc.).
- Tabs are real links (keyboard-reachable, `aria-current` on the active tab), not div-onclick.

**P0-2 — Header entry point in the owner-chrome cluster.**
A single **Owner** entry in the header's right-hand chrome group (the border-separated cluster with
`ActivityIndicator`, the Preview toggle, and the skin picker) — **not** a peer of Media/People/Tags. It is a
**gear icon** (`ti-settings`-equivalent) that shows an "Owner" text label at `≥sm` and collapses to
icon-only below `sm`, mirroring the skin picker's and Preview toggle's existing responsive label behavior
(Resolved Decisions #5). Rendered only on the effective owner gate (`activity.isOwner && adminMode.enabled`).
- Given I am owner in owner view, when the header renders, then the Owner gear appears in the chrome
  cluster, to the right of the content nav's separator.
- Given the viewport is below `sm`, then the gear shows icon-only (label hidden), like the skin picker.
- Given the Owner route is active, then the gear shows its active state via **`text-accent`** (the
  sanctioned active/primary semantic) — not a second solid fill.
- Tokens-only styling; `aria-label="Owner tools"`; a real link/button with an accessible name.

**P0-3 — Content nav reduced to three; tier separation.**
The content nav is exactly **Media · People · Tags**. Keys/Status/Trash are removed as header links (now
reached via the Owner gear). The owner-chrome cluster (activity · Preview · Owner · skins) stays visually
separated from the content nav by the existing `border-l border-rule` divider.
- Given the header renders for an owner, then Media/People/Tags are the only content-nav links and the
  former Keys/Status/Trash links are gone from that row.

**P0-4 — Owner-only, end to end (closes the leak).**
The Owner gear, the `/owner` hub, and every nested route are owner-only at both layers:
- **Client:** the gear renders only on `effectiveOwner`; `/owner*` routes, if reached in visitor view,
  auto-reveal (P0-6); if reached by a non-owner, redirect home.
- **Server:** the data behind Metadata keys / System Activity / Trash sits behind `requireOwner`. Activity
  and Trash were already gated; `/security-review` found **`GET /metadata-keys`** registered *outside* the
  owner group (a public F20-era exposure — anonymous callers could enumerate raw container keys + sample
  values on an exposed bind). It is now moved **inside** `requireOwner` (`internal/api/handlers.go`),
  covered by `auth_test.go`. The former public exposure of the `/keys` and `/status` **link visibility** is
  also closed.
- Given I am a visitor, when I load any page, then no Owner gear and no `/owner*` content are present in the
  DOM, and a direct `/owner/keys` URL does not render owner data.
- **Triggers `/security-review`** before merge (touches the access/owner-gating surface).

**P0-5 — Old paths redirect (no dead bookmarks).**
`/status`, `/keys`, and `/trash` redirect to their `/owner/*` equivalents (preserving the owner gate /
auto-reveal). This keeps existing links, the F29 auto-reveal references, and any docs pointing at `/trash`
working.
- Given I open `/status` (old path), then I land on `/owner/status` (subject to the owner gate).

**P0-6 — Auto-reveal at the `/owner` gate (single rule).**
If Preview is ON (visitor view) and the owner navigates to **any** `/owner` route directly, owner view
auto-reveals **once, at the `/owner` group level** (not per nested route) — inheriting and consolidating
F29's P0-6 behavior (Resolved Decisions #4). The Preview toggle reflects the new state.
- Given Preview ON, when I open `/owner/trash` by URL, then owner view turns on, the page renders fully,
  and the toggle shows owner view.
- The nested children do **not** each re-implement auto-reveal; the parent gate is the sole place it fires.

**P0-7 — Rename the F29 toggle to Preview / Owner view (user-facing only).**
The header view toggle's **label, title, and `aria-label`** change from "Admin" / "Admin mode" to
**Preview / Owner view** wording (e.g. switch labeled "Owner view"; titles "Owner view — switch to visitor
preview" / "Previewing as visitor — switch to owner view"). The internal `adminMode` store and key are
**unchanged** (Non-Goals; Resolved Decisions #6). No two controls share the word "Admin" — in fact "Admin"
leaves the UI entirely.
- Given the header renders for an owner, then the view toggle reads in Owner/Preview terms and the word
  "Admin" appears nowhere in the header.

**P0-8 — Tokens-only, all three skins, including the active-tab fix.**
Every new surface (gear, hub heading, tab row, active/inactive tab states) uses semantic tokens only — no
`zinc-*`/`sky-*`/hex/named fonts/fixed radii. The **active tab uses `bg-surface-2 text-ink`** (matching the
skin picker's active-segment precedent), reserving the single solid `bg-accent` for a page's one primary
action (e.g. Status's "Rescan"). The `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)'` check stays
empty.
- Given any of the three skins, when I view `/owner` and each tab, then tokens/radius/contrast read
  correctly, the active tab is distinguishable without a second accent fill, and nothing overflows or
  collides (including the `--radius: 0` square treatment in Broadcast/Brutalist).

### Nice-to-Have (P1)

- **P1-1 — Hub landing summary.** `/owner` (bare, no tab) shows a compact at-a-glance: library counts,
  scanner state, last job — a light dashboard above/instead of defaulting straight into the Status tab.
  *(Fast follow; default-to-Status covers v1.)*
- **P1-2 — Deep-link active tab persistence.** Returning to `/owner` remembers the last tab viewed
  (localStorage), like the sort-preference pattern. *(Polish.)*
- **P1-3 — Keyboard tab nav.** Left/Right arrow moves between tabs when the tab row is focused (roving
  tabindex), consistent with the project's keyboard-list convention.

### Future Considerations (P2)

- **P2-1 — New owner tabs:** Enrichment runs, Config, plugin/provider management, writeback history — each
  a new child route under `/owner`, no header change. *(This is the payoff of choosing a page over a menu.)*
- **P2-2 — Internal `adminMode` → `ownerView` rename** (store, key + migration, all gated surfaces), to
  realign code vocabulary with the UI. Deferred from this change (Non-Goals).
- **P2-3 — Per-account owner area** if multi-user ever lands — the `/owner` group becomes the natural
  settings/admin root behind a profile.

## Success Metrics

Single-user personal server — metrics are usage-quality signals, not funnel numbers.

**Leading indicators (immediate):**
- Header content nav is exactly 3 items; owner tooling is reachable in **one** entry-point click from any
  page. The former `/keys` + `/status` link leak to visitors → **0**.
- Switching among Status/Keys/Trash no longer triggers a full top-of-app reload (tabbed, in-group nav).

**Lagging indicators:**
- New owner tooling ships as a `/owner` tab with **zero** header/nav edits (the IA holds as the backlog
  lands).
- **Zero** security findings attributable to the relocation in `/security-review` (server gate unchanged;
  the change only removes public *link visibility* and adds route gating).

## Open Questions

*All resolved 2026-06-29 during spec intake — recorded for traceability.*

1. **RESOLVED — route shape: nested under `/owner`.** Pages move to `/owner/status`, `/owner/keys`,
   `/owner/trash` (hub at `/owner`) as real nested tabs, **not** kept at top level and merely linked. The
   area is **Owner**, deliberately avoiding "Admin" so it shares vocabulary with the Preview toggle. Old
   paths redirect (P0-5). *(See P0-1/P0-5.)*
2. **RESOLVED — visitor scope: hide all three.** Metadata keys, System Activity, and Trash are owner-only
   end to end; visitors see only Media/People/Tags + search. Closes the current `/keys` + `/status` leak.
   *(See P0-4.)*
3. **RESOLVED — kill the "Admin" × 2 collision.** The F29 view toggle is renamed **Preview / Owner view**
   (user-facing strings only); the tooling area is **Owner**. "Admin" leaves the UI. Internal store name
   unchanged. *(See P0-7, Non-Goals.)*
4. **RESOLVED — auto-reveal at `/owner` only.** Hitting any `/owner` route in Preview auto-reveals owner
   view once at the group gate; nested routes don't each re-implement it. Consolidates F29's P0-6.
   *(See P0-6.)*
5. **RESOLVED — entry affordance: gear icon, label below `sm`.** A gear in the chrome cluster showing an
   "Owner" label at `≥sm`, icon-only below — mirroring the skin picker / Preview toggle responsive pattern.
   *(See P0-2.)*
6. **RESOLVED — defer the internal `adminMode` rename.** Only user-facing strings change now; the store,
   key, and code identifiers keep their names. Divergence tracked as tech-debt (P2-2). *(See Non-Goals.)*

## Timeline / Phasing

No hard deadline. Suggested order:
1. **Route group + redirects (P0-1, P0-5)** — create `/owner` group, move the three pages under it as
   nested routes, redirect the old paths. Pure SvelteKit routing; pages' internals essentially unchanged.
2. **Owner gate + auto-reveal (P0-4, P0-6)** — gate the `/owner` group on `effectiveOwner`; consolidate
   auto-reveal at the group level; close any ungated `/keys` `/status` API exposure. → `/security-review`.
3. **Header rework (P0-2, P0-3, P0-7)** — Owner gear in the chrome cluster, content nav down to three,
   rename the toggle strings. → `/design-handoff` for the bar.
4. **Hub shell + tabs + theming (P0-1 cont., P0-8)** — the tabbed shell, active-tab `bg-surface-2`
   treatment, QA across all three skins in both Preview states.
5. **P1 polish** (landing summary, tab persistence, keyboard) as fast follows.

**Pre-merge gates (project working agreements):** `/simplify`; **`/security-review`** (owner-gating);
`/design-handoff` (bar + hub); `/testing-strategy` updated for the new routes + gating; QA across **all
three skins** in both Owner and Preview states.
