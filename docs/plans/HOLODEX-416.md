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

- [ ] spec `write-spec` — resolve Q4 in `docs/specs/job-history-digest-and-search.md`: two
  owner endpoints, digest excludes dismissed runs, history carries `dismissed_at`
- [ ] architecture `architecture` — ADR: `job_run_dismissals (job_run_id PK, dismissed_at)`
  keeps `job_runs` immutable (ADR-091 posture); retention sweep cascades
- [x] design `design-handoff` — `docs/design/status-dismiss-failures-handoff.md` +
  `status-dismiss-failures-mockup.svg`; D1–D4 locked 2026-09-18 from the inline mockup
- [ ] backend — migration → `repo.DismissJobRun` / `DismissJobFailures` → `POST
  /admin/activity/runs/{id}/dismiss` + `/admin/activity/failures/dismiss` → digest LEFT JOIN
- [ ] frontend — `api.ts` two calls; `JobDigest.svelte` row `btn-row btn-ghost` Dismiss +
  header `btn-quiet` Dismiss all N, owner-gated; `JobHistory.svelte` `· dismissed` marker
- [ ] testing `testing-strategy` — repo digest-exclusion test, handler owner gate, double
  dismiss no-op, component test that a row leaves and the kind's errors decrements
- [~] security `security-review` — n/a: both endpoints sit under the existing `requireOwner`
  group; no new input beyond a path id and the digest's `days`

## Up next — ordered (position = priority)

1. [ ] [—] Spec edit (Q4 → resolved decision) + ADR via `node scripts/adr-claims.mjs`
2. [ ] [—] Backend, then frontend, then three-skin QA on the Cinémathèque + two other skins
3. [ ] [—] Mark PR ready once every gate above is `[x]` — that fires In Review

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · design handoff
- skills: design-handoff
- Grounded in `JobDigest.svelte` + `app.css` button roles + ADR-091's "job_runs is the audit
  record". Four questions put to Kevin with an inline mockup; all four recommendations taken.
  Filed HOLODEX-416, renamed the branch, In Progress fired, Draft PR opened.
- handoff: design gate green; next session starts at the spec Q4 edit and the ADR.
