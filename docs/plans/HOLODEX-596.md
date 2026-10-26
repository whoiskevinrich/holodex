---
key: HOLODEX-596
status: in-progress
profile: full
release_note: "Regenerating a video's thumbnail is now owner-only. Visitors no longer see the player's Regenerate button."
---

# HOLODEX-596 · Thumbnail regenerate is not owner-gated

Done when `POST /api/v1/media/{id}/thumbnail` sits under `requireOwner`, a visitor gets 401 with no
state reset or enqueue, the owner still gets 200/202, and no visitor UI calls it. Settles
HOLODEX-592's question with its first option: gate the endpoint and drop the visitor button.

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — phase-2-mcp-polish F11.6 row: regenerate is owner-only, serving stays public
- [~] architecture `architecture` → `docs/architecture/**` — no technology fork (one route moves into the existing owner group)
- [~] design `design-handoff` → `docs/design/**` — no new surface; removes the visitor's hover-only Regenerate button (HOLODEX-592 option 1). The owner's control in the Metadata sources popover is unchanged
- [x] backend → `{cmd,internal,providers}/**` — route moved into the `requireOwner` group in `internal/api/handlers.go`
- [x] frontend → `web/src/**` — visitor Regenerate button removed from `routes/media/[id]/+page.svelte`
- [x] testing `testing-strategy` — `TestRegenerateThumbnailOwnerGated`; testing-strategy Phase 2 row updated
- [x] security `security-review` — clean sign-off, no findings. chi keeps the public GET and the gated POST as separate endpoints on one node, with no method fallback. No other public path resets state or high-enqueues

## Up next — ordered (position = priority)

1. [ ] [—] Sweep HOLODEX-596 to In Review when the PR is marked ready, Done on merge — Jira
2. [ ] [—] Close HOLODEX-592 as decided by this PR once it merges — Jira

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-25 · owner-gate thumbnail regenerate
- skills: code-review, security-review
- handoff: Route gated, test green, visitor button dropped, security sign-off clean. Next step: owner review of the PR, including the HOLODEX-592 call to drop the visitor button.

## Dropped — newest first (the reason is the point)
