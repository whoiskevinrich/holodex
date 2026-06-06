# ADR-013: Configurable Metadata Field Mapping

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Beyond Holodex's six first-class fields (title, people, tags, duration, resolution, date), media files carry many other container tags — `Publisher`, `Copyright`, `Comment`, `Description`, `Rating`, `Collection`, etc. Users want to surface selected extra tags under their own labels (e.g. display the file's `Publisher` tag as **Studio**), and — because different encoders write the same concept under different keys (`Publisher` / `Label` / `Studio` / `ProductionCompany`) — to **normalize several source keys into one canonical field**. This normalization is the same fragmentation problem that motivated the three-tool extraction strategy (ADR-004).

## Decision

Provide a **configurable mapping layer** that maps one or more raw file tag keys to a single canonical Holodex field with a user-defined display label. The general (many-to-one) form subsumes the simple 1:1 relabel.

### Data capture (Phase 1)
- During indexing, the scanner stores **all human-meaningful container/format-level tag key-values** for each video in a `video_metadata` table — independent of whether any mapping currently references them.
- This means mappings can be added or changed later with **no re-scan** — enabling a mapping is a re-interpretation of already-stored data.
- Excluded from capture: binary blobs (embedded cover art) and the source keys already consumed by the six first-class fields (to avoid duplication). Stream-level (track) tags are excluded per ADR-010.

```
VideoMetadata
  video_id   → Video
  source_key   string   -- tag key as extracted (matched case-insensitively)
  value        string
  -- index on (source_key, value) for facet queries
```

### Mapping configuration (Phase 2)
A config file (`metadata-mappings.yaml`) mounted into the container, consistent with the Docker/point-at-a-folder ethos and the Phase 3 plugin-config pattern. A UI editor is a future addition.

```yaml
fields:
  - canonical: studio
    label: Studio
    sources: [Publisher, Label, Studio, ProductionCompany]  # order = precedence
    filterable: true
  - canonical: collection
    label: Collection
    sources: [Album, Collection]
    filterable: true
  - canonical: director
    label: Director
    sources: [Director]
    filterable: true
    multi: true            # split/aggregate multi-valued fields
```

- **Precedence**: `sources` list order is priority — the first source key present on a file supplies the value.
- **multi**: single-valued by default; `multi: true` aggregates/splits multiple values (e.g. multiple directors).
- At config load, the app builds a `source_key → canonical field` index (case-insensitive).

### Display (Phase 2)
- The video detail page renders each configured canonical field under its `label`, resolving the value from `video_metadata` via the source precedence list.

### Discoverability (mapping authoring aid)
Because users author mappings by hand, they must be able to **see the underlying raw tags** to choose source keys correctly:
- **Per-video (Phase 1):** the detail page shows a "raw extracted metadata" panel listing every `video_metadata` key-value for that file. This also doubles as a verification surface for F2.9 capture.
- **Library-wide (Phase 2):** a "metadata keys" view enumerates all **distinct source keys across the library** with occurrence counts and a few sample values, and flags which keys are already covered by a mapping. This is the primary aid for writing `metadata-mappings.yaml`.

### Filtering (Phase 2)
- Canonical fields with `filterable: true` become **filter facets** alongside People/Tags.
- A facet's distinct values are derived from `video_metadata` where `source_key IN (sources)`; facet lists are cached (ADR-008).
- The filter query joins `video_metadata` on the mapped source keys and matches the selected value(s).

## Rationale

- **Capture-everything-once** decouples the indexing pass from the (later, evolving) mapping configuration — no destructive coupling, no forced re-scans.
- **Many-to-one normalization** solves real encoder fragmentation and costs nothing over a 1:1 relabel (1:1 is just a single-source mapping).
- **Config-file driven** keeps v1 simple and scriptable; matches the project's no-auth, Docker-composed deployment model.
- **Key-value table over JSON column** makes the filterable-facets requirement index-friendly in SQLite.

## Consequences

- Phase 1 data model gains the `video_metadata` table and the scanner persists extended tags (requirement F2.9).
- Phase 2 implements the config loader, detail-page display, and facet filtering (requirement F20).
- Storage cost: ~tens of rows per video (e.g. 50k files × ~30 tags ≈ 1.5M rows) — well within SQLite's comfort zone; the `(source_key, value)` index keeps facet queries fast.
- The mapping system creates **additional** fields; it does not override first-class extraction (ADR-004). Mapping a source key *into* a core field (e.g. feeding `people` from a nonstandard key) is a possible future extension, noted but out of scope.
- The filter API (`GET /api/v1/media`, ADR-006) gains dynamic query params for filterable canonical fields (e.g. `?studio=Acme`); these are enumerated from the loaded mapping config.
- MCP `search_videos` (Phase 2) likewise accepts filterable mapped fields as optional parameters, keeping the web UI and MCP surfaces consistent.
