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
- [/] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [backend] direct-children query + `GetTag.Children` — `internal/repo/tag_hierarchy.go:ChildrenForTag`, `internal/repo/repo.go`, tested (`TestChildrenForTag`)
2. [ ] [backend] category-memberships query + `GetTag.Categories` — `internal/repo/categories.go`, `internal/repo/repo.go`
3. [ ] [frontend] extract the `/tags` list's parent typeahead into shared logic, reuse for the parent control — `web/src/routes/tags/[id]/+page.svelte`
4. [ ] [frontend] children control incl. the confirm-flow handoff (add via resolve-or-create + reparent-confirm guard, × to unparent) — `web/src/routes/tags/[id]/+page.svelte`
5. [ ] [frontend] categories control (reuse `CategoryPicker` single-tag mode, chip + ×) — `web/src/routes/tags/[id]/+page.svelte`  ⛔ blocked on #2
6. [ ] [testing] cover the reparent-confirm branch + frontend controls — `docs/testing-strategy.md`
7. [ ] [security] confirm owner-gating on the two new mutation-adjacent read paths — light, since both call existing owner-gated endpoints

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-07 · Spec, design handoff, and first backend slice
- skills: product-brainstorming, write-spec, design-handoff, simplify
- handoff: spec and design are done. Backend: `ChildrenForTag` (direct children, name-ordered)
  landed and wired into `GetTag.Children`, tested (`TestChildrenForTag`), full repo+api suites
  green. Next: `GetTag.Categories` (category-memberships query), then the frontend controls.
