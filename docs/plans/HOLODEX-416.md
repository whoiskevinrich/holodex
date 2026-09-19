---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-416
status: in-progress
release_note: The status page's Recent failures callout now has a Dismiss on each row and a Dismiss all in its header, so failures the owner has already handled stop crowding out new ones — the Log tab keeps every run as the audit record.
---

# HOLODEX-416 · Owner can dismiss addressed failures from the status page's Recent failures callout

`/owner/status` → Recent jobs → Summary lists every error run of the last 30 days and nothing
clears one once it is handled, so the callout and the per-kind Errors column stay warn-coloured
and a new failure hides among old ones. Spec `job-history-digest-and-search.md` Q4 deferred this.

## Gates — definition of done

- [x] spec `write-spec` — Q4 resolved + P0-7 in `docs/specs/job-history-digest-and-search.md`
  (2026-09-18): two owner endpoints, digest excludes dismissed runs + `kinds[].last_dismissed`,
  history carries `dismissed_at`; D5 added to the handoff + SVG panel E
- [x] architecture `architecture` — [ADR-100](../architecture/ADR-100-job-run-dismissals.md)
  (2026-09-18): `job_run_dismissals (job_run_id PK REFERENCES job_runs ON DELETE CASCADE,
  dismissed_at)`; sweep cascades via FK, not code; digest `LEFT JOIN` + bare-column `last_dismissed`
- [x] design `design-handoff` — `docs/design/status-dismiss-failures-handoff.md` +
  `status-dismiss-failures-mockup.svg`; D1–D4 locked 2026-09-18 from the inline mockup
- [x] backend — `0048_job_run_dismissals` → `repo.DismissJobRun` / `DismissJobFailures` → `POST
  /admin/activity/runs/{id}/dismiss` + `/admin/activity/failures/dismiss` → digest/history
  `LEFT JOIN` + `last_dismissed` (2026-09-18). **Migration number: `0048` collides with
  HOLODEX-412's in-flight `0048_entity_completeness` — whoever merges second renumbers**
- [ ] frontend — `api.ts` two calls; `JobDigest.svelte` row `btn-row btn-ghost` Dismiss +
  header `btn-quiet` Dismiss all N, owner-gated; `JobStatusBadge` `muted` prop + `· dismissed`
  on a kind whose newest run is dismissed (D5); `JobHistory.svelte` `· dismissed` marker
- [ ] testing `testing-strategy` — Go half DONE: `jobrun_dismissals_test.go` (digest exclusion,
  D5 both columns, fresh-failure-after-dismiss, window-at-request-time, FK cascade) +
  `activity_dismiss_test.go` (owner gate, idempotent row/all); testing-strategy row added.
  Open: component test that a row leaves and the kind's errors decrements
- [~] security `security-review` — n/a: both endpoints sit under the existing `requireOwner`
  group; no new input beyond a path id and the digest's `days`

## Up next — ordered (position = priority)

1. [ ] [—] Frontend: `types.ts` (`dismissed_at?`, `last_dismissed`), `api.ts` two calls,
   `JobDigest.svelte` row/header controls + local mutation, `JobStatusBadge` `muted` prop,
   `JobHistory.svelte` marker; component test; three-skin QA
2. [ ] [—] Before marking ready: re-check migration number against main (HOLODEX-412 also
   holds `0048`) and merge main
3. [ ] [—] Mark PR ready once every gate above is `[x]` — that fires In Review

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · design handoff
- skills: design-handoff, code-review
- Grounded in `JobDigest.svelte` + `app.css` button roles + ADR-091's "job_runs is the audit
  record". Four questions put to Kevin with an inline mockup; all four recommendations taken.
  Filed HOLODEX-416, renamed the branch, In Progress fired, Draft PR opened.
- handoff: design gate green; next session starts at the spec Q4 edit and the ADR.

### 2026-09-18 · spec
- skills: write-spec
- Q4 answered "no window — a dismiss"; P0-7 added with acceptance criteria from D1–D4 and the
  handoff's API contract; `job_run_dismissals` + two POSTs in the Data & API section. Surfaced
  the one case D1–D4 missed — a kind whose *newest* run is the dismissed error still read
  `last_status: error` — as three mocked options; Kevin chose **D5** (muted badge +
  `· dismissed`, `kinds[].last_dismissed`). Recorded in handoff table + SVG panel E.
  ADR-100 reserved.
- handoff: spec gate green; next session writes ADR-100 (sweep-cascade mechanism is its one
  open choice), then backend.

### 2026-09-18 · ADR
- skills: architecture
- ADR-100 written + indexed. Cascade decided **by FK** (`foreign_keys(ON)` is on the DSN and
  0035–0037 already rely on it; 0024's triggers were forced by a polymorphic parent). Four
  options weighed; per-kind watermark rejected on D1 but noted as a future snooze shape. D3's
  `last_dismissed` rides the same bare-column-with-`MAX` rule as `last_status` — the test must
  pin both.
- handoff: all pre-implementation gates green; next session starts the backend at the migration
  (take the number at merge time).

### 2026-09-18 · backend
- skills: code-review high --fix (no findings)
- Migration `0048` + `job_run_dismissals`; `jobRunSelect` (columns + `LEFT JOIN`) replaces the
  bare column list in both run reads; digest kinds query gains `last_dismissed` as a bare-column
  expression off the `MAX(started_at)` row — verified by test in both directions, not assumed.
  Two repo methods, two owner routes. 3 repo tests + 1 HTTP test, `go test ./...` green.
- handoff: backend green; next session is the frontend (`JobDigest.svelte` first, then the
  badge `muted` prop, then `JobHistory.svelte`).
