-- HOLODEX-463 (ADR-109): the owner's per-studio, per-image-role halo choice, stored
-- separately for dark and light palettes. A row's presence means "halo on" for that
-- (studio, role, mode); absence is the default, off. Keyed on the studio + role, not
-- the studio_images row, so the choice survives a replace (which is delete + insert
-- there). Cascades with the studio, like studio_images.
CREATE TABLE studio_image_halo (
    studio_id INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL CHECK (role IN ('icon', 'logo', 'poster')),
    mode      TEXT    NOT NULL CHECK (mode IN ('dark', 'light')),
    PRIMARY KEY (studio_id, role, mode)
) WITHOUT ROWID;
