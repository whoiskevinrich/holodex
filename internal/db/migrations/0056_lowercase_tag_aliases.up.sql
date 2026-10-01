-- Lower-case tag alias text (HOLODEX-507, F43 RD12): the 0034 house rule for tag names
-- now covers a tag's aliases too. Collision-free: the tag alias_key (0022) is already
-- replace(lower(trim(alias)),' ',''), so lowercasing leaves every key -- and the
-- (entity_type, alias_key) unique index -- unchanged. entity_aliases_au (0022) keeps
-- entity_aliases_fts in sync. Person/studio/film aliases keep their casing.
UPDATE entity_aliases SET alias = lower(alias)
WHERE entity_type = 'tag' AND alias <> lower(alias);
