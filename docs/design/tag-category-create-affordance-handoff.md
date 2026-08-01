# Design Handoff: Tag & category create affordance (HOLODEX-243)

**Spec**: [tag-category-create-affordance.md](../specs/tag-category-create-affordance.md)
**ADR**: none — spec confirms no new ADR ("UI-only addition on an already-decided data model").
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Prior art**: [tag-categories-handoff.md](tag-categories-handoff.md) — the unified `/tags`
type-filter/search this feature's pill sits inside, and its "design-system-fit audit" format this
handoff follows. `CategoryPicker.svelte`'s inline "+ Create "query"" row is the create pattern
this feature promotes to a standing affordance (see audit below).
**Depends on**: HOLODEX-240 (merged, #194) — the unified pill grid, type filter, and search input
on `/tags` this feature's pill is inserted into.
**Surfaces**: `tags/+page.svelte` only. No new route, no new component file.

---

## Overview

One addition: a standing, always-visible, owner-only "+ New" pill, first in the `/tags` grid,
that expands in place into a small create form (name + Tag/Category type toggle) and reuses the
page's existing near-miss and collision handling. No backend changes — `POST /tags`
(`api.resolveOrCreateTag`) and `POST /categories` (`api.createCategory`) are both already wired
and unchanged by this work.

### Design-system-fit audit

**Zero new tokens, zero new components. Every visual piece already exists on this page or its
siblings — this is a recombination, not a new pattern.**

- **The pill shell** — the exact tag/category pill shape (`rounded-full border … px-3 py-1.5
  text-sm`), with a dashed rather than solid border (new *modifier*, not a new shell) to read as
  "add a thing" rather than "here is a thing" — the same solid-vs-dashed distinction
  `CategoryPicker`'s own "+ Create" row already makes with `text-accent` vs. a plain option row.
- **The expand-in-place popover** — literally the same shell the tag pill's own ⋯ menu already
  uses for its rename/alias/parent forms (`absolute … rounded-theme border border-rule bg-surface-2
  p-2 shadow-sm`), anchored to this pill instead of a ⋯ trigger.
- **The type toggle** — the exact segmented-control shell already used twice on this page
  (`SortToggle`, and the All/Tags/Categories filter): `flex overflow-hidden rounded-theme border
  border-rule text-sm`, same active/inactive classes, just two segments instead of three.
- **The name input + submit/cancel** — the identical `<form>` shape already coded for tag
  rename/alias (`tags/+page.svelte:659-714`): input styled `rounded-theme border border-rule
  bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none`, submit
  `rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60`,
  cancel `rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface
  disabled:opacity-60`.
- **The near-miss card** — the exact `actionNearMiss` card already coded for tag rename/alias
  (`tags/+page.svelte:576-606`), reused verbatim for a newly created tag.
- **The collision error copy** — the exact string already coded for category rename's 409
  (`tags/+page.svelte:298`: `"“{name}” already names a tag or another category."`), reused
  verbatim for category creation's 409.

Audit output: **zero new components, zero new tokens, one new pill modifier (dashed border), one
new local state group on an existing route file.**

---

## 1. The "+ New" pill

**Resting state** — first item in the pill grid (before any tag/category pills), owner-only,
rendered regardless of `typeFilter`, active search `query`, or `manage` mode:

```svelte
<button
  type="button"
  data-create-pill
  onclick={() => (createOpen = true)}
  class="relative rounded-full border border-dashed border-rule px-3 py-1.5 text-sm text-muted hover:border-accent hover:text-accent"
>
  + New
</button>
```

Dashed border is the only new visual primitive this feature introduces — everything else below
reuses an existing shape. `data-create-pill` mirrors the existing `data-tag-pill`/`data-cat-pill`
markers so it can be added as a third `inside` target on the grid's existing `use:dismissable`
wiring (`tags/+page.svelte:487-488`) — clicking outside or Escape closes the form the same way it
already closes the tag/category ⋯ menus.

**Placement gotcha (implementation-critical):** the empty-state branch
(`tags/+page.svelte:480-483`) currently renders *only* the `<p>{emptyMessage}</p>` line and skips
the grid `<div>` — meaning the pill, if left inside that `<div>`, would disappear exactly when
it's needed most (a fresh instance with zero tags and zero categories). **Hoist the pill above
the `{#if loading}…{:else if empty}…{:else}` conditional** so it always renders, and fold the
empty message into a short inline line beside it rather than a full-section replacement — see §3.

## 2. Expanded form

Clicking the pill swaps it for the same popover shell the tag ⋯ menu already uses for its
rename/alias/parent forms, anchored `left-0` (not `right-0` like the per-pill ⋯ menus — this pill
is always the leftmost item, so a right-anchored popover would risk clipping on narrow viewports;
left-anchoring never does):

```
┌ + New ──────────────────────────┐
│ ┌────────┬──────────┐            │
│ │  Tag   │ Category │  ← segmented, "Tag" active by default
│ └────────┴──────────┘            │
│ ┌────────────────────────────┐  │
│ │ Tag name                    │  │  ← autofocused on open
│ └────────────────────────────┘  │
│  [ Create ]  [ Cancel ]          │
└───────────────────────────────────┘
```

```svelte
<div class="absolute left-0 top-full z-10 mt-1 w-72 rounded-theme border border-rule bg-surface-2 p-2 shadow-sm">
  <div class="mb-2 flex overflow-hidden rounded-theme border border-rule text-sm">
    <button type="button" onclick={() => (createType = 'tag')} class={typeCls(createType === 'tag')}>Tag</button>
    <button type="button" onclick={() => (createType = 'category')} class={typeCls(createType === 'category')}>Category</button>
  </div>
  <form onsubmit={submitCreate} class="space-y-2">
    <input
      bind:this={createInput}
      bind:value={createValue}
      type="text"
      placeholder={createType === 'tag' ? 'Tag name' : 'Category name'}
      aria-label={createType === 'tag' ? 'New tag name' : 'New category name'}
      class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
    />
    <div class="flex flex-wrap gap-2">
      <button type="submit" disabled={createBusy} class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60">
        Create
      </button>
      <button type="button" onclick={closeCreate} disabled={createBusy} class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60">
        Cancel
      </button>
    </div>
    {#if createError}<p class="text-sm text-warn">{createError}</p>{/if}
  </form>
</div>
```

`typeCls` is the same active/inactive helper already defined for the All/Tags/Categories filter
(`typeCls(active: boolean)` at `tags/+page.svelte:32-33`) — reused, not reimplemented.

**Container semantics**: plain `<div>`, no `role="menu"`. This is a deliberate departure from the
tag/category ⋯ menus' `role="menu"` wrapper — those toggle between an *action list*
(`role="menuitem"` rows) and a form; this popover is *always* a form, so a form-region container
fits its actual content better than menu semantics borrowed from a sibling that also has a
menu-list state. `aria-label="Create a tag or category"` on the wrapper.

**Local state** — a plain state group, not `PopoverMenu` (that class models "one-of-many open,
keyed by an id"; this is a singleton control with no id to key on):

```ts
let createOpen = $state(false);
let createType = $state<'tag' | 'category'>('tag');
let createValue = $state('');
let createBusy = $state(false);
let createError = $state('');
let createInput = $state<HTMLInputElement | null>(null);
// Tag-only: createNearMiss reuses the exact actionNearMiss card/actions (§1 of this doc);
// wired to the *new* tag's id, not an existing one.
let createNearMiss = $state<EntityRef | null>(null);
```

**Open**: `createOpen = true`; reset `createType = 'tag'` (always — no sticky memory of the last
choice, per the spec's resolved open question), clear `createValue`/`createError`/`createNearMiss`;
focus `createInput` after tick (mirrors `startAction`'s `await Promise.resolve(); actionInput?.focus();`).

**Close/cancel**: `createOpen = false`; same reset as open, so reopening never shows stale state.

## 3. Submit behavior — diverges by type (important asymmetry)

The two backend calls have genuinely different collision semantics — the form must not treat
them identically:

**Tag** (`api.resolveOrCreateTag(name)`, i.e. `POST /tags`): **resolves silently on an exact
name match** — it never errors for "already exists." So there is no collision state for tags;
submitting an existing name just returns that tag. After *any* success (newly created or
resolved), run the same fuzzy near-miss check the media page's `+ Add tag` already runs post-add:

```ts
const { tag } = await api.resolveOrCreateTag(name);
reload();
const nm = await api.nearMiss('tag', tag.id, name).then((r) => r.near_miss);
if (nm) createNearMiss = nm;   // swap the form for the actionNearMiss card, reused verbatim
else closeCreate();
```

If `createNearMiss` is set, swap the popover's form content for the exact `actionNearMiss` card
markup (`tags/+page.svelte:579-606`), with its "Merge them in" / "Keep both" actions pointed at
`tag.id` (the just-created tag) instead of an edited existing one — the underlying calls
(`api.mergeEntities('tag', tag.id, nm.id)` / `api.dismissDuplicate('tag', tag.id, nm.id)`) are
identical to the existing `mergeNearMiss`/`keepBoth` functions; parameterize or duplicate them
rather than diverge the copy.

**Category** (`api.createCategory(name)`, i.e. `POST /categories`): **409s on an exact
collision** against either another category or an existing tag — there is no resolve-or-create
here, and categories have no near-miss/fuzzy system (HOLODEX-240 non-goal). Reuse
`submitCatRename`'s exact error-formatting pattern:

```ts
try {
  await api.createCategory(name);
  reloadCategories();
  closeCreate();
} catch (err) {
  createError =
    err instanceof ApiError && err.status === 409
      ? `“${name}” already names a tag or another category.`
      : toMessage(err);
}
```

No merge offer on a category collision — matches `CategoryPicker`'s own posture (no merge
concept for categories; the collision is just "pick a different name").

**Both paths**: `createBusy` disables the submit/cancel buttons for the duration (`disabled:opacity-60`
is fine here — it's on `text-accent-ink`/`text-ink`, not `text-muted`, so the "never dim
`text-muted`" contrast rule doesn't apply, matching the existing rename form's identical pattern).

## 4. Empty-state wiring

Current copy (`tags/+page.svelte:151-157`, `emptyMessage`) is a dead end. Once the pill is
hoisted above the conditional (§1), the empty branch becomes an inline invitation next to it
rather than the sole content of the section:

```svelte
{#if visibleTags.length === 0 && visibleCategories.length === 0 && !query.trim()}
  <p class="py-2 text-sm text-muted">
    {isOwner ? 'Nothing here yet — create your first one above.' : emptyMessage}
  </p>
{/if}
```

Non-owner empty state is unchanged (they have no pill to point at, so the existing status-only
copy stays).

## 5. Interaction states

| State | Trigger | Appearance |
|---|---|---|
| Idle | default | Dashed pill, `+ New`, muted text |
| Idle (hover/focus) | pointer/keyboard | Border and text shift to `border-accent`/`text-accent` |
| Expanded, empty | click | Popover open, Tag segment active, input focused, empty value, Create enabled (CDS "avoid disabled" posture — submitting empty is a no-op guarded in JS, not a disabled button) |
| Submitting | valid submit | `createBusy`, Create/Cancel both `disabled:opacity-60` |
| Category collision | 409 on create | Form stays, `text-warn` line below input, focus stays in input |
| Tag near-miss | fuzzy match post-create | Form content swaps for the `actionNearMiss` card (name, video count — 0 for a brand-new tag, "Merge them in" / "Keep both") |
| Success, no near-miss | 200, no fuzzy match | Popover closes, pill returns to idle, grid refetches and shows the new pill |
| Cancelled | Cancel / Escape / click-outside | Popover closes, all local state resets |

## 6. Edge cases

- **Empty submit**: `submitCreate` no-ops on a blank/whitespace-only trimmed value (`if (!name ||
  createBusy) return;`), mirroring every other form on this page — never a disabled button.
- **Very long names**: relies on the existing `POST /tags`/`POST /categories` 400 length-cap
  response, surfaced via `toMessage(err)` same as any other form error on this page — no new
  client-side length limit introduced.
- **Rapid double-submit**: guarded by `createBusy` exactly like `tagMenu.busy`/`catMenu`'s own
  submit guards — a second Enter/click while busy is a no-op.
- **`manage` toggled while the popover is open**: entering/exiting Manage mode doesn't touch
  `createOpen` — the create pill's own click handler is independent of pill-body selection
  toggling (only tag/category pill bodies change behavior in Manage mode), so leaving it open is
  harmless and expected.
- **`categories` not yet loaded**: `createCategory`'s 409 is a server-side check, so a
  client-side "does this already exist" pre-check isn't required for correctness — don't add one
  gated on the `categories` array being populated, since that would introduce a false-negative
  window on first paint before `reloadCategories()` resolves.

## Non-goals

Mirrors the spec's own: no backend changes, no bulk/paste-list creation, no change to
`CategoryPicker`'s existing inline "+ Create" row (both entry points coexist), non-owner access
unchanged.

---

## Accessibility notes

- Pill: `<button>`, no extra ARIA needed for the resting state (a plain toggle trigger, same
  posture as the tag ⋯ menu trigger before it's wired to `aria-expanded`/`aria-haspopup` — since
  this pill opens a form, not a menu, it should get `aria-expanded={createOpen}` but *not*
  `aria-haspopup="menu"`, matching the "form, not menu" semantics decided in §2).
- Popover: plain `<div aria-label="Create a tag or category">`, not `role="menu"` — see §2's
  rationale.
- Type toggle: plain buttons, no `radiogroup` semantics, matching `SortToggle`'s own unadorned
  pattern (same precedent the type-filter reused in HOLODEX-240).
- Focus order: pill → (on open) type-toggle Tag → type-toggle Category → name input (autofocused,
  so Tab from a fresh open goes input → Create → Cancel; a keyboard user who Shift+Tabs backward
  from the input reaches the toggle first) → Create → Cancel.
- Escape and click-outside close the popover and return focus to the `+ New` pill, matching the
  existing `use:dismissable` + focus-restore behavior already wired for the tag/category ⋯ menus.
- Near-miss card and collision error text reuse existing, already-accessible markup verbatim
  (`aria-live` is not newly needed here since the error/card appears synchronously in response to
  a user-initiated submit, same posture as the existing rename/alias forms).

## Measured contrast

No new color combination — every token used (`border-rule`, `text-muted`, `border-accent`,
`text-accent`, `bg-accent`/`text-accent-ink`, `text-warn`, `bg-surface-2`) is already measured in
[tag-writeback-exclusion-handoff.md](tag-writeback-exclusion-handoff.md) and
[tag-categories-handoff.md](tag-categories-handoff.md). QA re-verifies the dashed-border variant
specifically (a thinner visual weight than the solid borders already measured) rather than
re-measuring the underlying colors.

---

## Resolved decisions

1. **Type toggle always resets to "Tag" on open** — carried forward from the spec's own resolved
   open question; no sticky memory of the owner's last choice.
2. **Tag and category creation are NOT symmetric** — tag creation resolves-or-creates silently
   (no collision state, near-miss handling applies); category creation hard-409s on collision (no
   near-miss, error-only). The form must branch on `createType`, not share one success/error path.
3. **Popover uses form semantics (`aria-label`, no `role="menu"`), not menu semantics** — this
   popover is always a form, unlike the tag/category ⋯ menus which toggle between an action list
   and a form.
4. **The empty-state message becomes a short inline line beside the (now always-rendered) pill**,
   not a full-section replacement — requires hoisting the pill out of the existing
   loading/empty/grid conditional (`tags/+page.svelte:478-484`), flagged as implementation-critical
   in §1.

No open design questions remain — this handoff is ready for implementation. The QA checklist
(numbered, `[smoke]`/`[agent]`/`[human]`-tagged, per this repo's usual pairing) is left for the
implementation pass itself rather than authored ahead of code that doesn't exist yet — same
posture the Tag Categories epic (HOLODEX-240) took, which shipped its handoff without a
checklist stub.
