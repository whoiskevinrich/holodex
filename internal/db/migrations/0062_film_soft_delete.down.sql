DROP TRIGGER IF EXISTS films_ad_entity_rows;
DROP TRIGGER IF EXISTS cd_films_deleted_au;
DROP INDEX IF EXISTS idx_films_deleted_at;
ALTER TABLE films DROP COLUMN deleted_at;
