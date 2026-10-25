-- Tag poster images (F82, docs/specs/list-views.md P0-10): the owner's one image per tag,
-- shown on the Tags page's Posters view. The bytes live at
-- DATA_PATH/tag-images/{tag_id}/{id}.jpg|.png (internal/tagimage), the same layout the
-- studio and film stores use; this row is their index. One slot per tag, so a replace is
-- delete + insert and the id — the ?v= cache-buster — always advances. Tags have no
-- provider images (they sit outside the field-resolution model), so there is no role,
-- source or provider column: every row is an owner upload.
CREATE TABLE tag_images (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	tag_id     INTEGER NOT NULL UNIQUE REFERENCES tags(id) ON DELETE CASCADE,
	width      INTEGER NOT NULL,
	height     INTEGER NOT NULL,
	byte_size  INTEGER NOT NULL,
	created_at TEXT NOT NULL
);
