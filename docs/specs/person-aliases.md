# Spec: Person Aliases (F23)

**Status**: Accepted
**Phase**: 3 (Enrichment foundation — first slice)
**Depends on**: Phase 1 search (ADR-017), the owner gate (ADR-030).
**Related**: detailed realization of [Phase 3 F14.1](phase-3-enrichment.md); distinct from the
provider-sourced `aliases` enrichment field (F22, [ADR-033](../architecture/ADR-033-metadata-source-plugins.md)).
**Architecture**: [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md).
**Design handoff**: [`docs/design/person-aliases-handoff.md`](../design/person-aliases-handoff.md).

---

## Objective

Let a person be found by **alternate names** — stage names, nicknames, maiden names, romanizations —
and let the owner **merge duplicate people** that the library extracted as separate records. The owner
curates a person's aliases by hand and can fold a duplicate person into a canonical one; **every alias
is indexed for search and honored when scanning**, so typing any name surfaces the right person and the
merge holds across re-scans.

> **Why this is needed.** People are auto-created from the person tags embedded in media files
> (`UpsertVideo` → name resolution). Files spell a person exactly one way; the same human is often
> credited under several ("Jennifer Lawrence" in one file, "J Law" in another) and ends up as two
> separate person records. Aliases are the curation layer over that machine-extracted data: an **alias
> is a name→canonical-person routing rule** honored at *both* search time and scan time, so a merge
> isn't silently undone the next time the scanner re-reads "J Law" from a file.

> **The merge example (Kevin).** A library has "J Law" and "Jennifer Lawrence". The owner merges them,
> keeping "Jennifer Lawrence" canonical and "J Law" as an alias. Searching **either** name returns the
> **union** of their media (everything credited to J Law *and* to Jennifer Lawrence). Tags will work the
> same way later (out of scope here).

---

## Scope

### In scope
- A person record supports **N aliases**. Add, view, and delete them.
- **Person merge** — fold a duplicate person into a canonical one: the duplicate's videos move to the
  canonical person (de-duped union), its name becomes an alias, and the duplicate record is removed.
  Two entry points: from a person's page ("merge another person in") and from the `/people` list
  (multi-select → choose canonical).
- **Aliases are honored at scan time.** When the scanner reads a person name from a file, it resolves
  through the alias table to the canonical person, so a merge **survives every re-scan** (the file
  still says "J Law", but it links to "Jennifer Lawrence").
- **Search matches aliases.** A global-search query matching any alias returns the canonical person
  (once). Because merge moves associations, that person's video list is already the merged union.
- **Never auto-merge on a name collision.** Same-name people can be genuinely different (two "Chris
  Evans"; Michael Keaton's legal name is "Michael Douglas"). Adding an alias that already names a
  different person surfaces that person *with context* (video count) for the owner to confirm — merge
  or keep separate. Every merge is an explicit, informed confirmation showing both video counts.
- Aliases & merge are **owner-curated**: add/delete/merge are owner-gated (ADR-030), behind the same
  `requireOwner` choke point as enrichment. Viewing aliases is public (public metadata).

### Out of scope (tracked follow-ups, not gaps)
- **Split / un-merge** — merge is one-way; deleting an alias stops *future* scan-routing but does not
  retroactively re-split already-moved associations. A dedicated split is a future feature.
- **Showing which alias matched** in search results (e.g. "Jennifer Lawrence · also known as J Law").
  Deferred to keep the search payload unchanged.
- **MCP parity** — `list_people` / `get_person` returning aliases (mirrors deferred F22.5f).
- **Promoting provider-sourced aliases to searchable.** F22 enrichment can store an `aliases` field
  from a provider; those remain display-only. Only owner-curated/merged aliases route and index.
- **Tag aliases / tag graph** (F15) — same routing model, separate feature ("will work the same way").
- **Filtering** media by alias on browse — aliases affect *global search* discovery, not the id-based
  `?person=<id>` filter. (After a merge the canonical id already owns the union, so its filter is correct.)
- **Disambiguating two real humans who share an identical tag string** — they are already one
  indistinguishable person row before any alias feature; file metadata alone can't separate them. The
  collision prompt prevents *accidental* merges; it can't resolve true homonyms.

---

## Functional Requirements

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F23.1 | A person supports N aliases, each a non-empty trimmed string, unique per person (case-insensitive). | Adding "Rob" then "rob" to one person yields a single alias; adding the same alias to two different people is allowed. |
| F23.2 | The owner can **add** an alias to a person. | `POST /people/{id}/aliases {"alias":"Rob"}` (owner-gated) creates the alias and returns the person's full alias list. Adding an existing alias is idempotent (no duplicate, no error). |
| F23.3 | The owner can **delete** an alias from a person. | `DELETE /people/{id}/aliases/{aliasId}` (owner-gated) removes exactly that alias; deleting an unknown id returns 404. |
| F23.4 | The person detail response includes the person's aliases. | `GET /people/{id}` returns `aliases: [{id, alias}, …]` (empty array when none). |
| F23.5 | **Global search matches aliases.** | After adding alias "Ziggy" to "David Bowie", `GET /search?q=zig` returns the David Bowie person. The person appears **once** even if both the canonical name and an alias match. |
| F23.6 | Aliases display on the person page; add/delete controls are owner-only. | Non-owners see the alias chips (read-only); owners additionally see an add field and per-chip delete. |
| F23.7 | Aliases cascade with their person at the DB level. | `person_aliases.person_id` has `ON DELETE CASCADE`; an alias never outlives its person row. |
| F23.8 | **Scan-time resolution.** The scanner resolves each extracted person name via the alias table before creating a person. | After "J Law" is an alias of "Jennifer Lawrence", indexing a file tagged "J Law" links it to Jennifer Lawrence — no "J Law" person is (re)created — and a re-scan keeps it merged. |
| F23.9 | **Person merge.** The owner folds a duplicate person into a canonical one. | `POST /people/{canonical}/merge {"from_id":N}` moves the duplicate's videos to canonical (de-duped union), registers the duplicate's name as an alias, drops the duplicate's shadow enrichment, deletes the duplicate. Searching either name returns the canonical person with the merged video list. |
| F23.10 | **Collision is surfaced, never auto-merged.** Adding an alias that already names a different person returns that person for confirmation. | `POST …/aliases` with a name belonging to another person → `409` carrying `{conflict:{id,name,video_count}}`; the UI offers "merge them in" or "keep separate". No silent merge, no silent routing of a homonym's files. |
| F23.11 | Merge is **owner-confirmed and informed**. | Both merge entry points show each person's video count before executing; merge and self-merge guards (`from_id == id` → 400) apply; merge is owner-gated. |

### Validation rules (F23.1)
- Trim leading/trailing whitespace; reject empty/whitespace-only (`400`).
- Cap length at **200 characters** (`400` if exceeded) — generous for any real name, bounds the row.
- Uniqueness is per `(person_id, alias)`, case-insensitive (`COLLATE NOCASE`), matching the existing
  `people.name` collation convention.

---

## Data model

```
person_aliases
  id         PK
  person_id  → people(id)  ON DELETE CASCADE
  alias      TEXT NOT NULL COLLATE NOCASE
  UNIQUE (person_id, alias)
  INDEX (person_id), INDEX (alias)   -- alias index backs scan-time resolution

person_aliases_fts   (external-content FTS5 mirror of person_aliases.alias)
  alias      indexed, tokenize="unicode61 remove_diacritics 2"
  kept in sync by ai/ad/au triggers (mirrors people_fts, ADR-017)
```

Migration **0007** (next in sequence after 0006). See [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md)
for why aliases get their own FTS table (not a denormalized column), and how the scanner's people
branch resolves names through this table (name → alias → create) so merges survive re-scans.

---

## API

All under `/api/v1`. Reads ungated; mutations inside the `requireOwner` group (ADR-030).

| Method | Path | Gating | Body | Returns |
|--------|------|--------|------|---------|
| GET | `/people/{id}` | public | — | `{person, items, total, enriched}`; `person.aliases: [{id, alias}]` |
| POST | `/people/{id}/aliases` | owner | `{"alias":"…"}` | `200 {aliases:[…]}`; **`409 {error, conflict:{id,name,video_count}}`** when the name belongs to another person |
| DELETE | `/people/{id}/aliases/{aliasId}` | owner | — | `204` |
| POST | `/people/{id}/merge` | owner | `{"from_id":N}` | `200 {person}` — `id` is canonical, `from_id` is absorbed |

Errors: `404` person/alias not found; `400` invalid alias / `from_id` missing / self-merge; `409`
alias name belongs to another person; `401` unauthorized (gated, no/invalid token).

---

## UX

Person detail page (`/people/[id]`) gains an **"Also known as"** panel (rendered in the shared
`EntityVideos` detail snippet, above the existing Enrichment panel):

- **Chips** for each alias (rounded-full pills, themed). Hidden entirely when there are no aliases and
  the viewer is not the owner.
- **Owner controls**: a text input + "Add" button; each chip shows a "✕" remove affordance; and a
  **"Merge a person in…"** button opening a `PersonPicker` modal (search a person → informed confirm
  showing both video counts → merge).
- **Collision prompt**: when the add field hits another existing person, an inline prompt — "{name}
  ({n} videos) is already a separate person. Are they the same?" → **merge them in** / **keep separate**.

People list (`/people`) gains an owner **"Merge people…"** mode: multi-select two or more, then a
**"Keep which name?"** dialog picks the canonical; the rest fold into it.

All-tokens styling; QA in all three skins. Full layout, states, and a11y in the design handoff.

---

## Non-functional

- **Search cost**: one extra FTS `MATCH` query per global search (bounded by the per-group limit),
  joined to `people`. Negligible for a personal library; same shape as the existing people/tags queries.
- **Scan cost**: one extra indexed alias lookup only for names that aren't already a person (the
  common "name hit" path is unchanged); off the unchanged-file fast path entirely (ADR-018).
- **Re-scan-safe & durable**: the scanner *reads* aliases (to route names to canonical) but never
  *writes* them; a re-scan rebuilds only `people`/`video_people` and a merge holds.

## Open questions
1. Should search results annotate the matched alias? (Deferred — see out-of-scope.) Low effort if the
   need shows up; would add `matched_alias` to the search-only person payload.
2. Should merge **move** the duplicate's shadow enrichment to canonical instead of dropping it? v1
   drops it (canonical keeps its own). Revisit if enrichment-before-merge becomes common.
3. A dedicated **split/un-merge** to reverse a mistaken merge (today: delete the alias to stop future
   routing; already-moved associations stay).
