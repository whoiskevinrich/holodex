---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-467                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: feature             # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are trimmed to match.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The Enrich dialog now offers "Search again" for a provider you earlier marked as "None of these match", instead of a dead-end error.
approved:
  design:
    on: 2026-09-27
    at: 33718f23
---

# HOLODEX-467 · Enrich picker dead-ends on a dismissed provider

Done means a detail page's Enrich picker, reopened for a provider the owner dismissed ("None of
these match"), shows that as a notice with **Search {provider} again**, and a retry clears the
dismissal only when it finds candidates.

**Design package:** [spec amendment — P0-4](../specs/enrichment-review-workflow.md) ·
[handoff + three-skin mockup](../design/enrich-picker-undismiss-handoff.md) ·
[testing-strategy § Enrichment dismissals](../testing-strategy.md)

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default — the list below is only what you get with no config. Once `profile:` is
     set, trim them to that posture. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] design `design-handoff` → `docs/design/**`
- [x] backend → `{cmd,internal,providers}/**`
- [/] frontend → `web/src/**` — built + browser-QA'd; held on Kevin comparing the build to the approved mockup (§3d)
- [x] testing `testing-strategy`

## Up next — ordered (position = priority)

1. [ ] [frontend] Kevin: confirm the built dialog matches approved mockup A, then mark the PR ready — `web/src/lib/components/enrichment/EnrichPicker.svelte`
2. [ ] [—] On merge: CI moves HOLODEX-467 to Done (branch-keyed, no children to sweep)
3. [ ] [—] Optional follow-up: provider chip shows "not matched" before the picker opens — `web/src/lib/components/enrichment/EnrichProviderChips.svelte`

## Session log — newest first

### 2026-09-27 · session

- skills: design-critique, code-review high --fix, handoff
- Filed HOLODEX-467; option A approved from three-skin mockup; backend `retry` on four `/resolve` handlers, picker dismissed state, spec/handoff/testing docs; browser QA on media + person pages.
- handoff: Fix built, tested and QA'd on backend-films (commit 33718f23); only Kevin's build-vs-mockup look stands between the Draft PR and ready.

## Dropped — newest first (the reason is the point)

- [~] [design] Option B, inline "Undo and search" link in the status line — dropped 2026-09-27, a 12 px quiet link inside an aria-live line is easy to miss and announced as text
