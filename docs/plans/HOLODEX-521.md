---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-521
status: in-progress          # DERIVED from the Gates below; only `done`/`released` are read from here.
profile: full                # destructive Trash action + multi-record carry-over (security); a possible entity-identity.md edit (architecture)
depends-on: []
approved:
  design:
    on: 2026-10-08
    at: 83201455
release_note: "The Duplicates page now flags two files matched to the same provider item, shows them side by side, and lets you keep one (your playlists, film links and edits move to it), keep both, or label them as editions or parts."
---

# HOLODEX-521 · Duplicate videos v1: shared provider match queue, side-by-side compare, keep/label actions

Done when the owner's Duplicates page has a Videos lane listing every two live files that share a
provider match. Each pair can be compared side by side and resolved: keep one (the other goes to
Trash and the owner's work carries over), keep both, or label as editions or parts. First slice of
the scene model (HOLODEX-520).

**Design package:** [spec F76](../specs/duplicate-videos.md) · [handoff](../design/duplicate-videos-handoff.md) + [mockup](../design/duplicate-videos-mockup.svg) · testing-strategy § (pending)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — F76 `docs/specs/duplicate-videos.md`; no open questions
- [x] architecture `architecture` → `docs/architecture/**` — reopened from `[~]`: the build found a real fork (pairs computed on demand vs stored in the review queue); recorded in `entity-identity.md` "Duplicate videos are computed on demand, not queued"
- [x] design `design-handoff` → `docs/design/**` — `duplicate-videos-handoff.md` + measured SVG mockup; option B fact table, collapsed default (owner's choices 2026-10-08)
- [x] backend → `{cmd,internal,providers}/**` — `repo/video_pairs.go` + `api/duplicates_video.go`; 11 repo + 5 API tests, full Go suite green; `/code-review high --fix` applied
- [x] frontend → `web/src/**` — browser-verified on a seeded stress copy (desktop + 375 px); owner confirmed the build matches the approved mockup 2026-10-08
- [x] testing `testing-strategy` — §24 in `docs/testing-strategy.md`; five mutation checks each turned a named test red
- [x] security `security-review` — 2026-10-08: no vulnerabilities (all five routes owner-gated, all SQL bound, liveness re-checked in-transaction, clear tag names from the mapping only)

## Up next — ordered (position = priority)

1. [ ] [—] Run `make test-image` once so the mkvpropedit edition-clear case runs (skipped locally) — `internal/writeback/edition_clear_integration_test.go`
2. [ ] [—] After squash-merge: fill the "Decided in" sha — `docs/architecture/entity-identity.md`
3. [ ] [—] Permanent delete leaves a video's decisions/curation/dismissals orphaned (found writing F76) → HOLODEX-547

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-08 · session
- skills: system-design, write-spec, handoff, design-handoff, implement, testing-strategy, security-review
- handoff: All seven gates settled and PR #465 marked ready; the owner confirmed the build matches the mockup. Next: run `make test-image` for the mkvpropedit edition-clear case, then after merge fill the "Decided in" sha.

## Dropped — newest first (the reason is the point)
