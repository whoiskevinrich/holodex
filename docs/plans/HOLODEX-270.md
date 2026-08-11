---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-270                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: [HOLODEX-269]               # [KEY-…] cross-epic deps that must land first
release_note: Editing a video's title now warns you when another video already has the same title, people, date, and studio — so accidental duplicates don't slip in unnoticed.
---

# HOLODEX-270 · Video composite-key collision check (F56.3)

A video's real identity is {Title, People, Date, Studio}, not just its title. Done means a manual
title edit that would produce that exact combination on another active video is blocked with a
verdict panel (view the existing video, or save anyway) instead of silently committing — the
Studio (HOLODEX-271) and People (HOLODEX-272) trigger points reuse the same backend check once
those stories land.

**Design package:** [spec](../specs/video-composite-key-collision.md) ·
[design handoff](../design/video-collision-verdict-handoff.md) ·
[testing-strategy §5](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/video-composite-key-collision.md`
- [ ] architecture — not needed (no new ADR; reuses the existing 409-conflict response
      convention and the F36/ADR-051 decision model, per the spec's own framing)
- [x] design `design-handoff` → `docs/design/video-collision-verdict-handoff.md`
- [x] backend — `internal/repo/video_collision.go` (`FindTitleCollision`), wired into
      `internal/api/decisions.go`'s Title path with a 409+`override` gate
- [x] frontend — `NameEditControl` generalized over the conflict payload type,
      `CollisionOfferCard.svelte`, wired into the Video Title mount
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §5
- [x] security `security-review` — clean pass, zero findings ≥80% confidence (owner
      gate intact, all `FindTitleCollision` queries parameterized, no new cross-owner
      data exposure)

## Up next — ordered (position = priority)

1. [ ] [—] Studio trigger point → HOLODEX-271 (reuses `FindTitleCollision`'s sibling once
   that story adds a studio-composite variant)
2. [ ] [—] People trigger point → HOLODEX-272 (same, once that story's attach/detach
   surface exists)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-11c · committed, pushed, PR #230 ready, Jira synced
- skills: —
- handoff: Committed as `90a691f` (`feat(video): composite-key collision check on title
  edit`), pushed, opened [PR #230](https://github.com/whoiskevinrich/holodex/pull/230)
  directly ready-for-review (not draft — every gate was already green at PR-creation
  time). CI's `jira-sync.yml` fired the In-Review transition automatically on the
  non-draft PR. Cleared the stale `needs-design`/`needs-spec` labels on HOLODEX-270 now
  that both artifacts landed. HOLODEX-270 is fully done pending human review/merge. Next
  in the epic: HOLODEX-271 (studio relationship popover), starting with `/write-spec`.

### 2026-08-11b · testing-strategy doc, /simplify, /security-review, all gates green
- skills: testing-strategy, simplify, security-review
- handoff: Added the HOLODEX-270 row to `docs/testing-strategy.md` §5. `/simplify` ran 4
  parallel review agents (reuse/simplification/efficiency/altitude) against the story's
  own diff; two independent agents converged on the same finding — `FindTitleCollision`
  materialized a `group_concat` key across every video in the table just to rule out the
  common zero-match case — fixed by filtering candidates on the cheap indexed
  title+date columns first, then only comparing people/studio sets for survivors (both
  collision tests re-verified green after the rewrite, `go test ./...` and
  `npm run check` both clean). Two lower-confidence/out-of-scope findings were skipped
  with a note: the `api.ts` fetch/409-parsing duplication (fixing it would touch 3
  pre-existing functions outside this diff) and generalizing `FindTitleCollision`'s
  signature now for HOLODEX-271/272 (speculative ahead of those stories existing).
  `/security-review` (scoped to just this story's diff, since the branch-wide diff
  included HOLODEX-269's already-cleared code) came back with zero findings. All gates
  green — next: commit/push/PR/Jira sync.

### 2026-08-11 · backend + frontend implementation, live QA, bug found and fixed
- skills: testing-strategy
- handoff: Backend (`FindTitleCollision` + the `setFieldDecision` 409/override gate) and
  frontend (`NameEditControl` generalized to be generic over the conflict payload,
  `CollisionOfferCard`, wired into Video Title) are done and fully unit/API tested. Live
  QA in an isolated backend+frontend instance (own port, own scratch DB/media, two seeded
  fixture videos) found a real bug: a colliding video with zero linked people marshaled
  `people` as JSON `null` (Go nil-slice default), and `CollisionOfferCard` read
  `video.people.length` unconditionally — crashed the whole page. Fixed by initializing
  `VideoCollision.People` to `[]string{}` in `video_collision.go`, added a regression test
  (`TestFindTitleCollision_NoPeople`), and re-verified all three interaction paths (collision
  card renders, "Save anyway" commits with override, "View existing video" navigates away
  without persisting) live, plus contrast/token compliance across all three skins. Next
  session: `/security-review`, then commit/push/PR/Jira sync.

### 2026-08-10 · spec + design handoff written
- skills: write-spec, design-handoff
- handoff: Spec and design handoff are grounded in the real schema/endpoints (Title trigger
  via HOLODEX-269's `NameEditControl` conflict slot; Studio/People triggers deferred to
  HOLODEX-271/272). Backend+frontend implementation is next.
