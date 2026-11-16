# Spec: Entity Completeness Score (F55)

**Status**: Shipped. The completeness score, the extras number, the ring badge and the Completeness
sort were retired by [F90](provider-and-field-filters.md)
([HOLODEX-626](https://whoiskevinrich.atlassian.net/browse/HOLODEX-626)). What remains: the owner's
remediation queue, the per-entity field list, the missing-field filter, field criticality and the
not-applicable marking.
**Phase**: 4 (Curation tooling)
**Issue**: [HOLODEX-260](https://whoiskevinrich.atlassian.net/browse/HOLODEX-260) (F55) ·
[HOLODEX-412](https://whoiskevinrich.atlassian.net/browse/HOLODEX-412) (F65, the criticality revision)
**Depends on**: per-field source decisions and the file baseline
([field-resolution.md](../architecture/field-resolution.md)), provider enrichment
([metadata-providers.md](../architecture/metadata-providers.md)) and the owner gate
([security-perimeter.md](../architecture/security-perimeter.md)).
**Architecture**: [field-resolution.md](../architecture/field-resolution.md), the sections on
completeness criticality, not-applicable, and the missing-field cache.
**Design handoff**: [entity-completeness-handoff.md](../design/entity-completeness-handoff.md) (queue,
field list, missing-field filter). The ring-badge handoff was retired with the ring.

---

## Objective

Help the owner find the specific holes in the library's metadata and fill them one at a time. The
owner asks "which videos have no poster?", gets a list, and works through it.

There is no score. An entity isn't rated. Each of its fields is simply present, missing or not
applicable. Ranking by how much providers contributed is F90's **Most / Least enriched** sort and
**Enriched** column ([provider-and-field-filters.md](provider-and-field-filters.md)), not this spec.

## Personas

- **Owner**: the only person this feature serves. Works the queue, reads the field list, filters
  lists by a missing field, and marks a field not applicable.
- **Visitor**: sees none of it.

## User stories

1. As the owner, I want a queue grouped by missing field, split into "ready to apply" and "needs
   research", so I can do the quick wins first and know which gaps need me to go find something.
2. As the owner, I want to apply a provider's cached value straight from the queue, so a quick win
   doesn't need a trip to the entity's page.
3. As the owner, I want to filter a list to the entities missing one field, such as "Missing
   poster", so I can target one kind of gap at a time.
4. As the owner looking at one entity, I want to see which of its fields are present, missing or not
   applicable, and jump straight to an empty one.
5. As the owner, when a video truly has no external provider id (home or self-published content), I
   want to mark that field not applicable so it leaves my queue for good.
6. As the owner, I want fields that will never fill for my library to stop showing up as gaps.

---

## Fields that count

Each entity type has a fixed set of fields that can be **missing**. Each has a **criticality**:

- **Critical**: the gaps that matter most. Critical groups come first in the queue.
- **Nice to have**: a real gap, listed after the critical ones.
- **Optional**: never counts as missing. It isn't in the queue, the missing-field filter or the field
  list. A link straight to an optional field still opens an empty row to fill.
- **Excluded**: not part of this feature at all.

A field is **present** when the entity's page shows a value for it, from any source: the file, a
provider or the owner. It is **missing** otherwise. Which criticality a field has is a product call,
made in the tables below. Technical file properties, such as codec or frame rate, never count.

### Video

| Field | Criticality | Notes |
|---|---|---|
| Title | Critical | |
| Studio | Critical | |
| Actors | Critical | |
| Poster | Critical | A real poster, from the file's cover art, a provider or the owner. |
| Overview | Nice to have | |
| Release date | Nice to have | |
| Genres | Nice to have | |
| External provider id | Nice to have | The one field that can be marked not applicable. |
| Original title, tagline, status, original language, homepage, collection | Optional | Theatrical-release details that stay empty for most of the owner's content. |
| Director | Optional | Rarely applies to the owner's content. |
| Runtime | Optional | Always present from the file, so it says nothing. |
| Edition, part | Optional | |

### Person

| Field | Criticality | Notes |
|---|---|---|
| Photo | Critical | |
| Bio | Nice to have | |
| Birthdate | Nice to have | |
| Nationality, alternate names | Optional | Most people legitimately have no alternate names. |
| Website, deathdate | Excluded | Low signal, or absent for most (living) people. |
| Age, age at death | Excluded | Computed from the birthdate. |

### Studio

| Field | Criticality | Notes |
|---|---|---|
| Branding image | Nice to have | One field: present when the studio has **any** of an icon, logo or poster. |
| Description, country | Optional | |

Studios have no critical fields.

### Excluded fields

A field is excluded when it can't carry a useful "gap" signal:

- **Manual-only fields with no source.** Having no value is a normal state for them.
- **Computed fields**, such as age. They always follow their source field.
- **Low-signal or usually absent fields**, such as a person's website or deathdate.

## Not applicable

Some entities can never have a value for a field. The owner can mark a video's **external provider
id** not applicable. That is the only field with this toggle today.

- A field marked not applicable leaves the queue and the missing-field filter for that entity.
- The field list shows it as **Not applicable**, and the owner can undo the mark.
- It doesn't change F90's "has no value" condition: a field marked not applicable with no value
  still has no value there ([F90](provider-and-field-filters.md), RD4).
- An external provider id names its provider, for example `provider-1:603`, so the one field works
  whichever provider supplied it.

---

## Requirements

### Must-have (P0)

**P0-1 Criticality.** Every field that can be missing has the criticality in "Fields that count".
- [ ] A video with no title is missing a critical field.
- [ ] A video whose only empty fields are optional is missing nothing.
- [ ] A person with no deathdate is missing nothing.

**P0-2 Remediation queue.** An owner-only page lists entities grouped by missing field, critical
groups first. Each group is split into **ready to apply** (a provider's value is already cached and
not yet applied) and **needs research** (nothing cached).
- [ ] A "Missing poster · 9" group shows ready-to-apply and needs-research rows that add up to 9.
- [ ] No optional or excluded field ever forms a group.

**P0-3 One entity at a time.** Ready-to-apply rows offer **Apply**. Needs-research rows offer
**Search** and **Upload**. There is no bulk action.
- [ ] Applying a row's cached value fills that field and removes the row.
- [ ] No "select all" or "apply all" control exists on the page.

**P0-4 Missing-field filter.** Owner list pages for videos, people and studios offer a **Missing
field** filter. It lists the fields that entity type can be missing, each with a count.
- [ ] Choosing "Missing poster" on the video list shows exactly the videos missing a poster.
- [ ] The filter's count for a field equals the queue's count for the same field.
- [ ] A field no entity is missing isn't offered.

**P0-5 Field list on the detail page.** Video, person and studio pages show the owner the entity's
fields behind a **Show fields / Hide fields** toggle. Each row reads **Curated** (from the file or
the owner), the provider's badge (from a provider), **Missing** or **Not applicable**. There is no
score or percentage.
- [ ] A video whose title comes from its file and which has no director credit shows title as
      Curated, and no director row (director is optional).
- [ ] A video with no actors shows actors as Missing.
- [ ] Choosing a missing field jumps to that field, ready to fill.

**P0-6 Not-applicable toggle.** The owner can mark a video's external provider id not applicable,
and undo it.
- [ ] Marking it removes the video from the "Missing external provider id" group and filter.
- [ ] A visitor's attempt to mark it is refused.

**P0-7 Answers stay current.** After the owner changes a value, source decision or link, the queue,
the filter and the field list reflect it on the next read, with no restart or manual rebuild.
- [ ] Curating a poster on a video removes it from "Missing poster" on the next read.
- [ ] While the start-up refresh is running, lists still load, and any lag ends when it finishes.

**P0-8 Owner-only.** Everything here is owner-only, enforced by the server as well as hidden.
- [ ] A visitor sees no queue, no missing-field filter, no field list and no not-applicable toggle.
- [ ] A visitor's list responses carry no missing-field data.

**P0-9 Cinémathèque.** Every surface here uses the theme's tokens only
([theming.md](../design/theming.md)), in its loading, empty and populated states.

### Future considerations (P2)

- **Not applicable on more fields**, such as a person's deathdate or a studio's country.
- **Bulk apply from the queue**, once the single-apply queue has a track record.
- **Owner-tunable criticality** per library, instead of the fixed tables above.

---

## Access control

- Every surface is owner-only, behind the existing owner gate, like the extraction queue.
- The not-applicable toggle is the only write this feature adds. It is an owner-only flag on one field.
- Nothing new is fetched from providers. The queue reads values providers already supplied.

---

## Decisions

- **No score.** F90 replaced the percentage, ring and Completeness sort with filters that say which
  entities match, and an enriched count for ranking. A saved sort preference, shared link or saved
  playlist that used the Completeness sort opens with the list's default sort, with no error.
- **Presence is binary.** A provider's value and the owner's curated value count the same. Where a
  value came from shows on the field's row; it doesn't change whether the field is missing.
- **Fields that never fill are optional, not missing.** The theatrical-release fields, director and
  runtime on videos, nationality and alternate names on people, and description and country on
  studios are optional, so the queue describes the owner's library rather than a provider's catalogue.
- **Single apply only.** The queue has no track record yet, so bulk apply waits, as it did for the
  extraction queue ([HOLODEX-199](https://whoiskevinrich.atlassian.net/browse/HOLODEX-199)).
- **One external provider id field.** It holds one value that names its provider, rather than one
  field per provider.
