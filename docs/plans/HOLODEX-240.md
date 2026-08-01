---
key: HOLODEX-240
status: In Progress
depends-on: [HOLODEX-239]
release_note: "Tags can now be grouped into hand-curated categories — browsable and filterable alongside tags, with no effect on file writeback."
---

## Gates — definition of done
- [x] spec          docs/specs/tag-categories.md · S1
- [x] architecture  docs/architecture/ADR-078-tag-categories-entity.md · S3
- [x] backend       migration 0034 + internal/repo/categories.go + internal/api/categories.go · S4
- [x] frontend      tags/+page.svelte unified filter + category pills, entity/CategoryPicker.svelte,
                    categories/[id]/+page.svelte, browse Categories facet · S5
- [x] testing       docs/testing-strategy.md regenerated (§4/§5/§6/§9/§11 + Critical invariants) · S7;
                    closed both flagged S5 backend test gaps with new repo+API coverage
- [x] security      `/security-review` · S8 — clean, no findings ≥0.7 confidence

## Up next   (ordered — position is the priority; top line is the next action)
1. Mark PR #194 ready for review (drop Draft per ADR-069 — all six gates are now green)
2. Product decision on the `ResolveOrCreateTag` zero-video-tag visibility gap found in S7 (a
   brand-new tag added via `/categories/{id}`'s "+ Add tag" is invisible on `/tags`/search/the merge
   picker until a video is tagged with it, since `ListTags` inner-joins `video_tags`) — not a
   data-integrity bug, but a real UX surprise worth a call before it's forgotten  [product]
3. Fix the pre-existing `EntityPicker`/`CategoryPicker` focus-restore-to-trigger gap found in S7
   while confirming the `PickerShell` extraction preserved behavior (Escape/close lands focus on
   `<body>`, not the trigger button) — confirmed pre-existing on the pre-extraction code too, not a
   regression, but a real minor a11y bug  [frontend, a11y]

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
S3 · /architecture — docs/architecture/ADR-078-tag-categories-entity.md. `categories`/`category_tags`
mirror `tags`' pre-identity shape and `video_tags` respectively (no provenance, `ON DELETE CASCADE` both
sides — cascade-delete is free). Category deliberately kept outside `resolveOrCreateByName`'s identity
spine (D1/D4) — it never hits the scanner-duplicate problem that spine solves for. The one genuinely new
pattern: cross-table name collision (category vs. tag) has no existing precedent in this codebase, so D3
resolves it explicitly — paired DB triggers on both tables (not an app-layer-only check), matching the
codebase's existing posture for same-table `nameKey` uniqueness (real unique indexes), extended to a
two-table case SQLite can express (unlike ADR-075 D1's cycle guard, which genuinely couldn't be). Browse
facet expansion reuses `VideoFilter.build()`'s existing `TagIDs` clause shape with no new primitive. No
code yet — backend gate is next.
S4 · backend implementation — migration `0034_categories.{up,down}.sql` (categories +
ux_categories_namekey, category_tags junction, the four cross-table collision triggers);
`internal/repo/categories.go` (CreateCategory/RenameCategory/DeleteCategory/ListCategories/
GetCategory/TagsForCategory/AssignTagsToCategory/UnassignTagsFromCategory, each collision-checked
pre-flight per ADR-078 D3 via a shared `nameCollidesInTable` helper); `resolveOrCreateByName`'s tag
path gains the symmetric pre-flight check, with the new `ErrTagNameCollidesWithCategory` folded into
the scanner's and materialization's existing silent-skip lists alongside `ErrTagDenied`/
`ErrTagNameTooLong`; `internal/api/categories.go` (owner-gated CRUD + bulk assign/unassign at
`POST/DELETE /categories/{id}/tags`, public list/detail reads); `VideoFilter.CategoryIDs` browse
facet (repo.go, ADR-078 D2's exact `EXISTS(...)` clause shape, wired to `?category_id=`). Ran
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
after clearing a stale local `data/holodex.db` with a pre-migration-0034 schema) was denied by the
environment's auto-mode classifier. Automated coverage stands in for it, but nobody has eyeballed
the new surfaces in Cinémathèque/Broadcast/Brutalist yet — top of Up next.

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

S7 · /testing-strategy — regenerated docs/testing-strategy.md: new §4 backend rows (entity + cross-
table collision, junction/cascade/facet, `ResolveOrCreateTag`), new §5 frontend rows (`PickerShell`,
`EntityPicker`/`CategoryPicker` post-extraction, the four newer S5 surfaces), two new Critical-
invariants bullets (cross-table collision enforced at both app + DB-trigger layers, cascade-delete
leaves member tags intact), a new §6 E2E flow, a §9 phasing block documenting this epic's actual
build order (unlike F50's docs-first entries, S1–S5 here were already shipped before this update),
and two new §11 Known Gaps entries for the findings below. Closed the two S5 backend test gaps
`docs/plans/HOLODEX-240.md` itself flagged as outstanding: `ListCategories`' `tag_count`/`tag_ids`
fields (new `TestListCategoriesTagFields` + a list-endpoint assertion folded into
`TestCategoryEndpoints`) and `ResolveOrCreateTag`/`POST /tags` (new `TestResolveOrCreateTag` +
`TestResolveOrCreateTagEndpoint`, mirroring `TestAttachTagToVideo`'s deny-list/length-cap coverage
minus the video-link concern). Full Go suite green throughout, including through a large concurrent
ADR-077→078 / migration 0033→0034 renumbering another session ran mid-turn on this same branch —
verified `go build ./...` + the category test suite both stayed green against the renumbered tree
before finishing this pass.

Two real findings surfaced while writing these tests, not silently fixed (out of scope for a
testing-strategy pass, both now tracked in Up next): (1) a tag created via `ResolveOrCreateTag` with
no video attach is invisible to `ListTags`/`GET /tags` — and therefore `/tags`, the merge picker, and
search — because that query inner-joins `video_tags`; it still exists (`TagIDByName` resolves it,
`GET /tags/{id}` 200s) and shows in its category's own member-tag chips. (2) confirmed, by diffing
against the pre-`PickerShell`-extraction code, that `EntityPicker`/`CategoryPicker`'s focus-restore-
to-trigger-on-close was **already broken** before this epic touched those files — focus lands on
`<body>` instead, byte-identical behavior before/after the extraction — so it's a real a11y bug, but
not one this epic's frontend work introduced.

S8 · /security-review — focused review of the new owner-gated `POST /tags` resolve-or-create endpoint
and the category CRUD/assign-unassign mutation surface (`internal/api/categories.go`,
`internal/repo/categories.go`, `internal/repo/identity.go`, migration `0034_categories.up.sql`), plus
the new frontend surfaces for `@html`/XSS sinks. Checked: SQL injection (all new queries parameterized;
the one string-built table name in `nameCollidesInTable` is fed only hardcoded literals at all 5 call
sites, never request data), route gating (`mountCategoryMutations` — CRUD, assign/unassign, and the new
`POST /tags` — all sit inside the existing `requireOwner` router group; no new endpoint escapes it),
IDOR (category/tag IDs are existence-checked pre-mutation; single-owner model has no cross-tenant
boundary to violate; cascade-delete only touches `category_tags` join rows, never member `tags`), and
XSS (zero `@html` usage across `CategoryPicker`/`EntityPicker`/`PickerShell`/`/categories/[id]`/
`/tags`). Clean — no findings ≥0.7 confidence. Security gate closed; all six gates now green.

### 2026-07-31 · session
- skills: simplify, testing-strategy, security-review
