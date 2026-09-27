-- HOLODEX-401 / ADR-111 D1: the file's tag names as last read by the extractor (a JSON array),
-- denied names included. NULL = never read since this shipped (unknown); '[]' = read, no tags.
-- No backfill: a row fills in on its next scan or post-write re-extract.
ALTER TABLE videos ADD COLUMN file_tags TEXT;
