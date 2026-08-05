# Design Handoff: Unified nav search — live, tabbed, in-place filtering panel

**Spec**: [nav-search-live-filter.md](../specs/nav-search-live-filter.md)
**ADRs**: None (extends existing patterns — see spec's "New ADRs required")
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Issue**: [HOLODEX-249](https://whoiskevinrich.atlassian.net/browse/HOLODEX-249)
**Surface**: `web/src/routes/+layout.svelte` (nav box), new shared results-panel component,
`web/src/routes/search/+page.svelte`, `web/src/routes/+page.svelte` (root/Media),
`web/src/routes/people/+page.svelte`, `web/src/routes/studios/+page.svelte`,
`web/src/routes/tags/+page.svelte`.

---

## Overview

The spec defines the interaction model (in-place when the tab matches the page's scope,
overlay panel otherwise) but leaves the panel's exact layout, the tab row's persistent
placement, and two design-tagged open questions unresolved. This handoff pins all three,
grounded in the actual `+layout.svelte` markup and token set — no new tokens, one new
shared component.

### Design-system fit

**No new tokens.** Every treatment below reuses classes already live in
`+layout.svelte`'s history dropdown (`rounded-theme border border-rule bg-surface`, row
`px-3 py-1.5 text-sm`, `hover:bg-surface-2`, active `bg-surface-2 text-ink`) and
`EnrichPicker.svelte`'s roving-tabindex option pattern. **One new component**:
`SearchResultsPanel.svelte`, shared between the nav dropdown and the `/search` page body
(resolves the spec's third open question — see Part E). **One new module**:
`navSearch.svelte.ts`, holding query/tab/scope state (the spec listed this as a P1
"nice-to-have"; pulling it into P0 here because NS2's routing rule has nowhere else to
live without duplicating it across five pages).

---

## Part A — The tab row lives with the box, not inside the dropdown

The spec says the tab row is "always shown" but doesn't say *where* when the box is
collapsed. Rendering it as permanent page chrome (visible on every page load, unfocused)
would add height to every screen — directly working against Goal 1 (declutter) and Goal 5
(mobile). **Decision: the tab row is part of the box's *expanded* state, not the page's
static header.** It mounts the instant the box receives focus and stays mounted for as
long as focus (or an open panel) holds — whether that focus session shows history rows,
grouped panel rows, or nothing extra at all (the in-place case, where the tab row is the
*only* thing rendered below the box, because the content being filtered is the page grid
itself, already on screen).

This is what makes NS2's "tap Videos while in-place-filtering People" acceptance criterion
work: the tab row is reachable at any point the box is focused, in-place or not.

```
Collapsed (today, unchanged):
┌──────────────────────────────┐
│ 🔍 Search everything… (Ctrl-K)│
└──────────────────────────────┘

Focused, empty, on /people (in-place mode, matching tab pre-selected):
┌──────────────────────────────┐
│ 🔍 |                        × │  ← × only once text present
├──────────────────────────────┤
│ All  [People]  Videos  Studios  Tags │  ← tablist, People selected
└──────────────────────────────┘
  (no card below — history would show here if history exists AND query is empty;
   once query is non-empty and tab=People, page grid below filters in place, nothing
   renders under the tab row)

Focused, typing, on /people, tab switched to Videos (overlay case):
┌──────────────────────────────┐
│ 🔍 kson                     × │
├──────────────────────────────┤
│ All  People  [Videos]  Studios  Tags │
├──────────────────────────────┤
│ VIDEOS                              │
│  Jackson's Big Day             ▸    │
│  Jackson Interview 2024        ▸    │
│  Jack & Jackson: The Reunion   ▸    │
│  View all 14 in Videos →            │
└──────────────────────────────┘
```

### Tab row semantics

Real tabs, not the header's skin-switcher toggle-button group — the tab controls *what
renders below*, which is exactly what `role="tablist"` is for (WAI-ARIA APG Tabs, not a
`role="group"` of pressed buttons):

```svelte
<div role="tablist" aria-label="Search scope" class="flex items-center gap-1 border-t border-rule pt-1.5">
  {#each TABS as t (t.key)}
    <button
      role="tab"
      aria-selected={activeTab === t.key}
      tabindex={activeTab === t.key ? 0 : -1}
      onclick={() => selectTab(t.key)}
      onkeydown={onTabKey}
      class="rounded-theme px-2 py-1 text-xs transition {activeTab === t.key
        ? 'bg-surface-2 text-ink'
        : 'text-muted hover:text-ink'}"
    >{t.label}</button>
  {/each}
</div>
```

Arrow-Left/Right move the roving `tabindex` and activate immediately (standard APG
"automatic activation" tabs — no separate confirm step, matching how cheap a tab switch
is here). Tab key itself moves *out* of the tablist into the panel's first result row (or
out of the box entirely if nothing renders below), per NS5.

---

## Part B — `SearchResultsPanel.svelte` (NS1)

### Row density and grouping

- **3 rows per group** before a **"View all N in \<type\>"** row (4th row, always present
  when the group's total exceeds 3 — never when a group has ≤3 matches). Three keeps a
  4-group "All" result under 13 rows total, which is the budget for staying on-screen on a
  ~700px-tall phone viewport without scrolling past the tab row.
- Row markup matches the existing history row exactly: `px-3 py-1.5 text-sm`,
  `hover:bg-surface-2`, focused/active `bg-surface-2 text-ink`. Group header above each
  cluster: `px-3 pt-2 pb-1 text-xs font-medium uppercase tracking-wide text-muted`
  (new label style, but built from existing `text-muted`/spacing tokens — no new token).
- On a **single-tab selection** (People/Videos/Studios/Tags, not All), skip the group
  header entirely — one flat list, still capped at the same "3 then View all" rule but
  against a larger practical cap (e.g. show up to 8 rows before "View all" on a focused
  tab, since there's no sibling group competing for space). Only "All" uses the tight
  3-per-group budget.

### States

| State | Render |
|---|---|
| Debounce window elapsed, request in flight | Existing group skeleton: 2 muted `bg-surface-2` bars per group, no layout shift when real rows arrive (reserve the row height up front) |
| Query resolves, zero matches across all groups | Single centered line, `text-muted text-sm`, `py-6`: `No matches for "{query}"` — tab row stays visible so the owner can try another scope |
| Query resolves, some groups empty | Empty groups are omitted entirely (not shown with a "no results" sub-line) — matches the existing `/search` page's current omit-empty-section behavior |
| Request fails (network) | Reuse whatever inline-error idiom the root page's existing fetch-failure path uses today (`text-warn`, no icon) — do not invent a second error style for this one surface |

### Mobile (< 640px, the primary complaint driving this spec)

The dropdown becomes a **fixed full-width sheet**, not a clipped `absolute` box:

```css
/* ≥640px: today's behavior, unchanged */
.search-panel { position: absolute; left: 0; right: 0; top: 100%; }

/* <640px */
.search-panel { position: fixed; left: 0; right: 0; top: var(--header-height); bottom: 0; overflow-y: auto; }
```

Five tabs (All/People/Videos/Studios/Tags) at `px-2 py-1 text-xs` fit a 375px viewport
without wrapping or horizontal scroll — verify this specifically at 375px in QA, since
it's the one row most likely to break first. If a future tab is added and it doesn't fit,
the tablist scrolls horizontally on its own axis (`overflow-x-auto`, no visible
scrollbar) rather than wrapping to a second line, which would push results down.

---

## Part C — Per-page removal (NS4)

- **Root page (`+page.svelte`)**: delete the `#q` text input (lines ~298–304 today).
  Resolution/duration/year facet controls and `FacetFilter` pickers are untouched. The
  existing inline 200ms debounce (`+page.svelte:112,205-206`) becomes the shared
  `navSearch.svelte.ts` debounce — reuse the same 200ms value app-wide rather than
  introducing a second timing (`EnrichPicker`'s 300ms stays local to that dialog, it's a
  different interaction).
- **Tags page**: delete the `query` input (lines ~563–569). `filterByName` (already in
  `$lib/format`) becomes the shared client-side filter used by People, Studios, and Tags
  alike — no logic change, just called from `navSearch.svelte.ts` instead of local state.

---

## Part D — Accessibility (NS5)

Copy `EnrichPicker.svelte`'s roving-tabindex pattern verbatim for result rows:

```js
function onRowKey(e, i) {
  if (e.key === 'Enter') activate(rows[i]);
  else if (e.key === 'ArrowDown') focusRow(Math.min(i + 1, rows.length - 1));
  else if (e.key === 'ArrowUp') { if (i === 0) tablistEl.focus(); else focusRow(i - 1); }
}
```

- `tabindex={i === activeRow ? 0 : -1}` on each row, `role="option"` inside a
  `role="listbox"` per group (matching `EnrichPicker`'s structure, not the old
  `aria-activedescendant` combobox pattern).
- Escape: closes the panel, returns focus to the box, **does not** clear `searchTerm` or
  touch in-place filter state — matches NS5's acceptance criterion exactly.
- The box's `role` changes from `combobox`+`aria-activedescendant` (today) to a plain
  text input; the roving-tabindex rows replace the `aria-activedescendant` wiring
  entirely, including in the (now-rebuilt) history dropdown per the spec's own note.

---

## Part E — Resolves open question: `/search` page reuse

**Decision: full reuse.** `/search/+page.svelte` renders `<SearchResultsPanel>` as its
page body — same component the nav dropdown uses, just unwrapped from the
`absolute`/`fixed` positioning (static, full-width, no `role="dialog"`/dismiss handling).
Today's `/search` page already groups by type in flat sections (confirmed: `videos`,
`people`, `studios`, `tags`, each its own `<h2>` + list), so this is a re-skin onto the
new component's grouping logic, not new information architecture — avoids maintaining two
implementations of "grouped-by-type result rows" per NS1's explicit instruction. The page
body drops the "3 then View all" cap the dropdown uses (a "View all" row makes no sense
when you're already on the "view all" page); it renders every match, paginated the same
way the page does today for large groups.

---

## Part F — Resolves open question: NS6 (detail-page video lists)

Confirmed against `EntityVideos.svelte` and its callers: `api.getPerson(id)` /
`api.getStudio(id)` return the full `videos` array in one unpaged response — the same
shape as People/Studios/Tags, not Media's paginated fetch. **NS6 is a cheap client-side
`filterByName` add, identical to Part C's pattern, with no server-side work.** Recommend
promoting NS6 from P1 to P0 — it costs nothing beyond wiring the same shared filter one
more place, and shipping it alongside People/Studios avoids a visible gap ("detail pages
still have no filter") right after this spec ships everywhere else.

---

## Responsive summary

| Breakpoint | Tab row | Panel |
|---|---|---|
| Desktop (≥1024px) | Inline under box, all 5 labels visible | `absolute` dropdown, `max-w-md` matching the box |
| Tablet (640–1024px) | Same as desktop | Same as desktop |
| Mobile (<640px) | Same row, `text-xs`, no wrap | `fixed` full-width sheet, `top: var(--header-height)` to `bottom: 0`, scrolls internally |

## Accessibility notes (summary)

- Focus order: box → tablist (roving) → result rows (roving, per group) → "View all" link
  → (end of panel).
- Live region: reuse the existing `role="status" aria-live="polite"` pattern already in
  `+layout.svelte` (today used for admin-mode announcements) to announce result counts
  ("14 results across 3 categories") when the panel's content changes — screen-reader
  users get no other signal that typing produced results.
- All three skins: verify the group-header `text-muted uppercase` treatment and the
  skeleton loading bars read correctly against Broadcast's blue surface and Brutalist's
  near-black/lime combination, not just the default Cinémathèque skin.
