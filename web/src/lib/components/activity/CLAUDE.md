# Activity components

Background-job visibility (F21/F35, ADR-071): the header indicator, the Owner Status tab's
digest + history views, and their shared status badge.

| File | Purpose |
|---|---|
| `ActivityIndicator.svelte` | Compact header pill, shown only while background work is active; links to the Status tab. |
| `JobDigest.svelte` | Default job-history view: per-kind digest answering "still running? failed recently?" without loading every run; owner Dismiss / Dismiss all on the failures callout (HOLODEX-416). |
| `JobHistory.svelte` | 30-day job history log (scans + enrich runs), newest first; includes writeback Revert and the `?batch=` sweep filter chip (F66 P0-7). |
| `JobStatusBadge.svelte` | The one error/ok badge for a job run's status, shared by the digest and the full history log; `dismissed` mutes it and adds the `· dismissed` marker (HOLODEX-416 D5). |
| `dismissDigest.ts` | Pure local mutation of a `JobDigest` after a row / dismiss-all (HOLODEX-416): row leaves, kind's `errors` decrements, `last_dismissed` flips when the newest run was the one dismissed. Unit-tested. |
| `StatusCard.svelte` | Generic labelled surface panel; callers supply the body. |
| `SweepStatusLine.svelte` | Entity refresh sweep line for `/people` and `/studios` (F66 RD10, ADR-103): running counts or the last done line for its own kind only, page-local Dismiss, fires `onfinished` once on the running→idle edge. |
| `sweepLine.ts` | Pure pieces of `SweepStatusLine`: done-line derivation (`doneLineFor`), the once-only running→idle edge (`sweepFinished`), skipped-provider text grouped by reason. Unit-tested. |
