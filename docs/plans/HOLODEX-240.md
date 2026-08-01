---
key: HOLODEX-240
status: In Progress
depends-on: [HOLODEX-239]
release_note: "Tags can now be grouped into hand-curated categories — browsable and filterable alongside tags, with no effect on file writeback."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-categories.md · S1
- [x] architecture  docs/architecture/ADR-077-tag-categories-entity.md · S3
- [x] backend       migration 0033 + internal/repo/categories.go + internal/api/categories.go · S4
- [x] frontend      tags/+page.svelte unified filter + category pills, entity/CategoryPicker.svelte,
                    categories/[id]/+page.svelte, browse Categories facet · S5
- [ ] testing       not started — regenerate testing-strategy for the new endpoints/flows
- [ ] security      not started

## Up next   (ordered — position is the priority; top line is the next action)
1. Regenerate testing-strategy for the new endpoints/flows, including ADR-077's cross-table collision
   and cascade-delete cases, plus the new `POST /tags` resolve-or-create-tag endpoint  [testing]
2. Security review — new owner-gated `POST /tags` (S5) needs the same scrutiny as the rest of the
   category mutation surface  [security]

## Session log   (append-only)
S6 · live 3-skin browser QA — dev-server restart succeeded this time (`backend-amv` + `web` via
`preview_start`); ran the full S5 surface by hand through the accessibility tree + computed-style
checks (screenshots still time out in this environment): created a category via the inline
"+ Create" row in `CategoryPicker` (add mode), confirmed the pill's `tag_count` badge and reduced
Rename/Delete ⋯ menu in Manage mode; on `/categories/{id}` exercised rename, `+ Add tag` (both a
brand-new name and a case-variant that correctly resolved to the existing lowercase tag via the
identity fold rather than creating a duplicate — the case-fold path, not the near-miss path, which
only fires on a *different* look-alike name), and tag removal; ran the Manage-bar bulk "Add to
category…"/"Remove from category…" flow on two tags at once, confirming "Remove from category…"
correctly pre-filters to only the categories that intersect every selected tag's membership; and
confirmed the browse page's new Categories `FacetFilter` round-trips `?category_id=` and filters the
video grid correctly. Spot-checked computed styles (background/border/text color, SVG `currentColor`
resolution) across Cinémathèque/Broadcast/Brutalist for the category pill, the `CategoryPicker`
dialog, and the detail page's chip/button — accent hex changed per-skin as expected in all three,
confirming no hardcoded colors. Zero console errors, zero failed network requests, across the whole
pass. QA gate closed — no defects found.

Noted in passing (not acted on): both spawned follow-up sessions from S5 (`PickerShell` extraction,
tag-resolve/popover-menu dedup) had already landed on this branch by the time QA ran (`64ca46d`,
`596455c`, `ef5333d`) — the picker/menu surfaces QA'd above are the post-refactor versions, and
everything held up. One of those sessions still had uncommitted new test coverage in
`internal/api/categories_test.go` / `internal/repo/categories_test.go` in the working tree at QA
time; left untouched (not this session's work to commit).

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
S4 · backend implementation — migration `0033_categories.{up,down}.sql` (categories +
ux_categories_namekey, category_tags junction, the four cross-table collision triggers);
`internal/repo/categories.go` (CreateCategory/RenameCategory/DeleteCategory/ListCategories/
GetCategory/TagsForCategory/AssignTagsToCategory/UnassignTagsFromCategory, each collision-checked
pre-flight per ADR-077 D3 via a shared `nameCollidesInTable` helper); `resolveOrCreateByName`'s tag
path gains the symmetric pre-flight check, with the new `ErrTagNameCollidesWithCategory` folded into
the scanner's and materialization's existing silent-skip lists alongside `ErrTagDenied`/
`ErrTagNameTooLong`; `internal/api/categories.go` (owner-gated CRUD + bulk assign/unassign at
`POST/DELETE /categories/{id}/tags`, public list/detail reads); `VideoFilter.CategoryIDs` browse
facet (repo.go, ADR-077 D2's exact `EXISTS(...)` clause shape, wired to `?category_id=`). Ran
`/simplify` before committing: unified the two near-duplicate collision-check helpers into one
table-parameterized function, replaced the assign/unassign per-tag-id loops with single batched
SQL statements (and had them return the updated category directly, cutting the handler's redundant
re-fetch), and folded the create/rename validation + error-mapping duplication in the API layer into
shared helpers. Full Go test suite green, including new repo + API coverage for CRUD, both
collision directions, cascade-delete, and the facet query. Deferred item 4 (`/tags` name-search)
since the design's client-side-filter approach may already cover it — revisit at frontend build
time. Backend gate closed; frontend is next.

S5 · frontend implementation — resolved the deferred item 1 (`/tags` name-search) in favor of the
design's client-side-filter-over-the-unpaged-list approach, confirming no new search endpoint was
needed. `tags/+page.svelte` gained the All/Tags/Categories segmented control (reusing `SortToggle`'s
shell), the search input, category pills (plain + Manage-mode variants, the latter non-selectable
with a reduced Rename/Delete ⋯ menu backed by `ConfirmDialog`), a new "Add to category…" item on the
tag pill's own ⋯ menu, and the Manage-bar bulk "Add to category…"/"Remove from category…" actions.
New `entity/CategoryPicker.svelte` (sibling of `EntityPicker`, per the handoff's resolved decision) —
single-step search-or-create (`mode="add"`) / search-only (`mode="remove"`), no informed confirm.
New `routes/categories/[id]/+page.svelte` — sparse detail page, member-tag chips mirroring the media
page's Tags section (bespoke copy this session, not extracted into a shared `TagChipList`, per the
handoff's own "not a blocking requirement" flag — still open, see Up next). Browse page
(`routes/+page.svelte`) gained a fourth `FacetFilter` for Categories.

Two small, reasoned backend additions beyond what S4 shipped, both needed to satisfy the design
contract discovered while building the frontend: (1) `ListCategories` now also returns `tag_count`
and `tag_ids` per category (the pill's count badge and the "Remove from category…" picker's
client-side membership filter — no new search endpoint, personal-library scale); (2) a new
`ResolveOrCreateTag`/`POST /tags` (owner-gated, no video attach) backs the `/categories/{id}`
"+ Add tag" control, which needs to create brand-new tags without a video — reuses the existing
`resolveOrCreateByName` choke point `AttachTagToVideo` already routes through.

Ran `/simplify` (4 parallel review agents — reuse/simplification/efficiency/altitude) before
committing. Applied: `ListCategories` now derives `TagCount` as `len(TagIDs)` instead of a separate
COUNT/JOIN query; a shared `filterByName()` helper (`lib/format.ts`) replaced 3 copies of the same
name-substring filter; `/categories/{id}`'s rename/add-tag/remove-tag mutations now use the
already-returned `Category` instead of discarding it and re-fetching; `useTagNearMiss`'s two
independent writes now run via `Promise.all` (a reload still follows, since either individual
response could race the other's write); the category pill's tag-glyph SVG (duplicated twice in
`tags/+page.svelte`) is now one `{#snippet categoryIcon()}`; a straight-vs-curly-quote drift in the
rename-collision error copy was unified. Deferred (spawned as follow-up task suggestions rather than
risking untested changes to already-shipped code): extracting a shared `PickerShell` out of
`EntityPicker`/`CategoryPicker`'s 100%-duplicated modal/listbox/focus-trap code; unifying the tag-pill
and category-pill popover-menu open/close/toggle mechanism in `tags/+page.svelte`; sharing the
tx/error-translation boilerplate between `attachVideoTag` and the new `createOrResolveTag`. Also
skipped patching the `categories` array in place after a rename/delete/assign (vs. the current
`reloadCategories()` full refetch) — a real but low-value efficiency win against real sort-order/
correctness risk to verify without live QA this session.

Go test suite green throughout (`internal/repo`, `internal/api`) — S5's backend additions
(`tag_count`/`tag_ids` on `ListCategories`, `ResolveOrCreateTag`/`POST /tags`) have no dedicated new
test cases yet; the existing `categories_test.go` coverage didn't regress, but explicit assertions
for the new fields/endpoint are testing-gate work, not squeezed in ad hoc here. `npm run check` (0
errors) and `npm run test` (115/115) green throughout.

**Live 3-skin browser QA not completed this session** — the sandboxed dev-server restart (needed
after clearing a stale local `data/holodex.db` with a pre-migration-0033 schema) was denied by the
environment's auto-mode classifier. Automated coverage stands in for it, but nobody has eyeballed
the new surfaces in Cinémathèque/Broadcast/Brutalist yet — top of Up next.

### 2026-07-31 · session
- skills: simplify
