---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-521
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: full                # destructive Trash action + multi-record carry-over (security); a possible entity-identity.md edit (architecture)
depends-on: []
release_note: "The Duplicates page now flags two files matched to the same provider item, shows them side by side, and lets you keep one (your playlists, film links and edits move to it), keep both, or label them as editions or parts."
---

# HOLODEX-521 · Duplicate videos v1: shared provider match queue, side-by-side compare, keep/label actions

Done when the owner's Duplicates page has a Videos lane listing every two live files that share a
provider match. Each pair can be compared side by side and resolved: keep one (the other goes to
Trash and the owner's work carries over), keep both, or label as editions or parts. First slice of
the scene model (HOLODEX-520).

**Design package:** [spec F76](../specs/duplicate-videos.md) · handoff (pending) · testing-strategy § (pending)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F76 `docs/specs/duplicate-videos.md`; no open questions
- [ ] architecture `architecture` → `docs/architecture/**`
- [ ] design `design-handoff` → `docs/design/**`
- [ ] backend → `{cmd,internal,providers}/**`
- [ ] frontend → `web/src/**`
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [design] `/design-handoff` for the Videos lane, compare view, keep-one confirm and label actions — `docs/design/`
2. [ ] [—] `/implement` once the handoff is signed off: draft PR, then the build
3. [ ] [architecture] Record the review queue's video lane in `entity-identity.md` in the build PR, or `[~]` if it proves no fork — `docs/architecture/entity-identity.md`
4. [ ] [—] Permanent delete leaves a video's decisions/curation/dismissals orphaned (found writing F76) → HOLODEX-547

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-08 · session
- skills: system-design, write-spec, handoff
- handoff: Spec F76 is written (Videos lane on Duplicates; keep one with additive carry-over, keep both, label editions/parts; poster + facts, frame strips P1); scene model parked as HOLODEX-520, move detection HOLODEX-522. Next: `/design-handoff` for the Videos lane and compare view.

## Dropped — newest first (the reason is the point)
