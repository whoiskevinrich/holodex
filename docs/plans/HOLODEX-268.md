---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-268                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Owner field editing on Video/Person/Studio pages now matches the visitor view at rest, and confirming a suggested value always works.
---

# HOLODEX-268 · Two-tier field editing model

Video/Person/Studio detail pages replace the always-on `SourceSelect` radiogroup with a
two-tier model: Tier 1 fields (Title/People/Studio/Tags) edit in place, visually identical to
the visitor view at rest; Tier 2 fields (everything else) collapse to a `ProvenanceBadge`,
expand to a chip row on click, and require an explicit Confirm. Done means the redesign has
shipped for all in-scope replace fields *and* the confirmed pending-chip bug
(`SourceSelect.activate()`'s no-op guard on the RD6-pending chip) is fixed as a structural
side effect of the new Confirm step — not patched standalone.

**Design package:** [two-tier-field-editing.md](../specs/two-tier-field-editing.md) · ADR: none (extends ADR-051, no new architecture) · handoff: [two-tier-field-editing-handoff.md](../design/two-tier-field-editing-handoff.md) + [qa-checklist](../design/two-tier-field-editing-qa-checklist.md) · testing-strategy §: [§5 frontend row](../testing-strategy.md) + [§11 gap tracking](../testing-strategy.md)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/two-tier-field-editing.md`
- [~] architecture `architecture` → none needed — presentation-layer restructuring of ADR-051, no new persistence/API/access-control shape (see spec's Depends-on line)
- [x] design `design-handoff` → `docs/design/two-tier-field-editing-handoff.md` + `docs/design/two-tier-field-editing-qa-checklist.md`
- [~] backend — none needed, this story is a frontend-only presentation restructuring (no API/schema change)
- [/] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §5 frontend row + §11 gap-tracking bullet (frontend-only feature, no §4 backend row needed)
- [~] security `security-review` — until: a new mutation surface is introduced (none planned; see spec § Access control & security)

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [frontend] Build `SourceBadge.svelte` (badge/expand/chip-row/Confirm-Cancel), reusing `CurationChip` radio mode + `ProvenanceBadge` — `web/src/lib/components/curation/SourceBadge.svelte`
2. [x] [frontend] F56.4 RD6-confirm bug fix — verified live against `backend-films`: Confirm alone (no chip click) on an RD6-pending chip issued exactly one `PUT .../decision` and flipped `decision.standing` to `true`
3. [ ] [frontend] Roll `SourceBadge` out to the remaining Tier-2 replace fields on Video (`media/[id]/+page.svelte`) and Studio (`studios/[id]/+page.svelte`) detail pages, mirroring today's Person rollout

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-10 · Frontend implementation — `SourceBadge.svelte` + HOLODEX-245 fix
- skills: (none — direct implementation)
- handoff: Built `SourceBadge.svelte` (`web/src/lib/components/curation/SourceBadge.svelte`) as a structurally independent sibling to `SourceSelect`, per the design handoff — collapsed `ProvenanceBadge` at rest, click-to-expand `CurationChip` radio row with locally staged (not auto-committing) selection, explicit Confirm/Cancel. New `web/src/lib/expandedField.svelte.ts` module-level singleton coordinates F56.9 single-expansion across the page. Wired into `people/[id]/+page.svelte`'s `compactFields` and `longFields` owner branches, replacing `SourceSelect`; the Name field's `SourceSelect` (Tier-1, `onadopt`-intercepted) was left untouched. `npm run check` clean (0 errors). Live-QA'd against `backend-films` (TMDB-enriched `Oscar Isaac`, id 4): the F56.4/HOLODEX-245 RD6-confirm fix verified end-to-end — Confirm alone on the pending `nationality` chip issued exactly one `PUT .../decision` and flipped `decision.standing` to `true`; chip staging and inline-Custom-draft editing confirmed zero network calls until Confirm; Cancel discarded a staged custom value with zero calls; F56.9 single-expansion confirmed (expanding Website collapsed an unconfirmed Born); Escape returned focus to the badge; click-away (clicking the theme switcher) also collapsed via the shared `dismissable` action. All 3 skins checked via computed-style contrast (screenshots time out in this environment) — Confirm/Cancel contrast ratios all well above WCAG AA in Cinémathèque/Broadcast/Brutalist. Updated `docs/testing-strategy.md`'s F56 row from pre-implementation to implemented+manually-QA'd, and `curation/CLAUDE.md`'s component table. No automated component tests added — this codebase's Vitest setup has no `@testing-library/svelte`/`jsdom`, consistent with the established gap tracked in testing-strategy §11. Next session should tackle Up-next item 3: roll `SourceBadge` out to Video and Studio detail pages, replacing their `SourceSelect` call sites the same way.

### 2026-08-10 · Testing strategy for HOLODEX-268
- skills: testing-strategy
- handoff: Added the F56 block to `docs/testing-strategy.md` — a §5 frontend-strategy table row (frontend-only feature, no §4 backend row needed) plus a §11 gap-tracking bullet, both marked pre-implementation/target-coverage. The row calls out the F56.4 RD6-confirm case as the direct regression test for the bug this story exists to fix (Confirm alone, no chip click, must issue exactly one decision PUT and flip `decision.standing` true). Security-review and architecture stay not-applicable (no new mutation surface). All four applicable gates (spec/design/testing) are now green — only `[backend]`/`[frontend]` remain before this can leave Draft. Next session should start `[frontend]` implementation with `SourceBadge.svelte`, landing the F56.4 RD6-confirm bug fix + its regression test first since it has the clearest, most testable acceptance criterion.

### 2026-08-09 · Spec + design handoff for HOLODEX-268
- skills: write-spec, design-handoff
- handoff: Spec and design handoff (+ QA checklist) both landed. Handoff proposes a new `SourceBadge.svelte` (not extending `SourceSelect`) so the staged-selection Confirm/Cancel model can't inherit the old auto-commit ambiguity. Badge discoverability resolved to hover-only reveal (mocked both options, Kevin picked it). Next session should start `[frontend]` implementation with `SourceBadge.svelte`, landing the F56.4 RD6-confirm bug fix first since it has the clearest, most testable acceptance criterion.
