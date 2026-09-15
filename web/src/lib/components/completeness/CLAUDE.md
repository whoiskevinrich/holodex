# Completeness components

Entity Completeness Score UI (F55, ADR-081/082): the facet-first remediation queue and the
per-entity breakdown panel — both consumed across video/person/studio, so they live here rather
than in any single entity-type folder (see the parent `components/CLAUDE.md` classification rule).

| File | Purpose |
|---|---|
| `CompletenessQueueRow.svelte` | One (entity, missing facet) row on `owner/completeness`. Candidate-ready rows show a `ProvenanceBadge` + Apply button that pins the field to the cached candidate's provider; needs-research rows show a Search link (and, for image facets, an Upload link) that navigates to the entity page anchored at that facet's control. |
| `CompletenessPanel.svelte` | Per-entity breakdown panel card on video/person/studio detail pages (F55.13-15) — score bar + actionability line, facets grouped Critical/Nice to have with a status pill per tier (Curated/Provider via `ProvenanceBadge`/Missing/Not applicable), and the video-only not-applicable toggle on the `external_provider_id` row. |

## Rules

- **`facets[].curatable` and `criticality: 'optional'` (F60, HOLODEX-377).** `curatable` is true
  for a plain-text replace field — the shape the media page can hand to an empty `SourceBadge` row
  when a `#field-<canonical>` deep link targets a facet the resolver dropped for being empty and
  undecided; image, long-text and merge fields are never `curatable`. An `optional` facet is
  listed for exactly that reason but is **not** a completeness concern: `CompletenessPanel` groups
  only `critical` / `nice_to_have`, so it never renders one, and the queue API skips it — don't add
  an "Optional" group or count it in "missing".
