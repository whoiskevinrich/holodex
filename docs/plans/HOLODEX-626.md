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
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-626 · Filter and sort on provider links and field values (F90)

Done means the owner can ask which videos, people and studios are linked to a provider, which have or
lack a field's value, and how much providers contributed, with the Completeness score and ring gone.

**Design package:** [spec F90](../specs/provider-and-field-filters.md) · architecture: [field-resolution.md](../architecture/field-resolution.md#field-presence-and-provider-contribution-are-cached-in-entity_field_state-for-all-three-entity-types) · handoff: [provider-and-field-filters-handoff.md](../design/provider-and-field-filters-handoff.md) · testing-strategy: pending

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
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

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

1. [ ] [design] Run /implement: owner signs off the handoff and SVGs, then the Draft PR opens — `docs/design/provider-and-field-filters-handoff.md`
2. [ ] [testing] Strategy section, starting with a failing test for `matched:any` and filename-only videos — `docs/testing-strategy.md`
3. [ ] [security] Review the owner-only rules at design stage — `docs/architecture/security-perimeter.md`
4. [ ] [backend] At build, confirm every writer of a video's file poster slot moves `thumbnail_state` or `video_poster_sources`, so the poster trigger fires — `internal/api/video_poster.go`
5. [ ] [spec] At build, rewrite the F55 spec to the retired state, removing the score, ring and sort rules — `docs/specs/entity-completeness-score.md`
6. [ ] [frontend] HOLODEX-628 (ring in the visitor preview) is moot once P0-6 removes the ring; close it with F90's merge if not fixed first
7. [ ] [architecture] On this epic's squash, replace "pending squash (HOLODEX-626)" with the commit — `docs/architecture/field-resolution.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

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
