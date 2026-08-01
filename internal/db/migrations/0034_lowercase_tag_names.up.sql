-- Lower-case tag entity names for consistent display/writeback. Safe without a merge
-- step: ux_tags_namekey (migration 0022) already folds case + internal whitespace to
-- enforce one row per fold-group, so lowercasing casing-only cannot introduce a new
-- collision among existing rows. tags_au (migration 0001) keeps tags_fts in sync.
UPDATE tags SET name = lower(name) WHERE name <> lower(name);

-- entity_aliases.alias intentionally keeps its original casing (migration 0022:
-- "original casing, for display + FTS") -- untouched here.
