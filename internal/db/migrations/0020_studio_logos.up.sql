-- HOLODEX-130 (ADR-057): self-hosted studio logo. A studio's logo was a hotlinked
-- provider-CDN URL rendered straight from the `logo` enrichment field; this table is
-- the metadata index for a downloaded, normalized (metadata-stripped) local copy —
-- a derived cache of whatever URL the studio's `logo` field currently RESOLVES to.
--
-- Like thumbnails (0002) and person images (0009), the BYTES live on disk at
-- DATA_PATH/studio-logos/{studio_id}/{id}.jpg (ADR-014) and never in the DB. One logo
-- per studio: UNIQUE(studio_id) is the single-slot invariant, and a refresh is
-- delete + insert so the row id changes → the ?v={id} cache-buster changes → the
-- browser re-fetches past the immutable cache (the ADR-038 id-is-the-version trick;
-- no separate version column).
--
-- source_url is the resolved logo URL this cache was derived from — the idempotency
-- key: RelinkStudioLogo skips the re-download when the resolved URL is unchanged.
-- provider records the winning provider (provenance + the allowlist used to fetch).
-- ON DELETE CASCADE means a logo never outlives its studio (ADR-053 prune-on-empty
-- drops the studio row and, with it, this one).
CREATE TABLE studio_logos (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    studio_id   INTEGER NOT NULL UNIQUE REFERENCES studios(id) ON DELETE CASCADE,
    source_url  TEXT    NOT NULL,
    provider    TEXT    NOT NULL DEFAULT '',
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL
);
