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

**Design package:** [ADR-091](../architecture/ADR-091-fire-and-forget-writeback-status.md) · [handoff](../design/fire-and-forget-writeback-handoff.md) · [mockup](../design/fire-and-forget-writeback-mockup.svg)

## Gates — definition of done

- [ ] spec `write-spec` → `docs/specs/` (not yet written)
- [x] architecture `architecture` → `docs/architecture/ADR-091-fire-and-forget-writeback-status.md` (Proposed; supersedes ADR-073 **D4 only**)
- [x] design `design-handoff` → `docs/design/fire-and-forget-writeback-handoff.md` + committed SVG mockup
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy` → `docs/testing-strategy.md`
- [ ] security `security-review` — likely N/A (no new auth surface; the writeback route is already owner-gated), confirm against the real diff before merge

## Up next — ordered (position = priority)

1. [ ] [spec] write the behaviour spec — the gate that is still red; ADR-091 records the decision, not the acceptance detail
2. [ ] [backend] per-video queue status repo query (`writeback_queue` by `video_id`, pending + failed + carried fields) — `internal/repo/writequeue.go`
3. [ ] [backend] surface it on `GET /media/{id}` — `internal/api/handlers.go`
4. [ ] [backend] `failed` → `pending` reset + `kick()` for Retry (nothing resets `failed` today; `RecoverRunningWritebacks` only handles `running`) — `internal/repo/writequeue.go`, `internal/writequeue/writequeue.go`
5. [ ] [backend] owner-gated Retry / Dismiss routes
6. [ ] [frontend] close `WritebackFormDialog` on the 202 ack; keep it open on a failed enqueue — `web/src/lib/components/writeback/WritebackFormDialog.svelte`
7. [ ] [frontend] delete the transient status gutter (`isWriting` / `isError`), collapse `row.status`; row content unchanged
8. [ ] [frontend] pending + failed chips in the Metadata header, reusing `pollUntilSettled` from `$lib/writebackJob.ts` — `web/src/routes/media/[id]/+page.svelte`
9. [ ] [frontend] three-skin QA per `.claude/rules/frontend-theming.md`
10. [ ] [followup] re-evaluate HOLODEX-276 — per-field written/skipped is not a distinction an atomic job can honestly report; ADR-091 argues that ticket may no longer be wanted

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
