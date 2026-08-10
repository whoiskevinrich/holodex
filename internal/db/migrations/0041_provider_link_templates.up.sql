-- HOLODEX-266 (ADR-083 D2): persist each provider's advertised link_templates from
-- /describe so the provider-link badge (person/studio, video later) can build an
-- outbound URL server-side without contacting a provider on every detail read --
-- mirrors provider_field_hints (migration 0019, F39/ADR-056): the manifest is read
-- only during owner actions; this table is the durable copy the read path consults.
--
-- Keyed by (namespace, entity_type), NOT by provider: a namespace is a shared
-- identity space across providers (ADR-055 D2), so the link a namespace resolves to
-- must be provider-independent too -- a value's own namespace ("imdb:...") names the
-- real-world site that namespace belongs to, regardless which currently-configured
-- provider happens to have emitted or matched that id. The provider column is kept
-- for observability only (which provider most recently advertised this template); on
-- conflict, whichever provider's /describe was read most recently wins the row.
CREATE TABLE provider_link_templates (
    namespace   TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    provider    TEXT NOT NULL,
    template    TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    PRIMARY KEY (namespace, entity_type)
);
