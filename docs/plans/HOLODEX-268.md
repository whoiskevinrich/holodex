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

**Design package:** [two-tier-field-editing.md](../specs/two-tier-field-editing.md) · ADR: none (extends ADR-051, no new architecture) · handoff: not yet produced · testing-strategy §: not yet added

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/two-tier-field-editing.md`
- [~] architecture `architecture` → none needed — presentation-layer restructuring of ADR-051, no new persistence/API/access-control shape (see spec's Depends-on line)
- [ ] design `design-handoff` → `docs/design/**`
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [~] security `security-review` — until: a new mutation surface is introduced (none planned; see spec § Access control & security)

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [ ] [design] Design handoff for the Tier-2 wrapper (badge/expand/chip-row/Confirm-Cancel), all 3 skins — `docs/design/**`
2. [ ] [frontend] Build the Tier-2 wrapper component, reusing `CurationChip` radio mode + `ProvenanceBadge` — `web/src/lib/components/curation/`
3. [ ] [frontend] F56.4 RD6-confirm bug fix — verify Confirm commits a standing decision for the pending implicit winner — `web/src/lib/components/curation/SourceSelect.svelte`, `web/src/lib/f36.ts`
4. [ ] [frontend] Roll Tier-2 out to all remaining replace fields on Video/Person/Studio  ⛔ blocked on #2
5. [ ] [testing] Add F56 block to `docs/testing-strategy.md` + regression test for the RD6-confirm fix

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-09 · Wrote the F56 spec for HOLODEX-268
- skills: write-spec
- handoff: Spec is done and grounded in the real `SourceSelect`/`CurationChip`/`ProvenanceBadge`/`f36.ts` code; next session should start with `/design-handoff` for the Tier-2 wrapper component across all three skins.
