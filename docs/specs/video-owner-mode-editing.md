# Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)

**Status**: Draft
**Feature block**: **F52** — round out owner-mode editing on the video detail page. Title, Studio, and
Tags are already owner-editable (F36/F30); this spec closes the remaining gaps the owner asked for:
a new **Commentary** field, an **upload** path for the video poster, **Studio** shown next to the
title (not buried in the metadata list), and **file (technical) metadata hidden from visitors**.
Person/Studio *linking* — the one piece that looked like a gap but turns out to already be fully
designed — is **not** re-specced here; it implements **F40** ([HOLODEX-114](https://whoiskevinrich.atlassian.net/browse/HOLODEX-114),
[ADR-072](../architecture/ADR-072-person-link-resolved-derivation.md)) as already locked. See
"Relationship to F40" below.
**Owner**: Project owner
**Date**: 2026-08-05
**Jira**: [HOLODEX-114](https://whoiskevinrich.atlassian.net/browse/HOLODEX-114) (people/studio
linking) · [HOLODEX-251](https://whoiskevinrich.atlassian.net/browse/HOLODEX-251) (commentary) ·
[HOLODEX-252](https://whoiskevinrich.atlassian.net/browse/HOLODEX-252) (poster upload)

**Depends on** (all shipped):
- Per-field source-of-truth decisions ([ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md)) —
  Commentary rides this unchanged; no new decision mechanism.
- Configurable field mapping ([ADR-013](../architecture/ADR-013-metadata-field-mapping.md),
  [`internal/mapping`](../../internal/mapping/mapping.go)) — Commentary needs one small loosening
  (below).
- Thumbnail pipeline ([ADR-009](../architecture/ADR-009-thumbnail-strategy.md),
  [`internal/thumbnail`](../../internal/thumbnail/manager.go)) — the poster upload is a new tier on
  this existing pipeline, not a new asset store.
- The owner gate ([ADR-030](../architecture/ADR-030-access-control-gating-seam.md), `requireOwner`).

**Related / in-flight in the same branch**:
- [person-media-linking.md / F40](person-media-linking.md) + [ADR-072](../architecture/ADR-072-person-link-resolved-derivation.md) —
  implemented alongside this spec (same PR), covering People + Studio *linking*. Fully designed
  already; this spec does not duplicate it.

---

## Problem Statement

Owner mode on the video detail page ([`web/src/routes/media/[id]/+page.svelte`](../../web/src/routes/media/[id]/+page.svelte))
today lets the owner edit **Title**, **Studio** (as a text value — see "Relationship to F40" for the
*entity link*), and **Tags**, but:
1. There is no field for an owner's free-text commentary/note on a video, and no way for a provider
   to ever supply one.
2. The poster shown on the page is only ever derived *from the file itself* (embedded cover art or an
   ffmpeg frame-grab) — there is no way to upload a replacement image.
3. Studio renders as a small link line below the metadata block, disconnected from the title, even
   though it's one of the first things that identifies a video.
4. Technical file metadata (codec, container, bitrate, path) renders unconditionally — a visitor sees
   internal file-layout details that are only useful to the owner.

## Goals

1. **Commentary is a real, owner-editable field** — manual by default, but a provider can supply one
   later with zero code changes (a mapping entry, same as any other field).
2. **The owner can upload a poster image**, and it sticks — it is never silently replaced by
   auto-regeneration.
3. **Studio reads next to the title** on first view, not three sections down.
4. **File metadata is owner-only.**
5. Every new owner control is invisible/absent from the DOM for a visitor (`requireOwner` +
   `activity.effectiveOwner`, consistent with every existing owner control on this page).

## Non-Goals

- **Re-deciding People/Studio *linking*.** That is F40/ADR-072, already locked with the owner
  2026-07-04. This spec implements it, not redesigns it.
- **A rich-text/markdown commentary editor.** Plain text, same input chrome as any other manual
  field value (`SourceSelect`'s manual entry). Formatting is future scope if ever needed.
- **A poster gallery or multiple poster sizes.** One image, one slot — mirrors the single thumbnail
  slot that exists today; no promotion/reorder UI like the person image gallery.
- **Removing the existing "Regenerate from file" action.** Upload is an *additional* tier, not a
  replacement for the extract/frame-grab path.

## Resolved Decisions

*(Locked with the owner 2026-08-04 via question cards.)*

- **RD1 — Commentary semantics.** Owner-only free text by default; also a normal per-field-source-of-truth
  **replace** field (ADR-051) so a provider can optionally map into it. Not every video has one — an
  empty/undecided Commentary field simply doesn't render, exactly like every other optional field.
- **RD2 — People/Studio linking ships as F40, not a parallel mechanism.** A bare attach/detach table
  for `video_people` was considered and rejected — it is exactly ADR-072's rejected Option C (a
  second link-derivation pattern). This spec's implementation work for "People is editable" **is**
  implementing ADR-072 (`RelinkVideoEntity`, the curation-based link picker), not a lighter substitute.
- **RD3 — Poster upload extends the existing thumbnail pipeline; no new table.** The video's poster
  *is* the ADR-009 thumbnail (`videos.thumbnail_state`, served from `DATA_PATH/thumbnails/{id}.jpg`).
  An uploaded image becomes a new highest-precedence tier (`"uploaded"`) in that same pipeline, not a
  parallel asset store. (The initial idea of mirroring `studio_images`'s single-slot-per-role table
  was reconsidered once the existing thumbnail single-slot design was confirmed to already do exactly
  this job — reusing it avoids a second poster concept on the page.)

## Requirements

### Must-have (P0)

**Commentary (HOLODEX-251)**
- **P0-1 — Registry entry.** Add `commentary` to [`internal/registry/registry.go`](../../internal/registry/registry.go)
  (`Label: "Commentary"`, `Display: "long_text"`, a description noting it's optional and
  provider-mappable).
- **P0-2 — Allow a zero-source mapping field.** [`internal/mapping.parse`](../../internal/mapping/mapping.go#L96)
  currently skips any YAML field entry with an empty `sources` list ("skip malformed entries"). A
  manual-only field is not malformed — `resolveDecided`'s manual branch already ignores
  `ParsedSources` entirely (`internal/resolver/resolver.go`), and an undecided zero-source field
  already resolves to "no items" (dropped), exactly matching "not every video has commentary."
  Loosen the skip condition to `f.Canonical == ""` only.
  - Given a `metadata-mappings.yaml` entry `{canonical: commentary}` with no `sources:` key, Then
    the field loads, and `GET /media/{id}` includes a `commentary` row with `candidates: []` once the
    owner sets a manual decision, and no row at all until then.
- **P0-3 — Document in the example config.** Add a commented `commentary` entry to
  `metadata-mappings.yaml.example` showing the manual-only shape and a placeholder for a future
  provider source.
- **P0-4 — Owner edit UI.** Commentary renders via the existing `SourceSelect.svelte` (it is a plain
  replace field — no new frontend component), positioned in a dedicated "Commentary" area on the page
  (not buried in the generic metadata `dl`), visible only when it has a value or the owner is present.

**Poster upload (HOLODEX-252)**
- **P0-5 — New thumbnail state.** Add `model.ThumbnailUploaded = "uploaded"` alongside
  `Embedded`/`Generated`/`Failed` ([`internal/model/model.go`](../../internal/model/model.go)).
  `HasThumbnailImage` includes it.
- **P0-6 — Upload endpoint.** Owner-gated `POST /media/{id}/poster`: multipart `image` field, capped
  body size (mirror `person_images.go`'s `MaxBytesReader` pattern), decode + normalize (strip
  metadata, bound dimensions — reuse or sibling `personimage.Normalize`'s approach), write to the
  existing `thumbnail.ThumbPath(dir, id)`, `SetThumbnailState(ctx, id, model.ThumbnailUploaded)`.
  201 on success; 400 on an undecodable/oversized image; 503 when thumbnail storage is unconfigured.
- **P0-7 — Revert endpoint.** Owner-gated `DELETE /media/{id}/poster`: removes the uploaded file and
  resets state (`ResetThumbnailState` + re-attempt extract/enqueue, same as today's "Regenerate"
  action) so the owner can fall back to the file-derived poster.
- **P0-8 — Uploaded posters are never auto-replaced.** The startup sweep already only re-queues
  `thumbnail_state IS NULL OR = 'failed'` ([`internal/repo/repo.go:606`](../../internal/repo/repo.go#L606)) —
  `"uploaded"` is excluded by construction, same as `"embedded"`/`"generated"` today. No new guard
  code; called out here so a future change to that query doesn't accidentally regress it.
  - Given an uploaded poster, When a rescan or the startup sweep runs, Then the uploaded image is
    untouched.
  - Given an uploaded poster, When the owner clicks "Regenerate from file" (existing action), Then it
    explicitly overwrites the upload (an owner action always wins over another owner action) — the
    protection is against *automatic* replacement only.
- **P0-9 — Frontend.** The existing hover "Regenerate thumbnail" control gains a sibling "Upload
  poster" action (file picker) and, when `thumbnail_state === 'uploaded'`, a "Remove" action. Owner-only.

**Layout (no schema/API change)**
- **P0-10 — Studio next to the title.** Move the studio value (currently
  [`+page.svelte` L738-747](../../web/src/routes/media/[id]/+page.svelte#L738), a link line under the
  metadata `dl`) up into the header area, directly under/beside the `<h1>` title. Reuses the existing
  resolved `studio` value and its existing entity-link target — pure template relocation, the F40
  picker (P0-10/P0-11 in the F40 spec) attaches here once it ships.
- **P0-11 — File metadata is owner-only.** Wrap the existing File section
  ([`+page.svelte` L780-794](../../web/src/routes/media/[id]/+page.svelte#L780)) in
  `{#if isOwner}` — the same `activity.effectiveOwner` derivation already used elsewhere on this page.
  No data-shape change; purely a render gate.

### Should-have (P1)

- **P1-1 — Poster upload dimension/format guardrails surfaced in the UI** (max size, accepted types)
  before the request round-trips, mirroring the person-image upload's client-side hints.

### Future considerations (P2)

- **P2-1 — Commentary provider mapping** for a real provider, once one exposes a matching field (pure
  config, no code — the point of RD1).
- **P2-2 — Poster gallery / multiple sizes** — deferred, no current need.

## API

New/changed:
```
POST   /api/v1/media/{id}/poster     upload a poster image (multipart `image`)      (requireOwner)
DELETE /api/v1/media/{id}/poster     remove an uploaded poster, revert to file-derived (requireOwner)
```
Reused, unchanged:
```
PUT/DELETE /api/v1/media/{id}/fields/commentary/decision   (existing decision endpoint, ADR-051)
```
Commentary needs **no new endpoint** — `setFieldDecision`/`clearFieldDecision` already handle any
replace field once it's present in the loaded mapping (P0-2).

## UI (grounded in real components)

- **Commentary**: `SourceSelect.svelte`, same chrome as Title/Studio, in a new "Commentary" block
  under the header (near Studio, per P0-10's relocation), hidden entirely when empty and not owner.
- **Poster**: the existing hover-controls row over the `<video poster=…>` element
  ([`+page.svelte` L475-492](../../web/src/routes/media/[id]/+page.svelte#L475)) gains an "Upload"
  button (native file input, no new modal) and a conditional "Remove" button. Tokens only; QA all
  three skins (Cinémathèque / Broadcast / Brutalist), consistent with every other owner control on
  this page.
- **Studio near title**: `<h1 class="skin-title">` gains a sibling line (existing `border-rule`/
  `text-muted` link styling copied from the current studio line, just relocated).
- **File metadata**: no visual change for the owner; the section disappears entirely for a visitor
  (not just disabled/greyed).

## Success Metrics

Single-owner correctness/completeness feature, no metrics infra needed:
- **Leading:** an owner can set Commentary on a video with zero backend config beyond the (optional)
  registry entry that ships in this change.
- **Leading:** an uploaded poster survives a full rescan untouched (regression-style manual check).
- **Leading:** `rg 'File.*Metadata|codec|bitrate'` (or equivalent) inside the File section confirms
  it never renders server-side data to a visitor request — verified in `/security-review` as a
  data-exposure check, not just a UI toggle.

## Timeline / routing

Per the change-routing rules:
1. **`/architecture`** — not needed. Commentary rides ADR-051 unchanged (one parser loosening, not an
   architectural decision); poster upload rides ADR-009 unchanged (one new pipeline tier). Person/Studio
   linking's architecture is already **[ADR-072](../architecture/ADR-072-person-link-resolved-derivation.md)**
   (Proposed → implemented by this branch).
2. **`/design-handoff`** — [video-owner-mode-editing-handoff.md](../design/video-owner-mode-editing-handoff.md):
   studio-near-title layout, Commentary block, poster upload/remove controls, 3-skin QA. (People/Studio
   *linking* UI is the existing [F40 handoff](../design/person-media-linking-handoff.md), unchanged.)
3. **`/testing-strategy`** — [testing-strategy.md](../testing-strategy.md): new §9 block for F52
   (commentary zero-source field, poster upload/protect-from-sweep/revert, file-metadata gating) sitting
   alongside the existing F40 block (already written, unchanged).
4. **`/security-review`** — the poster upload is a new **owner-gated file upload/decode perimeter**
   (mirror the `person_images.go` review: size caps, decode safety, no path traversal via the
   server-assigned id-based path) — reviewed together with F40's file-write perimeter (person/studio
   writeback) since both land in the same PR.

### Before implementation
- Branch already renamed to carry HOLODEX-114 (the anchor issue for this combined change); HOLODEX-251/
  HOLODEX-252 track the commentary/poster slices and are linked via this spec.
