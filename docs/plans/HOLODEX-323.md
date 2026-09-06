---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-323
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []
release_note: Writing metadata to a file no longer holds you in a dialog — submit and walk away; the Metadata section tells you only if a write is still running or failed.
---

# HOLODEX-323 · Fire-and-forget writeback with page-level pending/failed signals

Done means: submitting a writeback closes the dialog on the enqueue ack and the owner can
navigate away freely; a pending or failed job for that video is visible near the Metadata
section, sourced from `writeback_queue` so it survives reload and restart; a failed job
persists until retried or dismissed; and with neither pending nor failed work, nothing
renders at all — silence is the success signal.

**Design package:** [spec](../specs/fire-and-forget-writeback.md) · [ADR-091](../architecture/ADR-091-fire-and-forget-writeback-status.md) · [handoff](../design/fire-and-forget-writeback-handoff.md) · [mockup](../design/fire-and-forget-writeback-mockup.svg)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/fire-and-forget-writeback.md` (RD1–RD6, 12 acceptance criteria)
- [x] architecture `architecture` → `docs/architecture/ADR-091-fire-and-forget-writeback-status.md` (Proposed; supersedes ADR-073 **D4 only**)
- [x] design `design-handoff` → `docs/design/fire-and-forget-writeback-handoff.md` + committed SVG mockup
- [x] backend → `internal/repo/writequeue.go` (`GetVideoWritebackStatus`/`RetryFailedWriteback`/`DismissFailedWriteback`), `internal/writequeue/writequeue.go` (`Queue.RetryFailed`), `internal/api/writeback.go` (Retry/Dismiss routes, RD5 clear-on-enqueue), `internal/api/handlers.go` (R2.1a redaction on `getMedia`) — all Go-test-covered
- [x] frontend → `WritebackFormDialog.svelte` (closes on ack, `=`/circle-minus/checkbox gutter, row-tier order), `writebackJob.ts` (`waitForVideoWriteback`), `media/[id]/+page.svelte` (badges, poll effect, Retry/Dismiss), `films/[id]/+page.svelte` (contract parity) — `writebackJob.ts` Vitest-covered; dialog/page verified live against real media (see session log), no new Playwright/component coverage (§0's standing gap)
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (§4 row, §5 row, §10 adversarial block, 3 Critical invariants, §11 gap)
- [x] security `security-review` — **one MEDIUM finding, closed in the spec before code** (R2.1a: the failure message must be visitor-redacted server-side); re-run against the real diff before merge

## Up next — ordered (position = priority)

1. [x] [backend] per-video queue status repo query (`writeback_queue` by `video_id`, pending + failed) — `internal/repo/writequeue.go`
2. [x] [backend] surface it on `GET /media/{id}` — `internal/api/handlers.go`
3. [x] [backend] `failed` → `pending` reset + `kick()` for Retry — `internal/repo/writequeue.go`, `internal/writequeue/writequeue.go`
4. [x] [backend] owner-gated Retry / Dismiss routes — `internal/api/writeback.go`
5. [x] [frontend] close `WritebackFormDialog` on the 202 ack; keep it open on a failed enqueue
6. [x] [frontend] delete the transient status gutter (`isWriting` / `isError`), collapse `row.status`; row content unchanged; fixed the check-glyph collision with a distinct `=`/circle-minus/checkbox gutter and a tier-based row order (R4.3/R4.4)
7. [x] [frontend] pending + failed badges in the Metadata header, reusing the `pollUntilSettled` backoff loop (new `waitForVideoWriteback` wrapper) — `web/src/routes/media/[id]/+page.svelte`
8. [x] [frontend] three-skin contrast check per `.claude/rules/frontend-theming.md`'s sanctioned computed-style method — found and spun off a **pre-existing** gap (see session log), not introduced by this feature
9. [ ] [followup] re-evaluate HOLODEX-276 — per-field written/skipped is not a distinction an atomic job can honestly report; ADR-091 argues that ticket may no longer be wanted
10. [ ] [followup] mark PR #303 ready for review once this session's push lands (all 5 gates + implementation are green)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-05 · pre-implementation gates: ADR + design handoff
- skills: product-brainstorming, design-handoff, architecture
- Started from a narrower question — "does closing the modal break the write?" Traced the path
  and established it does not: the enqueue is the only thing scoped to the request context, the
  worker runs on the app-lifetime context from `Queue.Start(ctx)`, and ADR-041's copy → write →
  rename plus `RecoverRunningWritebacks` cover crash and restart. The file is never at risk.
- The real finding is that the backend has been fire-and-forget since ADR-048 and only the
  frontend pretends otherwise, so this is mostly a subtractive change plus one small query.
- Pushed back on treating the pending chip as a nice-to-have: ADR-073 D4's wait exists because
  refetching on the 202 reads the pre-write baseline and renders just-written fields as "out of
  sync". Closing early *without* a page signal reproduces that exact bug. Owner confirmed D4's
  premises no longer hold and that this supersedes it.
- Found that ADR-073's own consequences flag a "`done`-means-absent" hazard for any second
  consumer of job status. It is harmless in this design because succeeded, swept and
  never-existed all render identically as nothing — which is precisely why "silence means
  success" is safe here.
- Owner redirected the dialog's per-field status area from post-hoc status to a pre-flight
  confirm view. On inspection that preview already exists (`→ write_target`, the `was:` line,
  the no-op and unwritable states), so the change is subtractive: remove the transient icons.
- handoff: ADR-091 and the design handoff are green; **the spec gate is still red**, and the
  backend Retry path needs a new repo method because nothing resets `failed` today. Next
  session should write the spec before touching code.

### 2026-09-06 · design critique applied, spec landed
- skills: design-critique, write-spec
- Critiqued the mockup and found that "align the badges" had been applied too literally: in the
  failed state `write failed` and `out of sync` shared fill, hue, shape and size and merged into
  one bar of warn colour — the one state where two facts must be parsed at once was the least
  legible. Split geometry from weight: events filled, steady state outline-only.
- That split also resolved a question left open for the owner. Pending had been *hiding*
  out-of-sync, which was wrong on the facts (the file still differs until the job lands, and a
  queued write can sit behind a large `EnqueueMany` batch). Hiding was only ever a workaround for
  two badges that looked identical, so with the weight split both now show whenever both are true.
- Also found a glyph collision predating the critique: a check marked both "will be written" and
  "already matches, won't be written", inverting its meaning for the excluded row. No-ops now use
  an equals glyph.
- Moved the badges from the section label to sit beside the write action — all three are about the
  write, and anchoring them to the label had put a problem 340px from its own remedy.
- Spec landed with RD1–RD6 and 12 acceptance criteria. Three open questions went to the owner as
  cards: Dismiss deletes the row (job_runs is the audit trail), a new write clears an
  unacknowledged failure, and the N-video batch dialog is explicitly out of scope for v1.
- handoff: **spec, ADR and design are all green.** Remaining gates are testing-strategy and
  security (likely N/A — no new auth surface, but the two new owner-gated routes want a look).
  Implementation is unblocked; start with the per-video status repo query.

### 2026-09-06 · testing strategy landed
- skills: testing-strategy
- Added a §4 backend row, a §5 frontend row, a §10 adversarial block, three Critical invariants
  and a §11 gap entry. The §4 row explicitly scopes itself as superseding the *SPA-poll half* of
  the ADR-073 row (D4), leaving D1/D2/D3's existing coverage in place — and reframes D1's
  post-write read-back test as a load-bearing regression guard, since this design depends on it.
- Two coverage traps named rather than discovered later. First, `pollUntilSettled`'s six existing
  tests stay green through this change and therefore prove nothing about it — the real assertion
  is that `WritebackFormDialog` no longer calls `waitForWritebackJob` at all, which needs a
  call-count-zero guard, because the plausible wrong implementation leaves the poll in and just
  unlocks the close button. Second, **nothing currently pins `FinishWriteback`'s delete-on-success**,
  which is the single behaviour "silence means success" rests on.
- Wrote the invariants so the tempting wrong test is called out in each: asserting out-of-sync
  *disappears* while pending pins the bug rather than the behaviour, and asserting auto-retry
  would assert a behaviour that does not exist (nothing moves `failed` → `pending` today).
- handoff: **four of five gates green.** Only `/security-review` remains, and it is not a
  formality — Retry and Dismiss are two new owner-gated mutation routes and Dismiss deletes a
  row, so it wants real scrutiny rather than inheriting "the writeback route was already gated".

### 2026-09-06 · security review — one MEDIUM finding, closed before code
- skills: security-review
- The diff is docs-only, so there were **no in-diff findings**. The value was in threat-modelling
  the surface the spec *specifies*, which turned up a real one that would otherwise have been
  written straight into the implementation.
- **Finding (MEDIUM, information disclosure):** R2.1 said the media payload carries "the failed
  job's error message". That error is `err.Error()` off `writeback.WriteBatch`, and **every**
  failure path in `internal/writeback/writeback.go` embeds absolute paths — `writeback copy: %w`
  and `writeback rename: %w` wrap `os` errors carrying both paths, and the
  exiftool/mkvpropedit/ffmpeg branches append raw tool stderr containing the `.holodex-tmp` path.
  `GET /media/{id}` is **not** owner-gated, and `redactFileMetadataForVisitor`
  (`internal/api/handlers.go:651`) already strips `FilePath`/codecs/container from exactly that
  payload — its doc comment states outright that the SPA gating its own display is insufficient.
  Attaching the raw error would have reintroduced path disclosure through a *new* field and
  defeated that control: the same shape as the `get_video` MCP leak fixed in HOLODEX-114.
- Closed in the spec as **R2.1a** rather than left as a review note: the error is omitted
  server-side for non-owners, redacted inside `redactFileMetadataForVisitor` (not a second
  parallel site a future serializer could miss), and the owner-facing message should be a stable
  summary with raw tool output confined to `job_runs`/Activity.
- Acceptance criteria 9 and 10 added, and a Critical invariant in the testing strategy — with the
  assertion pinned **at the API**, because a test that only checks the badge doesn't render passes
  against a payload that still carries the path.
- Retry/Dismiss route scoping was reviewed and **not** raised as a finding: both are owner-gated
  in a single-owner app, so a job-id-scoped Retry is a correctness/robustness concern, not an
  exploitable one. It is on the implementation checklist, not the findings list.
- handoff: **all five gates green.** Re-run `/security-review` against the real diff before
  marking the PR ready — this pass reviewed a design, not code.

### 2026-09-06 · implementation landed, verified live against real media
- Backend: `GetVideoWritebackStatus`/`RetryFailedWriteback`/`DismissFailedWriteback` on the repo,
  `Queue.RetryFailed` wrapping the reset with the existing `kick()`, owner-gated
  `POST /media/{id}/writeback/retry` and `.../dismiss`, and `writebackMedia` clearing a video's own
  prior failed row before enqueuing (RD5). `getMedia` carries `writeback_status`, with the R2.1a
  redaction applied inline — omitted for `!authorized`, present for the owner — rather than a
  second parallel redaction site.
- Frontend: `WritebackFormDialog` closes on the 202 ack (`onenqueued`, no more `jobStatus`/
  `waitForWritebackJob`), shows an inline `enqueueError` on a failed enqueue instead, and the
  gutter is now three mutually-exclusive glyphs (checkbox/`=`/circle-minus) with a tier-sorted row
  order inside the existing decided/undecided groups. `writebackJob.ts` gained
  `waitForVideoWriteback`, reusing the same `pollUntilSettled` backoff rather than a new loop. The
  media detail page polls via `api.getMedia` (extracting only `writeback_status` per tick, applying
  the full detail once on settlement — avoids reassigning `resolved`/`video` on every tick), guarded
  by a `writebackGeneration` counter so a poll from `/media/A` can never resolve into `/media/B`
  after in-place navigation (same idiom as `extractGeneration`). The film page's per-scene
  `WritebackFormDialog` instantiation was updated for prop-contract parity, since it shares the
  component — no badges added there, out of this spec's scope.
- Added Go tests (`TestGetVideoWritebackStatus`, `TestRetryFailedWriteback`,
  `TestDismissFailedWriteback`, `TestGetMedia_WritebackStatusRedactedForVisitor`,
  `TestRetryDismissWriteback_OwnerGated`, `TestRetryWriteback_ResetsFailedToPending`,
  `TestDismissWriteback_DeletesRow`, `TestEnqueueWriteback_ClearsPriorFailedForVideo`) and a Vitest
  suite for `waitForVideoWriteback` — matching the names/behaviors the testing-strategy doc
  committed to before any code existed. Full `go test ./...` and `npm run test`/`npm run check`
  green throughout.
- **Live-verified against `backend-amv`'s real library**, not just unit tests: authenticated as
  owner, forced a genuine out-of-sync field via a manual title decision, submitted a write — the
  dialog closed immediately, the page showed `writing to file` beside `out of sync` (RD6 confirmed:
  neither hides the other), and both cleared within 3s once exiftool actually wrote the real MP4's
  `QuickTime:Title` tag. Forced a real failure by flipping the file read-only: the header showed
  `couldn't write` + `out of sync` + Retry/Dismiss, with the raw path-bearing exiftool error visible
  to the owner. **Confirmed via a cookie-less `curl` request** that the same endpoint, unauthenticated,
  returns `{"pending":false,"failed":true}` with no `error` key at all — the R2.1a fix holds under a
  real failure, not just the synthetic one in the Go test. Retry re-enqueued and reprocessed
  correctly (hit an unrelated exiftool-internal temp-file leftover from the synthetic read-only
  test, not a real bug); Dismiss cleared the failure while correctly leaving the still-genuine
  out-of-sync badge alone.
- **Found and spun off a pre-existing gap, not introduced here**: a computed-contrast pass (the
  theming rule's own sanctioned method, no screenshots) across all three skins found
  `--warn-ink` on `--warn` in Cinémathèque measures 3.18:1 — below AA for small text. Broadcast
  (7.36:1) and Brutalist (6.52:1) pass. This exact pairing already ships in
  `ConfirmDialog.svelte`'s destructive button and `EntityImageSlot.svelte`'s badge, so it predates
  this feature; flagged as a separate follow-up task rather than touched here.
- handoff: **implementation complete, all runtime and static checks green, live-verified against
  real media for both the success and failure paths.** Only HOLODEX-276's re-evaluation remains as
  a deliberate followup. Next action is marking PR #303 ready for review (dropping Draft).
