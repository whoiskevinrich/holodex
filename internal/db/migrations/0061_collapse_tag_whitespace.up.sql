-- Collapse each run of spaces inside a tag name or tag alias to one space (HOLODEX-557,
-- F43 RD12): "sci  fi" is stored as "sci fi". Collision-free: the tag nameKey (0022
-- ux_tags_namekey), the tag alias_key, and the tag/category collision triggers (0035)
-- all key on replace(lower(trim(x)),' ',''), which strips every space, so no key -- and
-- no unique index or trigger -- sees a change. The <table>_au and entity_aliases_au
-- triggers keep the FTS mirrors in sync. Tabs and other whitespace are left alone here:
-- turning one into a space WOULD change its key and could collide; tagNorm folds them
-- on every new write.
--
-- One pass collapses a run of any length: each space becomes ' '||char(1), every
-- char(1)||' ' (a char(1) followed by another space) is dropped, then the remaining
-- char(1)s go -- "a   b" → "a \1 \1 \1b" → "a \1b" → "a b". char(1) never occurs in a tag.
UPDATE tags
   SET name = replace(replace(replace(name, ' ', ' ' || char(1)), char(1) || ' ', ''), char(1), '')
 WHERE name LIKE '%  %';

UPDATE entity_aliases
   SET alias = replace(replace(replace(alias, ' ', ' ' || char(1)), char(1) || ' ', ''), char(1), '')
 WHERE entity_type = 'tag' AND alias LIKE '%  %';
