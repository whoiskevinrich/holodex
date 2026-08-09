# Completeness components

Entity Completeness Score UI (F55, ADR-081/082): the facet-first remediation queue and the
per-entity breakdown panel — both consumed across video/person/studio, so they live here rather
than in any single entity-type folder (see the parent `components/CLAUDE.md` classification rule).

| File | Purpose |
|---|---|
| `CompletenessQueueRow.svelte` | One (entity, missing facet) row on `owner/completeness`. Candidate-ready rows show a `ProvenanceBadge` + Apply button that pins the field to the cached candidate's provider; needs-research rows show a Search link (and, for image facets, an Upload link) that navigates to the entity page anchored at that facet's control. |
