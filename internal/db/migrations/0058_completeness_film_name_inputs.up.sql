-- 0058: film-name inputs join the completeness trigger set (HOLODEX-515, ADR-099 D4
-- as extended by ADR-122).
--
-- Since ADR-122 a video's sole linked film supplies its `collection` (and, for a
-- full film, its `title`) candidate, spelled the way the film's page shows it:
-- the film's canonical name, or the spelling its standing `name` decision picks
-- (repo.DisplayNames — a manual literal or the decided provider's stored `title`).
-- Completeness now resolves with those film sources, so each of those inputs must
-- dirty every video linked to the film. Linking and unlinking already do
-- (0049's cd_film_videos_*); deleting a film cascades to film_videos and fires
-- those. What was missing is the film's spelling changing under its videos.
--
-- 0049's shadow-table triggers deliberately skip entity_type = 'film' (no film is
-- scored); these fan a film's name inputs out to its videos instead, scoped to
-- the one decision field and the one provider key DisplayNames reads, so a film's
-- overview or poster fetch dirties nothing.
--
-- internal/db/completeness_triggers_test.go checks the migrated schema's cd_* triggers;
-- extend it together with this set.

-- Every film-linked video was last scored without its film sources.
INSERT INTO completeness_dirty (entity_type, entity_id)
    SELECT DISTINCT 'video', video_id FROM film_videos
    WHERE video_id NOT IN (SELECT entity_id FROM completeness_dirty WHERE entity_type = 'video');

CREATE TRIGGER cd_films_au AFTER UPDATE OF name ON films
WHEN new.name IS NOT old.name
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv WHERE fv.film_id = new.id
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;

CREATE TRIGGER cd_film_name_decisions_ai AFTER INSERT ON field_source_decisions
WHEN new.entity_type = 'film' AND new.field_key = 'name'
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv WHERE fv.film_id = new.entity_id
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;
CREATE TRIGGER cd_film_name_decisions_au AFTER UPDATE ON field_source_decisions
WHEN (old.entity_type = 'film' AND old.field_key = 'name') OR (new.entity_type = 'film' AND new.field_key = 'name')
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv
        WHERE ((old.entity_type = 'film' AND old.field_key = 'name' AND fv.film_id = old.entity_id)
            OR (new.entity_type = 'film' AND new.field_key = 'name' AND fv.film_id = new.entity_id))
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;
CREATE TRIGGER cd_film_name_decisions_ad AFTER DELETE ON field_source_decisions
WHEN old.entity_type = 'film' AND old.field_key = 'name'
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv WHERE fv.film_id = old.entity_id
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;

CREATE TRIGGER cd_film_title_enrichment_ai AFTER INSERT ON entity_enrichment
WHEN new.entity_type = 'film' AND new.field_key = 'title'
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv WHERE fv.film_id = new.entity_id
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;
CREATE TRIGGER cd_film_title_enrichment_au AFTER UPDATE ON entity_enrichment
WHEN (old.entity_type = 'film' AND old.field_key = 'title') OR (new.entity_type = 'film' AND new.field_key = 'title')
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv
        WHERE ((old.entity_type = 'film' AND old.field_key = 'title' AND fv.film_id = old.entity_id)
            OR (new.entity_type = 'film' AND new.field_key = 'title' AND fv.film_id = new.entity_id))
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;
CREATE TRIGGER cd_film_title_enrichment_ad AFTER DELETE ON entity_enrichment
WHEN old.entity_type = 'film' AND old.field_key = 'title'
BEGIN
    INSERT INTO completeness_dirty (entity_type, entity_id)
        SELECT 'video', fv.video_id FROM film_videos fv WHERE fv.film_id = old.entity_id
          AND NOT EXISTS (SELECT 1 FROM completeness_dirty WHERE entity_type = 'video' AND entity_id = fv.video_id);
END;
