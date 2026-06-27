-- F25 (ADR-043): enrichment image suppression. When the owner deletes a gallery
-- image that arrived from a provider, its source URL is remembered here so a later
-- re-enrich does not silently re-add the same image. source_url records where an
-- enrichment-sourced row's bytes came from (empty for owner uploads/promotes), so a
-- delete knows which URL to suppress.
ALTER TABLE person_images ADD COLUMN source_url TEXT NOT NULL DEFAULT '';

-- Per-person suppression list keyed by the deleted image's source URL. ON DELETE
-- CASCADE keeps it from outliving its person; the composite PK makes a repeated
-- suppression of the same URL a no-op (INSERT OR IGNORE). created_at is RFC3339 UTC.
CREATE TABLE person_image_suppressions (
    person_id  INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    source_url TEXT    NOT NULL,
    created_at TEXT    NOT NULL,
    PRIMARY KEY (person_id, source_url)
);
