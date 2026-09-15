-- Reverses the schema only. Film rows already in the spine tables (aliases, keep-separate,
-- review-queue, suppressions) are left in place: they are inert once no code writes or
-- reads entity_type='film', and re-applying the up migration finds them intact.
DROP TRIGGER films_ad_alias_suppressions;
DROP TRIGGER films_ad_aliases;
DROP INDEX IF EXISTS ux_films_namekey;
