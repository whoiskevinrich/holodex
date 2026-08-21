---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-115                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Comments (mapped to Overview) are now manually editable on the video detail page and write back to the file, same as Title/Studio/Performers/Genres.
---

# HOLODEX-115 · Core file-metadata fields manually editable + writable

Owner can manually edit Title, Studio, Performers, Comments (`overview`), and Genres from the video
detail page, and each edit writes back to the file. Comments turned out to be `overview`'s existing
`long_text` replace field stuck read-only in the UI (fixed by extending the generic Metadata `dl`'s
`long_text` branch to render `SourceBadge`); Genres already worked via the existing Tags "+" affordance
(verified, no code change); Title/Studio/Performers were already editable pre-epic. The bespoke
`commentary` field (F52) — which this epic's investigation determined was never the right mechanism —
was retired in the same change.

**Design package:** [video-owner-mode-editing.md](../specs/video-owner-mode-editing.md) (superseded note added) · ADR-051/ADR-013 (existing, ridden unchanged) · [video-owner-mode-editing-handoff.md](../design/video-owner-mode-editing-handoff.md) (superseded note added) · [testing-strategy.md](../testing-strategy.md) F52 section (rewritten)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [~] spec `write-spec` — until: never; this change rides the existing video-owner-mode-editing.md spec (superseded note added in place) and the existing F56/SourceBadge mechanism — no new spec needed
- [~] architecture `architecture` — until: never; no new ADR — rides ADR-051 (per-field decisions) and ADR-013 (mapping) unchanged
- [~] design `design-handoff` — until: never; SourceBadge (F56) is the existing design, just extended to one more field per its own generic contract — no new design surface
- [x] backend — registry.go `commentary` removal; decisions_test.go `overview` coverage; registry_test.go/complete_test.go exemplar swap
- [x] frontend — `+page.svelte` long_text branch renders `SourceBadge`; Commentary section/derived/filter removed
- [x] testing `testing-strategy` — testing-strategy.md F52 section rewritten + retirement note; specs/architecture docs touched
- [x] security `security-review` — verified `replaceField` looks up `commentary` via the mapping config (not a hardcoded list), so it 404s generically once removed; `enrich.SanitizeValue` applies unconditionally before the field lookup, uniformly for `overview` same as `title`/`studio`. No new endpoint, no new file-tag mapping. Clean, no findings.

## Up next — ordered (position = priority)

1. [ ] [—] merge PR #247 once approved (the `a9327df` review-feedback fix that never actually shipped with #245)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-21 · session
- skills: (none — direct fix)
- handoff: Discovered via the flightplan Stop hook that `a9327df` (the PR #245 review-feedback fix: `.trim()` guard on the non-owner/non-replace `overview` fallback + missing `leading-relaxed` class on the `SourceBadge` wrapper) was committed locally but never pushed — #245 merged without it, so the fix never actually shipped. Pushed the branch and opened PR #247 to land it, plus this worklog catch-up commit. No gate changes — same-epic polish fix on already-`[x]` frontend work. Next session should merge PR #247 once approved.

### 2026-08-16 · session
- skills: testing-strategy, simplify, security-review
- handoff: Committed, pushed, and opened PR #245 ready for review (not draft) — all applicable gates green. Manually driven-browser QA'd `overview`'s new `SourceBadge` control against `backend-films` fixture video 211 ("DUNE"): renders, popover opens with manual-edit option, no stray "Commentary" text, confirmed across all 3 skins, zero attributable console/network errors. Next session should address any PR review feedback and merge once approved.
