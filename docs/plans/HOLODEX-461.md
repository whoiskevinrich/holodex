---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-461                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile:                     # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: On a media item's page, the People grid, the add-person picker and the "More with …" shelf now show each person's Displayed As name instead of their canonical name, and so does a film's Cast grid.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-461 · Cast grids show the Displayed As name, not the canonical one

Bug: Media Details labelled each person by their canonical `name`. Outside the Person details page, only
the Displayed As spelling (the one a standing `name` decision selects, F60 RD9) should show. Done means
every person label on Media Details (and the shared Film Cast grid) renders `display_name ?? name`,
while the canonical `name` stays the linking identity that attach/detach sends back.

**Design package:** none. This bug fix restores F60 RD9's intent with no new behaviour to specify.
Regression test: `internal/repo/display_names_test.go` → `TestCastDisplayNames`.

## Decisions — do not re-litigate

- **Overlay at the repo read, render in the component.** `attachPersonDisplayNames` sits beside
  `attachPersonImageVersions` in `GetVideo` and `FilmCast`, and `Related` fills `RelatedShelf.DisplayName`.
  `name` is never overwritten: it is the identity that pickers, detach and writeback read.
- **The picker's "Create …" row also checks `display_name`.** Otherwise typing a displayed spelling
  offers to mint a duplicate person beside the matching candidate.

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default — the list below is only what you get with no config. Once `profile:` is
     set, trim them to that posture. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [~] spec `write-spec` → `docs/specs/**` — bug fix restoring F60 RD9's intended display; no new requirement
- [~] architecture `architecture` → `docs/architecture/ADR-*` — no architectural change; reuses `Repo.DisplayNames` (HOLODEX-378)
- [~] design `design-handoff` → `docs/design/**` — no new UI; existing labels swap to the display spelling
- [x] backend → `{cmd,internal,providers}/**` — `GetVideo`/`FilmCast`/`Related` carry `display_name` (d9bcd5d)
- [x] frontend → `web/src/**` — `PeopleGrid`, `PersonPicker`, and the media page's shelf title + remove error (d9bcd5d)
- [~] testing `testing-strategy` — regression test `TestCastDisplayNames` added; no strategy change
- [~] security `security-review` — no auth/access change; `display_name` is already public on search and the person page

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

1. [ ] [—] Kevin's three-skin look at Media Details' People grid, then `gh pr ready` (CI moves 461 to In Review)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-09-26 · session
- skills: code-review (high --fix), handoff
- merged origin/main (cd254e1; ecc5207 geometry tests, no overlap) before opening the Draft PR
- handoff: Fix committed (d9bcd5d) and verified on backend-films (the Cast tile and "More with" shelf show the decided spelling, and the canonical name is absent from the page); the Draft PR waits only on Kevin's skin look before `gh pr ready`.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->

- [~] [backend] Id-scoped `DisplayNames` for the Cast overlay — dropped 2026-09-26, the name-decision set is small and search already runs the unscoped query per keystroke
- [~] [backend] Sort the Cast grid by display spelling — dropped 2026-09-26, it changes ordering behaviour beyond the bug; canonical order is stable and matches the file
