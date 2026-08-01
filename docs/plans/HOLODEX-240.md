---
key: HOLODEX-240
status: In Progress
depends-on: [HOLODEX-239]
release_note: "Tags can now be grouped into hand-curated categories — browsable and filterable alongside tags, with no effect on file writeback."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-categories.md · S1
- [ ] architecture  not started
- [ ] backend       not started
- [ ] frontend      not started
- [~] testing       deferred until: backend/frontend implemented
- [~] security      deferred until: architecture + backend implemented

## Up next   (ordered — position is the priority; top line is the next action)
1. Write the ADR covering the Category entity's reduced lifecycle + the many-to-many junction
   shape — now also covers `nameKeyExpr` reuse for the collision check and the category→tag-ID
   facet-expansion query, both settled at the design layer (see handoff)  [architecture]
2. Migrations: `categories` table + `tag_categories` junction; shared name-collision fold with tags  [backend]
3. Category CRUD endpoints (create/rename/delete, cascade-unassign on delete)  [backend]
4. `/tags` name-search endpoint/param (prerequisite — doesn't exist today) + All/Tags/Categories filter  [backend, frontend]
5. `/categories/{id}` page: member-tag chips, add/remove, no video grid — extract shared `TagChipList`
   from the media page's Tags section while building this (see handoff)  [frontend]
6. New `entity/CategoryPicker.svelte` (sibling of `EntityPicker`, not an extension) driving bulk
   "Add to category…" / "Remove from category…" on `/tags` Manage mode + the pill ⋯ menu's
   single-tag "Add to category…" item  [frontend]
7. Browse-page "Categories" facet, expanding to member tag IDs against the existing Tags facet  [backend, frontend]
8. Regenerate testing-strategy for the new endpoints/flows  [testing]
9. Security review  [security]

## Session log   (append-only)
S1 · /product-brainstorming /write-spec — spec drafted, epic filed (blocked-by HOLODEX-239), Draft PR opened
S2 · /design-handoff — docs/design/tag-categories-handoff.md; grounded every surface in the actual
component source (SortToggle, EntityPicker, MergeCanonicalDialog, ConfirmDialog, FacetFilter, the
media-page Tags chip section, the tag ⋯-menu inline rename form) rather than inventing new
patterns. Key calls: unified type filter reuses SortToggle's shell verbatim; category pills get an
accent border + decorative tag-glyph icon, non-selectable in Manage mode (click still navigates)
with their own reduced ⋯ menu (Rename/Delete only); `/categories/{id}` reuses the media page's
tag-chip+near-miss idiom verbatim (flagged extracting it into a shared `TagChipList`); new
`entity/CategoryPicker.svelte` as a sibling of `EntityPicker` (single-step assign/remove, no
merge-style two-step confirm — additive and reversible, unlike merge). Four open questions
resolved directly with the owner via AskUserQuestion: reuse `nameKeyExpr` for the collision fold;
close the single-tag "Add to category…" gap via a new pill ⋯-menu item rather than lowering the
Manage-bar threshold; ship `CategoryPicker` as a new sibling rather than extending `EntityPicker`;
category delete lives only in `/tags`'s ⋯ menu, not duplicated on the detail page. No code yet —
frontend gate stays open pending the ADR + backend.
