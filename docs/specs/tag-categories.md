# Spec: Tag Categories — grouping tags without merging them

**Status**: Draft
**Epic**: [HOLODEX-240](https://whoiskevinrich.atlassian.net/browse/HOLODEX-240)
**Owner**: Project owner
**Date**: 2026-07-31

**Depends on**:
- The `tags/{id}` Details-card scaffold and the `/tags` list's multi-select "Manage" bulk-action
  extension point, both originated by the tag writeback exclusion epic
  ([HOLODEX-239](https://whoiskevinrich.atlassian.net/browse/HOLODEX-239),
  `docs/specs/tag-writeback-exclusion.md`) — this epic builds additively on that scaffolding
  rather than restructuring `tags/{id}` or the Manage bar a second time.
- The existing tag identity system's merge UX (`web/src/routes/tags/+page.svelte` — select-pills,
  Merge bar, `MergeCanonicalDialog`, `EntityPicker`) as the interaction template for bulk
  category-assignment, without adopting its identity/merge *semantics*.
- The main library browse page's existing tag facet (`FacetFilter label="Tags"`,
  `web/src/routes/+page.svelte`), which the new category facet extends.

**New ADR required**: Likely — a small one covering the Category entity's deliberately reduced
lifecycle relative to Tag/Person/Studio (no provenance, no alias/merge system) and the
many-to-many junction shape.

---

## Problem Statement

Tags in Holodex are flat — there's no way to group related tags under a broader theme (e.g.
"Character," "Setting," "Vehicle") without either polluting search with a giant tag, or manually
remembering which tags relate to which theme. The owner wants a lightweight grouping layer, used
purely for browsing/filtering today, that also lays groundwork for semantic search later (e.g.
"movies where the main character wears a suit") without committing to that build now.

## Goals

1. An owner can group existing tags under categories they define, without creating a new tag or
   merging tags together.
2. A tag can belong to multiple categories.
3. Categories are fully CRUD-able, searchable, and filterable, on par with tags as a first-class
   browsing dimension.
4. Categorizing an existing, possibly large, tag library is tractable in bulk — not only
   one tag at a time.
5. Categories never affect file writeback or enrichment — they are strictly an in-app UX/browse
   layer.

## Non-Goals

- **Any file writeback or enrichment for categories.** Confirmed: never written to a file, never
  enriched. Only used to categorize existing tags.
- **"Suggested categories" / AI-assisted categorization.** Explicitly deferred — may be revisited
  later, not designed here.
- **Alias/merge machinery for categories** (the F43-style system Tag/Person/Studio have). A small,
  hand-curated list doesn't have the scanner-driven-duplicate problem that machinery solves for;
  v1 ships create/rename/delete only.
- **Aggregate video roll-up on a category's detail page.** A category doesn't attach to videos
  directly, only tags do; showing "every video across every tag in this category" is real added
  complexity (dedup, ordering) not requested for v1.
- **A separate top-level `/categories` list page.** Category management folds into the existing
  `/tags` page and its pill/Manage pattern — confirmed, not a parallel nav surface.

## User Stories

- As the owner, I want to create a category and assign existing tags to it, so I can group
  related tags under a theme without merging them or creating a new tag.
- As the owner, I want a tag to belong to more than one category, so overlapping themes (e.g. a
  tag that's both a Character and a Franchise) aren't forced into a single bucket.
- As the owner, I want to see categories alongside tags on `/tags`, filterable to "All / Tags /
  Categories," so I don't need a separate page to manage them.
- As the owner, I want to narrow the tag list by name and multi-select a batch of them to assign
  to a category in one action, so categorizing an existing library of tags doesn't mean visiting
  each tag individually.
- As the owner, I want to delete a category without being blocked by the tags still in it, so
  cleanup doesn't require manually emptying it first.
- As the owner, I want to filter the video library itself by category, so "everything in
  Character" works the same way filtering by a single tag already does.

## Requirements

### P0 — Must-Have

**Category entity**
- New `categories` table: `id`, `name`. No provenance/source, no alias system — create, rename,
  delete only.
- New many-to-many `tag_categories` junction table.
- Name collision check spans both tags and categories (confirmed) — a category can't share a
  name with an existing tag, or vice versa.
- [ ] Owner can create, rename, and delete a category.
- [ ] A tag can be linked to zero, one, or many categories.
- [ ] Creating a category or tag with a name that collides (case-insensitive fold, matching the
      existing tag-identity fold) with the other type is rejected with a clear error.

**Cascade delete**
- Deleting a category unassigns it from every tag (removes the junction rows) and deletes the
  category — no dependent-tag block.
- [ ] Deleting a category with N assigned tags succeeds in one action; those tags are unaffected
      other than losing the category link.
- [ ] The delete has a confirm step naming the affected tag count, consistent with the app's
      existing destructive-action confirms (e.g. media Move-to-Trash).

**`/tags` list — unified type filter**
- New "All / Tags / Categories" filter alongside the existing sort control.
- New name-search input over the combined pill grid — **does not exist today** (verified: no
  search/filter input currently on `/tags`, only an in-dialog typeahead scoped to the merge
  picker) — this epic adds it as a prerequisite for both findability and the bulk-assign flow
  below.
- Category pills are visually distinct from tag pills even in the unified "All" view: an
  accent-colored tag-glyph icon plus an accent border (confirmed direction).
- [ ] The filter narrows the grid to tags only, categories only, or both.
- [ ] The search input narrows the grid by name (both types) as the owner types.
- [ ] A category pill is visually distinguishable from a tag pill without relying on the filter
      state alone.

**`/categories/{id}` detail page**
- Deliberately sparse: category name, no ancestor breadcrumb (categories are flat), no video
  grid (confirmed non-goal).
- Shows member tags as chips (linking to each tag's own `/tags/{id}`), with add/remove — reusing
  the exact add/remove-chip interaction already shipped on the media page's Tags section.
- [ ] Owner can add a tag to the category by name (with the same near-miss handling the media
      page's tag-add already has) and remove one via its chip.
- [ ] Non-owners see the member-tag list read-only, no add/remove controls.

**Bulk category assignment from `/tags`**
- Reuses the existing multi-select "Manage" mode's *interaction shape* (select pills → an action
  bar appears → pick a target) — the same shape merge already uses — but the action is additive
  (assign), not destructive (merge), and the target is a category, not a surviving tag name.
- Selecting a target category reuses the `EntityPicker` search-or-create pattern already used for
  "Merge into…," so assigning to a brand-new category doesn't require leaving the flow to create
  it first.
- Depends on the search input above to be usable at any real scale — narrow by name, then
  multi-select, then assign.
- [ ] Selecting 2+ tags in Manage mode surfaces an "Add to category…" action.
- [ ] The category picker supports searching existing categories or creating a new one inline.
- [ ] Confirming assigns every selected tag to the chosen category (additive — doesn't disturb
      any category a tag is already in).
- [ ] A symmetric "Remove from category…" bulk action is available the same way.

**Browse-page category facet**
- New "Categories" facet on the main library page, parallel to the existing `FacetFilter
  label="Tags"` (`web/src/routes/+page.svelte`).
- Selecting a category server-side expands it to its member tag IDs and ORs them into the same
  filter the Tags facet already drives — no new filtering primitive, just a category → tag-ID
  expansion feeding the existing mechanism.
- [ ] Selecting a category in the facet returns every video tagged with any of its member tags.
- [ ] Combining a Categories facet selection with other existing facets (Tags, etc.) behaves the
      same way combining two Tags selections already does.

### P1 — Nice-to-Have

- Empty-state guidance on `/tags` when the Categories filter has zero categories yet (the default
  for a fresh instance — "assume nothing," confirmed from the outset), pointing at how to create
  the first one, rather than a bare empty grid.

### P2 — Future Considerations

- "Suggested categories" / AI-assisted assignment (explicitly deferred, see Non-Goals).
- A description field on Category, and any aggregate video roll-up on its detail page — the
  natural next asks once semantic search work actually starts. Not building the schema for this
  preemptively; noting it so P0's schema doesn't need a breaking change to add a nullable
  `description` column later.

## Success Metrics

Single-owner, self-hosted app — the practical bar:
- An existing tag library of 50+ tags can be sorted into categories in one sitting using the
  bulk-assign flow, without the search/narrow step feeling like the bottleneck.
- The browse-page category facet returns results indistinguishable in correctness from manually
  OR-ing the category's member tags in the existing Tags facet.
- Zero regressions in existing tag-identity test coverage (name-fold collision checks, merge
  flow) from the shared collision-check and reused `EntityPicker`/Manage-bar surfaces.

## Open Questions

- **Engineering**: exact migration numbering and whether `tag_categories` needs its own indexes
  beyond the natural composite key — resolve during implementation.
- **You**: should the category name-collision check use the exact same fold function as tag
  identity (`nameKeyExpr`), or is a simpler case-insensitive check sufficient given categories
  never go through the scanner/near-miss system tags do? Assumed: reuse the same fold for
  consistency, flagging since it's a small, easily-revisited implementation choice.

## Timeline Considerations

No hard deadline. Sequenced after tag writeback exclusion (HOLODEX-239), which this epic's
`tags/{id}` and Manage-bar work builds on directly.
