---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-323
status: in-progress
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
- [ ] backend
- [ ] frontend
- [x] testing `testing-strategy` → `docs/testing-strategy.md` (§4 row, §5 row, §10 adversarial block, 3 Critical invariants, §11 gap)
- [ ] security `security-review` — likely N/A (no new auth surface; the writeback route is already owner-gated), confirm against the real diff before merge

## Up next — ordered (position = priority)

1. [ ] [backend] per-video queue status repo query (`writeback_queue` by `video_id`, pending + failed + carried fields) — `internal/repo/writequeue.go`
2. [ ] [backend] surface it on `GET /media/{id}` — `internal/api/handlers.go`
3. [ ] [backend] `failed` → `pending` reset + `kick()` for Retry (nothing resets `failed` today; `RecoverRunningWritebacks` only handles `running`) — `internal/repo/writequeue.go`, `internal/writequeue/writequeue.go`
4. [ ] [backend] owner-gated Retry / Dismiss routes
5. [ ] [frontend] close `WritebackFormDialog` on the 202 ack; keep it open on a failed enqueue — `web/src/lib/components/writeback/WritebackFormDialog.svelte`
6. [ ] [frontend] delete the transient status gutter (`isWriting` / `isError`), collapse `row.status`; row content unchanged
7. [ ] [frontend] pending + failed chips in the Metadata header, reusing `pollUntilSettled` from `$lib/writebackJob.ts` — `web/src/routes/media/[id]/+page.svelte`
8. [ ] [frontend] three-skin QA per `.claude/rules/frontend-theming.md`
9. [ ] [followup] re-evaluate HOLODEX-276 — per-field written/skipped is not a distinction an atomic job can honestly report; ADR-091 argues that ticket may no longer be wanted

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
