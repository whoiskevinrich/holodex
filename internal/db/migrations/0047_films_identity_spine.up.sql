-- HOLODEX-376 (F60, ADR-096 D3): films join the ADR-061 identity spine. The four spine
-- tables (entity_aliases 0022, entity_keep_separate / identity_review_queue 0022,
-- entity_alias_suppressions 0044) already accept any entity_type -- there is no CHECK to
-- widen -- so the schema work is the film-side of what 0022 gave people/studios/tags:
--
--   * ux_films_namekey: the film identity key is COMPOSITE, (lower(trim(name)), year),
--     because two films with the same title and different years are legal (Superman II
--     1980 / 2006). It sits beside 0043's binary UNIQUE(name, year) rather than replacing
--     it (the 0043 table constraint cannot be dropped in place; both hold). A NULL year
--     is distinct from every other NULL under both, matching CreateFilm's `year IS ?`.
--     No fold precedes the index: CreateFilm has always trimmed and compared NOCASE, so
--     no existing pair can collide under it.
--   * films_ad_aliases / films_ad_alias_suppressions: the per-kind cleanup triggers the
--     other three kinds have, plus the keep-separate and review-queue rows (the spec's
--     "AFTER DELETE ON films cleans all four"), so a deleted film leaves no spine residue.
--
-- entity_aliases.alias_key (0022) gains no film branch: a film alias folds like a
-- person/studio alias (lower(trim())), which is the ELSE arm already.
CREATE UNIQUE INDEX ux_films_namekey ON films (lower(trim(name)), year);

CREATE TRIGGER films_ad_aliases AFTER DELETE ON films BEGIN
    DELETE FROM entity_aliases        WHERE entity_type = 'film' AND entity_id = old.id;
    DELETE FROM entity_keep_separate  WHERE entity_type = 'film' AND (id_lo = old.id OR id_hi = old.id);
    DELETE FROM identity_review_queue WHERE entity_type = 'film' AND (id_lo = old.id OR id_hi = old.id);
END;
CREATE TRIGGER films_ad_alias_suppressions AFTER DELETE ON films BEGIN
    DELETE FROM entity_alias_suppressions WHERE entity_type = 'film' AND entity_id = old.id;
END;
