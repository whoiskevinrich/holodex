---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-271
status: in-review
depends-on: [HOLODEX-270]
release_note: Reassigning a video's studio now goes through a single popover — pick a known candidate, search the full studio library, or create one inline — and it's protected by the same duplicate-video safeguard as renaming a title.
---

# HOLODEX-271 · Studio relationship-edit popover (F56.4)

Done means the video detail page's Studio field is edited through one popover (`StudioPicker`) —
known-candidate chips, full-library search, and an inline create-fallback — replacing
`SourceSelect`'s Studio radiogroup, and every studio pick (chip, search, or create) runs through
the HOLODEX-270 composite-key collision gate before committing, since a picker selection changes
a video's {title, people, date, studio} identity exactly as much as a typed rename does.

**Design package:** [spec](../specs/studio-relationship-popover.md) ·
[design handoff](../design/studio-picker-handoff.md) ·
[testing-strategy §5](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/studio-relationship-popover.md`
- [x] architecture — not needed (no new ADR; reuses HOLODEX-270's 409-conflict convention
      and the existing F36/ADR-051 decision model)
- [x] design `design-handoff` → `docs/design/studio-picker-handoff.md`
- [x] backend — `internal/repo/video_collision.go` (`FindStudioCollision`, sibling to
      `FindTitleCollision` via shared `compositeKeyCandidates`/`hydrateCollision`/
      `recordedAtOf` helpers + name-based `linkedNameKey`/`normalizedNameKey`), wired
      into `internal/api/decisions.go`'s Studio path with the same 409+`override` gate
- [x] frontend — `StudioPicker.svelte` (chips + debounced search + create-fallback,
      composing `NameEditControl`'s state machine and `EntityPickerDialog`'s search
      body), wired into the Video Studio field with `CollisionOfferCard` in the
      existing verdict slot
- [x] testing `testing-strategy` → `docs/testing-strategy.md` §5
- [x] security `security-review` — clean pass, zero findings ≥80% confidence (owner
      gate unchanged, all new queries parameterized, no new privilege boundary)

## Up next — ordered (position = priority)

1. [ ] [—] People trigger point → HOLODEX-272 (attach/detach surface doesn't exist
   yet; wires into the same composite-key collision mechanism once it does)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-11c · PR #231 code-review fixes + a second /simplify pass
- skills: code-review, simplify
- handoff: `/code-review PR #231` surfaced 5 findings, all fixed: an empty-studio pick
  skipping the collision check (two videos both dropping their studio, matching on
  every other axis, wasn't being caught), a non-atomic check-then-write race on the
  Studio decision write, a multi-studio collision response collapsing to one name
  (`VideoCollision.Studio *string` → `Studios []string`), a stale doc claim in
  `web/src/lib/components/entity/CLAUDE.md` (said `StudioPicker` commits via a
  `decideField('studio', ...)` path that doesn't exist — it goes through
  `decideStudio`/`saveStudioAnyway`), and duplicated studio-name resolve/extract logic
  between `relinkVideoStudios` and the collision check.
- The mandatory pre-commit `/simplify` pass on that fix diff then caught a deeper issue
  in the race-condition fix itself: my first attempt (`WithWriteLock`/
  `SetDecisionLocked`, both exported) leaked `repo`'s `writeMu` out to `internal/api`,
  breaking the encapsulation every other locked write in the package honors — confirmed
  by finding the real precedent (`SetTitleDecisionChecked`, merged to `main` via PR
  #230) keeps its lock private. Replaced with `Repo.SetDecisionChecked(ctx, ...,
  check func() (*VideoCollision, error))`: `writeMu` and the locked write
  (`setDecisionLocked`) stay unexported, `internal/api` injects its resolver-dependent
  check as a closure. Paired efficiency fix: the expensive `loadRelinkContext` +
  `resolver.Resolve` pass now runs unlocked (skipped entirely on override) with only a
  cheap recheck + the SQL upsert inside the lock, so a Studio decision no longer
  serializes every other app write behind a multi-query fetch. This branch is stacked
  on a stale, pre-merge copy of HOLODEX-270 (not `main`'s merged version), so the fix
  is same-branch and non-destructive rather than a rebase.
- `go build ./...`, `go test ./internal/repo/... ./internal/api/...` (incl.
  `TestDecisionAPI_StudioCollision`, `TestFindStudioCollision`, `TestFindTitleCollision`)
  and `npm run check` (490 files, 0 errors) all clean. Next: commit/push (PR #231
  already open, ready for review), Jira sync.

### 2026-08-11 · simplify + security-review, all gates green, ready to commit
- skills: simplify, security-review
- handoff: `/simplify` ran 4 parallel review agents (reuse/simplification/efficiency/
  altitude); three independently converged on the same mechanism — `studioCollision`
  and `relinkVideoStudios` both fetched + resolved the same data for the same decision
  on every Studio commit (8 queries + 2 resolver passes instead of 4 + 1) and had
  already diverged on how they extracted the resolved names (append vs. overwrite, a
  latent bug). Fixed by having `studioCollision` return its fetched `relinkContext` +
  resolved names, and adding `relinkStudiosWithContext` so the no-collision path
  reconciles `video_studios` directly instead of re-fetching/re-resolving; also fixed
  the append/overwrite divergence and had `normalizedNameKey` reuse the existing
  `foldNameKey` helper instead of re-implementing the fold rule inline. Skipped (noted,
  not fixed): `StudioPicker`'s debounced-search logic duplicating
  `EntityPickerDialog`'s (touches an unrelated already-shipped component, out of
  scope), generalizing `setFieldDecision`'s title/studio dispatch now (premature ahead
  of People being the third case in HOLODEX-272), and factoring `decideStudio`/
  `saveStudioAnyway` together with the pre-existing Title pair (would touch
  already-working HOLODEX-270 code outside this diff). `go test ./...` and
  `npm run check` both clean after the fixes. `/security-review`, scoped to this
  story's own diff, came back with zero findings — same clean-pass shape as HOLODEX-270.
  All gates green. Next: commit/push/PR/Jira sync.

### 2026-08-11b · backend + frontend implementation, 3-skin live QA
- skills: testing-strategy
- handoff: Backend (`FindStudioCollision` + the Studio 409/override gate) and frontend
  (`StudioPicker`, wired into the Video Studio field with `CollisionOfferCard` reused
  unmodified from HOLODEX-270) are done and unit/API tested
  (`TestFindStudioCollision`, `TestDecisionAPI_StudioCollision`). Live QA in an isolated
  backend+frontend instance confirmed the pencil affordance, single-candidate chip-row
  suppression, debounced search-select, and create-fallback all commit and close
  correctly; verified WCAG AA contrast across all three skins (Broadcast/Brutalist
  15.6–18.5:1 body text, 4.67–5.59:1 shared `text-muted` helper — same token as every
  other component, not a regression). The 409/`CollisionOfferCard` path wasn't
  re-driven live for Studio specifically (reproducing a real composite-key collision
  needs a second People-editing surface HOLODEX-272 hasn't built yet); covered instead
  by `TestDecisionAPI_StudioCollision` plus `CollisionOfferCard` being the same
  component already browser-QA'd for Title. Added the testing-strategy §5 row. Next:
  `/simplify`, `/security-review`, commit/push/PR/Jira sync.

### 2026-08-11a · spec + design handoff written
- skills: write-spec, design-handoff
- handoff: Spec and design handoff for the Studio popover, grounded in HOLODEX-270's
  collision mechanism and `EntityPickerDialog`'s existing search/create pattern.
  Backend + frontend implementation is next.
