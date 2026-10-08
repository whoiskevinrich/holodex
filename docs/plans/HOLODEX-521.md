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
- [/] frontend → `web/src/**` — built and browser-verified on a seeded stress copy (desktop + 375 px); held on the owner's row-title call and the build-vs-mockup comparison
- [ ] testing `testing-strategy`
- [ ] security `security-review`

## Up next — ordered (position = priority)

1. [ ] [frontend] Owner compares the built page against the approved mockup (handoff §3d) to settle the frontend gate — `docs/design/duplicate-videos-mockup.svg`
2. [ ] [testing] `/testing-strategy` for F76 — `docs/testing-strategy.md`
3. [ ] [security] `/security-review`: owner-only Trash action + multi-record carry-over + edition joins the clearable allowlist — `internal/api/duplicates_video.go`
4. [ ] [—] Run `make test-image` once so the mkvpropedit edition-clear case runs (skipped locally) — `internal/writeback/edition_clear_integration_test.go`
5. [ ] [—] After squash-merge: fill the "Decided in" sha — `docs/architecture/entity-identity.md`
6. [ ] [—] Permanent delete leaves a video's decisions/curation/dismissals orphaned (found writing F76) → HOLODEX-547

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-10-08 · session
- skills: system-design, write-spec, handoff, design-handoff, implement
- handoff: Backend, edition clearing, the frontend Videos group and the provider-title row label are in draft PR #465, browser-verified on a seeded fixture copy; design sign-off re-confirmed at 83201455. Next: owner compares build vs mockup, then `/testing-strategy` and `/security-review`.

## Dropped — newest first (the reason is the point)
