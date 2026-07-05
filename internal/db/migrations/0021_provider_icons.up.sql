-- HOLODEX-134 (ADR-059): self-hosted provider brand icon. A metadata provider may
-- advertise a brand_icon in its /describe manifest; this table is the metadata index
-- for a downloaded, normalized (metadata-stripped) local copy Holodex serves from its
-- own origin, so the SPA can show a provider glyph in place of the repeated
-- "from <provider>" provenance text instead of hotlinking the provider's CDN.
--
-- Like thumbnails (0002), person images (0009), and studio logos (0020), the BYTES
-- live on disk at DATA_PATH/provider-icons/{id}.jpg (ADR-014) and never in the DB. One
-- icon per provider: UNIQUE(provider) is the single-slot invariant, and a refresh is
-- delete + insert so the row id changes → the ?v={id} cache-buster changes → the
-- browser re-fetches past the immutable cache (the ADR-038 id-is-the-version trick; no
-- separate version column).
--
-- The key is the provider NAME, not a foreign key: providers are metadata-sources.yaml
-- registry entries, not a DB table, so there is no REFERENCES / ON DELETE CASCADE. A
-- provider removed from the registry leaves an orphan row that the boot reconcile
-- (RefreshProviderIcons) prunes. source_url is the advertised brand_icon URL this cache
-- was derived from — the idempotency key: the relink skips the re-download when the
-- advertised URL is unchanged.
CREATE TABLE provider_icons (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider    TEXT    NOT NULL UNIQUE,
    source_url  TEXT    NOT NULL,
    width       INTEGER NOT NULL,
    height      INTEGER NOT NULL,
    byte_size   INTEGER NOT NULL,
    created_at  TEXT    NOT NULL
);
