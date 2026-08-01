-- HOLODEX-239 (ADR-077 D1): tags gain a per-tag Genre-writeback participation
-- flag. A tag stays a normal, searchable, attachable Holodex tag either way --
-- this only decides whether TagNamesForVideo's final projection includes its
-- name (internal/repo/tag_hierarchy.go), not creation, search, filtering, or
-- attachment. Default 1 (included) makes the migration a no-op for every
-- existing tag's writeback behavior -- no silent behavior change on deploy.
ALTER TABLE tags ADD COLUMN writeback_enabled INTEGER NOT NULL DEFAULT 1;
