# Spec: Tag Writeback Exclusion — per-tag Genre writeback control

**Status**: Draft
**Epic**: [HOLODEX-239](https://whoiskevinrich.atlassian.net/browse/HOLODEX-239)
**Owner**: Project owner
**Date**: 2026-07-31

**Depends on**: Only shipped surfaces —
- the global tag deny-list (ADR-075 D2, `internal/repo/tag_denylist.go`, `denied_tags`)
- genre writeback value resolution (`internal/api/genre_writeback.go`, `GenreWritebackValues`)
- the durable write queue (`internal/writequeue`) and its existing batch-enqueue precedent, `propagateMerge` (`internal/api/merge_writeback.go`)
- the per-video writeback modal (`web/src/lib/components/writeback/WritebackFormDialog.svelte`, `POST /media/{id}/writeback`)
- the tag detail page's `EntityVideos` `detail` snippet extension point (already used by `people/[id]`, unused by `tags/[id]`)
- the "don't write" chip interaction language (`web/src/lib/components/curation/CurationChip.svelte`)
- the background-activity indicator convention (`.activity-dot`, `web/src/app.css`, F21.5)

**New ADR required**: Likely — a small one covering the new per-tag flag's place in genre
resolution and the manual-trigger batch-write seam (parallels ADR-041's scope). Touches file
I/O via the write queue → a `/security-review` sign-off is recommended before merge, consistent
with how other writeback-adjacent changes have been handled (e.g. ADR-047).

---

## Problem Statement

Holodex tags map onto a video file's `Genre` metadata tag on writeback
(`internal/writeback/tags.go`). Today the only lever an owner has to keep a term out of a file
is the global deny-list, which prevents a term from ever existing as a Tag anywhere in the app —
too blunt when the term is a genuinely useful, searchable tag in Holodex's own UI (e.g. "yoda")
that simply shouldn't also land in the file's Genre field. Without a narrower control, an owner
either accepts file bloat from every tag they keep, or gives up in-app usefulness for tags
they'd rather just mute on write. Affects any owner running file writeback with a nontrivial tag
vocabulary, and gets worse as the catalog grows.

## Goals

1. An owner can keep a tag fully functional and searchable in Holodex while excluding it from
   the file's Genre tag specifically.
2. The exclusion is enforceable at scale — actionable on many tags at once, not one at a time.
3. An owner can, when ready, push a tag's current writeback decision out to already-written
   files — not just future writes — without visiting every affected video individually.
4. The system stays predictable while this feature is new: no large, automatic batch of file
   writes fires as a side effect of flipping a flag.
5. The control lives where an owner already goes to think about a tag (`tags/{id}`).

## Non-Goals

- **Automatic resync on toggle.** Flipping the flag (single or bulk) never itself enqueues a
  file write. *(Why: avoid flooding the write queue/filesystem while this pattern is new and
  unproven. May become an opt-in "auto-sync" setting later once trust is established — not
  designed here.)*
- **Changing deny-list behavior.** Deny stays exactly as-is — forward-only, blocks tag creation,
  filters only the raw-genres union. A separate, deliberately distinct control (should this term
  ever exist as a Tag) from this one (should an existing tag reach the file).
- **Character-limit / truncation warnings.** No enforcement exists anywhere in the writeback
  layer today, and this spec adds none. A real, separate gap — tracked, not solved here.
- **Tag categories.** Related fast-follow, spec'd and shipped separately (follow-up epic),
  sequenced after this one.
- **Making "Refresh" a writeback mechanism.** `internal/refresh/refresh.go` only pulls
  file→DB; it stays that way. (An earlier assumption that Refresh could serve as a sync
  mechanism was checked and confirmed wrong during scoping.)
- **Per-video overrides.** The flag is global per tag, not a per-video decision — a different,
  already-partially-solved shape (`CurationFieldRow`'s per-value `nowrite`) that this isn't
  replacing.

## User Stories

- As the owner, I want to mark a tag as excluded from file writeback so its files don't
  accumulate noise terms while the tag stays useful for search/browse in Holodex.
- As the owner, I want to manually trigger a sync for a tag so that already-written files
  actually catch up to my current decision, on my schedule — not automatically the moment I
  change my mind.
- As the owner, I want to exclude or re-include writeback for several tags at once from the tags
  list, so cleaning up a batch of noise doesn't mean visiting each tag's page individually.
- As the owner, I want clear feedback that a manual sync is running and when it's done, so a
  batch affecting many files doesn't look like it silently did nothing.
- As the owner, I want the toggle and the sync action next to a tag's other info, not a separate
  flow, so I don't context-switch to manage it.

## Requirements

### P0 — Must-Have

**Per-tag writeback flag**
- New boolean column on the Tag entity, default `true` (included — no behavior change on
  deploy).
- `genreWritebackValuesForVideo` (`internal/api/genre_writeback.go`) filters
  `TagNamesForVideo`'s output through this flag before unioning with the (already
  deny-filtered) raw-genres side — that loop currently has no filter at all.
- [ ] A tag with the flag off contributes nothing to `GenreWritebackValues`, for any video,
      while remaining a normal attached/searchable tag.
- [ ] A tag with the flag on behaves identically to current behavior.
- [ ] The flag affects only the Genre writeback value — not creation, search, filtering, or
      attachment.
- [ ] Changing the flag alone never enqueues a write — it only updates the stored value.

**Manual sync trigger**
- A tag-scoped action ("Sync writeback now" or similar) that the owner explicitly invokes,
  enqueuing a writeback job for every video currently carrying that tag, via
  `writequeue.EnqueueMany`, following the existing `propagateMerge` batch pattern.
- The confirmation/progress UX reuses the existing `WritebackFormDialog` pattern rather than
  introducing new UI paradigms. Note: that dialog is currently scoped to one video's resolved
  fields — this extends the same visual/interaction shell and its existing job-status polling
  (`api.writebackJobStatus`) to a tag-scoped batch, rather than reusing it unmodified.
- [ ] Triggering sync on a tag attached to N videos enqueues N jobs in one batch and shows
      progress/completion via the extended dialog.
- [ ] A tag attached to zero videos: the trigger is disabled/no-op — nothing to sync.
- [ ] Enqueue failures are logged, non-fatal — visible in the dialog's result, but don't leave
      the flag or other jobs in an inconsistent state (mirrors `propagateMerge`'s existing
      handling).

**`tags/{id}` Details card**
- New detail-view scaffold on the currently read-only `tags/{id}` page, via `EntityVideos`'s
  existing `detail` snippet (already used by `people/[id]`, currently unused by tags).
- Contains the writeback toggle (styled after `CurationChip`'s "don't write" glyph/interaction,
  applied at tag-entity scope) and the manual sync trigger.
- Built generically enough to hold a second row later without restructuring (a follow-up
  categories feature will add one) — not hard-coded to only ever contain this toggle.
- [ ] Owner sees current flag state and can toggle it, and can trigger a manual sync,
      from `tags/{id}`.
- [ ] Non-owners see neither control (mirrors existing `activity.effectiveOwner` gating).

**Bulk actions from `/tags`**
- Extend the existing multi-select "Manage" mode (`web/src/routes/tags/+page.svelte`) with:
  "Turn off writeback for selected", "Turn on writeback for selected", and "Sync writeback now
  for selected."
- Flag-toggle actions are not a single button — a selection can span tags already in different
  states, so on/off stay explicit and separate.
- [ ] Selecting 2+ tags in Manage mode surfaces all three actions.
- [ ] Each flag-toggle action applies to every selected tag regardless of its individual prior
      state, without enqueuing any write.
- [ ] The sync action enqueues writeback for every video across every selected tag's current
      attachments, using the same batch mechanism as the single-tag trigger.

### P1 — Nice-to-Have

**Syncing indicator**
- A lightweight signal that a triggered sync is still draining through the write queue, reusing
  the existing `.activity-dot` "background work in progress" convention (`app.css`, F21.5),
  visible even after the owner navigates away from the dialog.
- [ ] Some ambient signal exists while jobs from a sync trigger are still in flight.
- Not blocking for P0 — the in-dialog job-status polling already covers the "did my click do
  something" need; this covers "is it still going after I left the page."

**Pre-trigger scope confirmation**
- Before enqueuing, show the affected video count ("This will write to 214 files") so a large
  sync isn't a surprise.
- [ ] The trigger's confirmation step states the number of videos that will be written.

### P2 — Future Considerations

- An opt-in "auto-sync on toggle" setting, once the manual-trigger pattern is proven reliable —
  explicitly not built now (see Non-Goals), but P0's schema/API shouldn't make adding it later
  awkward (e.g., don't couple the flag itself to any UI-only state).
- Character-count/near-limit warning on tags contributing to a video's Genre field, informed by
  real container-format limits — not investigated or designed here; P0 shouldn't foreclose it.
- Keep `propagateMerge`'s batch pattern (and this spec's extension of `WritebackFormDialog`)
  general enough for a future call site to reuse without duplication — a later categories
  feature may want something structurally similar. Not building that now, just not blocking it.

## Success Metrics

Single-owner, self-hosted app — adoption-funnel metrics don't apply. The practical bar:

- Genre tag noise measurably drops on files for tags the owner excludes and manually syncs
  (spot-check via `exiftool`, pre/post).
- A manual sync on a tag attached to 50+ videos completes (verified via `job_runs`) without
  getting stuck, with clear progress/completion feedback throughout.
- Zero regressions in existing deny-list / writeback-queue coverage
  (`genre_writeback_test.go`, `writequeue_test.go`, `tag_denylist_test.go`).

## Open Questions

- **Engineering**: exact migration numbering for the new column — trivial, resolve during
  implementation.
- **You**: confirm the flag should default `true` for every existing tag at migration time (no
  silent behavior change on deploy) — assumed, not yet explicitly stated.

## Timeline Considerations

No hard deadline. This is the first of two related fast-follows — a tag-categories feature is
intentionally sequenced after this one, since it reuses the Details-card scaffold and the
Manage-bar bulk-action extension point this spec originates.
