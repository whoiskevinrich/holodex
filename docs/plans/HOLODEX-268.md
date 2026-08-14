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
- [x] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §5 frontend row + §11 gap-tracking bullet (frontend-only feature, no §4 backend row needed)
- [~] security `security-review` — until: a new mutation surface is introduced (none planned; see spec § Access control & security)

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [frontend] Build `SourceBadge.svelte` (badge/expand/chip-row/Confirm-Cancel), reusing `CurationChip` radio mode + `ProvenanceBadge` — `web/src/lib/components/curation/SourceBadge.svelte`
2. [x] [frontend] F56.4 RD6-confirm bug fix — verified live against `backend-films`: Confirm alone (no chip click) on an RD6-pending chip issued exactly one `PUT .../decision` and flipped `decision.standing` to `true`
3. [x] [frontend] Roll `SourceBadge` out to the remaining Tier-2 replace fields on Video (`media/[id]/+page.svelte`) and Studio (`studios/[id]/+page.svelte`) detail pages, mirroring today's Person rollout

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-11b · discovered branch was stale, resynced with main, reopened PR
- skills: —
- handoff: The previous session's worklog claimed the implementation PR had been opened
  ready-for-review, but that referred to PR #232, which had already merged separately —
  this branch's own follow-up commit (`c2bf1e8`, the 14 code-review fixes) was never
  actually pushed or opened as its own PR. Pushed it and opened PR #234, then found via
  `gh pr view 234` that it was `mergeable: CONFLICTING` — this branch's merge-base with
  `main` was still `74de929` (right after PR #227), predating HOLODEX-269/270/271 all
  landing on `main` since. Merged `origin/main` in: only one real conflict
  (`docs/plans/HOLODEX-268.md`'s own session-log entry, trivial — kept the newer side);
  the three shared entity detail pages (`media`/`people`/`studios` `+page.svelte`)
  auto-merged cleanly with no manual resolution needed. `go build ./...`, `go test ./...`
  (full suite), `npm run check` (489 files, 0 errors, same 6 pre-existing a11y warnings),
  and `npm run test` (139/139) all clean after the merge. Next: commit, push, re-verify
  PR #234 is mergeable.

### 2026-08-11 · Code-review fixes applied, implementation PR opened
- skills: code-review, simplify, graphify
- handoff: Applied 14 of 15 findings from a code review of the `SourceBadge`/`expandedField`/`ProvenanceBadge` implementation (undercounted multi-source detection, owner-view double-render, collapse not gated on busy, an over-tracking effect, `close()` stealing Custom-input focus, `dismissable`'s capture-phase Escape preempting the Custom input's own handler, a `longFields` layout bug dropping `ProvenanceBadge` to its own line, wrong provider/aria-label for manual values, a stale `busy` flag on collapse, the `expandedField` singleton leaking across entity navigation, the pending indicator landing on the wrong chip, arrow-key nav clobbering staged Custom text, an unmuted empty-value placeholder, and a spec self-contradiction on Video's Studio field). Left `SourceBadge`/`SourceSelect` logic duplication unfixed — real but disproportionate to a review-fix pass; tracked as a known gap, not a new issue. `npm run check` clean (486 files, 0 errors). Pushed and opened the implementation PR (spec/design/testing docs already merged via PR #227; this PR covers the `SourceBadge` build-out + review fixes) — all epic gates were already green going in, so it was opened ready for review, not Draft.

### 2026-08-10 · Video/Studio `SourceBadge` rollout — frontend gate complete
- skills: (none — direct implementation)
- handoff: Rolled `SourceBadge` out to the two remaining entity pages. Resolved a spec ambiguity first: `docs/specs/two-tier-field-editing.md`'s F56.6 requirements table lists Video's Studio field as Tier-1, but its own §Non-Goals prose reads as converting it to Tier-2 like any other replace field — treated the later, more specific `docs/design/two-tier-field-editing-handoff.md` Overview line ("excluding Title/People/Studio on Video and name on Person/Studio") as authoritative, so Video's Studio field stays on `SourceSelect`, unconverted, pending HOLODEX-271's relationship popover. On `media/[id]/+page.svelte`: converted the Commentary field block and the generic Metadata `dl` replace-field rows to `SourceBadge` (both default `baselineKey`, matching existing `SourceSelect` usage); left the Studio field's `SourceSelect` untouched. On `studios/[id]/+page.svelte`: converted both the compact- and long-fields owner branches to `SourceBadge` (`baselineKey="record"`); found Studio's own `name` field was never on `SourceSelect` at all (it's routed through `AliasPanel`'s `allowRename` instead), so the `SourceSelect` import was removed entirely from that page — no remaining Tier-1 `SourceSelect` usage on Studio. Updated `curation/CLAUDE.md`'s component table to reflect final ownership. `npm run check` clean (486 files, 0 errors, 6 pre-existing unrelated a11y warnings). Live-QA'd the HOLODEX-245/F56.4 RD6-confirm regression on both new entity types with real provider-enriched data (not synthetic): Video's `tagline` field (id 8, "Dune," from TMDB) and Studio's `country` field (id 7, "Legendary Pictures," generated via the page's own "Enrich from tmdb" button) — both confirmed via a single Confirm click issuing exactly one `PUT .../decision` and flipping `decision.standing` to `true`, verified independently via direct `curl` against the resolved API response (network-log tooling reported an ambiguous `ERR_ABORTED` annotation alongside the real `204` on the Video case; the `curl` check confirmed the PUT genuinely committed). Updated `docs/testing-strategy.md`'s F56 row and PR #227's body/gate-status to drop the "Video/Studio remaining" qualifiers. All applicable gates (spec/design/testing/frontend) are now green — architecture/backend/security correctly stay N/A. PR #227 is ready to leave Draft.

### 2026-08-10 · Frontend implementation — `SourceBadge.svelte` + HOLODEX-245 fix
- skills: (none — direct implementation)
- handoff: Built `SourceBadge.svelte` (`web/src/lib/components/curation/SourceBadge.svelte`) as a structurally independent sibling to `SourceSelect`, per the design handoff — collapsed `ProvenanceBadge` at rest, click-to-expand `CurationChip` radio row with locally staged (not auto-committing) selection, explicit Confirm/Cancel. New `web/src/lib/expandedField.svelte.ts` module-level singleton coordinates F56.9 single-expansion across the page. Wired into `people/[id]/+page.svelte`'s `compactFields` and `longFields` owner branches, replacing `SourceSelect`; the Name field's `SourceSelect` (Tier-1, `onadopt`-intercepted) was left untouched. `npm run check` clean (0 errors). Live-QA'd against `backend-films` (TMDB-enriched `Oscar Isaac`, id 4): the F56.4/HOLODEX-245 RD6-confirm fix verified end-to-end — Confirm alone on the pending `nationality` chip issued exactly one `PUT .../decision` and flipped `decision.standing` to `true`; chip staging and inline-Custom-draft editing confirmed zero network calls until Confirm; Cancel discarded a staged custom value with zero calls; F56.9 single-expansion confirmed (expanding Website collapsed an unconfirmed Born); Escape returned focus to the badge; click-away (clicking the theme switcher) also collapsed via the shared `dismissable` action. All 3 skins checked via computed-style contrast (screenshots time out in this environment) — Confirm/Cancel contrast ratios all well above WCAG AA in Cinémathèque/Broadcast/Brutalist. Updated `docs/testing-strategy.md`'s F56 row from pre-implementation to implemented+manually-QA'd, and `curation/CLAUDE.md`'s component table. No automated component tests added — this codebase's Vitest setup has no `@testing-library/svelte`/`jsdom`, consistent with the established gap tracked in testing-strategy §11. Next session should tackle Up-next item 3: roll `SourceBadge` out to Video and Studio detail pages, replacing their `SourceSelect` call sites the same way.

### 2026-08-10 · Testing strategy for HOLODEX-268
- skills: testing-strategy
- handoff: Added the F56 block to `docs/testing-strategy.md` — a §5 frontend-strategy table row (frontend-only feature, no §4 backend row needed) plus a §11 gap-tracking bullet, both marked pre-implementation/target-coverage. The row calls out the F56.4 RD6-confirm case as the direct regression test for the bug this story exists to fix (Confirm alone, no chip click, must issue exactly one decision PUT and flip `decision.standing` true). Security-review and architecture stay not-applicable (no new mutation surface). All four applicable gates (spec/design/testing) are now green — only `[backend]`/`[frontend]` remain before this can leave Draft. Next session should start `[frontend]` implementation with `SourceBadge.svelte`, landing the F56.4 RD6-confirm bug fix + its regression test first since it has the clearest, most testable acceptance criterion.

### 2026-08-09 · Spec + design handoff for HOLODEX-268
- skills: write-spec, design-handoff
- handoff: Spec and design handoff (+ QA checklist) both landed. Handoff proposes a new `SourceBadge.svelte` (not extending `SourceSelect`) so the staged-selection Confirm/Cancel model can't inherit the old auto-commit ambiguity. Badge discoverability resolved to hover-only reveal (mocked both options, Kevin picked it). Next session should start `[frontend]` implementation with `SourceBadge.svelte`, landing the F56.4 RD6-confirm bug fix first since it has the clearest, most testable acceptance criterion.
