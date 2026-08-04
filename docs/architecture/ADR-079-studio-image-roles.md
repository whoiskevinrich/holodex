# ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field

**Status:** Proposed
**Date:** 2026-08-03
**Deciders:** Project owner

**Relates to:** [ADR-038](ADR-038-person-images.md) (person images — the on-disk store +
normalize spine this reuses verbatim) · [ADR-039](ADR-039-provider-asset-urls.md) (provider
asset URLs — the SSRF perimeter, unchanged) · [ADR-049](ADR-049-manual-image-precedence.md)
(owner-set images beat enrichment — the precedence rule this generalizes to a second
entity) · [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field decisions —
this ADR **removes** the studio `logo` field from that model) · [ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md)
(studio entity) · **Supersedes** [ADR-057](ADR-057-self-hosted-studio-logo.md) (self-hosted
studio logo — realizes its explicitly deferred **Option D**). **Spec:**
[Studio image roles / F51](../specs/studio-images.md). **Ticket:** [HOLODEX-247](https://whoiskevinrich.atlassian.net/browse/HOLODEX-247).

---

## Context

ADR-057 gave Studio one self-hosted image: a derived cache of whatever URL the resolved
`logo` **field** pointed to, re-synced on enrich/decision triggers. It explicitly framed
this as the *minimal* generalization of Person's image system (ADR-038) and named the road
not taken:

> **Option D** — Switch the provider to emit the logo as an asset (`{kind:"logo"}`) and
> route it through the person `downloadAssets` path — **Rejected for v1.** Forces a
> provider + contract change and drags in the person lock/suppress/gallery/seed logic that
> a single logo does not use.

F51 needs three things ADR-057's field-derived cache cannot give it without contortion:

1. **A second independent image** (icon) for a different context (the list), not just a
   different crop of the same logo URL.
2. **A third slot reserved for future use** (poster) that costs nothing to add if the
   pipeline is already role-generic.
3. **Owner upload**, which a field-derived cache has no notion of — a `logo` field is a
   *value the resolver picked*, not a slot the owner can put their own bytes into.

(1) and (2) are cheap under ADR-057's model — Studio would just get two more field-derived
caches keyed by field name. (3) is the actual fork: owner upload only makes sense as an
**asset slot** with `source ∈ {upload, enrichment}`, exactly Person's shape, not as a field
decision (ADR-051's `{record, provider:<name>, manual}` grammar has no way to hold *bytes*,
only a value pointer). The owner confirmed this directly when scoping F51: manual-vs-
enrichment precedence should work like Person's provenance lock (ADR-049), not like a
per-field source chip. That decision settles the fork — Studio moves onto the asset-slot
model, and ADR-057's Option D stops being deferred.

## Decision

### 1. `studio_images` replaces `studio_logos` — three core roles, no gallery

```sql
CREATE TABLE studio_images (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    studio_id   INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    role        TEXT    NOT NULL,            -- 'icon' | 'logo' | 'poster'
    source      TEXT    NOT NULL,            -- 'upload' | 'enrichment'
    provider    TEXT    NOT NULL DEFAULT '',
    external_id TEXT    NOT NULL DEFAULT '',
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL
);
CREATE UNIQUE INDEX idx_studio_images_slot ON studio_images(studio_id, role);
CREATE INDEX idx_studio_images_studio ON studio_images(studio_id);
```

Unlike `person_images` (migration 0009), **every** studio role is core — there is no
`extra`/gallery role, so the unique index is a plain composite, not partial. There is no
`sort_order` (nothing to order) and no `promoted` source (nothing to promote from — a
promote only makes sense when a gallery exists to promote *out of*). This is the same
"take the pattern, drop what a single-image-per-role entity doesn't need" cut ADR-057 made
for its one-role version, applied to three roles instead of one.

Bytes live on disk at `DATA_PATH/studio-images/{studio_id}/{id}.jpg` (ADR-014 layout,
server-assigned `id` in the path — never a request value, ADR-038's traversal invariant).
Cache-busting is the same id-is-the-version trick: replace = delete + insert = new `id` =
new `?v=`.

**Migration 0036** carries forward existing data before dropping the old table:

```sql
INSERT INTO studio_images (studio_id, role, source, provider, external_id, width, height, byte_size, created_at)
SELECT studio_id, 'logo', 'enrichment', provider, '', width, height, byte_size, created_at
FROM studio_logos;

DROP TABLE studio_logos;

DELETE FROM field_source_decisions WHERE entity_type = 'studio' AND field_key = 'logo';
```

The on-disk files move from `studio-logos/{studio_id}/{id}.jpg` to
`studio-images/{studio_id}/{id}.jpg` in the Go migration runner's post-step (best-effort —
a missed file just means that slot re-fetches on the next enrich, the same self-healing
posture ADR-057 already established for a failed fetch). The down migration reverses both:
recreates `studio_logos`, copies `role='logo'` rows back, drops `studio_images`.

### 2. `enrich.ImageSink` / `downloadAssets` become entity-generic

Today `ImageSink` and `Service.downloadAssets` are hard-wired to Person
(`personID int64` parameters throughout; a non-person entity's assets are logged and
discarded — `service.go:344`, "no image sink in v1, use a fields entry instead"). Both gain
an `entityType string` leading parameter, mirroring the shape `EnrichmentForEntity`,
`RecordJobRun`, and the whole `entity_enrichment` shadow store already use everywhere else
in this codebase:

```go
type ImageSink interface {
    StoreAsset(ctx context.Context, entityType string, entityID int64, role, provider, externalID, url string, raw []byte, overCap bool) error
    StoreAssetIfAbsent(ctx context.Context, entityType string, entityID int64, role, provider, externalID, url string, raw []byte) error
    SuppressedAssetURLs(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error)
    LockedCoreRoles(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error)
    ExistingAssetURLs(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error)
}
```

The repo-backed adapter wired in `main` dispatches on `entityType` to the right table
(`person_images` vs. `studio_images`) — two physical stores behind one interface, the same
"entity-generic orchestration over per-entity storage" shape the resolver (ADR-052),
name-identity (ADR-061), and enrichment shadow store already use. `assetRoleFor` gains the
same leading parameter and branches once at the top:

```go
func assetRoleFor(entityType, kind string) (string, bool) {
    switch entityType {
    case "person":
        switch kind { /* unchanged: photo/portrait/headshot→headshot, banner/backdrop→banner, poster→poster, gallery→extra */ }
    case "studio":
        switch kind {
        case "logo", "":
            return model.StudioImageLogo, true
        case "icon":
            return model.StudioImageIcon, true
        case "poster":
            return model.StudioImagePoster, true
        default:
            return "", false
        }
    }
    return "", false
}
```

`icon`/`poster` kinds are reserved, not yet emitted by any provider (Non-Goal — see spec).

**The studio `ImageSink` implementation is a reduced Person:**
- `StoreAsset` always replaces (delete + insert) — every studio role is core, so there is
  no cap/`extra` branch to carry.
- `SuppressedAssetURLs` / `ExistingAssetURLs` return an empty map, unconditionally. Studio
  has no gallery and no delete-suppression store (ADR-043 was built for the *gallery*
  case — deleting a core slot already just empties it and re-fills on next enrich, per
  ADR-049's existing "delete unlocks" behavior, which needs no suppression tracking).
  Returning empty is not a stub to fill in later; it is the structurally correct answer
  for an entity with no gallery, machine-checked by the interface rather than by a comment.
- `LockedCoreRoles` is a real query: `source='upload'` rows in `studio_images` for the
  studio, identical shape to Person's `repo.LockedCoreRoles` (ADR-049).
- No poster-seed-from-headshot equivalent. Person seeds an empty poster from a freshly
  stored headshot (F25.29) because a portrait is a natural poster crop; nothing about a
  studio logo implies a poster, so `downloadAssets` simply omits that step when
  `entityType == "studio"`.

**Behavior-preservation bar for Person:** every existing call site passes
`entityType = "person"`; the adapter's person branch is the current `repo.person_images.go`
logic unchanged, just reached through one more parameter. The acceptance test is the
existing Person image test suite passing with zero behavior diffs — the same bar F37/F38
set for resolver genericity ("zero resolver-core diffs... the F37 proof, repeated").

### 3. The studio `logo` field is retired; TMDB emits it as an asset

`providers/tmdb/tmdb.go`'s company enrichment currently sets `Fields["logo"] = url`, a
plain canonical `image_url` field resolved and decided through ADR-051 like `description`
or `country`. That field entry is **removed** from the registry (`internal/registry/registry.go`
~200-204); the provider instead emits `Assets: [{Kind: "logo", URL: url}]`, flowing through
the now-entity-generic `downloadAssets` exactly like a person photo. `description`,
`country`, and every other studio field are untouched — this is a one-field, one-provider
change, not a contract or `/describe` protocol change (assets are already part of the
`EnrichResult` shape; only the studio provider is newly permitted to use it).

A studio's pre-existing `logo` field decision (a provider pin or blank pin, ADR-051) has
no equivalent in the asset-slot model — a decision pins a *value the resolver returns*, not
image bytes. Migration 0036 deletes any such rows outright (see §1) rather than attempting
a lossy translation into an upload/lock state.

### 4. Serving, upload, delete — mirrors ADR-057 §4 with an owner-write path added

`GET /api/v1/studios/{id}/images/{role}` replaces `GET /studios/{id}/logo`: same
`Cache-Control: public, max-age=31536000, immutable` + `X-Content-Type-Options: nosniff`,
404 on an empty slot (no placeholder route — the SPA owns the empty state). New:
`POST .../images/{role}` (`requireOwner`, multipart, `personimage.Normalize`d before
storage — the same untrusted-bytes gate every upload passes) and
`DELETE .../images/{role}` (`requireOwner`) — Studio's first owner-write image path,
structurally identical to `POST/DELETE /people/{id}/image[s/{imageId}]` minus the
gallery-specific reorder/promote routes Studio doesn't have.

## Options considered

| # | Option | Verdict |
|---|---|---|
| **A** | **Keep the field-derived-cache model, add two more caches** (`studio_icons`, `studio_posters`) keyed by two new fields, no owner upload | **Rejected.** Solves the multi-image problem but not the owner-upload requirement (RD1/RD2 in the spec) — a field-derived cache structurally cannot hold owner-supplied bytes; would need bolting an upload path onto a value-resolution mechanism never designed for it. |
| **B (chosen)** | **Move to the asset-slot model** (ADR-057 Option D) — `studio_images`, entity-generic `ImageSink`/`downloadAssets`, TMDB emits an asset instead of a field | **Chosen.** One coherent model for "an image the owner can also set by hand," proven once already (Person); realizes the option ADR-057 named and deferred rather than deferring it again. |
| **C** | **Clone `internal/personimage`/`person_images` wholesale into a parallel `studioimage`/`studio_images` orchestration**, duplicating `downloadAssets` rather than generalizing it | **Rejected.** Duplicates ~150 lines of orchestration logic Studio mostly doesn't need (gallery cap, dedup, suppression, poster-seed) and would drift from Person's fixes over time. The entity-generic cut is barely larger than the duplication and stays in sync by construction. |
| **D** | **Full Person parity** — give Studio a gallery/extra role too, "for consistency" | **Rejected.** No requirement asks for more than one image per role; matches the spec's Non-Goals and the project's standing bias against speculative flexibility. |

## Consequences

**Easier / better**
- Studio gets owner-correctable images for the first time — the gap ADR-057 explicitly
  left open ("No owner override of the logo image in v1... Cloning person's upload/lock/
  suppress is the deferred path... if that need appears") is closed.
- `downloadAssets` becomes entity-generic at its second real use, matching this codebase's
  established generalize-at-second-use pattern (`BaselineSource`/ADR-052, `resolveOrCreateByName`
  generalized across Person/Studio/Tag at ADR-061). A third entity needing image assets
  (should one ever) adds a role set + a repo adapter, not a new orchestration.
- One fewer field/decision special case: `logo` no longer needs the `image_url` "not a
  downloaded asset" caveat the registry currently documents for it (registry.go:203) —
  the *reason* for that caveat (it wasn't an asset) is gone.

**Harder / to revisit**
- **`ImageSink`'s signature changes for every implementer and call site**, including
  Person's. This is the one change in F51 that touches shipped, working code rather than
  adding new surface — mitigated by keeping Person's behavior byte-for-byte identical
  (parameter-widening only) and holding the existing Person test suite as the regression
  gate.
- **No lossless path for a pre-existing `logo` field decision.** An owner who had pinned a
  specific provider's logo via the field decision loses that pin (falls back to whatever
  enrichment/upload state exists after migration) and must reset it via the new upload/lock
  controls if wrong. Scoped as acceptable for a single-owner instance mid-evolution (see
  spec Non-Goals).
- **Two image tables, one interface.** `person_images` and `studio_images` diverge in
  columns (`sort_order`, partial vs. full unique index) by design; a future third
  image-bearing entity repeats this shape rather than unifying into one polymorphic table —
  consistent with how `person_images`/`studio_logos` already diverged before this ADR, and
  avoids a lossy shared schema that must special-case per entity anyway.

**Security review touch-points** (for the `/security-review` gate)
- New owner-write surface: two upload endpoints per studio role, `requireOwner`-gated,
  bytes routed through the existing `personimage.Normalize` guard (decode-and-re-encode,
  metadata strip, decompression-bomb size cap) before anything touches disk — the same gate
  every other image upload in this codebase passes, no new decode path introduced.
- Serve route streams only server-assigned-`id` paths under `studio-images/` — no request
  value ever becomes a filesystem path (ADR-038's invariant, unchanged).
- The provider-asset fetch path is unchanged: same `AssetClient` (ADR-039) host allowlist,
  https-for-cross-host, redirect refusal, size/timeout caps. Switching TMDB's logo from a
  field to an asset changes *which code path* fetches image.tmdb.org, not the allowlist or
  guard it fetches through.
- The `ImageSink` interface widening is mechanical (added leading parameter) and does not
  itself introduce a new trust boundary — worth a lighter-touch confirmation pass rather
  than a full re-review of Person's existing, already-reviewed image path.

## Action items

- [ ] Migration `0036_studio_images.{up,down}.sql` (table + data carry-forward + old-table
      drop + stale-decision cleanup; down reverses all three).
- [ ] `internal/model`: `StudioImageIcon/Logo/Poster`, `StudioImageSourceUpload/Enrichment`
      constants; `Studio.IconURL/LogoURL/PosterURL`.
- [ ] `internal/registry`: remove the `logo` field entry.
- [ ] `internal/enrich`: widen `ImageSink` + `downloadAssets` + `assetRoleFor` with
      `entityType`; Person call sites pass `"person"` unchanged.
- [ ] `internal/repo`: `studio_images` CRUD (`GetStudioImage`/`UpsertStudioImage`/
      `DeleteStudioImage`/`LockedStudioImageRoles`), studio `ImageSink` adapter, disk-layout
      package (extend/rename the existing `internal/studioimage`).
- [ ] `providers/tmdb`: company enrichment emits `Assets` for `logo` instead of `Fields`.
- [ ] `internal/api`: `GET/POST/DELETE /studios/{id}/images/{role}`; remove
      `GET /studios/{id}/logo` and the `logo` decision special-casing.
- [ ] SPA: role-generic image control on `/studios/{id}` (icon/logo/poster); list well
      switches to `icon_url`.
- [ ] Tests: migration carry-forward, entity-generic `downloadAssets` (Person regression +
      Studio new), `LockedCoreRoles` studio matrix, upload/serve/delete endpoints, SSRF
      perimeter unchanged. Update `docs/testing-strategy.md`.
- [ ] Update the ADR index (README) and studio-entity.md's Non-Goal (mark reversed, point
      here).
