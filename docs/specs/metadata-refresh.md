# Spec: Refresh Metadata — per-item re-extract + re-enrich (F31)

**Status**: Draft
**Phase**: Post–Phase 3 (enrichment follow-up; standalone owner-facing feature)
**Owner**: Project owner
**Date**: 2026-06-28
**Feature block**: **F31** — Refresh Metadata. Completes the deferred **F22 "re-enrich UI"**
follow-up and pairs it with a new **forced file re-extract** into one owner action.

**Depends on**: Only shipped surfaces —
- the metadata extraction pipeline (exiftool + ffprobe; [ADR-004](../architecture/ADR-004-metadata-extraction.md), `internal/metadata`)
- the scanner per-file index path & change-detection ([ADR-018](../architecture/ADR-018-scanner-change-detection.md), `internal/scanner`)
- the enrichment shadow store + per-item provider apply ([F22](metadata-plugins.md) / [ADR-033](../architecture/ADR-033-metadata-source-plugins.md), `internal/enrich`, `POST /media/{id}/enrich`)
- the unified field resolver with `file:`/`{provider}:` provenance (F27, `internal/resolver`)
- the owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner` / `X-Admin-Token`)
- the activity surface & job history ([F21](system-activity.md) / [ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md))

**New ADR required**: **Yes** — one ADR for the **per-item refresh orchestration**: a forced
(change-detection-bypassing) single-file re-extract seam in the scanner, the per-item
"re-run all linked providers" loop reusing persisted matches, and how a per-item operation is
recorded in the library-wide `job_runs` model. **[ADR-047](../architecture/ADR-047-per-item-metadata-refresh.md)**. Touches **access + file
I/O + subprocess** → a `/security-review` sign-off is required before merge.

---

## Problem Statement

Holodex indexes each media file once and then only re-reads it when the periodic scan notices
its **size or mtime** changed ([ADR-018](../architecture/ADR-018-scanner-change-detection.md)).
But the owner routinely edits the same files from **other systems** — a desktop tagger, a
different media manager, a script — and two gaps follow. **(1)** Some taggers rewrite tags
**in place without bumping mtime**, so Holodex's change-detection never re-reads them and the
library shows stale title/tags/people indefinitely. **(2)** Even when mtime *does* change, the
owner has no way to say "pick up this one file's changes **now**" short of waiting for (or
forcing) a whole-library rescan. Separately, provider enrichment (TMDB, etc.) is fetched once
and goes stale upstream with **no re-fetch button** at all (the [F22](metadata-plugins.md)
re-enrich UI was deferred). The cost is a library that silently drifts out of sync with the
files it mirrors, with no owner-visible remedy.

## Goals

1. **Give the owner a one-click "Refresh metadata" on any media item** that brings Holodex back
   in sync with the file as it exists *right now* — re-reading the file **and** re-pulling its
   linked providers in a single action.
2. **Catch in-place edits that don't change mtime** — the refresh **forces** re-extraction,
   bypassing the `(size, mtime)` fast-path, so changes another system made are always captured.
3. **Close the deferred F22 re-enrich gap** — refreshing also re-fetches each provider the item
   is already matched to, using the **persisted match** (no re-prompting for identity), so
   upstream provider changes flow in too.
4. **Keep provenance correct and lossless** — after a refresh the resolved view still shows
   per-field "from file" vs "from TMDB" badges, and a refresh never silently destroys the other
   layer (re-extract leaves the enrichment store intact; re-enrich leaves file fields intact).
5. **Make the action legible and safe** — owner-gated, visible only when admin features are,
   honoring soft-delete, recorded in activity history, and themed across all three skins.

## Non-Goals

- **Bulk / library-wide forced re-extract.** v1 is **single-item only**. A whole-library pass is
  already served by the F13.3 admin rescan; a *forced* (ignore-change-detection) full re-read is
  a separate, heavier feature. *(Why: a forced full re-read can re-`exiftool`/`ffprobe` thousands
  of files and hammer disk; scope it on its own merits later — see Future Considerations.)*
- **Automatic / scheduled staleness refresh.** No crawler, no "re-enrich everything nightly,"
  no upstream-change polling. Refresh is **always an explicit owner click**, preserving the
  on-demand ethos of [ADR-033](../architecture/ADR-033-metadata-source-plugins.md). *(Why: the
  whole enrichment design is deliberately pull-only; a scheduler is a different posture.)*
- **A new enrichment identity flow.** Refresh re-fetches only providers the item is **already
  matched to**. If an item has no persisted provider match, the provider step is a no-op — it
  does **not** open the match picker. Establishing a *new* match stays the existing Enrich flow.
  *(Why: refresh means "redo what's linked," not "go find new links.")*
- **Writing anything back to the file.** Refresh **reads** the file (and providers); it never
  writes. Pushing values *into* the file is the existing F28 writeback. *(Why: read vs. write are
  separate, separately-gated operations; conflating them is surprising and risky.)*
- **Reactivating or re-reading soft-deleted items.** A soft-deleted row
  ([ADR-037](../architecture/ADR-037-soft-delete-and-purge.md)) is untouchable; refresh is
  unavailable for it and the endpoint refuses it. *(Why: honors the #26 reactivation guard — a
  deleted item must never be resurrected by any path.)*
- **Thumbnail regeneration.** Refresh updates *metadata* (incl. cover-art detection). Rebuilding
  the thumbnail image stays the existing per-video "Regenerate thumbnail" action. *(Why: already
  shipped and independently triggerable; keep the two buttons single-purpose. They may be re-run
  back-to-back by the owner.)*

---

## User Stories

**Library owner (admin features visible)**
- As the **library owner**, when I've edited a file's tags in another app, I want to click
  "Refresh metadata" on that item in Holodex so that its title/tags/people/codecs update to match
  the file without my waiting for a scan.
- As the **library owner**, I want refresh to pick up edits **even when the file's modified-time
  didn't change** so that taggers that preserve mtime don't leave me with stale data.
- As the **library owner**, I want refresh to also re-pull the providers this item is matched to
  so that upstream changes (a corrected TMDB overview, a new headshot) come in at the same time.
- As the **library owner**, I want refresh to **reuse the existing provider match** so that I'm
  never re-asked "which TMDB record is this?" for an item I already confirmed.
- As the **library owner**, after a refresh I want to see clearly **what changed** (or that
  nothing changed) so that I trust the action did something and know the file is now in sync.
- As the **library owner**, I want the refresh recorded in **System Activity** so that I can see
  when an item was last re-read and whether it errored.

**Edge / boundary**
- As the **library owner**, if the file is **missing from disk**, I want refresh to fail with a
  clear message (and to **not** mark the item active or wipe its data) so that a temporarily
  unmounted volume doesn't corrupt my library.
- As the **library owner**, if an item has **no provider match**, I want refresh to still re-read
  the file and simply skip the provider step (no error, no picker) so that the file-only path
  always works.
- As the **library owner**, if a **provider is down or slow**, I want the file re-extract to still
  succeed and the provider failure to be reported per-provider (not crash the whole refresh) so
  that a flaky sidecar never blocks me from syncing the file.
- As a **non-owner** (admin features hidden), I should **not** see the refresh control at all, and
  the endpoint must reject me, so that re-reading files stays an owner-only operation.

---

## Requirements

### Must-Have (P0)

| ID | Requirement | Acceptance criteria |
|---|---|---|
| **F31.1** | **Owner-gated refresh endpoint.** A new owner route triggers a single-item refresh: `POST /api/v1/media/{id}/refresh` (behind `requireOwner`; per-item path mirrors `/media/{id}/enrich` and `/media/{id}/writeback`, **not** the `/admin/` library-wide group). | • Without a valid `X-Admin-Token` (when `ADMIN_TOKEN` is set) → **401/403**.<br>• Unknown `{id}` → **404**.<br>• Soft-deleted `{id}` → **409** (or **404**) with a body naming the reason; the row is **not** reactivated.<br>• Valid owner + live item → **202 Accepted** (async) and the work runs. |
| **F31.2** | **Forced file re-extract.** Refresh re-runs the exiftool + ffprobe extraction on the item's file **unconditionally**, bypassing the `(size, mtime)` change-detection fast-path, and updates the file-sourced layer (`videos` row + `video_metadata` extras + cover-art flag). | Given an item whose file's tags were changed **in place with mtime preserved**, When the owner refreshes, Then the new title/tags/people/codecs are stored and shown — proving the fast-path was bypassed. |
| **F31.3** | **Re-enrich linked providers.** Refresh re-fetches **every provider the item is currently matched to**, reusing the persisted external match (no identity prompt), and updates the enrichment shadow store. | • An item matched to TMDB: refresh re-applies TMDB and the enrichment fields update.<br>• An item with **no** match: provider step is a clean no-op (no picker, no error).<br>• The match record is **not** cleared or changed by a refresh. |
| **F31.4** | **Non-destructive layering (load-bearing invariant).** Re-extract updates **only** the `file:` layer; re-enrich updates **only** the `{provider}:` layer; refresh **never flattens** the two into a single stored value — the resolver remains the sole merge point and re-merges afterward with correct `file:` / `{provider}:` provenance. *This invariant is what keeps a future batch conflict-resolution policy (F31.11) implementable without re-extraction.* | After refresh, the media detail `resolved[]` reflects new values with the **same provenance semantics** as before (file-won badged "from file," provider-won "from <provider>"). No enrichment row is lost by the re-extract; no file field is lost by the re-enrich. **No code path writes a resolved/merged value back into `videos.*` or the enrichment store as the stored truth.** |
| **F31.5** | **Resilient, per-source error handling.** A provider failure (timeout, 5xx, down) fails **only that provider's** step; the file re-extract result is committed regardless. A file-read failure (missing/locked file) fails the refresh **without** mutating the row's active state or data. | • Provider down + file OK → file fields update, response/activity reports the provider error, item not corrupted.<br>• File missing → refresh errors, item retains prior data and prior `active` state, **not** deactivated by this action. |
| **F31.6** | **Recorded in activity history (flat `job_runs`, no FK).** Each refresh appends **one** `job_runs` row following the established per-entity pattern ([F22.6b](metadata-plugins.md), [migration 0006](../../internal/db/migrations/0006_job_detail.up.sql)): a new `kind="refresh"` constant, `trigger="manual"`, and a free-text `detail` summarizing both halves — **never** a new FK column and **never** a filesystem path (the [ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md) no-secrets invariant). | A completed refresh appears in System Activity as a single row with `kind=refresh`, the item referenced as `#<id>` inside `detail` (e.g. `"#42 — file: 3 fields; tmdb: 5 fields"`), the combined `status`, and (on failure/partial) an error message. Scan-count columns are `0` for this kind. **No `video_id` column is added to `job_runs`.** |
| **F31.7** | **Owner-only, single-flight, scan-safe.** The control renders only when `capabilities.owner === true`; concurrent refreshes of the same item, or a refresh racing a full scan, must not corrupt the row or double-write. The single-file refresh **does not** acquire the global `scanMu` (a one-file op must not wait behind a 10k-file scan); row safety rides on `repo.writeMu`, and a small **per-item in-flight guard** de-dupes a double-click server-side (returning "already running", mirroring `TriggerRescan`). | • Non-owner UI never shows the control; non-owner request is rejected (F31.1).<br>• A second refresh of the same item while one is in flight is de-duplicated server-side (no torn writes).<br>• A refresh during a running library scan completes without DB corruption (both paths read the same file and write the same derived data via single-statement `UpsertVideo` under `repo.writeMu` — a race is redundant work, not corruption). |
| **F31.14** | **Structured `RefreshReport` (batch-ready outcome).** The refresh service returns a typed result — per source, per field: `previous`, `incoming`, `winner`, `changed`, and a derived **`sources_disagree`** flag (file value ≠ provider value). Because the resolver discards losing candidates ([resolver.go:130](../../internal/resolver/resolver.go)), `sources_disagree` is computed in the **refresh layer**, which already holds both inputs. This struct feeds F31.8's "what changed" *and* is the seam a future batch op (F31.11) aggregates for conflict triage. | The endpoint response and the activity `detail` are both derived from the same `RefreshReport`. For an item whose file and TMDB titles diverge, the report marks that field `sources_disagree: true`. Computing the report adds **no** resolver change and **no** extra I/O. |
| **F31.15** | **Separable `plan` / `apply` internals (batch-ready seam).** The service is structured as `plan(id) → RefreshPlan` (re-extract + provider re-fetch + diff/disagreement detection, **no writes**) then `apply(plan) → RefreshReport` (commit). F31 calls them back-to-back so the split is invisible single-item, but a future batch (F31.11) can run `plan` across N items, interpose conflict resolution, then `apply`. **No public `plan` endpoint** is exposed in v1. | The two phases are independent functions with no hidden coupling; `plan` performs no DB writes (verifiable by test). Single-item refresh behavior is unchanged by the split. |

### Nice-to-Have (P1)

| ID | Requirement | Notes |
|---|---|---|
| **F31.8** | **"What changed" feedback.** After a refresh, surface a concise summary — which fields changed (or "no changes") and any per-provider error — rather than a bare success. | Improves trust; can be a toast/inline summary from the endpoint's response payload (changed-field count + per-provider status). |
| **F31.9** | **`last_refreshed_at` on the item.** Track and (optionally) display when the item was last force-refreshed, distinct from `indexed_at`. | Lets the owner see staleness at a glance; small schema add. |
| **F31.10** | **Confirm before refresh on large/slow files** *(only if testing shows the op is slow enough to be surprising)*. | Likely unnecessary for a single file; revisit after measuring. |

### Future Considerations (P2)

| ID | Requirement | Notes |
|---|---|---|
| **F31.11** | **Bulk forced re-extract + conflict resolution** (selection or whole-library "force re-read all"), with progress + throttling, **plus** triage of items where file and provider metadata disagree. | Explicitly out of v1 (Non-Goals). The per-item seams are built to carry it: a batch driver runs `plan` across N items (F31.15), filters the `RefreshReport`s for `sources_disagree` (F31.14), applies an operator policy / per-item precedence override (new store — see Resolved Decisions), then `apply`s. The F31.4 non-destructive invariant guarantees both raw layers survive for any policy. |
| **F31.12** | **Staleness indicator / opt-in periodic re-enrich.** A badge when provider data is older than N days, or an opt-in scheduled refresh. | Tracks the open question in [metadata-plugins.md §Open](metadata-plugins.md); keep pull-only for now. |
| **F31.13** | **One-click "refresh then write back"** chaining refresh → F28 writeback for a true round-trip sync. | Compose two existing owner actions; only if the workflow proves common. |

---

## Resolved Decisions

These were the load-bearing open questions; resolved with the owner before drafting.

1. **What "re-run enrichment from the file" means → full refresh.** The action re-reads the file
   **and** re-runs the item's linked providers in one click (not file-only). This deliberately
   completes the deferred **F22 re-enrich UI** follow-up rather than shipping a narrower file-only
   button. *(Drives F31.2 + F31.3 as a unit.)*
2. **Force past change-detection → yes, always.** The manual refresh **unconditionally**
   re-extracts, bypassing the `(size, mtime)` fast-path, because the headline value is catching
   external edits — *including* in-place edits that preserve mtime. *(Drives F31.2's "forced.")*
3. **Trigger scope → single item only.** No bulk/library-wide forced re-read in v1; the existing
   F13.3 admin rescan covers a normal full pass, and a forced full pass is deferred to F31.11.
4. **Orchestration shape → unified operation built on reusable seams.** Refresh is **one**
   operation (a `refresh` service) that orchestrates the forced single-file extract seam and the
   existing `enrich.Apply`, returning one `RefreshReport` — *not* two independently-recorded
   operations stitched together by the handler. Rationale: one owner click should read as one
   activity entry with one combined status, and a future batch op wraps the single
   `Refresh(id) → RefreshReport` function rather than re-orchestrating two subsystems. *(Drives
   F31.6, F31.14, F31.15. The batch consideration reinforced this over the "composed" alternative.)*
5. **Activity model → flat `job_runs` row, `kind=refresh`, no FK.** The house pattern for
   per-entity jobs ([F22.6b](metadata-plugins.md)) is a denormalized row with a free-text `detail`
   referencing the entity as `#<id>`, *not* a foreign key. Refresh follows it exactly — no
   `video_id` column. *(Corrects an earlier draft assumption. Drives F31.6.)*
6. **Partial-success contract → HTTP 202 + per-source status array mirroring a combined row
   status.** The `job_runs` row is `success` only if both halves succeed, else `error` with
   `detail` naming the failed half; the response carries the same per-source breakdown (e.g.
   `file: ok`, `tmdb: failed`). Derived from the single `RefreshReport`. *(Drives F31.5, F31.6.)*
7. **Concurrency → no global `scanMu`; per-item in-flight guard + `repo.writeMu`.** A one-file
   refresh must not block behind a full scan; row safety is already provided by the single-statement
   `UpsertVideo` under `repo.writeMu`, and a per-item guard de-dupes double-clicks server-side.
   *(Drives F31.7.)*
8. **Soft-deleted target → `409 Conflict`.** The row exists but the action is disallowed; 409 is
   more truthful than 404 and lets the SPA show a real message. The ADR-037 #26 "never reactivate"
   guard holds regardless. *(Drives F31.1.)*

**Designed for a future batch op without speccing it now.** Decisions 4, 5, and the F31.14/F31.15
seams exist specifically so a later **bulk forced re-extract with conflict resolution** (F31.11)
layers on without reworking F31. The one data-model batch would most likely add — a **per-item /
per-field precedence override** (operator pins "file wins" or "provider wins" for an item,
overriding the global precedence of [ADR-013](../architecture/ADR-013-metadata-field-mapping.md))
— is deliberately **not** built now; the F31.4 non-destructive invariant is what lets it be added
later cleanly. We bank the seams (struct + plan/apply + invariant), not the batch machinery.

---

## Success Metrics

This is a single-owner personal server, so metrics are **owner-observed correctness signals**,
not adoption funnels.

**Leading (verify at implementation / first use):**
- **Captures silent edits**: editing a file's tags in another app with mtime preserved, then
  refreshing, updates Holodex's displayed metadata — target **100%** of tested fields
  (title, tags, people, codecs, cover-art flag).
- **Provider re-fetch works without re-prompt**: refreshing a matched item re-pulls the provider
  and never re-opens the match picker — **100%** of the time.
- **Isolation holds**: across the test matrix, a re-extract never drops enrichment data and a
  re-enrich never drops file data — **0** cross-layer losses.
- **Resilience holds**: with a provider stubbed down, file fields still update and the error is
  reported — **0** whole-refresh crashes from a single provider failure.
- **Latency**: a single-file refresh (file local, one provider) completes within a few seconds
  (provider call bounded by the existing 8s per-call cap); no UI hang.

**Lagging (observed in normal use):**
- The owner stops needing to trigger full rescans to sync one edited file (the friction this
  feature targets disappears in practice).
- No library-corruption or stale-data incidents attributable to refresh over the following weeks.

---

## Open Questions

The architectural questions are resolved above (see Resolved Decisions 4–8) and carried into
**[ADR-047](../architecture/ADR-047-per-item-metadata-refresh.md)**. The **design** call —
placement, label/icon, in-flight treatment, and how F31.8 feedback renders — is resolved in the
**[design handoff](../design/metadata-refresh-handoff.md)**: a ghost **Refresh** control first in the
Metadata header cluster, an inline `aria-live` status line for feedback (no toast), and
`sources_disagree` deliberately **not** surfaced single-item (the existing provenance chips suffice;
rich triage waits for the batch feature, F31.11). No open questions remain; what's left is
implementation, the embedded three-skin QA, `/security-review`, and a `/testing-strategy` update.

---

## Timeline Considerations

- **No hard deadline.** Standalone backlog feature on a personal project.
- **Dependencies**: all prerequisites are shipped (extraction, scanner index path, enrichment
  apply, resolver, owner gate, activity). The one true blocker is the **ADR** (forced-extract
  seam + per-item refresh orchestration + activity recording) — write it before implementation.
- **Suggested phasing within F31** (each independently shippable):
  1. **F31.2 + F31.1 + F31.15** — forced single-file re-extract behind the owner endpoint
     (file-only path, no providers), structured as the `plan`/`apply` split from the start.
     Smallest viable slice; de-risks the headline "catch silent edits" goal and lays the seam.
  2. **F31.3 + F31.4 + F31.5 + F31.14** — fold in linked-provider re-enrich, the non-destructive
     invariant, per-source error handling, and the `RefreshReport` (incl. `sources_disagree`) —
     the full "refresh" semantics and the batch-ready outcome.
  3. **F31.6 + frontend control + F31.8** — activity recording (`kind=refresh`) and the
     owner-facing button with "what changed" feedback; QA across all three skins.

---

## References

- **F22 Metadata Source Plugins** — [metadata-plugins.md](metadata-plugins.md) ·
  [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) (enrichment store, persisted
  match, on-demand ethos; this spec completes its deferred re-enrich UI).
- **F27 Resolver / F28 Writeback** — [ADR-041](../architecture/ADR-041-metadata-writeback.md) ·
  [qa-writeback.md](qa-writeback.md) (provenance model; writeback is the complementary *write*
  path — refresh is the *read* path).
- **F21 System Activity** — [system-activity.md](system-activity.md) ·
  [ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md) (job history surface).
- **Owner gating** — [ADR-030](../architecture/ADR-030-access-control-gating-seam.md)
  (`requireOwner`, `X-Admin-Token`, capabilities `owner` flag).
- **Scanner change-detection** — [ADR-018](../architecture/ADR-018-scanner-change-detection.md)
  (the `(size, mtime)` fast-path this feature deliberately forces past).
- **Soft-delete guard** — [ADR-037](../architecture/ADR-037-soft-delete-and-purge.md)
  (#26 reactivation guard refresh must honor).

---

> **Change-routing reminder (per project working agreements).** This functional spec is the
> **functionality** artifact. Status of the matching artifacts: **ADR — done**
> ([ADR-047](../architecture/ADR-047-per-item-metadata-refresh.md)); **design handoff — done**
> ([metadata-refresh-handoff.md](../design/metadata-refresh-handoff.md), with embedded three-skin
> QA). Still required before merge: a **`/testing-strategy`** update + tests (auth/validation,
> forced-extract proof incl. the mtime-preserved case, provider isolation, soft-delete guard,
> activity recording) and a **`/security-review`** (it touches access + file I/O + subprocess).
