---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-301                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The Completeness panel on video/person/studio detail pages now folds its facet checklist behind a collapsible toggle, collapsed by default, so the page stays compact until you want the detail.
---

# HOLODEX-301 · Completeness panel collapsible facet fold

Ad hoc UX request (no epic — filed retroactively after implementation, see session log). Done
means: `CompletenessPanel.svelte`'s Critical/Nice-to-have facet groups collapse behind a
chevron toggle beside the score, collapsed by default; score/bar/actionability summary always
stays visible; `npm run check` clean; verified live across all three skins.

**Design package:** design [completeness-panel-fold-handoff.md](../design/completeness-panel-fold-handoff.md)
+ [mockup SVG](../design/completeness-panel-fold-mockup.svg) · no spec/ADR needed (pure UI
addition on an already-specced component, extends
[entity-completeness-handoff.md](../design/entity-completeness-handoff.md) §2 DD4-DD8) ·
testing-strategy: none needed (no new backend/logic surface — see session log)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → not needed; pure UI addition to an already-specced component (extends entity-completeness-handoff.md §2 DD4-DD8, no new behavior/data surface)
- [x] architecture `architecture` → not needed; no architectural change
- [x] design `design-handoff` → `docs/design/completeness-panel-fold-handoff.md` + `completeness-panel-fold-mockup.svg`, three rounds of mockup iteration
- [x] frontend → `web/src/lib/components/completeness/CompletenessPanel.svelte`
- [x] testing `testing-strategy` → not needed; no new logic surface to test beyond Svelte-check + live browser verification (pure presentational fold over existing data)
- [x] security `security-review` → not needed; no auth/access/infra surface touched

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [design] mockup iteration (3 rounds) + committed handoff doc/SVG — `docs/design/completeness-panel-fold-handoff.md`
2. [x] [frontend] chevron toggle + `max-height`/`inert` collapsible wrapper — `web/src/lib/components/completeness/CompletenessPanel.svelte`
3. [x] [frontend] three-skin QA (Cinémathèque/Broadcast/Brutalist) — verified via computed-style checks

Nothing left — this worklog was filed after the work was already done (see session log).

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-31 · design handoff + implementation + retroactive ticket/worklog
- skills: design-handoff, simplify
- handoff: HOLODEX-301 is implementation-complete, three-skin QA'd, and merged-ready on
  [PR #282](https://github.com/whoiskevinrich/holodex/pull/282). This ticket and worklog were
  filed retroactively, after the mockup/handoff/implementation were already done on a bare
  `claude/`-prefixed branch (following the `media-detail-reorder` no-epic precedent) — the owner
  asked for a worklog anyway, so a Story was created and the branch renamed to carry the key via
  GitHub's branch-rename API. That rename auto-closed the original PR #281 (GitHub closes a PR
  when its head ref is renamed out from under it — the rename endpoint does **not** carry the PR
  along despite what the GitHub UI's rename flow implies); recovered by opening #282 from the
  renamed branch with the same body and leaving a pointer comment on #281. Also note for future
  ad hoc-then-retroactive tickets: firing the `In Progress`/`In Review` Jira transitions by hand
  in the wrong order (after CI's `jira-sync` had already fired `In Review` on PR-open) briefly
  regressed the status back to `In Progress` — caught and corrected by re-firing `In Review`
  manually. Prefer letting CI's `opened`+`draft==false` path fire `In Review` on its own and
  only fire `In Progress` by hand *before* opening the PR, not after.
