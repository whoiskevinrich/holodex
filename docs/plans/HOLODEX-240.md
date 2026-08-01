---
key: HOLODEX-240
status: In Progress
depends-on: [HOLODEX-239]
release_note: "Tags can now be grouped into hand-curated categories — browsable and filterable alongside tags, with no effect on file writeback."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-categories.md · S1
- [x] architecture  docs/architecture/ADR-077-tag-categories-entity.md · S3
- [ ] backend       not started
- [ ] frontend      not started
- [~] testing       deferred until: backend/frontend implemented
- [~] security      deferred until: architecture + backend implemented

## Up next   (ordered — position is the priority; top line is the next action)
1. Migration `0033_categories.{up,down}.sql`: `categories` table + `ux_categories_namekey`,
   `category_tags` junction, the four cross-table collision triggers (tag↔category, insert+rename) — see
   ADR-077 D1-D3  [backend]
2. `internal/repo/categories.go`: `CreateCategory`/`RenameCategory`/`DeleteCategory`, each with an
   app-layer pre-flight collision check for a friendly `409` ahead of the trigger backstop (ADR-077 D3);
   `resolveOrCreateByName`'s tag path gains the symmetric pre-flight check against `categories`  [backend]
3. Category CRUD endpoints (create/rename/delete, cascade-unassign on delete is free via `ON DELETE
   CASCADE` — ADR-077 D2)  [backend]
4. `/tags` name-search endpoint/param (prerequisite — doesn't exist today) + All/Tags/Categories filter  [backend, frontend]
5. `/categories/{id}` page: member-tag chips, add/remove, no video grid — extract shared `TagChipList`
   from the media page's Tags section while building this (see handoff)  [frontend]
6. New `entity/CategoryPicker.svelte` (sibling of `EntityPicker`, not an extension) driving bulk
   "Add to category…" / "Remove from category…" on `/tags` Manage mode + the pill ⋯ menu's
   single-tag "Add to category…" item  [frontend]
7. Browse-page "Categories" facet: `VideoFilter.CategoryIDs` expands to member tag IDs via
   `category_tags`, feeding the existing `TagIDs` `EXISTS(...)` clause shape — no new query primitive
   (ADR-077 D2/Consequences)  [backend, frontend]
8. Regenerate testing-strategy for the new endpoints/flows, including ADR-077's cross-table collision
   and cascade-delete cases  [testing]
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
S3 · /architecture — docs/architecture/ADR-077-tag-categories-entity.md. `categories`/`category_tags`
mirror `tags`' pre-identity shape and `video_tags` respectively (no provenance, `ON DELETE CASCADE` both
sides — cascade-delete is free). Category deliberately kept outside `resolveOrCreateByName`'s identity
spine (D1/D4) — it never hits the scanner-duplicate problem that spine solves for. The one genuinely new
pattern: cross-table name collision (category vs. tag) has no existing precedent in this codebase, so D3
resolves it explicitly — paired DB triggers on both tables (not an app-layer-only check), matching the
codebase's existing posture for same-table `nameKey` uniqueness (real unique indexes), extended to a
two-table case SQLite can express (unlike ADR-075 D1's cycle guard, which genuinely couldn't be). Browse
facet expansion reuses `VideoFilter.build()`'s existing `TagIDs` clause shape with no new primitive. No
code yet — backend gate is next.
