# Spec: Admin Mode (F29)

**Status**: Draft
**Phase**: Quick win / polish (cross-cutting UX over the owner gate)
**Depends on**: the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md)) — `activity.isOwner`;
the skin/theme preference pattern (`web/src/lib/theme.svelte.ts`, [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).
**Related**: every owner-gated surface (enrichment F22, person images F25, writeback F28, delete-media,
person aliases/merge F23, rescan/reload on `/status`).
**Design handoff**: [`docs/design/admin-mode-handoff.md`](../design/admin-mode-handoff.md).
**Follow-up:** consolidated **Admin page** (folding
`/status`, `/trash`, `/keys` into one tabbed page) — tracked separately in `TASKS.md`, not part of F29.

---

## Problem Statement

When an owner is authenticated, admin-only controls and data are rendered **everywhere** — enrich/clear
buttons, writeback, regenerate, delete, person-image management, merge, the Trash nav link, the
"Recently Added" home toggle, and the `/status` admin actions. There is no way to **see the library as a
regular visitor sees it** without clearing the admin token and re-authenticating, and the persistent admin
chrome adds visual noise while simply browsing. The owner both **curates** and **consumes** the same
library; today those two modes are conflated, so QA-ing the public experience (across all three skins) and
distraction-free viewing are each awkward.

## Goals

1. **One-click visitor preview.** From any page, an owner can turn **Admin mode** off (and back on) in a
   single action, with no re-auth and no page reload — state flips instantly.
2. **Preview fidelity.** With Admin mode OFF, the rendered UI matches what a non-owner sees: every
   owner-only **control _and_ owner-only data surface** is hidden, not just dimmed. (Decision: hide
   controls **and** admin data — faithful visitor view.)
3. **Mirror the theme toggle's ergonomics.** The control lives in the header beside the skin picker,
   persists per-device in `localStorage`, and applies reactively app-wide — the proven pattern owners
   already understand.
4. **Zero security regression.** The toggle is a **presentation filter only**. It never changes what the
   browser is authorized to do; the admin token, capabilities, and every server-side `requireOwner` gate
   are untouched.

## Non-Goals

- **Not a logout or token change.** Turning Admin mode OFF does not clear the admin token or drop
  `isOwner`; it only hides admin UI. _Why: re-auth is the existing, deliberate path for actually dropping
  privileges; this is a view preference._
- **Not a server-enforced permission.** This is purely client-side chrome. It does not and must not be
  relied on for access control — the server gate remains the only authority. _Why: a UI toggle is not a
  security boundary._
- **Not per-account / cross-device sync.** Persistence is per-device (`localStorage`), matching the skin
  picker. _Why: keeps it a zero-backend quick win; a synced preference is a separate, heavier initiative._
- **Not a per-surface granular control.** It is a single global on/off, not "hide enrich but keep delete."
  _Why: the use cases (visitor preview, declutter) are all-or-nothing; granularity is scope creep._
- **Not shown to non-owners.** The toggle only renders when `isOwner` is true. A logged-out visitor never
  sees it. _Why: it would be meaningless and confusing to a regular user._

## User Stories

**As the library owner (admin):**
- I want to turn Admin mode off with one click so that I can see exactly what a regular visitor sees —
  across Cinémathèque, Broadcast, and Brutalist — without logging out.
- I want the toggle to sit next to the skin picker in the header so that it's where I already reach for
  view preferences, on every page.
- I want my choice remembered on this device so that I'm not re-toggling every visit.
- I want to turn Admin mode back on with one click so that I can resume curating (enrich, writeback,
  delete, manage images) right where I was, with no reload.
- I want Admin mode **on by default** when I first authenticate so that gaining admin access visibly does
  something and controls don't stay hidden after I enter my token.

**As the owner doing QA:**
- I want Admin mode OFF to hide owner-only **data** too (provenance badges, the Trash link, the "Recently
  Added" toggle, admin-only status actions) so that my preview faithfully represents the public surface and
  I catch skin-specific regressions a visitor would hit.

**Edge / boundary cases:**
- When **not** authenticated as owner, the toggle is absent and the app behaves exactly as today.
- If Admin mode is OFF and the owner opens an **owner-only route** by direct URL (e.g. `/trash`), Admin
  mode **auto-reveals** (flips back ON) so the page is usable — see P0-6.
- If the admin token is cleared/expires while Admin mode is OFF, the toggle disappears (no longer owner);
  the stored preference is retained for the next authenticated session.

## Requirements

### Must-Have (P0)

**P0-1 — Header toggle control (owner-only).**
A control in the global header (`web/src/routes/+layout.svelte`), adjacent to the skin segmented control,
rendered only when `activity.isOwner`. Labeled with the term **"Admin mode"** and communicating its two
states clearly (ON = admin elements visible; OFF = visitor view). Exact affordance TBD in design handoff.
- Given I am authenticated as owner, when the header renders, then I see the toggle.
- Given I am not owner, when the header renders, then the toggle is absent.
- Tokens-only styling (no hardcoded palette/radii — [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md)).

**P0-2 — Single global reactive state (dedicated store).**
A reactive `adminMode` boolean in a small dedicated store **mirroring `theme.svelte.ts`** (kept separate
from the `activity` store for clean separation of concerns — resolved, see Open Questions). Every
owner-gated surface reads the **effective** gate `activity.isOwner && adminMode.enabled`.
- Given Admin mode is ON, when any owner-gated surface renders, then its admin controls/data show.
- Given Admin mode is OFF, when any owner-gated surface renders, then its admin controls/data are hidden
  (removed from the DOM, not merely visually hidden) — see P0-4.
- Toggling updates all surfaces with **no page reload**.

**P0-3 — Per-device persistence, default ON.**
Persist to `localStorage` (key `holodex-admin-mode`), restored on mount like `theme.init()`. Absent or
unparseable value defaults to **ON** (admin elements visible).
- Given I turn Admin mode OFF and reload, when the app mounts, then Admin mode is still OFF.
- Given a fresh browser with no stored value, when I authenticate as owner, then Admin mode is ON.

**P0-4 — Complete, faithful hide set.**
With Admin mode OFF, **all** of the following are hidden so the surface equals the visitor surface:
- Header: `/trash` nav link (and `/keys`, `/status` if/when those become owner-only — see P0-6/follow-up).
- Home (`/`): "Recently Added" owner toggle.
- Media detail (`/media/[id]`): Regenerate thumbnail, Enrich, Clear-provider, Writeback, Delete
  (soft/purge) — and owner-only data: provenance/source badges, plus the raw **"Enrichment data:
  File Extraction"** and **"Enrichment data: {provider}"** audit disclosures (moved to the bottom of
  the page, under the Manage block, and now owner-gated — formerly public).
- Person detail (`/people/[id]`): alias add/delete, merge, image upload (headshot/banner/poster), gallery
  management, enrich, clear-provider — and owner-only badges.
- People list (`/people`): "Merge people…" batch control.
- Status (`/status`): Rescan, Reload config, admin-token input, and any owner-only diagnostics that a
  visitor wouldn't see.

  Each is gated on the effective `isOwner && adminMode.enabled` rather than `isOwner` alone.
- Given Admin mode OFF, when I visit each surface above, then no owner-only control or data is present in
  the DOM and the layout reads as the public view.

**P0-5 — Security: presentation-only, no token/gate change.**
Toggling must not modify the admin token, the `X-Admin-Token` header, capability polling, or any
client/server authorization. The server's `requireOwner` choke points remain the sole authority.
- Given Admin mode OFF, when an owner-only API is invoked by other means, then the server still authorizes
  it normally (the toggle changed nothing server-side).
- **Triggers `/security-review`** before merge (touches the access/owner-gate surface), per project working
  agreements.

**P0-6 — Auto-reveal on owner-only routes.** _(resolves Open Question #1)_
If Admin mode is OFF and the owner navigates directly to an **owner-only route** (today `/trash`; later the
consolidated Admin page), Admin mode **auto-reveals** — it flips back ON (persisting the change) so the
page is usable rather than appearing empty/forbidden. The toggle reflects the new ON state.
- Given Admin mode OFF, when I open `/trash` by direct URL, then Admin mode turns ON and the page renders
  fully, and the header toggle shows ON.
- _Rationale:_ an owner-only route is an explicit "I want to administer" intent; honoring it beats showing a
  dead page. (Distinct from public routes, which stay in visitor view.)

### Nice-to-Have (P1)

- **P1-1 — Keyboard shortcut** to flip Admin mode (e.g. a single key when not in an input), for fast QA
  sweeps.
- **P1-2 — Subtle persistent indicator** while Admin mode is OFF (a small badge/border) so the owner never
  forgets controls are hidden — distinct from accent, using tokens; auto-hides in true visitor sessions.
- **P1-3 — Reduced-motion-friendly transition** when controls appear/disappear.

### Future Considerations (P2)

- **P2-1 — Per-account synced preference** (server-stored) so the choice follows the owner across devices.
- **P2-2 — Granular preview scopes** (e.g. "hide destructive actions only").
- **P2-3 — "Impersonate role" beyond owner/visitor** if non-owner roles are ever introduced.

## Success Metrics

This is a personal/self-hosted tool, so metrics are usage-quality signals rather than funnel numbers.

**Leading indicators (immediate):**
- Owner can complete a full visitor-preview sweep of all three skins **without ever entering/clearing the
  admin token** (today this requires a token round-trip). Target: token round-trips for QA → **0**.
- Toggle latency: state flips with **no page reload** and no visible reflow jank (< 1 frame of perceptible
  delay).

**Lagging indicators:**
- Skin-specific public-view regressions caught **before** merge increases (qualitative — the toggle makes
  the "QA all three skins" pre-commit rule cheap to honor).
- Zero security findings attributable to the toggle in `/security-review` (the gate stays server-side).

## Open Questions

1. ~~**Owner-only pages while in visitor view**~~ — **Resolved: auto-reveal** (see P0-6). Owner-only routes
   flip Admin mode back ON. A separate follow-up consolidates `/status` + `/trash` + `/keys` into one tabbed
   **Admin page** (tracked in `TASKS.md`).
2. **Control placement & affordance** _(design)_ — **Resolved in the handoff**: a single binary **switch**
   (`role="switch"`) labeled "Admin", accent-filled when ON, placed between `ActivityIndicator` and the skin
   picker. Open/closed-eye icon swap. (Binary = switch; the 3-way segmented shape stays the skin picker's.)
3. ~~**Naming**~~ — **Resolved: "Admin mode"** for the control, the indicator (P1-2), and docs.
4. ~~**State location**~~ — **Resolved: dedicated `adminMode` store** mirroring `theme.svelte.ts` (not a
   property on the `activity` store).

## Timeline Considerations

- **No backend work** for the P0 scope — frontend-only, no migration, no new API. This is a quick win.
- **Dependencies:** none beyond the existing owner gate and theme-store pattern, both already in `main`.
- **Phasing:** P0 ships the toggle + complete hide set + auto-reveal + security review. P1 (shortcut,
  indicator) is a fast follow. P2 (synced preference) is deferred and explicitly out of scope.
- **Related follow-up (separate task):** consolidate `/status`, `/trash`, and `/keys` into one tabbed
  **Admin page**. Sequencing note — landing it first would shrink F29's hide-set to a single Admin nav
  entry; either order works, so they can ship independently.
- **Pre-merge gates (project working agreements):** `/simplify` on changed code; **`/security-review`**
  (touches the owner-gate surface); `/design-handoff` for the control; QA across **all three skins** in
  both toggle states; `/testing-strategy` updated for the new gating.
