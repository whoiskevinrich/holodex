-- HOLODEX-226 (ADR-075 D2): the global tag deny-list -- a third, genuinely
-- different shape of durable negative assertion. entity_keep_separate
-- (entity_type, id_lo, id_hi) and enrichment_dismissals (entity_type,
-- entity_id, provider) are both relationship-shaped (a pair, or an entity+
-- provider pair); a deny-list blocks a bare TERM, globally, with no entity or
-- provider dimension at all. Forcing it into either existing shape would mean
-- inventing a synthetic id/provider this concept doesn't have.
--
-- term_key uses the identical fold tags themselves resolve by (F43 RD2,
-- nameKeyExpr in internal/repo/identity.go: lowercase + trim + strip all
-- internal whitespace), so denying "Gnome" blocks "GNOME" but never "Garden
-- Gnome" -- exact-string, not substring.
CREATE TABLE denied_tags (
    term_key   TEXT PRIMARY KEY,
    term       TEXT NOT NULL,
    created_at TEXT NOT NULL
);
