---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-287                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix inconsistent entity-linking UI across pages by giving Tags a proper picker component, matching the pattern already used for People and Studios.
---

# HOLODEX-287 · Frontend component-reuse discipline (ADR-088)

Formalize a narrow, frontend-only fix for two concrete gaps found while investigating
UX drift between entity-linking UI on Video and Film detail pages: the auto-loading
`.claude/rules/frontend-theming.md` rule never pointed at the existing
`web/src/lib/components/CLAUDE.md` classification/inventory system, and Tags never got
a sibling picker component (still a hand-rolled inline form on the Video detail page).
Done means: ADR-088 accepted, the rule-file cross-link added, and `TagPicker.svelte`
built and wired in with parity to `PersonPicker`/`StudioPicker`.

**Design package:** no spec (discipline/tooling change, not product behavior) ·
[ADR-088](../architecture/ADR-088-frontend-component-reuse-discipline.md) ·
[design-handoff](../design/tag-picker-handoff.md) done · testing-strategy pending

## Gates — definition of done

- [ ] spec `write-spec` → `docs/specs/**` — N/A, no spec needed (see ADR-088 header)
- [x] architecture `architecture` → `docs/architecture/ADR-088-frontend-component-reuse-discipline.md`
- [x] design `design-handoff` → [`docs/design/tag-picker-handoff.md`](../design/tag-picker-handoff.md) + [mockup](../design/tag-picker-handoff-mockup.svg)
- [ ] backend — N/A, frontend-only
- [ ] frontend — TagPicker.svelte build + wiring; cross-link bullet still open too
- [ ] testing `testing-strategy`
- [ ] security `security-review` — likely N/A (no new mutation surface; confirm at implementation time)

## Up next — ordered (position = priority)

1. [ ] [frontend] Add "reuse before you create" cross-link bullet to `.claude/rules/frontend-theming.md` pointing at `components/CLAUDE.md`
2. [ ] [frontend] Build `TagPicker.svelte` in `web/src/lib/components/entity/` per the handoff spec: `PersonPicker`-shaped (multi-valued, no per-row role, stays open across commits), absorb the page's near-miss advisory (`tagNearMiss`/`useTagNearMiss`), quiet-button trigger (not a tile or pencil); add its row to `entity/CLAUDE.md`
3. [ ] [frontend] Wire `TagPicker` into `web/src/routes/media/[id]/+page.svelte`, replacing the hand-rolled inline form (`:106-116`, `:599-687`, `:924-1011`); page keeps only thin `attach`/`detach` wrappers  ⛔ blocked on #2
4. [ ] [—] `testing-strategy` pass for TagPicker parity + near-miss suggestion coverage
5. [ ] [—] Resolve the two "Open questions for implementation" in the handoff (`AliasPanel` near-miss call ownership; confirm `addVideoTag` never conflicts) before or during #2

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-26 · Design-handoff for TagPicker written
- skills: design-handoff
- handoff: Wrote `docs/design/tag-picker-handoff.md` + committed SVG mockup. Resolved trigger-affordance choice (quiet-button, matching today's page — not PersonPicker's tile or StudioPicker's pencil) and specified the near-miss advisory migration in detail (sequencing, prop ownership). Two implementation-time open questions flagged in the doc, not blocking. Next session: item 1 (rule cross-link, trivial) then item 2 (build TagPicker.svelte).

### 2026-08-26 · Merged main; renumbered ADR-086 → ADR-088
- skills: (none — mechanical merge)
- handoff: Pulled main, which had since taken ADR-086 (film provider enrichment) and ADR-087 (film-studio cascade) for HOLODEX-279/285. Renamed our ADR to ADR-088 (file + README index + this worklog) to resolve the numbering collision — no content changes. Next available ADR number is 089.

### 2026-08-25 · ADR-088 written and committed; Draft PR #257 opened
- skills: product-brainstorming, architecture
- handoff: ADR-088 is Proposed and merged into this branch; corrected mid-brainstorm from an initial (wrong) premise that StudioPicker/PersonPicker needed consolidating — they're documented-intentional siblings and are NOT being touched. Next session should start with Action Item 1 (the rule-file bullet) or Action Item 2 (`/design-handoff` for TagPicker), not re-open the consolidation question.
