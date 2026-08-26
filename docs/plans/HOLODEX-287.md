---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-287                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix inconsistent entity-linking UI across pages by giving Tags a proper picker component, matching the pattern already used for People and Studios.
---

# HOLODEX-287 · Frontend component-reuse discipline (ADR-086)

Formalize a narrow, frontend-only fix for two concrete gaps found while investigating
UX drift between entity-linking UI on Video and Film detail pages: the auto-loading
`.claude/rules/frontend-theming.md` rule never pointed at the existing
`web/src/lib/components/CLAUDE.md` classification/inventory system, and Tags never got
a sibling picker component (still a hand-rolled inline form on the Video detail page).
Done means: ADR-086 accepted, the rule-file cross-link added, and `TagPicker.svelte`
built and wired in with parity to `PersonPicker`/`StudioPicker`.

**Design package:** no spec (discipline/tooling change, not product behavior) · [ADR-086](../architecture/ADR-086-frontend-component-reuse-discipline.md) · design-handoff pending (TagPicker interaction spec) · testing-strategy pending

## Gates — definition of done

- [ ] spec `write-spec` → `docs/specs/**` — N/A, no spec needed (see ADR-086 header)
- [x] architecture `architecture` → `docs/architecture/ADR-086-frontend-component-reuse-discipline.md`
- [ ] design `design-handoff` → `docs/design/**` — TagPicker interaction spec
- [ ] backend — N/A, frontend-only
- [ ] frontend — TagPicker.svelte build + wiring
- [ ] testing `testing-strategy`
- [ ] security `security-review` — likely N/A (no new mutation surface; confirm at implementation time)

## Up next — ordered (position = priority)

1. [ ] [frontend] Add "reuse before you create" cross-link bullet to `.claude/rules/frontend-theming.md` pointing at `components/CLAUDE.md`
2. [ ] [design] `/design-handoff` for TagPicker's concrete interaction spec
3. [ ] [frontend] Build `TagPicker.svelte` in `web/src/lib/components/entity/`, shaped after `PersonPicker` (multi-valued, no per-row role); preserve existing near-miss-suggestion logic; add its row to `entity/CLAUDE.md`  ⛔ blocked on #2
4. [ ] [frontend] Wire `TagPicker` into `web/src/routes/media/[id]/+page.svelte`, replacing the hand-rolled inline form  ⛔ blocked on #3
5. [ ] [—] `testing-strategy` pass for TagPicker parity + near-miss suggestion coverage

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-25 · ADR-086 written and committed; Draft PR #257 opened
- skills: product-brainstorming, architecture
- handoff: ADR-086 is Proposed and merged into this branch; corrected mid-brainstorm from an initial (wrong) premise that StudioPicker/PersonPicker needed consolidating — they're documented-intentional siblings and are NOT being touched. Next session should start with Action Item 1 (the rule-file bullet) or Action Item 2 (`/design-handoff` for TagPicker), not re-open the consolidation question.
