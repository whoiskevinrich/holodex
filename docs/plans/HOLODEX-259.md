---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-259                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Tag pages now show and let you edit a tag's parent, children, and category memberships in place.
---

# HOLODEX-259 · Tag detail hierarchy & categories

Show a tag's parent, direct children, and category memberships on `tags/{id}`, with owner-gated
controls to edit all three in place — done when the owner never needs the `/tags` list's bulk
Manage mode just to fix a tag's place in the tree.

**Design package:** [tag-detail-hierarchy-and-categories.md](../specs/tag-detail-hierarchy-and-categories.md) · extends ADR-075 / ADR-078 (no new ADR) · design TBD · testing-strategy TBD

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/tag-detail-hierarchy-and-categories.md`
- [~] architecture `architecture` → not required (spec explicitly extends ADR-075/ADR-078, no new decision)
- [x] design `design-handoff` → `docs/design/tag-detail-hierarchy-reparent-confirm-handoff.md`
- [x] backend
- [x] frontend
- [x] testing `testing-strategy`
- [x] security `security-review`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [backend] direct-children query + `GetTag.Children` — `internal/repo/tag_hierarchy.go:ChildrenForTag`, `internal/repo/repo.go`, tested (`TestChildrenForTag`)
2. [x] [backend] category-memberships query + `GetTag.Categories` — `internal/repo/categories.go:CategoriesForTag`, `internal/repo/repo.go`, tested (`TestCategoriesForTag`)
3. [x] [frontend] extract the `/tags` list's parent typeahead into shared logic, reuse for the parent control — `web/src/lib/tagHierarchy.ts`, `web/src/routes/tags/[id]/+page.svelte`
4. [x] [frontend] children control incl. the confirm-flow handoff (add via resolve-or-create + reparent-confirm guard, × to unparent) — `web/src/routes/tags/[id]/+page.svelte`
5. [x] [frontend] categories control (reuse `CategoryPicker` single-tag mode, chip + ×) — `web/src/routes/tags/[id]/+page.svelte`
6. [x] [testing] cover the reparent-confirm branch + frontend controls — `docs/testing-strategy.md`
7. [x] [security] confirm owner-gating on the two new mutation-adjacent read paths — light, since both call existing owner-gated endpoints

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-07 · Frontend controls complete — spec, design, backend, and frontend all done
- skills: product-brainstorming, write-spec, design-handoff, simplify, security-review
- handoff: backend gate landed first this session (`ChildrenForTag`/`CategoriesForTag`, wired
  into `GetTag`, tested). Then the frontend gate: `web/src/lib/tagHierarchy.ts` extracted
  (`findTagByName`/`cycleMessage`), `/tags`' Manage-mode parent control refactored onto it,
  and `tags/[id]/+page.svelte` gained a new "Hierarchy & categories" card with all three owner-
  gated controls — Parent (chip + typeahead, × clears immediately), Children (chip list, "+ Add
  child" resolve-or-create with the reparent-confirm `ConfirmDialog` flow from the design
  handoff for candidates with their own parent/children), and Categories (chip list + reused
  `CategoryPicker` single-tag mode). Read states render for every visitor; only add/× is owner-
  gated. `types.ts`'s `Tag` gained `children`/`categories`. Manually QA'd end-to-end in the
  browser (set/clear parent, immediate-attach child, reparent-confirm dialog incl. cancel-
  preserves-input-text and refocus, remove child, add/remove category) across all three skins
  (Cinémathèque/Broadcast/Brutalist) — no theming issues. `/simplify` run and applied
  (parameterized `childCycleMessage` to drop a closure capture); `npm run check` 0 errors,
  `npm run test` 134/134 green.

### 2026-08-07 · Testing gate closed
- skills: testing-strategy
- handoff: added a `docs/testing-strategy.md` entry for HOLODEX-259 in each of the three places
  the file's own convention expects it — a §4 backend row for `ChildrenForTag`/`CategoriesForTag`
  (both already Go-test-covered by `TestChildrenForTag`/`TestCategoriesForTag`), a §5 frontend row
  documenting the Hierarchy & categories card's decision tree (immediate-attach vs. the three
  reparent-confirm copy variants) and the manual 3-skin driven-browser QA already performed, and a
  §11 Known Gaps bullet flagging the one thing still open — no automated Vitest/Playwright
  coverage for the new card, the same standing gap every other recent frontend feature in that
  file carries at this stage. Updated the file's date header. No test code was added this session
  (the backend tests already existed; the frontend automation gap is logged, not silently closed).
  Next: the security gate — a light `/security-review` pass on the two new mutation-adjacent read
  paths, both of which ride existing owner-gated endpoints (`SetTagParent`, `ResolveOrCreateTag`,
  `AssignTagsToCategory`/`UnassignTagsFromCategory`) rather than introducing a new mutation
  surface.

### 2026-08-07 · Security gate closed — clean bill of health
- skills: security-review
- handoff: ran `/security-review` over the full diff (`internal/model/model.go`,
  `internal/repo/categories.go`, `internal/repo/repo.go`, `internal/repo/tag_hierarchy.go`,
  `web/src/lib/tagHierarchy.ts`, `web/src/lib/types.ts`, `web/src/routes/tags/+page.svelte`,
  `web/src/routes/tags/[id]/+page.svelte`). **No findings.** Verified independently (not just
  trusting the review agent): `ChildrenForTag`/`CategoriesForTag` are both parameterized queries
  (`WHERE parent_tag_id = ?` / `WHERE ct.tag_id = ?`, matching the existing repo-wide pattern —
  no string concatenation); `GET /tags/{id}` is registered at `internal/api/handlers.go:319`,
  before the `r.Use(h.requireOwner)` line at 335, confirming it was already public pre-diff, and
  the new `Children`/`Categories` fields disclose nothing not already reconstructable from the
  already-public `GET /tags` list and `GET /categories/{id}`; every mutation the new UI calls
  (`SetTagParent`, `ResolveOrCreateTag`, `AssignTagsToCategory`/`UnassignTagsFromCategory`) is a
  pre-existing endpoint already inside the `requireOwner` group, untouched by this diff — this PR
  adds zero new mutation endpoints. No `{@html}` or other unsafe sink in the new Svelte markup.
  **All gates now closed for HOLODEX-259** (spec, design, backend, frontend, testing, security).
  Next: mark PR #221 ready for review (drop Draft) — this fires the Jira In Review transition
  per ADR-069.
