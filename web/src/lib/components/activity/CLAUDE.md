# Activity components

Background-job visibility (F21/F35, ADR-071): the header indicator, the Owner Status tab's
digest + history views, and their shared status badge.

| File | Purpose |
|---|---|
| `ActivityIndicator.svelte` | Compact header pill, shown only while background work is active; links to the Status tab. |
| `JobDigest.svelte` | Default job-history view: per-kind digest answering "still running? failed recently?" without loading every run. |
| `JobHistory.svelte` | 30-day job history log (scans + enrich runs), newest first; includes writeback Revert. |
| `JobStatusBadge.svelte` | The one error/ok badge for a job run's status, shared by the digest and the full history log. |
| `StatusCard.svelte` | Generic labelled surface panel; callers supply the body. |
