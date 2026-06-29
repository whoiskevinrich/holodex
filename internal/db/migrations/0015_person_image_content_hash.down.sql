DROP INDEX IF EXISTS idx_person_images_hash;
ALTER TABLE person_images DROP COLUMN content_hash;
