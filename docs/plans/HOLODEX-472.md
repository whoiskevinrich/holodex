---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-472                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Media, People, Studios, Films and Tags now share one sort, filter and view toolbar that fits on a phone, and any filtered list can be shared or bookmarked by its URL.  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
approved:
  design:
    on: 2026-09-27
    at: 5ffff2f  # re-confirmed 2026-09-27: D1-D3 amendments approved as built (was c99b221)
---

# HOLODEX-472 · F73 One list toolbar: shared sort, filter and view across list pages

Every list page (Media, People, Studios, Films, Tags) draws sort, filters and view from one
`ListToolbar`. Sort is a single choice, which fixes the People/Studios double-highlight bug
HOLODEX-473. On a phone, data starts after the title, one toolbar row and at most one chip row.
List state follows ADR-114: the URL holds what you see, and storage holds only preferences.

**Design package:** [spec F73](../specs/list-toolbar.md) · [ADR-114](../architecture/ADR-114-list-state-model.md) · [handoff + 3 SVGs](../design/list-toolbar-handoff.md) · testing-strategy § (not yet written)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default — the list below is only what you get with no config. Once `profile:` is
     set, trim them to that posture. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/ADR-*`
- [x] design `design-handoff` → `docs/design/**`
- [~] backend → `{cmd,internal,providers}/**` — every param already exists on the list APIs; frontend-only — until: `q`/`type` need server support on an entity list
- [ ] frontend → `web/src/**`
- [/] testing `testing-strategy`
- [~] security `security-review` — no auth, access or infra change; list state is client-side and public params only — until: a new endpoint or param is added

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

1. [ ] [—] ADR-115 (Cinémathèque only, HOLODEX-476): once on main, drop three-skin QA from handoff/§20 and re-measure the label budget in Archivo — `docs/design/list-toolbar-handoff.md`
2. [ ] [testing] Finish live QA: Films on backend-films, People/Studios with rows, §20.3 items 3 and 5 (items 1, 2, 4, 6, 7 passed), then the §20.4 mutation checks; record results in §20 — `docs/testing-strategy.md`
3. [ ] [frontend] Coin "list toolbar" and "scope chip" in the UI vocabulary — `docs/reference/ui-vocabulary.md`
4. [ ] [—] Hand-sweep child HOLODEX-473 with the epic: In Review on ready, Done on merge → HOLODEX-473
5. [ ] [—] Navigation behaviour harness (same-route nav, Back, removal exits) → HOLODEX-475
6. [ ] [—] Purge legacy `holodex:filters:*` keys → HOLODEX-474  ⛔ blocked on production confirming F73
7. [ ] [—] Fast follow: `listScroll` survives a full reload (sessionStorage on the D5 key; not in F73 scope) → HOLODEX-477  ⛔ blocked on this epic merging

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-27 · brainstorm → spec F73, ADR-114, design handoff
- skills: product-brainstorming, write-spec, architecture, design-handoff, handoff, implement, testing-strategy, code-review
- decisions: bug fixed inside the redesign; Missing filter dropped (it's a work queue); Media entity filters only as scope chips; Tags type = tabs; filters URL-only, sort sticky; desktop Filters = popover, density inline. SP5 sticky filters superseded; R8 reuses HOLODEX-41's `history.back()`.
- handoff: Geometry harness extended (phone width, list pages, 6 F73 assertions) and cut to Cinémathèque × 4 widths; all F73 assertions pass, no new phone failures (§20.2). Filters sheet contains scroll (owner picked B: `touch-none` backdrop + `overscroll-contain`, no document lock). Next: finish live QA (item 2); item 1 waits on ADR-115 reaching main.

## Divergences from the approved design (signed off at c99b221, amended 5ffff2f)

The approved handoff stays frozen at `c99b221`. Built changes the owner didn't choose are logged here,
never written into the handoff (flightplan#38). Each is **open** until the owner picks from a visual
comparison. Only then is the chosen option built or kept, and the handoff re-issued for sign-off.

| # | Surface | Approved | Built (not chosen) | Status |
|---|---|---|---|---|
| D1 | People/Studios sweep status | Decision 2: folded into the count line ("· 3 need a refresh") | `SweepStatusLine` kept as its own row below the toolbar, shown only while a sweep runs or just after. The real component is a progress report, not a count | **chosen 2026-09-27: C**, one ellipsized line under the count ("Refreshing 120 of 412 · 3 need review · Details"); **built and approved 2026-09-27** (owner approved as built; declined a render) |
| D2 | People merge select mode | Not drawn (only "Merge people…" in `⋯` was approved) | Count line replaced by the hint + "Merge N selected" + Cancel | **chosen 2026-09-27: B**, the toolbar row becomes a mode bar ("2 selected · Merge · Cancel"), hint as the count line; **built and approved 2026-09-27** (owner approved as built; declined a render) |
| D3 | Tags manage mode | Not drawn (only "Manage tags" in `⋯` was approved) | A **Done** button added to the existing manage bar | **chosen 2026-09-27: B**, the same mode bar ("Managing · N selected · Merge… · ⋯ · Done"), category and writeback actions in that ⋯; **built and approved 2026-09-27** (owner approved as built; declined a render) |

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
