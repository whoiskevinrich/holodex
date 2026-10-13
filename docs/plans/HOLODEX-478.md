---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-478
status: in-review
profile: ui                  # frontend bug fix; the Back exit's snapshot restore, no UX change
depends-on: []
release_note: After moving a video to Trash, going back to the Media grid no longer shows its card, and your scroll position is kept.
---

# HOLODEX-478 · Trashed video lingers in the Media grid after the Back exit

A successful delete on the media page now drops that video from the pending `browseCache` snapshot
(`forgetVideo`, `web/src/lib/browse.svelte.ts`) before the ADR-114 D4 exit. The restored grid keeps
its scroll, `total` drops by one, and the paging offset shifts back one so the next "Load more" page
skips nothing.

**Design package:** none (restores the HOLODEX-41 back-exit as intended)

## Gates — definition of done

- [~] design `design-handoff` → `docs/design/**` — no UX change; the card simply isn't restored
- [x] frontend → `web/src/**`
- [x] testing `testing-strategy` — `web/src/lib/browse.test.ts` pins drop/offset/no-op; the media-page call site has no component harness, browser-verified on `backend-amv` (trash → Back: card gone, scroll kept, count 200; Load more matched the server order with no gap or duplicate; video restored after)

## Up next — ordered (position = priority)

1. [ ] [—] On merge, confirm CI moved HOLODEX-478 to Done (Bug on its own keyed branch — jira-sync handles it)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-13 · drop a trashed video from the pending grid snapshot
- skills: code-review
- handoff: Fixed and browser-verified (`forgetVideo` in `browse.svelte.ts`, called from the media page's `confirmDelete`); the PR is open and ready — next: confirm CI moves the issue to Done on merge.

## Dropped — newest first (the reason is the point)
