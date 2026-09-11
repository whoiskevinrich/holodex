---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-370
status: in-progress
release_note: Clearing a provider's data — or answering "None of these match" — now tells you when that provider's values were also written into the file, and points at the write's Revert; Revert asks first and says what it costs.
---

# HOLODEX-370 · Clear / Dismiss never revert a written-back provider — say so, point at Revert

Filed out of the HOLODEX-367 brainstorm on the provider team's model that ADR-066 auto-apply
writes to the file with no owner present, so a wrong match would poison the file-layer baseline
and every later resolve would search for it.

**The premise was wrong, and the ticket was corrected before code.** Auto-apply
(`refreshOneProvider`, `enrichVideoApply`) writes to the `entity_enrichment` shadow store only.
No field-decision path enqueues a file write. The *only* route from provider data to file tags is
the owner clicking "Write decisions to file". So there is no unattended loop, 370 does not block
367 (relink: relates), and priority dropped to Medium.

**What is real is narrower.** Owner writes back provider X's values → later Clears X. The DB rows
go, but re-extract has already made X's values the file-layer baseline (`file:title` *is*
`videos.title`), so the page keeps showing X's title. The second click silently doesn't do what it
says.

**Why not the ticket's original fix (link dismissal to the snapshot revert).** Two facts:
`file_writebacks` has per-field `source` but no `batch_id`, snapshots have `batch_id` but no
source — a real link needs a migration and can't be backfilled. And a write batch is one
"Write decisions to file" click carrying fields from *several* sources; `Revert(batchID)` restores
every field in it. A per-provider revert needs a **per-field** snapshot revert — a new primitive,
not a linkage. Offering "revert N file writes" inline on Clear would undo unrelated fields. Chosen
instead: **detect and point.** The audit `source` is the resolver's winning-source token
(`tmdb:title`), so an exact `<provider>:` prefix match on `file_writebacks` is a sound
attribution for *detection*; the response carries `written_back`, the page shows a notice
pointing at the existing Revert, and Revert gains the confirmation the provider team asked for
(later owner edits are lost too).

## Gates — definition of done

- [x] spec `write-spec` — `enrichment-review-workflow.md` API block (dismiss → `200 {written_back}`,
  media clear → `200 {written_back}`, the detection-only rationale), `metadata-extraction.md`
  F48.9e (Revert confirms, names the cost), `metadata-plugins.md` F22.7b acceptance line
- [~] architecture `architecture` — n/a: no seam touched; no migration; the decision *not* to
  build per-provider revert is recorded on the ticket and in the spec, with the prerequisite named
  (per-field snapshot revert + `batch_id` on `file_writebacks`) should it ever matter
- [~] design `design-handoff` — n/a: one inline notice in the existing chip-row outcome slot
  (`text-xs text-warn`, accent link, `aria-live`) and the shared `ConfirmDialog` on an existing
  button; no new pattern, no new component
- [x] backend — `repo.HasWritebackFromProvider` (EXISTS over `file_writebacks`, `substr` prefix —
  not LIKE, so a provider name is never a pattern); `Handlers.providerWrittenBack` (video only,
  lookup failure logged not surfaced); `enrichVideoClear` → 200 JSON; `enrichDismissalAction`
  answers 200 JSON for `dismiss`, 204 for `undismiss` unchanged
- [x] frontend — `api.ts` types the flag on `enrichVideoClear` / `enrichDismiss`; media page
  `writtenBackProvider` set from Clear and from the picker's `dismiss` wrapper, reset on item
  change, rendered beside `enrichError`; `JobHistory` Revert → `ConfirmDialog` (Cancel focused)
  → `revert()`. Verified live on `backend-films` video 2 (seeded `tmdb` rows + a `tmdb:title`
  audit row): Clear drops the enrichment disclosure and shows the notice; contrast 5.6 / 7.3 /
  6.5:1 text and 9.2 / 12.1 / 17.2:1 link across Cinémathèque / Broadcast / Brutalist at 1280px,
  no horizontal overflow; Revert dialog opens in all three skins, Cancel closes, Confirm dispatches
  (404 on the seeded batch — the existing no-snapshots path)
- [x] testing `testing-strategy` — `TestVideoClearAndDismissReportWrittenBack` (no rows → false;
  another provider's row, a same-prefix lookalike `fakery:`, `manual`, `revert` → false; one
  `fake:` row → true on both Clear and Dismiss; undismiss stays 204); person/film dismiss tests
  updated to `200 {written_back:false}`; `go test ./internal/api ./internal/repo` green,
  `npm run test` 272 / `npm run check` 0 errors; strategy doc row added
- [~] security `security-review` — n/a: no auth, access or infrastructure change; both handlers
  stay `requireOwner`; the new field discloses only "a writeback row exists" to the owner who
  wrote it

## Up next — ordered (position = priority)

1. [ ] [—] **Remove the stale `blocks` link** HOLODEX-370 → HOLODEX-367 in Jira by hand (the MCP
   has no delete-link tool); the `relates` link is already in place.
2. [ ] [—] Mark the PR ready when the owner has eyeballed the notice; CI moves this to In Review.
3. [ ] [M] If per-provider revert is ever wanted: file *per-field snapshot revert + `batch_id` on
   `file_writebacks`* — the prerequisite this issue deliberately did not build.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-11 · premise corrected, option A built and verified
- skills: product-brainstorming (the 367 thread that spawned this), code-review (high --fix), code-review
- Explored before coding and found auto-apply never writes to the file — corrected the ticket,
  downgraded blocks → relates, offered three scopes with tradeoffs; owner chose detect-and-point.
  Built backend + frontend + specs + tests in one pass; QA'd three skins live.
- Handoff: PR open; the only manual step left is deleting the stale `blocks` link in Jira.
