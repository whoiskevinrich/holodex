# Spec: Tag Detail — Hierarchy & Category Controls

**Status**: Draft
**Story**: [HOLODEX-259](https://whoiskevinrich.atlassian.net/browse/HOLODEX-259)
**Epic**: [HOLODEX-240](https://whoiskevinrich.atlassian.net/browse/HOLODEX-240) (Tag Categories) — also extends F50 tag hierarchy ([HOLODEX-224](https://whoiskevinrich.atlassian.net/browse/HOLODEX-224), ADR-075)
**Owner**: Project owner
**Date**: 2026-08-07

**Depends on**:
- `internal/repo/tag_hierarchy.go` — `SetTagParent` (owner-gated, `ErrTagCycle` guard via `isTagDescendant`), `AncestorNamesForTag` (already surfaced on `tags/[id]` today as the read-only breadcrumb). ADR-075.
- `internal/repo/categories.go` — `ResolveOrCreateTag`, `CreateCategory`/`nameCollidesInTable`, `AssignTagsToCategory`/`UnassignTagsFromCategory`, `TagsForCategory`. ADR-078.
- `web/src/routes/tags/+page.svelte` — the `/tags` list's owner-gated "Manage tags" mode, which already has a per-pill parent-set typeahead (`applyParent`/`submitParent`, matching only already-loaded exact tag names — deliberately not resolve-or-create) with cycle-guard error passthrough, and `CategoryPicker` in single-tag mode for add/remove-to-category (HOLODEX-240 §4, "Bulk/single" comment confirms single-tag scope already exists).
- `web/src/routes/categories/[id]/+page.svelte` — the curation-chip idiom this spec ports in the reverse direction: chip + × to remove, "+ Add" opens an inline resolve-or-create form, non-blocking near-miss nudge card.
- `web/src/routes/tags/[id]/+page.svelte` — the current tag detail page. Its Details-card comment explicitly reserves "a future tag-categories row (spec P2)" for this work.

**New ADR required**: No — this extends the already-decided data models from ADR-075 (strict one-parent tree) and ADR-078 (flat categories, many-to-many); the only new backend surface is one additive read query (a tag's direct children), symmetric to the ancestor query ADR-075 already specified.

---

## Problem Statement

A tag's place in the hierarchy (its parent and children) and its category memberships can currently only be seen or changed from `/tags`' owner-gated bulk "Manage tags" mode. `tags/{id}` — the page a user is actually on when browsing a specific tag — shows a read-only ancestor breadcrumb and nothing else: no children, no categories, no way to fix a wrong parent without leaving the page and finding the tag again in the full list. The driving complaint is concrete: landing on a tag's own page and being unable to see or fix its place in the tree.

## Goals

1. The owner can see a tag's parent, direct children, and category memberships without leaving `tags/{id}`.
2. The owner can set or clear a tag's parent from `tags/{id}`, with the same cycle-guard protection and error messaging the `/tags` list already has.
3. The owner can attach an existing tag as a child, or create and attach a new one, from `tags/{id}`, in one input — without silently uprooting an existing branch if the typed name happens to match an established tag.
4. The owner can add or remove a tag's category memberships from `tags/{id}`, reusing `CategoryPicker` exactly as the list already does.
5. Non-owners can see all three (parent, children, categories) read-only — parity with the ancestor breadcrumb, which is already public today.

## Non-Goals

- **Full descendant subtree display.** Direct children only, mirroring the ancestor breadcrumb's one-hop-at-a-time feel. A grandchild is reached by clicking into its parent's own page, not shown inline — keeps the query cheap and the page bounded regardless of tree depth.
- **Removing the `/tags` list's Manage-mode parent control.** Resolved during scoping: keep both. The list's bulk-workflow value (reparenting while already scanning many tags) stays; this spec is additive, not a replacement. Revisit consolidating the two call sites later if the list control proves redundant in practice.
- **Bulk children or category operations from the detail page.** Single-tag actions only, matching every other control already on `tags/{id}` (writeback toggle, sync).
- **A new architectural decision on the tree or category data model.** ADR-075 and ADR-078 stand unchanged; see "New ADR required" above.
- **Any change to writeback or genre-materialization behavior.** Untouched — still governed by ADR-075/ADR-077. Setting a parent from this new surface calls the exact same `SetTagParent` endpoint the list already uses, so writeback's ancestor-expansion behavior is unaffected by where the call originates.

## User Stories

- As the owner, I want to see a tag's parent chip on its own detail page, so I don't have to leave and search the full `/tags` list to find where a tag sits.
- As the owner, I want to change a tag's parent from its detail page, so fixing a miscategorized tag doesn't require opening Manage mode.
- As the owner, I want to see a tag's direct children on its detail page, so I can navigate its subtree without already knowing which tag names to look for.
- As the owner, I want to attach an existing tag as a child, or spawn a brand-new child tag, from the same input, so building out a subtree doesn't require switching between two different controls.
- As the owner, if my "add child" input happens to match a tag that already has its own parent or children, I want a confirmation before anything moves, so I don't silently uproot an existing branch by typo.
- As the owner, I want to add or remove a tag's categories from its own page, so tag identity and category membership live in one place instead of two.
- As any visitor, I want to see a tag's parent, children, and categories read-only even without owner access, so browsing the taxonomy doesn't require admin rights — matching the ancestor breadcrumb that's already public today.

## Requirements

### P0 — Must-Have

**Backend: direct-children query**
- [ ] A new repo query returns a tag's direct children (id, name), name-ordered — the downward counterpart to `AncestorNamesForTag`'s upward walk. Not the full subtree; one level of `parent_tag_id = id`.
- [ ] `GetTag` attaches `Children` to its response the same way it already attaches `Ancestors`.

**Backend: category memberships on GetTag**
- [ ] A new repo query returns a tag's category memberships (id, name) — the reverse of `TagsForCategory`.
- [ ] `GetTag` attaches `Categories` to its response.

**Parent control**
- [ ] Owner-only edit. A single chip (0-or-1, not a list) shows the tag's current parent, or an empty/root state if none.
- [ ] Clicking opens the same existing-tags-only typeahead the `/tags` list's Manage-mode parent control already uses — shared/extracted logic, not a reimplementation. No create-on-typo: a name that doesn't match an existing tag is rejected, not silently turned into a new tag.
- [ ] Selecting a parent calls `SetTagParent`; a cycle response (`{cycle: true}`) surfaces the identical "Can't set X as its own ancestor" message the list already shows.
- [ ] × on the chip clears the parent (`SetTagParent(id, null)`) immediately — no confirm step, matching the writeback toggle's existing "lowest-stakes, no confirm" precedent on this same page.
- [ ] Non-owner: parent shown as a plain link to the parent's own page, no ×, no click-to-edit.

**Children control**
- [ ] Owner-only add. Direct children render as a chip list, each linking to that child's own detail page.
- [ ] "+ Add child" opens an inline resolve-or-create-by-name input, same idiom as `categories/{id}`'s "+ Add tag."
- [ ] If the resolved name is a brand-new tag: create it and set its parent to the current tag immediately, no confirm (low blast radius — an empty new leaf).
- [ ] If the resolved name matches an existing tag that is currently a root with no children of its own: attach immediately, no confirm (same low-risk profile as a new tag).
- [ ] If the resolved name matches an existing tag that already has a non-null parent, or at least one child of its own: interrupt with a confirm step before committing ("X already has its own parent/subtree — move it under here?") rather than silently reparenting an established branch. Exact copy deferred to implementation (see Open Questions).
- [ ] × on a child chip clears that child's parent (`SetTagParent(childID, null)`) — unparents it to root, does not delete the tag.
- [ ] Non-owner: children render as plain links, no × and no add-input.

**Categories control**
- [ ] Owner-only add/remove. Existing memberships render as chips (name links to `/categories/{id}`) with a × that calls `UnassignTagsFromCategory` for this one tag.
- [ ] "+ Add category" opens `CategoryPicker` in its existing single-tag mode (`tagIds: [tag.id]`) — not a freehand text input, since categories don't support resolve-or-create the way tags do (`CreateCategory` errors on an exact-name collision rather than resolving to the existing row). Picking an existing category or using the picker's own "+ Create" fallback both flow through the picker unchanged.
- [ ] Non-owner: categories render as plain links, no × and no add control.

**Gating**
- [ ] All three add/edit controls gated on `isOwner && tag`, matching the page's existing writeback section.
- [ ] All three read states (parent chip, children list, categories list) visible to every visitor, matching the existing ancestor breadcrumb.

### P1 — Nice-to-Have

- Surface a brief count next to "Children" and "Categories" headings (e.g. "Children · 2"), matching the existing `categories/{id}` "Tags · N" convention.

### P2 — Future Considerations

- Consolidating the `/tags` list's Manage-mode parent control with this page's parent control into one shared component invocation, if maintaining two call sites for the same cycle-guard flow proves error-prone in practice (see Non-Goals).
- Full descendant subtree view, if direct-children-only proves insufficient for deep trees (see Non-Goals).

## Success Metrics

Single-owner, self-hosted app — the practical bar:
- The owner can fix a wrong parent, attach or spawn a child, or manage a tag's categories entirely from `tags/{id}`, without a detour through `/tags`' Manage mode.
- The reparent-confirm guard never fires for the common case (creating a genuinely new child) and always fires when an existing branch would move — zero silent reparents of a tag that already had a parent or children.
- Zero regressions to the `/tags` list's existing Manage-mode parent/category controls — this spec adds a second surface, it does not touch the first.

## Open Questions

- ~~**Design**: exact copy and interaction shape for the reparent-confirm beat (point 3 of the Children control)~~ — **Resolved**: [tag-detail-hierarchy-reparent-confirm-handoff.md](../design/tag-detail-hierarchy-reparent-confirm-handoff.md). Reuses `ConfirmDialog` (`variant="destructive"`), not the near-miss card — the interrupt needs to be blocking, not advisory. Copy names what's actually moving (current parent name and/or child count, fetched via `api.getTag` on the resolved candidate — no new endpoint) rather than a generic "has a subtree" line.
- **Engineering**: the `/tags` list's parent typeahead currently lives as inline logic (`applyParent`/`submitParent`) in `tags/+page.svelte`'s script, not a standalone component. This spec assumes that logic gets extracted/shared rather than duplicated on `tags/{id}` — confirm during implementation planning rather than allowing a second, divergent copy to appear.

## Timeline Considerations

No hard deadline. Depends only on already-merged work (F50/HOLODEX-224, HOLODEX-240) — no other in-flight work blocks this.
