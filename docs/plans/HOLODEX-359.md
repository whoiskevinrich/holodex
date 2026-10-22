---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-359                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: ui                  # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The categories list no longer scrolls sideways on a phone when a category has a very long name.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-359 · Film and category text rungs for the geometry harness

Give the film and category overflow fixes (PRs #314–#316) a fixture that can reproduce them, and
guard the ExpandableText chevron gate (HOLODEX-361) with a short-text case as well as a long one.
The phone-width cell this ticket first asked for already landed with HOLODEX-472.

**Design package:** no spec, ADR or handoff (owner ruling: harness + fixture work; the one product
change is a wrap fix) · [testing-strategy §12.4](../testing-strategy.md)

## Gates — definition of done

<!-- Rows match the `ui` posture. Spec and architecture are not rows of it; the owner ruled all three
     design-phase gates out on 2026-10-21. -->

- [~] design `design-handoff` → `docs/design/**` — owner ruling 2026-10-21: no new UI; the category pill only gains `max-w-full wrap-anywhere`, the rule SourceBadge and ExpandableText already follow
- [x] frontend → `web/src/**` — `max-w-full wrap-anywhere` on both category pills (plain and Manage) in `routes/tags/+page.svelte`
- [x] testing `testing-strategy` — `filmtext`/`categorytext` rungs, two chevron-gate assertions (mutation-tested), overflow coverage widened; §12.4 rows

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The CHECKBOX settles an item and nothing else does: [x] done, [~] dropped. -->

1. [ ] [—] Mark the draft PR ready once the owner has looked; sweep HOLODEX-359 in Jira (Atlassian connector unauthorized this session)
2. [ ] [—] The plain tag pill on /tags has the same markup; no rung reaches it yet — `web/src/routes/tags/+page.svelte`

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-10-21 · session
- skills: code-review, implement, handoff
- built: `filmtext` (films 1100…) and `categorytext` (categories 1200…) stress-seed dimensions, in a
  parallel child session; the harness gained `prose-chevron-only-when-clamped` (60 checks,
  mutation-tested) and `prose-chevron-when-clamped` (8), and `no-horizontal-page-overflow` now covers
  both new rungs (188 checks, all pass). On the full matrix the new category names make
  `list-page-no-horizontal-overflow` fail on `/tags?type=categories` at phone by 155px: the pill does
  not wrap an unbroken name.
- not ours, not verified against main: the `enrich-picker-open:flood` assertions ERR (no "Enrich from
  flood" button), `az-rail-clear-of-rows` and `list-first-row-near-top-phone` are VOID, and
  `tag-chips-stay-tappable` reports NEWS under its HOLODEX-357 marker.
- then: crossed into build (design `[~]` by owner ruling, `main` already in), fixed the category pills,
  `list-page-no-horizontal-overflow` 32/32 and `list-toolbar-single-row` 32/32.
- handoff: Crossed into build with design skipped by owner ruling; every gate is settled and the draft PR is open. Next: the owner looks, then mark it ready.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
