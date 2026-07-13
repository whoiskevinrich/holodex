-- F47 (ADR-066 D2): a durable, owner-set "not matched" verdict for one (entity,
-- provider) pair — the direct structural sibling of entity_keep_separate (ADR-061): a
-- persisted negative assertion that stops a review workflow from re-nagging. Scoped to
-- (entity_type, entity_id, provider) rather than a relationship between two entities,
-- because the thing being rejected is a provider's candidate set for this entity, not
-- an entity-to-entity pair. A dismissal excludes the pair from the review queue's
-- "needs review" state and blocks /resolve from firing again for it until the owner
-- clears it (undismiss — "Try again"). No expiry/TTL by design (spec Non-Goals).
--
-- dismissed_at is RFC3339 UTC, matching every other timestamp column. Real ON DELETE
-- CASCADE triggers (not entity_enrichment's manual per-merge cleanup) since this table
-- is new — a dismissal never outlives the entity it names, across every delete path
-- (merge-loser removal and video hard-delete alike).
CREATE TABLE enrichment_dismissals (
    entity_type  TEXT    NOT NULL,   -- 'person' | 'studio' | 'video'
    entity_id    INTEGER NOT NULL,
    provider     TEXT    NOT NULL,
    dismissed_at TEXT    NOT NULL,
    PRIMARY KEY (entity_type, entity_id, provider)
);

CREATE TRIGGER people_ad_enrichment_dismissals AFTER DELETE ON people BEGIN
    DELETE FROM enrichment_dismissals WHERE entity_type = 'person' AND entity_id = old.id;
END;
CREATE TRIGGER studios_ad_enrichment_dismissals AFTER DELETE ON studios BEGIN
    DELETE FROM enrichment_dismissals WHERE entity_type = 'studio' AND entity_id = old.id;
END;
CREATE TRIGGER videos_ad_enrichment_dismissals AFTER DELETE ON videos BEGIN
    DELETE FROM enrichment_dismissals WHERE entity_type = 'video' AND entity_id = old.id;
END;
