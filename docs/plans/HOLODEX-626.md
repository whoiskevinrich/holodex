---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-626                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: [HOLODEX-619]               # [KEY-…] cross-epic deps that must land first
release_note: Owners can now filter and sort videos, people and studios by provider link, by whether a field has a value, and by how much providers contributed, which replaces the Completeness score and ring.
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
approved:
  design:
    on: 2026-11-09
    at: f6e95759
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-626 · Filter and sort on provider links and field values (F90)

Done means the owner can ask which videos, people and studios are linked to a provider, which have or
lack a field's value, and how much providers contributed, with the Completeness score and ring gone.

**Design package:** [spec F90](../specs/provider-and-field-filters.md) · architecture: [field-resolution.md](../architecture/field-resolution.md#field-presence-and-provider-contribution-are-cached-in-entity_field_state-for-all-three-entity-types) · handoff: [provider-and-field-filters-handoff.md](../design/provider-and-field-filters-handoff.md) · testing-strategy: [§42](../testing-strategy.md#42-provider-link-and-field-filters-the-enriched-count-the-score-retired-f90-holodex-626)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**` — F90 written; F55, F73 and F87 point to it
- [x] architecture `architecture` → `docs/architecture/**` — `entity_field_state` cache and declared-provider `matched` in field-resolution.md
- [x] design `design-handoff` → `docs/design/**` — provider-and-field-filters-handoff.md + two SVGs; owner picked option A and a Studios Table view, sign-off pending at /implement
- [ ] backend → `{cmd,internal,providers}/**` — `matched` counts only declared providers (A1 green); `entity_field_state` and `has:` not started
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy` — §42 strategy written; A1 passes with the fix; the rest is written with the build
- [/] security `security-review` — design-stage rules SEC-12 to SEC-15 in security-perimeter.md; the code review waits for the build

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced in the SessionStart banner, clipped if long (the trailing path
     survives; the middle is dropped) — so keep it to one line.
     The CHECKBOX settles an item and nothing else does: [x] done, [~] dropped, same states as a gate
     row. A strikethrough is emphasis, not a state — `~~half~~ now do the rest` is a live item
     (ADR-008). This queue holds LIVE work only: delete a done item, move a dropped one to
     ## Dropped at the end of this file (ADR-010). The banner counts any settled item left here. -->

1. [ ] [backend] `entity_field_state` migration 0073, its drain writer and triggers, then `has:` in the parser and compiler — `internal/db/migrations/`
2. [ ] [testing] Retire §4's score, ring and Done-column rows with the code they cover — `docs/testing-strategy.md`
3. [ ] [security] Re-run /security-review on the built code against SEC-12 to SEC-15, then settle the gate — `docs/architecture/security-perimeter.md`
4. [ ] [backend] At build, confirm every writer of a video's file poster slot moves `thumbnail_state` or `video_poster_sources`, so the poster trigger fires — `internal/api/video_poster.go`
5. [ ] [spec] At build, rewrite the F55 spec to the retired state, removing the score, ring and sort rules — `docs/specs/entity-completeness-score.md`
6. [ ] [frontend] HOLODEX-628 (ring in the visitor preview) is moot once P0-6 removes the ring; close it with F90's merge if not fixed first
7. [ ] [architecture] On this epic's squash, replace "pending squash (HOLODEX-626)" with the commit — `docs/architecture/field-resolution.md`, `security-perimeter.md`
8. [ ] [backend] Migrations start at 0073 (main's 0072 is video trim); re-check against origin/main before adding one — `internal/db/migrations/`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-11-10 · design-stage security review
- skills: security-review, handoff, code-review
- handoff: Perimeter rules SEC-12 to SEC-15 are written, and `matched:` now counts only providers declared in metadata-sources.yaml (A1 green, mutation-checked); the owner chose to refuse a saved `matched:<undeclared>` (recorded under spec P0-9); next is the `entity_field_state` cache.

### 2026-11-09 · testing strategy
- skills: handoff
- handoff: Testing strategy §42 is written and the `matched:any` premise is confirmed by a failing test (A1, committed skipped); next is the security review, then the backend fix that removes A1's skip.

### 2026-11-08 · crossed into build
- skills: implement
- handoff: Crossed into build — design signed off at f6e95759 (F90 files unchanged since 3cf89354, main merged in); the Draft PR is open. Start at the testing strategy, with its failing `matched:any` test first.

### 2026-11-07 · F89 merged; architecture
- skills: architecture, design-handoff
- handoff: Architecture (one `entity_field_state` cache, declared-provider `matched`) and the design handoff (add-a-field picker, Studios Table view, ring removed) are committed; every design gate is settled, so next is /implement for the owner's sign-off and the Draft PR.

### 2026-11-06 · brainstorm and spec
- skills: write-spec, code-review, handoff
- handoff: Spec F90 is written and pushed from the owner's decisions, assuming F89 (HOLODEX-619, PR 580) lands first, and nothing is built; next, once F89 merges, is the architecture choice for people, studios and images, then the design handoff.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
