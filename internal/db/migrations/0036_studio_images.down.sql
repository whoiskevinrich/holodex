-- Reverse 0036: recreate studio_logos and copy the 'logo' role rows back, then drop
-- studio_images. Deleted field_source_decisions rows and any 'icon'/'poster' images are
-- not recoverable (golang-migrate has no data-level undo beyond what the down script
-- expresses); on-disk bytes under DATA_PATH/studio-images/ are left for the operator to
-- remove, matching every other image-store down migration's posture.
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

INSERT INTO studio_logos (studio_id, source_url, provider, width, height, byte_size, created_at)
SELECT studio_id, '', provider, width, height, byte_size, created_at
FROM studio_images
WHERE role = 'logo';

DROP TABLE IF EXISTS studio_images;
