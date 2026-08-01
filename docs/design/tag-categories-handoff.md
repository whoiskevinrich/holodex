# Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)

**Spec**: [tag-categories.md](../specs/tag-categories.md)
**ADR**: not yet written — the spec flags "New ADR required: Likely" (the Category entity's
reduced lifecycle vs. Tag/Person/Studio, the `tag_categories` junction shape, the facet-expansion
query). This handoff's UI-reuse decisions are inputs that ADR should treat as settled; the "Open
design decisions" section at the end lists what's still genuinely open.
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Prior art**: [tag-writeback-exclusion-handoff.md](tag-writeback-exclusion-handoff.md) — the
`tags/[id]` `detail`-snippet and `/tags` Manage-mode-bar extension points this epic builds on,
plus the "no dimming" / design-system-fit-audit format this handoff follows.
[entity-identity-handoff.md](entity-identity-handoff.md) (F43) — `EntityPicker`,
`MergeCanonicalDialog`, and the pill-native Manage-mode idiom (selectable pills + action bar + a
per-pill ⋯ menu) this epic must fold into, not fork.
**Depends on**: HOLODEX-239 (merged, #193) — `tags/[id]`'s `detail` snippet and `/tags`'s
Manage-mode bar both originate there.
**Surfaces**: `tags/+page.svelte` (unified type filter + search + category pills + two new bulk
actions), new `routes/categories/[id]/+page.svelte`, a new `entity/CategoryPicker.svelte`, and a
fourth `FacetFilter` in `routes/+page.svelte`.

---

## Overview

Five additions. Read is open to everyone (matching tags today); every mutation is owner-gated
(`activity.effectiveOwner`), matching the existing tag pattern exactly:

1. **`/tags` unified filter** — an All/Tags/Categories segmented control + a name-search input,
   both new to this page (confirmed: no search input exists on `/tags` today).
2. **Category pill** — a visually distinct pill variant folded into the same grid as tag pills.
3. **`/categories/{id}`** — a new, deliberately sparse detail page, sibling to `/tags/{id}` but
   smaller (no video grid, no ancestor breadcrumb — categories are flat and don't attach to
   videos directly).
4. **Bulk "Add to category…" / "Remove from category…"** — two new Manage-mode bar actions,
   plus a new picker component to drive them.
5. **Browse-page "Categories" facet** — a fourth `FacetFilter`, parallel to the existing Tags one.

### Design-system-fit audit

**No new tokens. One genuinely new interaction; everything else is class-for-class reuse.**

- **Type filter** — `SortToggle`'s exact shell (`flex overflow-hidden rounded-theme border
  border-rule text-sm`, active segment `bg-accent px-3 py-1 text-accent-ink`, inactive `px-3 py-1
  text-muted hover:text-ink`), plain `<button>`s with no extra ARIA — `SortToggle` itself uses
  none, so the type filter shouldn't invent `radiogroup` semantics it doesn't have either.
- **Category pill** — the existing tag-pill shell (`rounded-full border … px-3 py-1.5 text-sm`)
  plus one new accent border + a new decorative tag-glyph icon. No new pill primitive.
- **Category rename** — the exact inline-editor `<form>` already coded for tag rename (input +
  Rename/Cancel buttons, same classes), reused verbatim.
- **Category delete confirm** — `ConfirmDialog`, `variant="destructive"`, same copy pattern as
  the trash "Delete permanently?" dialog (bold the name, state the consequence, "This can't be
  undone.").
- **`/categories/{id}` member-tag chips** — the exact `curation-chip` idiom + near-miss nudge
  card already shipped on the media page's Tags section, reused verbatim (see §3; flagging a
  worthwhile extraction into a shared component while we're at it).
- **Browse facet** — the existing `FacetFilter` component, zero changes, one new call site.
- **The one new thing**: a category **target picker** that supports create-inline (`EntityPicker`
  today doesn't — see §4). Recommend a new sibling component, not a fork of `EntityPicker`'s merge
  semantics.

Audit output: **one new component (`CategoryPicker.svelte`, copying `EntityPicker`'s search
shell), one new route, zero new tokens, one new decorative icon.**

---

## 1. `/tags` — unified type filter + search

### Type filter

Sits beside the existing `SortToggle` in the header cluster (`tags/+page.svelte:276-295`) — two
segmented controls of the same shell, side by side:

```svelte
<div class="flex overflow-hidden rounded-theme border border-rule text-sm">
  <button onclick={() => (typeFilter = 'all')} class={segCls(typeFilter === 'all')}>All</button>
  <button onclick={() => (typeFilter = 'tags')} class={segCls(typeFilter === 'tags')}>Tags</button>
  <button onclick={() => (typeFilter = 'categories')} class={segCls(typeFilter === 'categories')}>Categories</button>
</div>
```

`segCls` is `SortToggle`'s own `cls()` helper, reused as-is. Default `typeFilter = 'all'`.

### Search input

New — there's no existing "plain page-filter input" component to copy verbatim (`FacetFilter`'s
input lives inside a multi-select chip-well; this needs a single-purpose live filter, closer to
`EntityPicker`'s step-1 input minus the popover/listbox semantics — it narrows the on-page grid
directly, it doesn't open a dropdown):

```svelte
<input
  type="search"
  bind:value={query}
  placeholder="Search tags and categories…"
  aria-label="Search tags and categories"
  class="w-full max-w-xs rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
/>
```

Sits in its own row below the header cluster (`flex flex-wrap` — wraps under the type filter +
sort on narrow viewports). Filters client-side against the already-loaded, unpaged tag+category
lists (same "personal-library scale, no dedicated search endpoint" posture `EntityPicker` already
takes) — folds into the existing `displayed` `$derived` pipeline as one more `.filter()` stage
after sort. A results-count line, `aria-live="polite"`, mirrors `EntityPicker`'s status-line
pattern: `"{n} result{s} for “{query}”"` when non-empty, `"No tags or categories match “{query}”."`
when empty (parallels `EntityPicker`'s `No other {noun}{ matching "…"}.` copy).

Name matching uses the same fold as tag-identity collision checks (`nameKeyExpr`) — **confirmed**:
the category/tag name-collision check reuses `nameKeyExpr` verbatim rather than a separate,
simpler fold, for one consistent collision-check code path across both entity types.

---

## 2. Category pill

**Plain (non-Manage) pill** — folds into the same grid as tag pills, distinguished by an accent
border plus a decorative tag-glyph icon (not color alone — satisfies scanability for the "All"
view even for a colorblind reader):

```svelte
<a
  href={`/categories/${c.id}`}
  class="inline-flex items-center gap-1.5 rounded-full border border-accent bg-surface px-3 py-1.5 text-sm text-ink hover:bg-surface-2"
>
  <svg class="h-3.5 w-3.5 text-accent" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
    <path stroke-linecap="round" stroke-linejoin="round" d="M9.568 3H5.25A2.25 2.25 0 003 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 005.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 009.568 3z" />
    <path stroke-linecap="round" stroke-linejoin="round" d="M6 6h.008v.008H6V6z" />
  </svg>
  {c.name} <span class="text-xs text-muted">{tagCount(c.tag_count)}</span>
</a>
```

Icon path/weight matches `CurationChip`'s existing glyphs (`viewBox="0 0 24 24"`,
`stroke-width="2"`, `stroke-linecap/linejoin="round"`) — same visual family, just a new path
(heroicons-style outline "tag"). Count badge shows member-tag count (`tagCount()` — a new
pluralizing helper in `format.ts`, parallel to the existing `videoCount()`), not a video count —
categories don't have one (no aggregate roll-up, confirmed non-goal).

**Manage-mode pill — deliberately asymmetric from tag pills.** Category pills are **not
selectable**: bulk actions in this epic only ever target tags (assign *tags into* a category),
never categories themselves, so there's nothing to select a category *for*. Concretely:

- Clicking a category pill's body still **navigates** to `/categories/{id}`, even while `manage`
  is on — unlike a tag pill's body, which toggles selection in Manage mode. This is the one place
  a reader could mistake the asymmetry for a bug; call it out in review.
- The pill keeps its own reduced ⋯ menu (same trigger-button shell as the tag pill's) with two
  items only: **Rename** (opens the identical inline `<form>` editor already coded for tag
  rename, reused verbatim) and **Delete**. No Merge, no Add alias, no Set parent — none of those
  concepts exist for categories (confirmed non-goal: no alias/merge machinery).
- **Delete** opens `ConfirmDialog` (`variant="destructive"`): *"Delete “{name}”? {N} tag{s} will
  be unassigned from it — the tags themselves aren't affected. This can't be undone."* — same
  bold-the-name-then-state-the-consequence copy shape as the trash "Delete permanently?" dialog.
  **Confirmed: this is the only place category delete lives** — not duplicated on
  `/categories/{id}` itself, avoiding "you just deleted the page you're standing on" navigation
  handling (see §3).

**A tag pill's own ⋯ menu gains one new item too**: **"Add to category…"**, directly below
"Merge into…", opening `CategoryPicker` (§4) in `mode="add"` for that single tag
(`tagIds={[t.id]}`) — the single-tag counterpart to the Manage-bar bulk action below, giving
category assignment the same single-vs-bulk symmetry Merge already has (per-pill "Merge into…" +
Manage-bar "Merge…").

---

## 3. `/categories/{id}` detail page (new route)

Sibling to `/tags/{id}`, deliberately smaller: no `EntityVideos`, no ancestor breadcrumb, no
video-count hero line (categories are flat and don't attach to videos directly — confirmed
non-goals).

```svelte
<div class="flex items-center gap-2">
  <h1 class="skin-title text-2xl font-semibold text-ink">{category.name}</h1>
  {#if isOwner}
    <button aria-label="Rename category" onclick={openRename}
      class="rounded-theme border border-rule p-1.5 text-muted hover:border-accent hover:text-ink">
      <!-- pencil glyph, same house stroke style as CurationChip's icons -->
    </button>
  {/if}
</div>
<p class="text-sm text-muted">{tagCount(category.tags.length)}</p>

<section class="space-y-1.5">
  <h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
  <div class="flex flex-wrap items-center gap-2">
    {#each category.tags as t}
      <!-- identical curation-chip idiom to the media page's Tags section: link to
           /tags/{t.id}, owner-only hover-reveal × remove -->
    {/each}
    {#if isOwner}<!-- + Add tag control, identical to the media page's: collapsed button →
                       plain input → submit, plus the same near-miss nudge card on a
                       near-duplicate name -->{/if}
  </div>
</section>
```

Non-owner: read-only chips, no rename button, no add-tag control — same posture as the media
page's non-owner tag chips (`<a class="rounded-theme bg-surface-2 px-2.5 py-1 …">`).

**Worth extracting while building this**: the media page's Tags section (chip + add-input +
near-miss nudge, `media/[id]/+page.svelte:509-597`) is inline today, not a shared component. This
page needs the identical behavior for a different owning entity (category, not video). Rather
than copy-pasting ~90 lines a second time, pull it into a shared `TagChipList.svelte` (params:
`tags`, `onadd`, `onremove`, `isOwner`) that both pages call. Flagging as a build-time
simplification, not a blocking requirement.

Rename reuses the tag ⋯-menu's exact inline `<form>` (input + Rename/Cancel), opened from the
pencil button instead of a popover menu (there's no ⋯ menu on this page — only one action lives
here besides tag add/remove).

---

## 4. Bulk "Add to category…" / "Remove from category…" (Manage-mode bar)

New buttons in the existing Manage bar (`tags/+page.svelte:299-347`), appearing once `manage` is
on and `selectedIds.length >= 2` (the spec's own P0 acceptance threshold). **Confirmed**: the
single-tag gap this threshold would otherwise leave is closed by the pill ⋯ menu's own new "Add
to category…" item (§2), not by lowering this threshold — mirrors the existing Merge split
(bulk bar for 2+, per-pill ⋯ menu for one).

```svelte
{#if selectedIds.length >= 2}
  <button class="btn-ghost px-3 py-1 text-sm" onclick={() => (assigning = 'add')}>
    Add to category…
  </button>
  <button class="btn-ghost px-3 py-1 text-sm" onclick={() => (assigning = 'remove')}>
    Remove from category…
  </button>
{/if}
```

Both open the new `CategoryPicker` (below), which is where the two modes actually diverge.

### New component: `entity/CategoryPicker.svelte`

Filed in `entity/` per the component-folder rule ("grouped by the mechanism it implements, not
its current call sites" — same rationale that puts `writeback/CropEditor.svelte` there despite
one caller): this copies `EntityPicker`'s search/listbox/keyboard shell, the same mechanism
`EntityPicker`/`MergeCanonicalDialog` already implement for "search and pick a target." **Confirmed
as a new sibling component**, not a fork of `EntityPicker` itself — assign/remove are structurally
different from merge's two-step, irreversible fold-and-delete confirm, the same reasoning that put
`WritebackBatchDialog` beside `WritebackFormDialog` rather than behind a mode-flag on it. Two
callers from the start: the Manage-bar bulk actions above, and the pill ⋯ menu's single-tag "Add
to category…" (§2).

**Props**:

```ts
{
  tagIds: number[];           // the selected tags this action applies to
  mode: 'add' | 'remove';
  categories: CategoryOption[]; // for 'remove', pre-filtered by the caller to categories that
                                 // intersect the selected tags' current memberships
  onclose: () => void;
  onapplied: () => void;
}
```

**Layout** — reuses `EntityPicker`'s backdrop/panel/title/search-input/listbox chrome verbatim
(`merge-pop` rise-in panel, `role="dialog"`, roving-tabindex `role="listbox"`/`role="option"`
rows, focus trap, `Escape`/backdrop-click close, focus-return-to-trigger):

```
┌ Add to category ──────────────────────────────┐
│ 3 tags selected                                 │
│ ┌───────────────────────────────────────────┐ │
│ │ Find or create a category…                 │ │
│ └───────────────────────────────────────────┘ │
│  ○ Character            (12 tags)               │
│  ○ Setting               (4 tags)                │
│  + Create "Vehicle"                              │
└──────────────────────────────────────────────────┘
```

- **Title** swaps by mode: `Add to category` / `Remove from category`.
- **`mode: 'add'`** — search-or-create, same posture as `EntityPicker`'s "Merge into…" search
  but with an extra trailing row when the query has no exact-name match: `+ Create "{query}"`
  (`text-accent`, `+` glyph prefix, same row shell as the other options, pinned last). Clicking an
  existing row or the create row **immediately assigns and closes** — no second confirm step.
  Unlike merge, assigning is additive and easily reversed with the symmetric "Remove from
  category…" action, so it doesn't need `EntityPicker`'s two-step "are you sure" (that step exists
  specifically because merge folds one entry away and deletes it — assign does neither).
- **`mode: 'remove'`** — same search shell, but the option list is exactly the `categories` prop
  passed in (no create row). If `categories` is empty (none of the selected tags share a
  category), skip opening the picker entirely and show an inline hint on the Manage bar instead —
  mirrors the Merge button's existing "select two or more" hint-on-click pattern rather than a
  dialog with nothing in it: *"None of the selected tags belong to a category yet."*
- Status line (`aria-live="polite"`) reads `"{n} categor{y/ies} — pick one to {add every selected
  tag to / remove every selected tag from}"`.
- On success: `onapplied()` → caller clears `selectedIds` and reloads, same post-mutation pattern
  every other Manage-mode action already follows.

---

## 5. Browse-page "Categories" facet

One new line in the existing filter row (`routes/+page.svelte:336-338`), inserted right after
Tags:

```svelte
<FacetFilter label="Tags" items={tagOptions} bind:selected={tagIDs} />
<FacetFilter label="Categories" items={categoryOptions} bind:selected={categoryIDs} />
<FacetFilter label="Studios" items={studioOptions} bind:selected={studioIDs} />
```

Zero component changes — `FacetFilter`'s `Option` type already treats `video_count` as optional
and already suppresses the count badge when it's falsy/undefined (`{#if m.video_count}`).
Categories have no video count of their own (no aggregate roll-up); `categoryOptions` should
simply omit the field rather than send a number that would misstate what a category "contains."

The category→tag-ID expansion is entirely a backend/query concern per the spec ("no new
filtering primitive, just a category → tag-ID expansion feeding the existing mechanism") — from
`+page.svelte`'s side this is just another `FacetFilter` bound to another ID array param
(`category_ids`) that the videos-query endpoint ORs in alongside `tag_ids` server-side. No
client-side "expand category to its member tag IDs" logic; that would put the expansion in the
wrong layer and get out of sync with the category's actual membership.

---

## Non-goals

Mirrors the spec's own: Suggested/AI categorization, category alias/merge machinery, aggregate
video roll-up on `/categories/{id}`, a separate top-level `/categories` list page.

---

## Accessibility notes

- Type filter: plain buttons, no extra ARIA — matches `SortToggle`'s existing (unadorned) pattern
  rather than inventing `radiogroup` semantics it doesn't have.
- Search input: `aria-label="Search tags and categories"`; results count is `aria-live="polite"`.
- Category ⋯ menu (Rename/Delete): identical keyboard model to the existing tag ⋯ menu — roving
  focus, `use:dismissable`, already fully specified there; nothing new to design.
- `CategoryPicker`: same combobox/listbox/keyboard model as `EntityPicker` — arrow-key roving,
  Enter/Space to pick, Esc to close, focus trap, focus returned to the trigger on close. Literally
  copy that implementation rather than re-deriving it.
- Tag-glyph icon on category pills: `aria-hidden="true"` (decorative — the pill's own text carries
  the meaning), same posture as the existing selected-state `✓` glyph and `CurationChip`'s icons.

## Measured contrast

Reuses tokens already measured in
[tag-writeback-exclusion-handoff.md](tag-writeback-exclusion-handoff.md) and
[writeback-selection-handoff.md](writeback-selection-handoff.md) — `border-accent`,
`bg-surface-2`, `text-accent`, `text-muted` (4.67–16.76:1 across all three skins for these
tokens). No new color combination is introduced by this change. The new tag-glyph icon renders at
`text-accent`, the same size class (`h-3.5 w-3.5`, close to `CurationChip`'s `h-3 w-3`) as icons
already covered by that pass — QA re-verifies rather than re-measures once built.

---

## Resolved decisions

Four decisions this handoff surfaced were resolved directly with the project owner rather than
left open for the ADR:

1. **Name-collision fold function** reuses `nameKeyExpr` — the same fold tag identity already
   uses — throughout (search, rename, and the category/tag collision check), for one consistent
   collision-check code path across both entity types.
2. **The single-tag "Add to category…" gap** closes via a new item on the tag pill's own ⋯ menu
   (§2), not by lowering the Manage-bar's 2+ threshold — mirroring the existing Merge split (bulk
   bar for 2+, per-pill ⋯ menu for one).
3. **`CategoryPicker.svelte` ships as a new sibling component**, not an extension of
   `EntityPicker` — assign/remove are structurally different from merge's two-step, irreversible
   fold-and-delete confirm, the same reasoning that put `WritebackBatchDialog` beside
   `WritebackFormDialog` rather than behind a mode-flag on it.
4. **Category delete lives only in `/tags`'s ⋯ menu**, not on `/categories/{id}` itself — avoids
   designing "you just deleted the page you're standing on" navigation handling.

The **frontend gate** (implementation + QA checklist, following the same S4 pattern as
[HOLODEX-239](../plans/HOLODEX-239.md)) is still open — `docs/plans/HOLODEX-240.md` tracks it.
This handoff is the input to that work and to the still-unwritten ADR, not a substitute for
either.
