# ADR-050: Deduplicate enrichment photos by image content hash

**Status**: Proposed
**Date**: 2026-06-29
**Deciders**: Project owner
**Relates to**: [ADR-038](ADR-038-person-images.md) (person-image store, normalize-on-ingest, `source`/`source_url` provenance), [ADR-039](ADR-039-provider-asset-urls.md) (provider asset fetch + first-success-per-*core*-role dedup), [ADR-043](ADR-043-gallery-cap-and-enrichment-suppression.md) (gallery cap + delete-suppression by `source_url`), [ADR-049](ADR-049-manual-image-precedence.md) (owner-set core images locked from enrichment), spec [People Images / F25](../specs/people-images.md) (F34).

---

## Context

Of a person's four image roles, the three core roles (headshot/banner/poster) are
single-slot — a new image *replaces* the slot — and the enrichment path already dedups
them per run to first-success-per-role ([ADR-039 §5](ADR-039-provider-asset-urls.md)).
The fourth role, the gallery `extra`, is **append-only**: `enrich.downloadAssets`
deliberately never marks the gallery role `done`, and `repo.InsertPersonImage` simply
appends after the current max `sort_order` (capped at `GalleryCap`). There is **no dedup
on gallery extras at all** — not by URL, not by content.

So the same photo accumulates in a person's gallery whenever:

1. **A re-enrich re-fetches a still-present URL.** ADR-043 suppression only skips URLs the
   owner *deleted*; an image still in the gallery is fetched and appended again next run.
2. **A provider lists the same gallery URL twice** in one `/enrich` response (the per-run
   `done` map is keyed by role, and gallery never sets it).
3. **Two providers — or one provider's different size/crop variant — return the same image
   under different URLs.** Nothing keys on the bytes.
4. **The headshot also arrives as a gallery asset** (or the F25.29 headshot→poster seed
   bytes reappear as an `extra`), so the same face is both the avatar and a grid tile.

The owner's only recourse today is to delete each duplicate by hand — and non-deleted
copies return on the next enrich. This is the **byte-level** sibling of two URL-level
guards already in the repo: ADR-043 keeps a *deleted* image deleted (`source_url`
suppression); ADR-049 keeps an *owner-set* core image from being overwritten (`source`
lock). Both key on a URL or a provenance flag; neither catches "the same picture arrived
again under a different URL." The user's framing is explicit: *the rule should apply
across any photo enrichment* — so identity must be the **image content**, not its URL.

### Constraints

- **Identity is the bytes Holodex serves**, i.e. the *normalized* JPEG produced by
  `personimage.Normalize` (decode → bound → re-encode → metadata strip), not the raw
  provider bytes — two providers shipping the same photo at different sizes/encodings
  should still collide after normalization where the output matches, and EXIF/ICC noise
  must not defeat the match.
- **No new SSRF surface.** This only *skips* fetches/stores; it adds no fetch target
  (ADR-039 perimeter unchanged).
- **Core-role behavior must not regress.** Single-slot replace (F25.7), the locked-core
  rule (ADR-049), and the headshot→poster seed (F25.29 — which *intentionally* reuses the
  headshot's exact bytes under a different role) all stay correct.
- **Small footprint** (ADR-008/023): single Go binary, SQLite, no new dependency
  (`crypto/sha256` is stdlib).

## Decision

Give every person image a **content hash** and make an enrichment gallery `extra` a no-op
when its hash already exists for that person. Four parts.

### 1. `content_hash` column (migration 0015), per-person identity

Migration `0015` adds `person_images.content_hash TEXT NOT NULL DEFAULT ''` (hex sha256 of
the normalized JPEG) plus a **non-unique** lookup index `idx_person_images_hash
(person_id, content_hash)`. The hash is computed in Go right after `Normalize`, so it
describes the stored bytes, and threaded through a new `PersonImageInsert.ContentHash` on
every ingest path (upload, enrichment, promote) — the column is populated going forward
for *all* sources, not just enrichment.

Scope is **per person**: the same provider portrait legitimately belongs to two different
people, so the dedup key is `(person_id, content_hash)`, never a global hash.

### 2. App-layer enforcement, not a DB unique constraint

Uniqueness is enforced in the repo/sink layer — a transactional existence check — **not**
a `UNIQUE` index. Three reasons make the app layer the right seam:

- The hash is computed in Go (SQLite has no built-in sha256 over BLOBs we'd want to lean
  on), so the value only exists application-side at insert time anyway.
- **Core roles legitimately repeat a hash.** A re-enrich that refreshes an `enrichment`
  headshot re-inserts the same bytes (delete-then-insert); the F25.29 seed stores the
  headshot's hash again under `poster`. A blanket `UNIQUE(person_id, content_hash)` would
  reject both. The rule we want is narrower ("don't *append a gallery extra* that
  duplicates anything"), which is a policy, not a table invariant.
- The desired outcome is a **silent skip**, not an error surfaced to the owner — matching
  `ErrGalleryFull`'s handling. A constraint violation is the wrong control-flow shape.

`InsertPersonImage`, for an `extra` insert, checks
`SELECT 1 FROM person_images WHERE person_id = ? AND content_hash = ?` (across **all**
roles) inside the existing write transaction; on a hit it returns a new sentinel
`ErrDuplicateImage`. `StoreAsset` computes the hash, sets `ContentHash`, and treats
`ErrDuplicateImage` the way `downloadAssets` already treats `ErrGalleryFull` — log-free
skip, no disk file written, not counted against the cap. Core-role inserts **do not** run
the check (their single-slot/seed/replace semantics are unchanged).

### 3. URL fast-path before fetch

A content match still requires downloading + normalizing the bytes to learn the hash. For
the common case — a re-enrich re-offering a URL already in the gallery — we skip the
*fetch* entirely: `downloadAssets` loads `ImageSink.ExistingAssetURLs(personID)` once per
run (alongside the suppressed and locked sets it already loads) and skips any gallery
asset whose `source_url` is already stored. The content-hash check (part 2) remains the
authoritative backstop for the same image under a *different* URL. The fast-path **fails
open** like its siblings — a lookup error logs and treats nothing as already-present.

### 4. One-time backfill of existing rows (Go startup pass)

A pure-SQL migration can't sha256 on-disk bytes, so the backfill is a Go startup repair
pass (the established pattern), gated on rows with an empty `content_hash` so it's
idempotent and runs effectively once. For each such row it reads the on-disk JPEG,
computes the hash, and writes it back. It then collapses duplicates **per person**: within
a `(person_id, content_hash)` group it keeps the earliest occurrence and deletes the other
**`extra`** rows (and their disk files); an `extra` whose bytes match a **core** image is
removed in favor of the core image. **Core images are never deleted by the backfill** — at
most two core slots could share a hash (e.g. the F25.29 seed), and collapsing those would
break a role's display. Re-running the pass is a no-op once every row is hashed and every
duplicate extra is gone.

Owner uploads are hashed (part 1) but **never auto-skipped** at ingest: an owner
deliberately uploading a duplicate is honored, consistent with the cap posture
(enrichment bounded; the owner may over-cap, ADR-043 F25.24). Only enrichment-sourced
`extra` inserts are silently deduped.

## Options Considered

### Dedup key

| Option | Complexity | Catches | Notes |
|---|---|---|---|
| **A. Content hash of normalized bytes + URL fast-path (chosen)** | Med | re-runs, same-URL-twice, cross-provider, different size/crop variants, headshot-in-gallery | One additive column + index; hash is stdlib; URL fast-path avoids most refetches |
| B. `source_url` only | Low | re-runs, same-URL-twice | Misses the same image under a different URL — fails the "across any enrichment" goal; no new column |
| C. `provider` + `external_id` | Low | within one provider | Two providers (and the headshot→poster seed) don't share an `external_id`; misses cross-provider, the most visible case |

**Pros (A):** identity is the bytes the user actually sees, so it holds across providers,
URL variants, and re-encodes; degrades safely (empty hash ⇒ never matches). **Cons (A):**
must fetch+normalize before the content check can fire (mitigated by the URL fast-path for
the common re-run case); a near-duplicate that normalizes to *different* bytes (a
genuinely different crop) is not caught — acceptable, and arguably correct (it's a
different image).

### Enforcement seam

| Option | Notes |
|---|---|
| **A. App/repo-layer existence check (chosen)** | Hash lives Go-side; core roles legitimately repeat a hash; wants a silent skip, not an error; backfill must precede any constraint anyway |
| B. DB `UNIQUE(person_id, content_hash)` | Would reject legitimate core repeats (re-enrich refresh, F25.29 seed) and the empty-hash pre-backfill rows; surfaces a constraint error where we want a no-op; can't be created until dupes are collapsed |
| C. Dedup in `downloadAssets` only (in-memory per run) | Catches same-URL-twice in one response but not across runs/providers — the main complaint; the durable guard must be at the store |

### Backfill of existing duplicates

| Option | Notes |
|---|---|
| **A. Go startup pass: hash + collapse existing duplicate extras (chosen)** | Cleans today's duplicated galleries, not just future ones; SQL can't hash disk bytes; idempotent via empty-hash gate |
| B. Prevent-new only | Smaller/safer (no destructive step), but leaves every already-duplicated gallery for the owner to clean by hand |

## Trade-off Analysis

The core trade is **a fetch-then-discard cost** (the content check needs the bytes) for
**provider-agnostic correctness** (identity is the served image). The URL fast-path
removes that cost for the dominant case — a re-enrich re-offering URLs already stored —
leaving the full fetch only for a genuinely new URL, which we'd fetch anyway. Choosing the
app layer over a DB constraint trades a hard table invariant for the flexibility to keep
core-role semantics intact and to skip silently; the non-unique index still makes the
existence check a single indexed lookup. The backfill trades a one-time read of every
person-image file at first post-upgrade start (bounded: 3 core + ≤20 extras per person,
each ≤16 MiB, already capped) for cleaning galleries that are *already* duplicated —
without it the feature would only help people enriched after upgrade. The destructive step
(deleting duplicate `extra` rows + files) is bounded to gallery extras and never touches a
core image, keeping the blast radius to the exact rows the dedup rule says are redundant.

## Consequences

- **Positive**: a person enriched repeatedly, or by multiple providers, no longer
  accumulates duplicate gallery tiles; the headshot stops doubling as a gallery image;
  existing duplicated galleries self-heal on upgrade. The rule is content-based, so it
  holds "across any photo enrichment" as specified.
- **Behavioral contract** (the test matrix): enrichment `extra` duplicating any existing
  image (any role) → skipped, no row, no file, cap untouched; same gallery URL twice in
  one response → one row; re-enrich with a still-present URL → not refetched (URL
  fast-path); same bytes under a new URL → fetched once, then dropped (hash); `enrichment`
  headshot → still refreshes on re-enrich (core unaffected); F25.29 poster seed → still
  fills from headshot bytes though the hash exists; owner upload of a duplicate →
  succeeds; backfill → collapses existing duplicate extras, never deletes a core image,
  idempotent.
- **Migration**: `0015` is additive (one `NOT NULL DEFAULT ''` column + one non-unique
  index); `down` drops both. No data step in SQL — the hash population + dedup is the Go
  startup pass, gated on empty `content_hash` (safe to ship before the pass runs; rows
  simply stay unhashed and never match until backfilled).
- **Security**: no new exposure. The change only *removes* writes/fetches; it adds no
  fetch target (ADR-039 perimeter unchanged), reaches no new owner-gated path, and
  introduces no secret/PII. The backfill reads files the process already owns. A
  `/security-review` is **not** gated by this change beyond confirming the above (no
  binary-ingest or access-model change — those were ADR-038's gate, unchanged here).
- **Performance**: steady-state adds one sha256 over already-in-memory normalized bytes
  per ingest and one indexed existence query per enrichment `extra`. First-start backfill
  reads each unhashed person-image file once.
- **What we'll revisit (deferred)**: perceptual / near-duplicate matching (a different
  crop or re-touch of the same shot is *not* caught by exact-byte hashing — out of scope,
  and debatable whether it should be); extending the silent skip to owner uploads behind
  an explicit "this looks like a duplicate" prompt; suppressing re-add by *hash* as well
  as URL (ADR-043) so a deleted image stays gone even under a new URL.
- **Generality**: the mechanism (hash column + per-entity content check + backfill) is
  entity-agnostic; if tag images (F15.3) reuse the person-image store/serve model, the same
  dedup applies with no redesign.
- **Supersession**: immutable per repo convention — superseded, not edited, if the
  identity model changes (e.g. to perceptual hashing or a global content-addressed store).

## Action Items

1. [ ] Migration `0015_person_image_content_hash`: add `person_images.content_hash TEXT
   NOT NULL DEFAULT ''` + `idx_person_images_hash (person_id, content_hash)`; `down` drops
   both.
2. [ ] `repo`: add `PersonImageInsert.ContentHash`; in `InsertPersonImage`, for `role =
   extra`, existence-check `(person_id, content_hash)` across all roles and return a new
   `ErrDuplicateImage` on a hit (no insert). Add `ExistingPersonImageURLs(personID)`.
3. [ ] `personimage.Sink.StoreAsset`: compute the sha256 of the normalized bytes, set
   `ContentHash`, and treat `ErrDuplicateImage` as a skip (no disk write); add
   `ImageSink.ExistingAssetURLs` passthrough. Set `ContentHash` on the upload/promote
   paths too (no skip there).
4. [ ] `enrich.downloadAssets`: load the existing-URL set once per run; skip a gallery
   asset whose `source_url` is already stored (fail open). Confirm `ErrDuplicateImage`
   skips like `ErrGalleryFull`.
5. [ ] Startup backfill pass: hash rows with empty `content_hash` from their on-disk
   bytes; collapse per-person duplicate `extra` rows (keep earliest; drop an `extra`
   matching a core image), deleting their files; idempotent; never delete a core image.
6. [ ] `/testing-strategy`: add the F34 matrix (repo dedup + URL fast-path + enrich skip +
   backfill collapse + core-role-unaffected + owner-upload-not-skipped).
7. [ ] Update the [ADR index](README.md) and the spec's reference block (already added as
   the F34 addendum in [people-images.md](../specs/people-images.md)).
