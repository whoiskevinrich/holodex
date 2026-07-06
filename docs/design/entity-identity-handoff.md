# Design Handoff: Entity name-identity — merge, alias & duplicate review (F43)

**Spec**: [entity-identity.md](../specs/entity-identity.md) · **ADR**: [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**
**Stack**: SvelteKit (Svelte 5 runes) + Tailwind v4 CSS-first (ADR-025).

This handoff **generalizes the F23 person-alias surfaces** ([person-aliases-handoff.md](person-aliases-handoff.md))
to Studio and Tag, and adds two new things: the **editor near-miss soft-warning** and the **Duplicates
review tab** in the Owner hub. The alias/merge chip, input, picker, and collision-card mechanics are
**inherited from F23 unchanged**; this document specifies what is new per surface. Everything is
**tokens-only** (no literal palette/radius/font — [theming.md](theming.md)) and QA'd in all three skins.

New/changed surfaces, in build order:
1. **"Also known as" alias panel** — extract F23's inline block to `AliasPanel.svelte`; reuse on person + studio (not tag).
2. **Merge picker** — generalize `PersonPicker` → `EntityPicker` (person | studio | tag).
3. **`/people` + `/studios` list merge** — studios gain the existing `/people` multi-select mode.
4. **`/tags` identity actions** — a pill-native manage mode (rename · alias · merge); no tag detail page.
5. **Editor near-miss soft-warning** — the non-blocking sibling of the F23 exact-collision card.
6. **"N possible duplicates" banner** — on `/people`, `/studios`, `/tags`.
7. **`/owner/duplicates`** — the review-queue tab (new route in the F35 hub).

---

## Overview

Two audiences, one gate (`activity.effectiveOwner`). A **visitor** sees only read-only alias chips and
unchanged pill/row lists. An **owner** gets: add/remove aliases and merge on person & studio pages; rename ·
alias · merge on the `/tags` list; a soft heads-up when a new name looks like an existing one; a banner that
counts likely duplicates; and a Duplicates tab that works the whole near-miss queue (tags dominate it).

### Design-system fit (the `/design-system` check)

Almost nothing new. This feature is F23's alias/merge idioms, applied three times, plus one new list surface:

- **Chips / input / picker / collision-card** — reused verbatim from F23 (`people/[id]/+page.svelte`,
  `PersonPicker.svelte`). The picker becomes entity-generic; the markup is unchanged.
- **Banner** — the existing owner-notice idiom (`rounded-theme border border-warn bg-surface px-3 py-2`,
  `text-warn` marker) already used on `/owner/status`. No new component.
- **Duplicates tab** — a new route under the built `/owner` hub (`owner/+layout.svelte` tab array), using the
  established StatusCard/section rhythm. The only genuinely new layout is the **pair row**.
- **Tag manage mode** — reuses the `/people` multi-select idea, adapted to the pill list.
- **Merge confirm** — `ConfirmDialog` (`variant='accent'`) or the `EntityPicker` step-2 confirm; no new modal.

Net new component files: `AliasPanel.svelte` (extraction), `EntityPicker.svelte` (generalized
`PersonPicker`), `DuplicatePairRow.svelte`, and the `owner/duplicates/+page.svelte` route. **Introduce no new
tokens.**

---

## Reuse map (don't build from scratch)

| Need | Reuse from | Notes |
|------|-----------|-------|
| Owner gating | `activity.effectiveOwner` (`activity.svelte.ts`) | Same predicate the F23 controls use; controls render only when owner. |
| Alias chips + add input | `people/[id]/+page.svelte` L630–672 | Chip `rounded-full bg-surface-2 px-2.5 py-0.5 text-sm text-ink`; input `border-rule bg-surface focus:border-accent`; add btn `bg-accent text-accent-ink`. Extract to `AliasPanel.svelte`. |
| Merge modal | `PersonPicker.svelte` → `EntityPicker.svelte` | 2-step (search→informed confirm); roving tabindex + focus-trap + Esc + focus-return (mirrors `EnrichPicker`). Add an `entityType` prop; dialog/rows/buttons unchanged. |
| Exact-collision card | `PersonPicker.svelte` L675–698 | Inline `rounded-theme border border-rule bg-surface-2 p-3`; "Yes, merge them in" (accent) / "No, keep separate" (border-rule). |
| Merge confirm | `ConfirmDialog.svelte` (`variant='accent'`) | Shows both counts + "videos move, name becomes an alias, can't be auto-undone"; `bg-accent text-accent-ink` confirm; error `text-warn`. |
| `/people` multi-select merge | `people/+page.svelte` | "Merge people…" mode + "Keep which name?" dialog — copy to `/studios` verbatim. |
| List pages | `people/+page.svelte` (rows), `tags/+page.svelte` (pills) | People/studios = grid rows; **tags = flat pill-wrap** — the two identity-action patterns differ (below). |
| Owner tab shell | `owner/+layout.svelte` | Add `{ href:'/owner/duplicates', label:'Duplicates' }` to the tab array; tab bar `flex flex-wrap gap-2 border-b border-rule pb-3`, active `bg-surface-2 text-ink`. |
| Banner | `owner/status/+page.svelte` L150–155 notice | `rounded-theme border border-warn bg-surface px-3 py-2 text-sm text-ink`, `text-warn` marker. |

---

## 1. "Also known as" panel — person + studio (extract `AliasPanel.svelte`)

F23 shipped this inline on the person page; F43 puts it on **studio pages too**, so extract it to
`AliasPanel.svelte(entityType, entityId, aliases, isOwner)` and mount it on both `people/[id]` and
`studios/[id]`, **above** the Enrichment/Details panel (aliases are core identity; enrichment is shadow).

- **Tag exception (RD7):** tags have **no** detail page, so no `AliasPanel` for tags — tag aliasing lives in
  the `/tags` manage mode (§4).
- Layout, chip treatment, states, and a11y are **exactly F23's** ([person-aliases-handoff.md](person-aliases-handoff.md)
  §Layout/§Chip treatment/§States) — reuse that spec; nothing changes except the entity the panel points at.
- Studio pages currently render `name` as read-only (F38 RD5); F43 adds the alias panel **and** a
  rename affordance (the studio `name` becomes editable via rename → the F23 rename flow, not a decision chip).

## 2. Merge picker — `EntityPicker.svelte` (generalize `PersonPicker`)

Add an `entityType: 'person' | 'studio' | 'tag'` prop; everything else is `PersonPicker` verbatim:

- **Step 1 — pick.** Search input filters the entity's list client-side (excludes the canonical); each row =
  `name` + count. Active row `border-l-2 border-accent bg-surface-2`. Roving tabindex (↑/↓ move, Enter picks,
  ↑ from top returns to search).
- **Step 2 — informed confirm.** "Merge **{name}** ({n}) into **{canonical}**?" + muted "videos move, the
  name becomes an alias, can't be auto-undone" (RD8). **Back** (border-rule) / **Merge** (`bg-accent
  text-accent-ink`). Errors inline `text-warn`.
- Dialog `rounded-theme border border-rule bg-surface p-4 shadow-xl`; `role=dialog aria-modal`; focus trap;
  Esc; focus-return to trigger. For **studio**, the confirm copy notes the merged name is kept as an alias
  **so a re-enrich/rescan won't recreate it** (RD6 — the derivation-survival guarantee, made visible).

## 3. `/people` + `/studios` list merge

`/people` already has the owner "Merge people…" multi-select + "Keep which name?" dialog. **Copy it to
`/studios` verbatim** (studios use the same grid-row layout). Both feed the same per-entity merge endpoint.
No change to `/people`.

## 4. `/tags` identity actions — pill-native manage mode

The `/tags` list is a **flat pill-wrap** (`rounded-full border border-rule bg-surface px-3 py-1.5 text-sm
text-ink hover:border-accent`), not a grid — so tag identity gets a **manage mode** rather than per-row
controls. See the mockup (`tags_identity_actions_manage_mode`).

- **Default (visitor + owner):** pills unchanged; each links to browse. No visual change.
- **Owner "Manage tags" toggle** (a button by the page heading, gated on `effectiveOwner`):
  - **Merge** — pills become **selectable**; a selected pill is `border-accent bg-surface-2` with a leading
    `ti-check` in `text-accent`. A manage bar above the pills shows "N selected" + **Merge…** (`border
    border-accent text-accent`, opens `EntityPicker`/"Keep which name?" for 2+). Mirrors the `/people`
    multi-select semantics, pill-adapted.
  - **Rename · Add alias · Merge into…** — each pill carries a small **`ti-dots` menu button** (owner-only,
    `text-muted hover:text-accent`, `aria-label="Tag actions: {name}"`) opening a popover
    (`bg-surface-2 border border-rule rounded-theme`) with the three actions. Rename/alias open a small inline
    input (reuse the F23 add-input classes); Merge opens `EntityPicker`.
- **No decision chips, no `SourceSelect`, no detail page** (RD7) — tags are identity-only.

## 5. Editor near-miss soft-warning (the non-blocking sibling)

Two distinct prompts — keep them visually different so "blocking" vs "advisory" reads at a glance:

| Prompt | When | Treatment | Blocking? |
|---|---|---|---|
| **Exact-collision** (P0-5, F23) | add-alias / rename returns `409` (name is *exactly* another entity's `nameKey`) | **Bordered card** `rounded-theme border border-rule bg-surface-2 p-3`: "**{X}** ({n}) is already a separate {entity}. Are they the same?" → **Yes, merge them in** (accent) / **No, keep separate** (border-rule) | The create/rename does **not** happen until resolved |
| **Near-miss** (P1-5) | on submit, a *fuzzy* (loose-key, not exact) match exists | **Quiet inline line** below the input, `text-muted`: "Looks like **{X}** ({n}) — <a class='text-accent'>merge instead?</a>" with a secondary **Create anyway** | **Non-blocking** — the entity is created; "Create anyway" also records keep-separate (RD5) so it won't nag again |

The near-miss is deliberately **not** a bordered warn card — it's a muted suggestion that never traps the
owner. `--warn` is reserved for errors; the merge affordance uses `--accent`.

## 6. "N possible duplicates" banner

On `/people`, `/studios`, `/tags`, when the entity's queue is non-empty, render the owner-notice idiom above
the list (owner-only):

```
rounded-theme border border-warn bg-surface px-3 py-2 text-sm text-ink
  <span class="font-semibold text-warn">N possible duplicate {tags|studios|people}</span>
  <a class="text-accent" href="/owner/duplicates?type={entity}">Review ↗</a>
```

Count = the queue rows for that entity type (P1). Hidden when zero, or for non-owners. The link deep-links the
Duplicates tab filtered to that entity.

## 7. `/owner/duplicates` — the review-queue tab

A new tab in the F35 Owner hub. Pairs **grouped by entity, tags first** (they're 41 of 56). Two layouts were
mocked (`owner_duplicates_tab_layout_options`); **Option A — dense pair rows — is ratified** (owner, 2026-07-05):
56 pairs, tag-heavy → density wins; Option B's cards are too heavy to scroll. Build the dense pair-row layout;
the card layout is not carried forward.

- **Group header:** `text-xs uppercase tracking-wide text-muted` "Tags · 41" (the StatusCard label idiom).
- **Pair row** (`DuplicatePairRow.svelte`): `flex items-center gap-2 border-t border-rule px-3 py-2.5` —
  leading entity icon (`ti-tag`/`ti-building`/`ti-user`, `text-muted`); `name · count`, a muted
  `ti-arrows-left-right` connector, `name · count`; the `variation` label (`text-xs text-muted`
  "internal-whitespace"/"punctuation"); right-aligned **Merge** (`text-accent`, opens the merge confirm with
  the surviving-name choice) and **Keep separate** (`text-muted`, records keep-separate, RD5, row fades out).
- **Empty state:** `py-16 text-center text-sm text-muted` "No possible duplicates." (the healthy state —
  reachable after the backfill folds the 14 and the owner clears the 56).
- The tab's own **badge**: the tab label may carry a small count (total queue) using the muted pill idiom.

---

## Design tokens used

All inherited (no new tokens). For reference ([theming.md](theming.md)):

| Token | Usage here |
|---|---|
| `bg-surface` / `bg-surface-2` | panels, banner bg, alias chip, selected pill, popover, active picker row |
| `text-ink` / `text-muted` | names / counts, variation labels, connectors, near-miss hint |
| `border-rule` / `border-accent` | card/pill/tab borders / selected pill + focus + active row |
| `bg-accent` / `text-accent-ink` / `text-accent` | Add + Merge CTAs / on-accent text / Merge links, ✓, active |
| `border-warn` / `text-warn` | the duplicates **banner** + all action **errors** — never on a chip at rest |
| `rounded-theme` | cards, inputs, tabs, buttons, popovers |
| `rounded-full` | alias chips + tag pills (intentional pill shape, allowed by CLAUDE.md) |
| `skin-title` / `font-display` | page `<h1>`s (`/owner` tab uses the hub heading) |

**Load-bearing separation:** the **banner/errors use `--warn`**; the **merge affordances use `--accent`**.
On Brutalist (accent = bright lime) these must stay distinguishable — the same F36 regression risk.

**Token guard** stays clean: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
empty (`rounded-full` pills OK).

---

## States and interactions

| Element | State | Behavior |
|---|---|---|
| Alias panel | no aliases, not owner | Not rendered. |
| Alias panel | no aliases, owner | Heading + add row + muted "No aliases yet." |
| Alias chip (owner) | hover/focus ✕ | ✕ → accent; optimistic remove (restore + `text-warn` on failure). |
| Add alias | duplicate | Idempotent (server); list doesn't grow; clear input, no error. |
| Add alias / rename | exact collision | `409` → the **bordered** collision card (§5); no create until resolved. |
| Create / rename | fuzzy near-miss | **Non-blocking** muted hint (§5); "Create anyway" proceeds + records keep-separate. |
| Merge (any entity) | confirm | `ConfirmDialog`/picker step-2 shows both counts; one-way; on success reload the list/page. |
| Merge (studio) | after | Merged name persists as an alias; a later rescan/re-enrich does **not** recreate it (RD6). |
| Tag manage mode | off (default) | Pills are plain browse links. |
| Tag manage mode | on, <2 selected | Merge button disabled-look but **kept enabled**, responds "Select two or more." (CDS: avoid disabled). |
| Duplicates row | Keep separate | Row fades out (`prefers-reduced-motion` aware); pair recorded in keep-separate; never re-surfaces. |
| Duplicates tab | empty | "No possible duplicates." — the healthy end state. |
| Banner | zero queue / non-owner | Not rendered. |
| Any owner control | not owner | Absent from the DOM (not merely hidden). |

---

## Responsive behavior

| Breakpoint | Changes |
|---|---|
| Desktop / tablet (≥640) | Alias chips + tag pills wrap; `/studios` merge grid 2–3 col; duplicates rows single-column full width. |
| Mobile (<640) | Alias add input `flex-1 min-w-0` shrinks, Add wraps; pair-row actions wrap under the names (`flex-wrap`); manage bar stacks. |

No new breakpoints. The duplicates list is single-column at every width (a worklist, not a grid).

---

## Edge cases

- **Long names** (romanized person, "Warner Bros. Pictures / United Artists") — pair row `truncate`s each
  name with the count outside the truncation; the merge confirm shows full names.
- **International / CJK / diacritics** ("宮崎駿", "Beyoncé") — render in all skins (mono faces on
  Broadcast/Brutalist); no tofu. Detection folds case/whitespace only — diacritic pairs surface as near-misses.
- **Large queue** (56 today, could grow) — single scroll, grouped; no pagination v1 (personal scale). If it
  ever balloons, paginate per group — not designed now.
- **Pair resolved elsewhere** (owner merges via the entity page while the tab is open) — the stale row 404s on
  action; refetch drops it. Treat a missing pair as already-handled (no error toast).
- **Both sides of a pair have curation/decisions** — merge moves non-conflicting; conflicts keep the
  survivor's. The confirm names this ("keeps {canonical}'s values"); detail lives in the spec, not the UI.
- **Self / same pill** in tag manage — Merge needs 2 distinct; guard in the bar.

---

## Animation / motion

Reuse F23/EnrichPicker: modal `merge-rise`/`enrich-rise` and the Keep-separate row fade are gated behind
`@media (prefers-reduced-motion: no-preference)`. No transforms in markup; skin flourishes stay in `app.css`
under `[data-theme]`. The banner does not animate in.

---

## Accessibility

- **Merge modals** (`EntityPicker`) — `role=dialog aria-modal`, focus trapped, Esc closes, focus returns to
  the trigger; the pick list is `role=listbox` with **roving tabindex** (mirrors `EnrichPicker`); confirm
  buttons carry words ("Yes, merge them in"), never color alone.
- **Tag manage menu** — the `ti-dots` opener is a real `<button aria-label="Tag actions: {name}">`; the
  popover is a menu (`role=menu`/`menuitem`), arrow-navigable, Esc closes, focus returns to the opener.
- **Selectable pills** — expose selected state via `aria-pressed`; the ✓ is decorative (`aria-hidden`), the
  state is the ARIA, not the glyph.
- **Banner** — a `role=status`/`aria-live=polite` region so a newly-appearing count is announced; the
  "Review" link names its destination.
- **Duplicates rows** — each pair is a group with an accessible name ("Possible duplicate: sci fi and scifi");
  Merge/Keep-separate are real buttons; after Keep-separate, move focus to the next row (never `<body>`).
- **Error text** uses `text-warn` **and** words, associated via `aria-describedby`.
- **Owner controls** absent from the DOM for non-owners — nothing misleading in the a11y tree.

---

## QA checklist (3-skin)

Conventions ([[feedback-qa-checklist-numbering]]): every item numbered `section.item`, tagged by verifier —
`[smoke]` automated, `[agent]` agent-driven live QA, `[human]` needs human eyes. Skins: **Cinémathèque ·
Broadcast · Brutalist**, switched via the header picker. (The exhaustive matrix lives in the paired
`entity-identity-qa-checklist.md` from `/testing-strategy`; this is the design-surface subset.)

### §1 Setup
- **1.1** `[agent]` Start a backend with a library that has ≥1 case pair (`fox`/`Fox`) and ≥1 fuzzy tag pair
  (`sci fi`/`scifi`) across person, studio, tag ([[reference-holodex-preview-testbeds]]); run the backfill.
- **1.2** `[agent]` Enter the admin token so `effectiveOwner` is true; confirm a second pass as visitor (no token).

### §2 Smoke
- **2.1** `[smoke]` Alias/merge/rename endpoints for studio + tag mirror the person ones (owner-gated; `409`
  on cross-entity name; `400` self-merge/empty; `204` delete).
- **2.2** `[smoke]` `GET /owner/duplicates` returns pairs grouped by type with counts; `dismiss` records
  keep-separate and the pair never returns.

### §3 Agent live QA (all 3 skins)
- **3.1** `[agent]` **Alias panel** on `/people/[id]` and `/studios/[id]`: chips read on the panel card; ✕
  goes accent on hover; add input focus ring visible on each accent (gold/cyan/lime); "Add" solid-accent legible.
- **3.2** `[agent]` **Studio merge survives derivation** (RD6): merge `WB`→`Warner Bros.`, trigger a rescan
  and a re-enrich, confirm `WB` does **not** reappear and both libraries sit under `Warner Bros.`.
- **3.3** `[agent]` **Tag manage mode**: toggle on → pills selectable (`border-accent bg-surface-2` + ✓);
  select 2 → Merge opens the picker; a pill's `ti-dots` menu offers rename/alias/merge; default view unchanged.
- **3.4** `[agent]` **Exact-collision card vs near-miss hint** render distinctly: the `409` card is bordered
  and blocks; the near-miss is a muted non-blocking line with "Create anyway". Confirm `--warn` (banner/errors)
  and `--accent` (merge) stay distinguishable on **Brutalist**.
- **3.5** `[agent]` **Banner**: appears on a list with a non-empty queue, `text-warn` marker reads, "Review"
  deep-links the Duplicates tab filtered to that entity; hidden at zero and for visitors.
- **3.6** `[agent]` **Duplicates tab**: pairs grouped (tags first), Option-A rows legible; Merge → informed
  confirm (both counts); Keep-separate fades the row and it doesn't re-surface on reload; empty state themed.

### §4 Human
- **4.1** `[human]` Open a studio and a person with aliases in each skin. The "Also known as" panel should feel
  identical to the person page you already know — same chips, same fonts reacting to the skin, nothing stray.
- **4.2** `[human]` In the Owner area, open Duplicates. It should read like a tidy worklist, not a wall —
  scanning the tag pairs and clearing a few should feel quick. Merging shows you what you're about to fold in
  before it happens.
- **4.3** `[human]` Rename a tag to something close to an existing tag. The heads-up should feel like a helpful
  nudge you can ignore, not a roadblock — and "Create anyway" should just work.

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` returning empty
> for the changed markup.
