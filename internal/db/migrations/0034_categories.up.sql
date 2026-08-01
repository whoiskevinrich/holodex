-- HOLODEX-240 (ADR-078): Tag Categories -- a deliberately reduced entity.
--
-- categories mirrors tags' pre-identity shape (no provenance/alias/hierarchy);
-- category_tags mirrors video_tags exactly (D2). Category is NOT part of the
-- entity-name-identity spine (D1/D4) -- CRUD lives in internal/repo/categories.go,
-- not resolveOrCreateByName. The four triggers below are the cross-table name
-- collision backstop (D3): a category can't share a name with a tag, or vice
-- versa, enforced at insert AND rename, on both tables, using the exact fold
-- tags already use (nameKeyExpr's tag variant) so the comparison is meaningful.

CREATE TABLE categories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_categories_namekey ON categories (replace(lower(trim(name)), ' ', ''));

CREATE TABLE category_tags (
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    tag_id      INTEGER NOT NULL REFERENCES tags(id)       ON DELETE CASCADE,
    PRIMARY KEY (category_id, tag_id)
);
CREATE INDEX idx_category_tags_tag ON category_tags(tag_id);

CREATE TRIGGER trg_tags_no_category_collision_ins
BEFORE INSERT ON tags
FOR EACH ROW WHEN EXISTS (
    SELECT 1 FROM categories
    WHERE replace(lower(trim(name)), ' ', '') = replace(lower(trim(NEW.name)), ' ', '')
)
BEGIN
    SELECT RAISE(ABORT, 'name collides with an existing category');
END;

CREATE TRIGGER trg_tags_no_category_collision_upd
BEFORE UPDATE OF name ON tags
FOR EACH ROW WHEN EXISTS (
    SELECT 1 FROM categories
    WHERE replace(lower(trim(name)), ' ', '') = replace(lower(trim(NEW.name)), ' ', '')
)
BEGIN
    SELECT RAISE(ABORT, 'name collides with an existing category');
END;

CREATE TRIGGER trg_categories_no_tag_collision_ins
BEFORE INSERT ON categories
FOR EACH ROW WHEN EXISTS (
    SELECT 1 FROM tags
    WHERE replace(lower(trim(name)), ' ', '') = replace(lower(trim(NEW.name)), ' ', '')
)
BEGIN
    SELECT RAISE(ABORT, 'name collides with an existing tag');
END;

CREATE TRIGGER trg_categories_no_tag_collision_upd
BEFORE UPDATE OF name ON categories
FOR EACH ROW WHEN EXISTS (
    SELECT 1 FROM tags
    WHERE replace(lower(trim(name)), ' ', '') = replace(lower(trim(NEW.name)), ' ', '')
)
BEGIN
    SELECT RAISE(ABORT, 'name collides with an existing tag');
END;
