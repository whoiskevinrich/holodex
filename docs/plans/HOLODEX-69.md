---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-69                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: You can now see and restore the values you removed from a field, and source choices that differ only in case, spacing or punctuation show as one.
approved:
  design:
    on: 2026-10-25
    at: 63c1bec2
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
---

# HOLODEX-69 · Source choices — same-ish collapse and restore

Done means two things. Candidates that differ only in case, whitespace or punctuation share one
source choice on every entity (HOLODEX-138). And an owner can see and restore the values they
removed from a multi-value field, through **Removed (N)** (HOLODEX-69). This is epic 5 of the
frontend backlog plan. HOLODEX-44 (an explicit per-slot image lock) was closed as Won't Do on
2026-10-25: `docs/architecture/images.md` rejects that lock, and promoting an image already
locks it.

**Design package:** spec [F36 P1-6](../specs/field-source-of-truth.md) and
[F30.3g](../specs/metadata-curation.md) · no architecture doc (no technology fork) ·
[handoff](../design/source-choices-restore-handoff.md) + [mockup](../design/source-choices-restore-mockup.svg)

## Gates — definition of done

<!-- States: [ ] not started · [/] in progress · [~] deliberately skipped · [x] done. [~] and [x]
     both count as SETTLED. PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**` — F36 P1-6 (same-ish fold) + F30.3g (Removed (N) restore). The owner reviewed both on 2026-10-25 and kept: all punctuation and symbols dropped, the top provider's spelling when only providers fold, and stale removals listed and tagged
- [~] architecture `architecture` → `docs/architecture/**` — no technology fork. Removed values ride the owner's detail read the way offered empty fields already do (field-resolution.md, "Empty replace fields are offered through the resolver"). The resolver takes an owner-only option, lists each field's removals from the curation rows it already loads (which store the displayed value), and marks a removal no source sends now. The same option keeps a field whose every value was removed. Visitor reads are unchanged. A separate owner fetch would have to re-resolve to know "not sent now" and would add a second request to every curated page, so it is not a credible alternative
- [x] design `design-handoff` → `docs/design/**` — `docs/design/source-choices-restore-handoff.md` + SVG mockup (option A, the inline disclosure, picked by the owner 2026-10-25; signed off at ee0c6268)
- [x] backend → `{cmd,internal,providers}/**` — `resolver.Options.ListRemoved` + `ResolvedField.Removed` (not-sent-now marked), set on the owner's media, person and studio detail reads; person spellings fold to one removal; the genre writeback pass keeps an all-removed genres row. Resolver and endpoint tests pass, with two mutation checks
- [x] frontend → `web/src/**` — the fold (`foldKey`/`sameChoice` in f36.ts), CurationChip removed mode, and the CurationFieldRow Removed (N) disclosure with Restore, focus and announcements; + Add or the person picker lifts a removal. Live QA'd on the stress fixture 2026-10-25 (§34.5)
- [/] testing `testing-strategy`
- [/] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [testing] Owner's [human] QA: handoff items 9 (keyboard, screen reader) and 10 (does the dashed chip read as "not in effect"?) — docs/design/source-choices-restore-handoff.md
2. [ ] [design] /handoff §3d: owner compares the built Removed (N) to the approved mockup before the last gate settles
3. [ ] [—] Sweep HOLODEX-138 by hand with the PR (In Review on ready, Done on merge); CI moves only HOLODEX-69

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-25 · Epic 5 scoped against HOLODEX-267; spec rules and design handoff
- skills: docs, design-handoff, handoff, implement, testing-strategy, code-review, security-review
- handoff: Crossed into build: the design was signed off at ee0c6268, and Draft PR #554 is open. The test plan is §34 of testing-strategy.md. The backend landed, with owner-only removals on media, person and studio reads. Building it showed that typing a removed value into + Add never restored it (suppress beats a manual add), so the handoff now says + Add lifts the removal. The owner re-confirmed the sign-off at 63c1bec2 with that delta. The frontend landed: the fold in f36.ts, and Removed (N) on CurationFieldRow, live QA'd on the stress fixture. /security-review passed at e932f810 with no findings. Next: the owner's [human] QA (Up next 1) and the build-versus-mockup check, then /handoff marks the PR ready.

## Dropped — newest first (the reason is the point)
