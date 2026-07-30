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
>
> **A key listed in a field's `sources:` no longer auto-registers** (F49,
> [ADR-074](../architecture/ADR-074-claimed-provider-keys.md)). It is already a candidate of that field, so
> rendering it again as its own display-only row was a duplicate — see
> [Claiming a provider key](#claiming-a-provider-key) below.

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
| `filename:<field>` | `entity_enrichment` shadow store, `filename` namespace — parsed from a configured filename pattern (F48, [ADR-067](../architecture/ADR-067-filename-extraction-confidence-and-rollback.md)); same shape as any other provider, no schema change |
| `<Key>` (no colon) | Legacy bare key — treated as `file:<Key>`. Being a file tag, it **claims nothing**: `Comment` has no effect on any provider's `comment` key |

`filename:` currently produces four field keys: `title`, `people`, `studio`, `release_date` (year granularity) — see the [F48 spec](../specs/metadata-extraction.md#concepts--model) for the token grammar and confidence model.

Sources are walked left-to-right; the first non-empty value wins (`WinningSource` in the API response records which one).

## Claiming a provider key

*(F49, [ADR-074](../architecture/ADR-074-claimed-provider-keys.md) · [spec](../specs/claimed-provider-keys.md))*

Providers rarely agree on names. Three of them can describe the same plot as `overview`, `synopsis` and
`comments`, and each unrecognized key auto-registers its own row (F39) — so one paragraph renders three times.

Listing a key in a canonical field's `sources:` **claims** it: the key contributes its value as a candidate
of that field and stops auto-registering separately. Claiming is not a new gesture — it is what `sources:`
has always meant. What changed in F49 is that the auto-registration pass now honours it.

**First, the choice that decides everything else:**

| The key is… | Do this | Result |
|---|---|---|
| the same thing as a field you already have | **claim** it — add it to that field's `sources:` | one row; the key becomes a candidate of that field |
| its own thing, deserving a row and curation | **promote** it — give it a `canonical:` entry of its own | a new first-class, curatable field |
| its own thing, fine as read-only | nothing | it auto-registers display-only (F39) |
| noise you never want to see | *not supported* — there is no suppress-without-a-home | — |

A key may be claimed **or** promoted, never both.

### S1 — One value, several provider names

The common case. Add each provider's spelling to the field that already means it:

```yaml
fields:
  - canonical: overview
    sources:
      - tmdb:overview          # winner — first non-empty wins
      - provA:synopsis         # claimed → no longer its own row
      - provB:comments         # claimed → no longer its own row
      - Comment                # bare = file tag (file:Comment); claims nothing
```

One **Overview** row. `provA` and `provB` become candidates behind the source chip, so their text is still
reachable — it just stops being three paragraphs of the same thing.

Order is precedence. Moving `provA:synopsis` to the top makes it the winner without changing what is claimed:
claiming a key says *this is that field*, not *this should now win*.

### S2 — Two providers, same key name, different meanings

`provA:rating` is an age certificate; `provB:rating` is a 1–10 score. Claiming is **provider-scoped**, so
naming one leaves the other alone:

```yaml
fields:
  - canonical: content_rating
    sources:
      - provA:rating           # claimed
                               # provB:rating untouched → still its own row
```

This is why a bare `rating` may never claim — it would swallow both.

### S3 — The key is on a person or a studio

`metadata-mappings.yaml` governs **video only**. There is no person or studio YAML, so config cannot express
this — use the in-app gesture on the row itself.

On any video, person or studio page, an auto-registered row carries an **Attach to…** control for the owner.
It asks which existing field the key belongs to, and — when the row carries more than one provider — which of
those providers you mean, since `provA:rating` and `provB:rating` can be entirely different things. It then
tells you what will happen before you commit: on a merge field the values join immediately, on a replace field
the key becomes a candidate you pick from the field's source chip.

The gesture is **global for the entity type**, exactly like a YAML `sources:` entry — it is config, not a
per-person edit. It also works the other way: a key that already holds an in-app *promotion* cannot also be
attached, so attaching removes the promotion (you are told first).

### S4 — Claiming onto a merge field

A merge field (`multi: true`) unions its sources rather than picking a winner, so a claimed key there
**contributes values** instead of waiting behind the chip as a runner-up:

```yaml
fields:
  - canonical: genres
    multi: true
    sources:
      - tmdb:genres
      - provA:categories       # claimed → its values join the set
```

Worth knowing before you claim: on a **replace** field the claimed source is usually invisible until you pick
it; on a **merge** field its values appear immediately.

### S5 — The key deserves its own field

`provA:filming_locations` is real information no canonical field covers. Claiming it onto something would bury
it — give it a `canonical:` entry instead:

```yaml
fields:
  - canonical: filming_locations
    label: Filming Locations
    multi: true
    sources:
      - provA:filming_locations
```

A claim and a promotion are the same YAML gesture. The difference is whether the `canonical:` names an
existing field (claim) or a new one (promote).

### S6 — Undoing a claim

Remove the source line and reload config. The key auto-registers again on the next render. Nothing in the
shadow store is rewritten, so an unclaim is always a clean reversal.

For an in-app attachment there are two ways back. Straight after the gesture, the confirmation strip standing
where the row was carries an **Undo**. Later, **Owner → Attached keys** lists every key you have attached,
grouped by entity type, each with a one-click **Remove**; the row returns on the next load of an affected page.
That list is also where an attachment whose target field no longer exists shows up, marked **Inactive** — it
suppresses nothing, so the key is already auto-registering again, but the attachment is kept in case the field
comes back.

Two things Remove does not do: it does not restore an in-app promotion that attaching cleared (that clear is a
real delete), and it never touches YAML — a `sources:` claim is your own file, edited there.

### Two things that will not work, and why

- **A bare key never claims.** `sources: [Comment]` means `file:Comment` — a file tag. It has no effect on any
  provider's `comment` key.
- **You cannot claim a canonical name.** `bio` is already a field; there is nothing to attach it to.

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
| `genres` | Genres | text | Genre list. Use `multi: true` to split/deduplicate. Resolved values also auto-materialize into real Tags — see [below](#genre-tag-materialization--governance-f50-adr-075). |
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

## Genre tag materialization & governance (F50, [ADR-075](../architecture/ADR-075-tag-governance-and-video-enrichment.md))

`genres` does double duty beyond display text. Whenever a video is (re-)enriched, its **resolved** `genres`
value — the merge-type union across every source listed in its `sources` — automatically materializes into
real `Tag` rows on that video (`provider:<name>` provenance), the same identity spine `/tags` and the media
page use for manually-applied tags. This needs **no extra configuration**: any provider you wire into
`genres`, including a second provider added to its `sources` list, feeds the tag system for free the next
time a video is enriched.

Three governance controls apply globally to every tag, regardless of origin (file scan, manual tagging, or
this materialization):

- **Deny-list** (`/owner/tags`, the "Deny-list" tab) — an owner-maintained list of terms that can never
  become a tag. A denied genre value is silently skipped during materialization — it never reaches
  `video_tags` — and is filtered out of genre *writeback* too (below), so a blocked term is a structural
  guarantee end-to-end, not a display-only filter.
- **Hierarchy** (`/tags`' parent-setter row action) — a tag may have one parent. Filtering or searching by a
  tag transitively includes every descendant, so a broad genre like "Animal" also catches videos tagged only
  "Dog" or "German Shepherd" underneath it.
- **Genre writeback** — the writeback modal's `genres` field is sourced from the union of the video's
  attached tags (ancestor-expanded to canonical names) and the raw resolved `genres` union (deny-list
  filtered), not the raw union alone. Curating a video's tags — adding, removing, merging, denying — changes
  what a subsequent writeback writes to the file's `Genre` tag.

None of this introduces a new `metadata-mappings.yaml` key or `holodex.yaml`/env setting — it rides the
existing `genres` canonical field unchanged. See [ADR-075](../architecture/ADR-075-tag-governance-and-video-enrichment.md)
and the [F50 spec](../specs/tag-governance-and-video-enrichment.md) for the full mechanism.

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
**computed** field (`winning_source` = `computed:<canonical>`, F45) shows **no badge**;
instead the value carries a "calculated from …" **hover tooltip** (`title` + `aria-label`)
naming its `derived_from` inputs.
