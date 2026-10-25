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
release_note:
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
[F30.3g](../specs/metadata-curation.md) · no architecture doc yet · handoff pending

## Gates — definition of done

<!-- States: [ ] not started · [/] in progress · [~] deliberately skipped · [x] done. [~] and [x]
     both count as SETTLED. PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [/] spec `write-spec` → `docs/specs/**` — F36 P1-6 (same-ish fold) + F30.3g (Removed (N) restore) drafted
- [ ] architecture `architecture` → `docs/architecture/**`
- [ ] design `design-handoff` → `docs/design/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [design] /design-handoff: the Removed (N) disclosure on CurationFieldRow, plus a mockup of a folded chip — docs/design/
2. [ ] [architecture] Decide how removed values reach the owner: a field on the detail response, or a separate owner fetch. Record [~] if it's no fork — docs/architecture/
3. [ ] [spec] Owner reviews P1-6's punctuation rule and F30.3g's list-everything rule — docs/specs/

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-25 · Epic 5 scoped against HOLODEX-267; spec rules drafted
- skills: docs
- handoff: Epic 5 is now HOLODEX-69 + HOLODEX-138 (HOLODEX-44 is Won't Do, linked Relates). Two spec rules are drafted: F36 P1-6 folds candidates that differ only in case, whitespace or punctuation, which the owner chose over whitespace-and-case only; F30.3g adds the Removed (N) restore. Next: the design handoff, starting from the F30 handoff's existing "Show removed (N)" sketch.

## Dropped — newest first (the reason is the point)
