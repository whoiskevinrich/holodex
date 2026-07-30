-- HOLODEX-227 (ADR-075 D1): tags gain a strict one-parent hierarchy.
--
-- A single nullable self-reference is the least structure that expresses a
-- strict tree (spec RD6: one parent per tag, no DAG) -- not a closure table or
-- materialized path, neither of which this codebase's scale (a single owner's
-- tag count, shallow hierarchy depth) needs yet. Cycle prevention is enforced
-- at the application layer (internal/repo/tag_hierarchy.go), not in DDL --
-- SQLite has no native graph-cycle constraint, mirroring how ADR-061 enforces
-- its own cross-namespace identity guarantee at the resolve layer. Descendant
-- expansion for tag-based filter/search is a query-time WITH RECURSIVE over
-- this column -- no denormalized descendant set is stored anywhere.
ALTER TABLE tags ADD COLUMN parent_tag_id INTEGER REFERENCES tags(id) ON DELETE SET NULL;
CREATE INDEX idx_tags_parent ON tags(parent_tag_id);
