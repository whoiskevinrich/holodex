---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-32                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Media, People and Tags can switch to a sortable Table view, and Tags gains a Posters view with an image you can upload on each tag's page.
approved:
  design:
    on: 2026-10-24
    at: 6ca51996
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-32 · List views — a shared Table view, and Posters for Tags (F82)

Done means Media, People and Tags each offer the shared Table view from the list toolbar's View
slot, header clicks sort through the one toolbar sort, and Tags has a Posters view backed by an
owner-uploaded tag image with a placeholder. Epic 4 of the frontend backlog plan; covers
HOLODEX-32, HOLODEX-42 and HOLODEX-46. HOLODEX-57 (grid multi-select) was split out on 2026-10-24.

**Design package:** [spec F82](../specs/list-views.md) · no architecture doc (no technology fork) ·
[handoff](../design/list-views-handoff.md) · [testing-strategy §33](../testing-strategy.md#33-list-views--the-shared-table-view-and-tags-posters-f82-holodex-32--42--46)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**` — `docs/specs/list-views.md` (F82)
- [~] architecture `architecture` → `docs/architecture/**` — no technology fork: tag images reuse the studio image store and upload gate
- [x] design `design-handoff` → `docs/design/**` — `docs/design/list-views-handoff.md` + 3 SVG mockups
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

1. [ ] [backend] Tag image store, owner upload/delete, serve route, list URL, merge/delete lifecycle — `internal/`
2. [ ] [frontend] Tags Posters view + tag page poster control; extract the per-tag ⋯ menu so cards AND Table rows get it, then drop Manage mode's switch to Chips (spec P0-8) — `web/src/routes/tags/`
3. [ ] [frontend] Media Table thumbnail retries while generating, sharing VideoCard's backoff rather than copying it — `web/src/routes/+page.svelte`
4. [ ] [testing] §33 Go, geometry and nav harness cases; live QA checklist — `docs/testing-strategy.md`
5. [ ] [security] `/security-review` on the tag image upload — `internal/api/`
6. [ ] [—] Sweep HOLODEX-42 and HOLODEX-46 with this epic: In Review when the PR is ready, Done on merge

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-24 · Epic 4 started; spec, handoff and test plan landed
- skills: write-spec, design-handoff, testing-strategy, handoff, implement, code-review
- handoff: Table view live on Media, People and Tags (V1–V7 tested; /tags list now carries parent_name); the handoff's column widths moved to rem after a measured overflow, so the design sign-off needs a re-confirm at /handoff; start at the tag image backend (Up next 1).

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
