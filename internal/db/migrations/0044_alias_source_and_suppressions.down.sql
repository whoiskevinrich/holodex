-- Reverses the schema only. The promoted aliases stay in entity_aliases.
--
-- Once promoted they are indistinguishable from any other owner-authored row (source=''), so
-- reconstructing metadata_curation from them would have to guess which ones came from curation
-- and which the owner typed. Rather than guess, this down migration leaves them: the owner keeps
-- names they had asserted, and the worst case is a duplicate display row if the up migration is
-- re-applied (INSERT OR IGNORE makes that a no-op). Documented in spec F58 P0-1 rather than
-- silently lossy.
-- The cleanup triggers must go first and explicitly: they live on people/studios/tags, not on
-- entity_alias_suppressions, so dropping that table does not drop them. Left behind, the next
-- DELETE FROM people would fail on a missing table.
DROP TRIGGER people_ad_alias_suppressions;
DROP TRIGGER studios_ad_alias_suppressions;
DROP TRIGGER tags_ad_alias_suppressions;
DROP TABLE entity_alias_suppressions;
ALTER TABLE entity_aliases DROP COLUMN source;
