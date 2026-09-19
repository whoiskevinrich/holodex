---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-416
status: in-review
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
- [x] frontend — `api.ts` two calls; `JobDigest.svelte` row `btn-row btn-ghost` Dismiss +
  header `btn-quiet` Dismiss all N, owner-gated, local mutation via `dismissDigest.ts` +
  `onchange`; `JobStatusBadge` `dismissed` prop (muted badge + `· dismissed`, both tabs);
  `JobHistory.svelte` marker; focus parks on next row / `h2` (2026-09-18)
- [x] testing `testing-strategy` — Go: `jobrun_dismissals_test.go` (digest exclusion, D5 both
  columns, fresh-failure-after-dismiss, window-at-request-time, FK cascade) +
  `activity_dismiss_test.go` (owner gate, idempotent row/all). Web: `dismissDigest.test.ts`
  (row leaves, kind's errors decrements, D5 flips only for the newest run, dismiss-all) — the
  component-harness gap (HOLODEX-395) means the mutation is a pure module, tested there.
  Strategy row added
- [~] security `security-review` — n/a: both endpoints sit under the existing `requireOwner`
  group; no new input beyond a path id and the digest's `days`

## Up next — ordered (position = priority)

1. [ ] [—] On merge: HOLODEX-412 (#350) must renumber its `0048_entity_completeness` → `0049`
   before it lands (golang-migrate never applies a version below current) — leave a note on #350
2. [ ] [—] Once merged: memory + this worklog to done; release note stands
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

### 2026-09-18 · frontend
- skills: code-review high --fix (no findings)
- `dismissDigest.ts` holds the local mutation as pure functions (no component harness —
  HOLODEX-395) with 6 unit tests; `JobDigest.svelte` reports the next digest up via `onchange`
  rather than owning a copy, so the page's `loadDigest()` refresh path is unchanged.
  `JobStatusBadge` gained `dismissed` (one prop, both tabs). Verified live on a seeded session
  DB (ports 7811/5183, `data/416`): row dismiss → row leaves, count drops, badge mutes
  `error · dismissed`, focus lands on the next row; Dismiss all → callout unmounts, focus on
  `h2`; reload agrees with the server; Log shows the marker on every dismissed run.
  Three skins + 375px checked — muted badge resolves to each skin's `--rule`/`--muted`, no
  horizontal overflow. One as-built note recorded in the handoff: at 375px the Dismiss button
  holds a stable right column rather than trailing the detail's last line.
- handoff: every gate green; next = Kevin's prod-skin look, then migration-number re-check,
  merge main, mark ready.

### 2026-09-18 · ready
- skills: —
- Main still tops out at `0047` (HOLODEX-412 #350 is an open Draft), so `0048` stays ours and
  412 renumbers on its side. Merged `origin/main` clean (`e5130cd`); Go + web checks and tests
  green on the merged tree. Marked PR #354 ready → In Review.
- handoff: in review; nothing to do until merge, then sweep the worklog to done.
