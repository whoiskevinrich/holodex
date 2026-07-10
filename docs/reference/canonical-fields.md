# Canonical Field Registry

This document is the operator reference for Holodex's canonical field vocabulary (F27).
Every field that a metadata provider or file-tag mapping may produce has an entry here.

Fields are registered in `internal/registry/registry.go`. Unknown keys still work — they
resolve with a title-cased label and default (text) rendering — but registered fields get
accurate labels and correct render behaviour.

> **Non-canonical provider fields (F39, [ADR-056](../architecture/ADR-056-provider-field-render-hints.md)).**
> A provider may advertise **non-canonical** keys (outside this registry) with per-field render hints in
> `GET /describe.field_hints` (label / render mode / ordering group). Any such key that has a **stored value**
> for an entity is **auto-registered** — surfaced as a **display-only** row on the video/person/studio detail
> page — with **zero** `metadata-mappings.yaml` config, ordered after the canonical fields. Precedence for a
> key's label/render/order is a four-tier ladder: **operator mapping > this code registry > provider hint >
> title-case fallback** — so a provider hint applies only to keys this registry does not define, and an
> operator mapping (or a canonical entry here) always wins. Auto-registered fields carry no source-decision or
> curation controls; an operator promotes one to a first-class curatable field by adding a mapping entry.

## How to reference fields in `metadata-mappings.yaml`

Use the namespaced source syntax:

```yaml
fields:
  - canonical: title
    browse: true               # overwrite the browse-card title
    sources:
      - tmdb:title             # TMDB enrichment → field "title"
      - file:title             # videos.title (filename-derived)
  - canonical: genres
    multi: true
    sources:
      - tmdb:genres
  - canonical: studio
    label: Studio
    sources:
      - file:Publisher         # raw file tag key (case-insensitive)
      - file:Label
```

**Source namespace syntax:**

| Prefix | Resolves from |
|--------|---------------|
| `file:title` | `videos.title` column (scanner's primary title) |
| `file:<Key>` | `extra_metadata` raw file tag (case-insensitive) |
| `<provider>:<field>` | `entity_enrichment` shadow store (e.g. `tmdb:overview`) |
| `<Key>` (no colon) | Legacy bare key — treated as `file:<Key>` |

Sources are walked left-to-right; the first non-empty value wins (`WinningSource` in the API response records which one).

---

## Video / Film fields

| Canonical | Default Label | Render | Description |
|-----------|--------------|--------|-------------|
| `title` | Title | text | Display title. Use `browse: true` to overwrite browse-card titles. |
| `original_title` | Original Title | text | Title in the original language. |
| `overview` | Overview | long_text (paragraph) | Plot summary. Trimmed to ≤4 000 chars. |
| `tagline` | Tagline | text | Short marketing tagline. |
| `release_date` | Released | text | Release date in `YYYY-MM-DD` format. |
| `runtime` | Runtime (min) | text | Runtime in minutes (integer string). |
| `genres` | Genres | text | Genre list. Use `multi: true` to split/deduplicate. |
| `status` | Status | text | Release status (e.g. `Released`, `Post Production`). |
| `original_language` | Language | text | ISO 639-1 language code. |
| `homepage` | Website | text | Official website URL. Rendered as plain text (not a link). |
| `imdb_id` | IMDb | text | IMDb title identifier (`tt…` format). |
| `poster_url` | Poster | image_url (`<img>`) | Poster URL. **Must be on an `asset_hosts`-allowlisted CDN** (ADR-039). |

---

## Person fields

| Canonical | Default Label | Render | Description |
|-----------|--------------|--------|-------------|
| `bio` | Bio | long_text (paragraph) | Biography. Trimmed to ≤4 000 chars. |
| `birthdate` | Born | text | Birth date in `YYYY-MM-DD` format. |
| `deathdate` | Died | text | Death date. Omitted for living persons. |
| `nationality` | Nationality | text | Place of birth or nationality string. |
| `website` | Website | text | Personal or professional website URL. |
| `aliases` | Aliases | text | Alternate names. Use `multi: true`. |
| `photo` | Photo | image_url (`<img>`) | Portrait image. Delivered as an asset by providers that support it. |

---

## Derived / computed fields (F45, [ADR-063](../architecture/ADR-063-derived-computed-fields.md))

A **third field genre** — beyond canonical (this registry) and non-canonical auto-registered (F39) — is
**computed**: source-less, read-only, **calculated on read** from other resolved fields by a pure `Derive`
pass, and **never stored** (a now-relative value would be stale the moment it was written). There is **nothing
to configure** — no mapping, no source, no YAML; a computed field simply appears when its inputs resolve.

| Canonical | Default Label | Computed from | Description |
|-----------|--------------|---------------|-------------|
| `age` | Age | `birthdate` | Current age in whole years — `floor(now − birthdate)`. Shown only for a **living** person (no `deathdate`). |
| `age_at_death` | Age at death | `birthdate`, `deathdate` | Age in whole years at death — `floor(deathdate − birthdate)`. **Replaces** the running Age for a deceased person (never both). |

A computed row carries `computed: true`, a `winning_source` of `computed:<canonical>`, and the human labels of
its inputs in `derived_from` (for the "calculated from …" provenance icon). It carries **no**
`decision`/`candidates` — it is **never adoptable or curatable** (a decision naming a computed field is
rejected `400`). A missing or unparseable input yields **no row** (no placeholder). The genre is
entity-generic; person Age/Age-at-death is the seed. Formulas are a **closed Go registry**
(`internal/resolver/derive.go`) — there is no formula DSL.

---

## Example file-metadata fields

These are operator-defined and not required. The registry entries below exist as
documented examples; operators add their own via `metadata-mappings.yaml`.

| Canonical | Default Label | Render | Description |
|-----------|--------------|--------|-------------|
| `studio` | Studio | text | Production company, publisher, or label. |
| `collection` | Collection | text | Album or collection. |
| `director` | Director | text | Director(s). Use `multi: true`. |

---

## Render modes

| `display` value | How the SPA renders it |
|-----------------|------------------------|
| *(absent)* | Inline text: `Label: value1, value2` |
| `long_text` | Block paragraph below the label |
| `url` | Link(s) via `UrlValueList` (http/https only, opens in a new tab) |
| `chips` | Static pill list (read-only; F39) — used by auto-registered multi-valued non-canonical fields |
| `image_url` | `<img src=values[0]>` (thumbnail-sized, border-rule). A provider-hinted value renders as an image only if its host is on the `asset_hosts` allowlist (ADR-039/056); otherwise it falls back to text |

For a **canonical** field, `display` is set in `internal/registry/registry.go`. Since F39
([ADR-056](../architecture/ADR-056-provider-field-render-hints.md)) a mapping may also set `display:`
explicitly (operator override), and a provider may suggest a render mode for a **non-canonical** key via
`field_hints`; the resolution ladder is operator mapping > code registry > provider hint > default (text).

---

## Provenance badge

When a field's value comes from an enrichment provider (i.e. `winning_source` does not
start with `file:`), the SPA shows a **ProvenanceBadge** with the provider name so
operators can tell at a glance which source won. File-sourced fields show no badge. A
**computed** field (`winning_source` = `computed:<canonical>`, F45) shows a distinct
**icon-only** "calculated" badge instead of a provider icon, with a "calculated from …"
hover/`aria-label` naming its `derived_from` inputs.
