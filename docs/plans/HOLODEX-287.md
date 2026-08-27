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
- [x] frontend — `TagPicker.svelte` built + wired in; cross-link bullet added
- [ ] testing `testing-strategy`
- [ ] security `security-review` — likely N/A (no new mutation surface; confirm at implementation time)

## Up next — ordered (position = priority)

1. [ ] [—] `testing-strategy` pass for TagPicker parity + near-miss suggestion coverage
2. [ ] [—] Mark PR #257 ready for review once the testing gate closes (fires `In Review`)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-26 · TagPicker.svelte built and wired in
- skills: (none — direct implementation per handoff spec + PersonPicker precedent)
- handoff: Built `TagPicker.svelte` (`web/src/lib/components/entity/`) per the handoff spec —
  `PersonPicker`-shaped (multi-valued, no role machinery), quiet-button trigger, absorbs the
  near-miss advisory via a direct `api.nearMiss('tag', ...)` call (resolved open question 1:
  matches `AliasPanel.flagNearMiss`'s direct-call pattern, not a prop). Added a `busyKey`
  bindable prop beyond the literal spec text, mirroring `PersonPicker`'s shared-busy-gate
  precedent, since the page's own persistent tag chip row and the popover's echo list both
  detach the same video's tags. Wired into `media/[id]/+page.svelte`: replaced the seven old
  tag-form functions + state with three thin wrappers (`attachTag`/`detachTag`/`removeTag`)
  mirroring `attachPerson`/`detachPerson`/`removeGridPerson`; added the `entity/CLAUDE.md` row.
  Verified end-to-end in the browser against `backend-amv` (open/close, debounced search,
  create-fallback, exact-name dedup, attach, detach, shared busy-key between the page's chip
  row and the popover, Escape-close) and confirmed token-driven contrast across all three
  skins. `npm run check` clean (0 errors). Next session: `/testing-strategy` pass, then mark
  PR #257 ready for review.

### 2026-08-26 · Design-handoff for TagPicker written
- skills: design-handoff
- handoff: Wrote `docs/design/tag-picker-handoff.md` + committed SVG mockup. Resolved trigger-affordance choice (quiet-button, matching today's page — not PersonPicker's tile or StudioPicker's pencil) and specified the near-miss advisory migration in detail (sequencing, prop ownership). Two implementation-time open questions flagged in the doc, not blocking. Next session: item 1 (rule cross-link, trivial) then item 2 (build TagPicker.svelte).

### 2026-08-26 · Merged main; renumbered ADR-086 → ADR-088
- skills: (none — mechanical merge)
- handoff: Pulled main, which had since taken ADR-086 (film provider enrichment) and ADR-087 (film-studio cascade) for HOLODEX-279/285. Renamed our ADR to ADR-088 (file + README index + this worklog) to resolve the numbering collision — no content changes. Next available ADR number is 089.

### 2026-08-25 · ADR-088 written and committed; Draft PR #257 opened
- skills: product-brainstorming, architecture
- handoff: ADR-088 is Proposed and merged into this branch; corrected mid-brainstorm from an initial (wrong) premise that StudioPicker/PersonPicker needed consolidating — they're documented-intentional siblings and are NOT being touched. Next session should start with Action Item 1 (the rule-file bullet) or Action Item 2 (`/design-handoff` for TagPicker), not re-open the consolidation question.
