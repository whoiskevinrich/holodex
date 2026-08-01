DROP TRIGGER IF EXISTS trg_categories_no_tag_collision_upd;
DROP TRIGGER IF EXISTS trg_categories_no_tag_collision_ins;
DROP TRIGGER IF EXISTS trg_tags_no_category_collision_upd;
DROP TRIGGER IF EXISTS trg_tags_no_category_collision_ins;
DROP INDEX IF EXISTS idx_category_tags_tag;
DROP TABLE IF EXISTS category_tags;
DROP INDEX IF EXISTS ux_categories_namekey;
DROP TABLE IF EXISTS categories;
