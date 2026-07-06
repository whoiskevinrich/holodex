# Spec: Video Credits — cast/crew People with headshots (F32, "Populate")

> **Rebased onto F40 / [ADR-059](../architecture/ADR-059-person-link-resolved-derivation.md) (2026-07-04).**
> F40 now owns the `video_people` **data model** this spec used to propose: the `role` column
> (derived from the source field, PK `(video_id, person_id, role)`, unset-capable) and the migration
> of `video_people` to **resolved-value derivation** (`RelinkVideoPeople`, sole writer). F32 no longer
> adds a role column or writes `video_people` directly — it **layers on top**: the provider emits
> structured `people[]` (external_id + headshot + billed order); core stores the **external_id**
> (de-dup) and **downloads the headshot**, while the video↔person link itself is derived by
> `RelinkVideoPeople` from the resolved `actors`/`director` fields the enrich already populates. See
> the superseding notes inline (§2, §3, Open Q2).

**Status**: Draft — implementation handoff (not yet built). Decisions locked; **rebased onto F40/ADR-059** — build F40 first (or together). Needs `/security-review` before/at implementation (the ADR is ADR-059 + F32's own external-id/asset notes).
**Feature block**: F32
**Phase**: 3 (Enrichment) — follow-on to F22/F26/F27/F30
**Decision**: **Choice A** (provider-side) — locked with the owner 2026-06-28.
**Depends on**: [F22 metadata source plugins](metadata-plugins.md)/[ADR-033] (provider client, SSRF perimeter), [F25 person images](people-images.md)/[ADR-038]/[ADR-039] (image store + ingest normalizer + `asset_hosts`), F26 video enrichment, [F30 curation](metadata-curation.md)/[ADR-048] (actor/director chips already link to person pages).
**Provider contract**: already updated — [metadata-provider-contract.md §2.2/§2.4/§4.5](metadata-provider-contract.md#45-video-credits--per-person-castcrew-with-headshots) defines the `people[]` shape (`credits: true`, per-person `name`/`role`/`external_id`/`order`/`headshot`).

---

## Why this exists / problem

Today a film enrich stores cast/crew only as **flat text** (`fields.actors`, `fields.director`), and the F30 actor/director chips link to Person pages — but:

- The TMDB provider's video `/enrich` returns **names only, zero image assets** (verified 2026-06-28).
- Core downloads provider image assets **only for `entity_type == person`** ([`internal/enrich/service.go`](../../internal/enrich/service.go) — the `downloadAssets` guard), never for video enrichment.
- The **director is not a Person record at all**; cast People exist (created at scan time from file `Artist` tags or by video enrich) but have **placeholder headshots only** (`headshot_version: null`).

So the chips render person-list-style but the faces are silhouettes, and the director has no link. "Populate" closes that: real cast **and** director People with real headshots, created/linked during a film enrich.

## Decision (Choice A — provider-side)

The provider returns structured per-person credits with a stable `external_id` and a `headshot` asset URL; **core consumes** it. Chosen over the core-orchestration alternative (N per-person name-search enriches) because it is **deterministic** (real upstream person IDs → correct de-dup, no fuzzy name matching), **one call**, and reuses the existing asset-download pipeline. The contract is already published so an out-of-repo provider can implement ahead.

---

## Components to change (the work)

### 1. TMDB provider (`providers/tmdb/`) — emit `people[]`
- It already fetches `/credits` (actors/director land in `fields`). Add the structured `people[]` to the **video** `/enrich` response: per cast/crew member `{ name, role, external_id: "tmdb:<personId>", order, headshot: { kind:"photo", url: <image.tmdb.org profile_path> } }`.
- Advertise `"credits": true` in `/describe`.
- Headshot URL host is `image.tmdb.org` (already an operator `asset_hosts` entry in the films profile). Keep flat `actors`/`director` fields too (fallback).
- Cap the list (top ~20 billed + director/key crew).

### 2. Core enrich (`internal/enrich/`) — consume `people[]`
- Extend the enrich response type with `People []ProviderPerson` (name, role, external_id, order, headshot asset).
- In the **video** enrich path (where today only fields/assets are stored): for each `people[]` entry,
  - **resolve-or-create the Person** — by `external_id` when present (deterministic de-dup), else by normalized `name`;
  - ~~**link to the video with its role**~~ — **superseded by ADR-059:** the video↔person link (and its role) is **not** written here. Enrich populates the resolved `actors`/`director` fields (it already emits `tmdb:actors`/`tmdb:director`), and `RelinkVideoPeople` derives `video_people` (with role) from those fields on enrich completion. F32's job is to make sure the **Person entity exists with its external_id and headshot** *before* the reconcile runs, so the derived link resolves to the enriched person (via `resolveOrCreatePerson`, external-id-first per ADR-055/HOLODEX-125). The `people[]` `role`/`order` inform the field values, not a direct `video_people` write.
  - **download the headshot** through the existing SSRF-guarded `AssetClient` + `ImageSink.StoreAssetIfAbsent` (reuse `downloadAssets`, generalized to accept a personID per asset — currently it's keyed to the single enriched entity). Respect `asset_hosts`, the 16 MiB/4096px caps, and the suppress-on-delete set.
- Treat all provider person data as **untrusted**: sanitize `name`/`role` (reuse `enrich.SanitizeValue`), clamp the list, validate `role` against the enum (unknown → generic).

### 3. Data model (migration)
- **`video_people.role` + resolved-derivation — now owned by [ADR-059](../architecture/ADR-059-person-link-resolved-derivation.md) (F40), not F32.** The `role` column
  (derived from the source field, PK `(video_id, person_id, role)`, unset-capable) and the migration of
  `video_people` to `RelinkVideoPeople` land in F40. F32 **depends on** that model and adds nothing to it.
  *(The "a person can be both actor+director" case F32 flagged is resolved there: two rows, distinct by role.)*
- `people` has **no `external_id`** (unique by `name` only). Add person external-id de-dup: either a `person.external_id` (namespace-qualified, nullable, unique) or a `person_external_ids(person_id, external_id)` table. Prefer a small join table (a person can carry IMDb+TMDB ids) — mirrors `person_aliases`. **This is F32's remaining data-model work**, and it is the person case of [ADR-055](../architecture/ADR-055-enrichment-unique-key-invariant.md) ([HOLODEX-125](https://whoiskevinrich.atlassian.net/browse/HOLODEX-125)) — coordinate so there is one person-external-id table, not two.
- Person de-dup at scan time uses name (`resolveOrCreatePerson`, [`internal/repo/aliases.go`](../../internal/repo/aliases.go)); extend to also match on external_id so a scan-created "Denis Villeneuve" and an enrich-created one converge. **`RelinkVideoPeople` calls the same `resolveOrCreatePerson`**, so the external-id-first upgrade benefits derivation automatically.

### 4. Frontend — mostly already done (F30)
- Actor/director chips already link to `/people/{id}` and the People poster section shows faces; once headshots populate, both light up automatically. Likely **no chip change**. Verify the director (now a real linked Person) appears in `video.people`/credits and its chip becomes a link. Possibly surface `role` (actor vs director grouping) if desired.

---

## Security (must re-confirm at implementation — `/security-review` required)

- **SSRF / asset perimeter**: per-cast headshots flow through the **same** `asset_hosts` allowlist + cross-host-redirect refusal + https-cross-host + 16 MiB/4096px/raster-only normalizer as F25/ADR-039. No new perimeter — but the review must confirm the generalized per-person `downloadAssets` still routes every URL through `AssetClient` (no direct fetch).
- **Untrusted provider data**: provider `name`/`role`/`external_id` are untrusted → sanitize + validate before DB writes; `external_id` parsed as `<ns>:<id>`, namespace allowlisted. Person creation from provider data must not allow role/name injection into anything executed.
- **No new file-write surface**: headshots use the existing person-image on-disk store (atomic, metadata-stripped). No media-file writes here.
- **Owner-gated**: triggered only by the existing owner-gated video enrich action (ADR-030).

## Routing / artifacts required at implementation
- **ADR** — the data-model decisions F32 once needed its own ADR for are now **covered elsewhere**:
  `video_people.role` + resolved-derivation → **ADR-059** (F40); person external-id de-dup → **ADR-055**
  ([HOLODEX-125](https://whoiskevinrich.atlassian.net/browse/HOLODEX-125)). F32's residual
  ADR-worthy bit is only the **"core downloads per-cast assets during video enrich"** generalization of
  `downloadAssets` (extends ADR-033/038/039; relates ADR-048) — small enough to fold into the F32 PR
  description if it doesn't warrant its own record.
- **`/testing-strategy`**: provider `people[]` contract, resolve-or-create-by-external-id de-dup, role linking, headshot download through the allowlist, name-fallback when external_id absent, unknown-role handling.
- **`/security-review`**: the conditions above.
- Update the **TMDB worked example** ([tmdb-provider.md](tmdb-provider.md)) to show the `people[]` mapping.

## Suggested slices
1. **Data model + repo**: migration (person external-ids + `video_people.role`); `resolveOrCreatePerson` external-id match; link-with-role.
2. **Provider**: TMDB emits `people[]` + `credits:true` (+ worked-example doc).
3. **Core enrich**: parse `people[]`, resolve-or-create + link + per-person headshot download (generalize `downloadAssets`).
4. **Frontend polish + QA**: confirm director links + headshots render; 3-skin QA.

## Open questions
1. **Name-only fallback** (no `external_id`): match to an existing Person by normalized name, or always create? Risk of merging two real people who share a name (homonyms) — reuse the F23 "never auto-merge same-name" caution. Lean: external_id is the only auto-merge key; name-only always resolve-or-create by exact name (existing behavior).
2. ~~**Role cardinality**~~ — **decided by ADR-059**: PK `(video_id, person_id, role)` allows multiple roles per (video, person); role is unset-capable. No longer open.
3. **Headshot refresh**: re-enrich re-downloads? Reuse the F25 suppress-on-delete + StoreAssetIfAbsent semantics (don't clobber an owner-curated headshot).
4. **De-dup migration of existing name-created People**: backfill external_ids on first enrich; no bulk migration needed.
