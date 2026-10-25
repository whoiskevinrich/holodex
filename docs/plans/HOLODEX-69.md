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

- [/] spec `write-spec` → `docs/specs/**` — F36 P1-6 (same-ish fold) + F30.3g (Removed (N) restore) drafted
- [~] architecture `architecture` → `docs/architecture/**` — no technology fork. Removed values ride the owner's detail read the way offered empty fields already do (field-resolution.md, "Empty replace fields are offered through the resolver"). The resolver takes an owner-only option, lists each field's removals from the curation rows it already loads (which store the displayed value), and marks a removal no source sends now. The same option keeps a field whose every value was removed. Visitor reads are unchanged. A separate owner fetch would have to re-resolve to know "not sent now" and would add a second request to every curated page, so it is not a credible alternative
- [x] design `design-handoff` → `docs/design/**` — `docs/design/source-choices-restore-handoff.md` + SVG mockup (option A, the inline disclosure, picked by the owner 2026-10-25; sign-off still to be recorded at /implement)
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [spec] Owner reviews P1-6's punctuation rule and F30.3g's list-everything rule — docs/specs/
2. [ ] [testing] /testing-strategy: the fold's match key, Removed (N) states and focus, visitor gating — docs/testing-strategy.md
3. [ ] [—] /implement once 1 settles (records the design sign-off, opens the Draft PR)
4. [ ] [backend] Owner-only resolver option listing removals + "not sent now" on media, person and studio detail reads — internal/resolver, internal/api

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-25 · Epic 5 scoped against HOLODEX-267; spec rules and design handoff
- skills: docs, design-handoff, handoff
- handoff: Epic 5 is HOLODEX-69 + HOLODEX-138; HOLODEX-44 is Won't Do. Spec rules F36 P1-6 (same-ish fold) and F30.3g (Removed (N)) are drafted, and both specs now link topic docs, not archived ADRs. The design handoff is committed: the owner picked option A, the inline disclosure, and accepted the folded chip, which needs no new visual, only a wider match in sourceChips. ui-vocabulary gained "Removed disclosure" and "Folded choice". Architecture settled as no fork: removals ride the owner's detail read through a resolver option, like offered empty fields. Next: the owner reviews the two spec rules (Up next 1), then /testing-strategy and /implement.

## Dropped — newest first (the reason is the point)
