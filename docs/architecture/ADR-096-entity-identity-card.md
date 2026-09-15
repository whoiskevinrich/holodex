# ADR-096: Entity identity card — one reference, one external-id store, films in the spine, edition as a file field, display name as a decision

**Status:** Proposed
**Date:** 2026-09-13
**Deciders:** Project owner (brainstorm + design review 2026-09-12; spec questions and the exiftool verification 2026-09-13)

**Revisits:** [ADR-061](ADR-061-unified-entity-name-identity.md) — the **entity set** (Person/Studio/Tag →
adds **Film**) and **D1's key shape** (for Film only, `nameKey` becomes composite with `year`); D2–D7 and
the tag scoping call ("identity spine, not the field-resolution model") stand and are *upheld* here
against the spec's first draft (D5 below). [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) —
the one field its decision model never covered: `name` (rejected in code at `person_decisions.go:118`,
`studio_fields.go:104`, `film_fields.go:237`); D5 admits it for Person/Studio/Film.
**Extends:** [ADR-054](ADR-054-studio-external-id-dedup.md) / [ADR-055](ADR-055-enrichment-unique-key-invariant.md)
(external-id-first resolve — D2 moves the per-kind tables onto one polymorphic store and fills ADR-055's
anticipated `tag_external_ids` row without a new table) · [ADR-083](ADR-083-provider-link-badge-person-studio.md)
(the badge now projects the unified table) · [ADR-041](ADR-041-metadata-writeback.md) /
[ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) (edition is one more `formatMap` row with a
verified read-back key) · [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md) /
[ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md) (the `filename:edition` candidate rides
the existing auto-apply / queue routing) · [ADR-090](ADR-090-two-layer-entity-metadata-management.md)
(edition and display name are *precedence-layer* decisions, never adoption rows).
**Relates to:** [ADR-085](ADR-085-films-entity.md) / [ADR-089](ADR-089-film-enrichment-field-vocabulary.md)
(the entity D3 brings into the spine) · [ADR-033](ADR-033-metadata-source-plugins.md) (no provider
contract change — edition has no provider source) · [ADR-030](ADR-030-access-control-gating-seam.md)
(every new mutation stays behind `requireOwner`).
**Spec:** [Entity identity card (F60)](../specs/entity-identity-card.md) — RD1–RD12.
**Design:** [entity-identity-card-handoff.md](../design/entity-identity-card-handoff.md) + mockup.
**Issue:** [HOLODEX-373](https://whoiskevinrich.atlassian.net/browse/HOLODEX-373) (epic) ·
[374](https://whoiskevinrich.atlassian.net/browse/HOLODEX-374) reference · [375](https://whoiskevinrich.atlassian.net/browse/HOLODEX-375)
external ids · [376](https://whoiskevinrich.atlassian.net/browse/HOLODEX-376) films · [377](https://whoiskevinrich.atlassian.net/browse/HOLODEX-377)
edition · [378](https://whoiskevinrich.atlassian.net/browse/HOLODEX-378) display name.

---

## Context

Five recurring confusions were traced, in one brainstorm, to a single gap: an entity's only *surfaced*
identity is its name. An inventory of the four entity kinds showed the underlying concepts mostly
exist — but unevenly, and with one structural duplication:

| | Person | Studio | Tag | Film |
|---|---|---|---|---|
| Name-identity spine (ADR-061) | ✅ | ✅ | ✅ | **✗** — not in migration 0022's set, no trigger, `canonicalTable` returns `""`, uniqueness is `UNIQUE(name, year)` |
| Provider id as *identity* (consulted before name at scan) | `person_external_ids` | `studio_external_ids` | ✗ | ✗ — "which film is `tmdb:603`?" has no query |
| Provider id as *re-enrich memo* | `entity_enrichment.external_id` | same | same | same |
| Display name distinct from canonical | ✗ | ✗ | ✗ | ✗ — every kind rejects a decision on `name` |
| A stable handle other than the name | numeric URL only | | | |

Two things follow. First, provider ids are stored **twice with two meanings** — `externalIDTable()` in
`internal/repo/identity.go` hard-codes person+studio for the identity store while `MatchExternalID`
reads the memo column — so a reader cannot tell which is "the" TMDB id, and two kinds have only the
memo. Second, Film — added by ADR-085 after ADR-061 shipped — is the one kind outside the spine, and
it is also the kind where name-only matching gives *wrong* answers rather than awkward ones: *Superman
II* (1980) and *Superman II: The Richard Donner Cut* (2006) are two TMDB movies, and a file titled
"Superman II" can land on either.

The owner's own example exposed a gap the original ask did not cover at all: **editions**. *Blade
Runner* theatrical and *Blade Runner: The Final Cut* are **one** TMDB movie with two files; nothing
distinguishes the files today except whatever the filename happened to say. None of slug / display
name / alias / external id addresses this — it is a "what is a Film?" question.

Constraints that shaped the answer: the file layer is the baseline truth and provider data is an
additive shadow (ADR-033/052); the pure resolver is the single merge point and standing decisions
override at resolve time (ADR-051); file writes are atomic and batched per file (ADR-041); Holodex
writes tags, never filenames; and the owner actively vetoes scope that "will never be completed" —
so a new subsystem was off the table, and each piece had to be an *extension of a seam that exists*.

Two facts were verified rather than assumed, because a first pass got one of them wrong:
exiftool reads a Matroska `EDITION` SimpleTag back as `Matroska:Edition`; and MP4/MOV has **no**
writable `QuickTime:`/`ItemList:`/`Keys:` edition atom — a bare `-Edition=` lands in an embedded XMP
packet as `XMP-prism:Edition`, which reads back as `Edition`. (The earlier "verified" claim came from
`exiftool -listw -ItemList`, which does not filter by group. Only a write-then-read is verification.)

## Decision

Five sub-decisions. Together they give every entity one uniform, surfaced identity card without adding
a subsystem: each is a new row in a table, a new kind in a registry, or a new field in a mapping.

### D1 — A server-produced reference `kind:id` is the handle; slugs are cut

Every entity payload — person, studio, tag, film, video; detail and list items — carries
`ref = kind ":" id` (`film:42`). Every `{id}` route segment and every MCP tool id argument accepts a
ref wherever it accepts a bare id; a ref whose kind does not match the route is **400** (malformed
request — the row may well exist), never 404. One parser (`internal/api/ref.go`) serves both the HTTP
router and the MCP argument decoder; the SPA never assembles a ref, it renders the one it was given.

Slugs were considered (§Options) and cut: a slug either breaks on rename or needs a redirect table,
and its only advantage over `kind:id` — readability — is exactly the property that makes it ambiguous
again once two entities share a name. URLs stay numeric.

### D2 — One polymorphic `entity_external_ids` table replaces two per-kind tables and the memo column

```sql
CREATE TABLE entity_external_ids (
    entity_type TEXT    NOT NULL,   -- 'person' | 'studio' | 'tag' | 'film'
    entity_id   INTEGER NOT NULL,
    external_id TEXT    NOT NULL,   -- '<provider>:<id>'
    PRIMARY KEY (entity_type, external_id)
);
```

Shaped like `entity_aliases` (0022): global uniqueness **per entity_type**, per-kind AFTER DELETE
cleanup triggers (a polymorphic table has no FK cascade — the lesson ADR-061 recorded). `person_external_ids`
and `studio_external_ids` fold in and are dropped. `entity_enrichment.external_id` is dropped once
`MatchExternalID` reads the new table — the "which id did we last enrich against" question is answered
by the same row, so the memo was never a second fact, only a second copy.

**The resolve order does not change** — ADR-061 D5 (provider id → name key → alias → create) is
*generalised*, not revisited: `resolveOrCreateByName` consults the unified table for all four kinds,
and `externalIDTable()`'s person/studio special-casing goes away. The invariant ADR-036/F23 taught —
an external id is consulted **before** the name so a merge survives rescan — is carried by a named
test in the story, not by prose.

Tags are in the type set even though nothing enriches them today; that is ADR-055's anticipated
`tag_external_ids` row delivered as one string in a CHECK-free column rather than a table.

### D3 — Film joins the ADR-061 spine on a composite key; rename and alias follow the shared path

Film enters `entity_aliases` / `entity_keep_separate` / `entity_alias_suppressions` /
`identity_review_queue` as `entity_type='film'`, gains a cleanup trigger, and `canonicalTable('film')`
returns `films` so the shared `RenameEntity` works (old spelling → alias, year unchanged).

ADR-061 D1 says `nameKey = normalize_<entity>(name)`. For Film that is **not** the identity — two
films with the same name and different years are legal and must remain so (Superman II 1980 / 2006).
Film's key is therefore composite: `filmKey = (lower(trim(name)), year)`, enforced by
`ux_films_namekey`. The alias-routing consequence is the one new rule: an alias routes to a film only
when its year matches the film's, **or** the alias carries no year and exactly one film has that
name key. Any other case goes to the near-miss queue and is never auto-routed — the same
"detect + prompt, never auto-merge homonyms" posture ADR-061 took for people.

Provider alternative titles land as `source='provider'` aliases (the 0044 pattern), so films get the
"also known as" row people already have; `AliasPanel` is entity-generic since F43 S2 and mounts as-is.

### D4 — Edition is a property of the file, modelled as an ordinary canonical video field

**Film = the work as the provider defines it. Edition = a property of a file, never of the Film.**
One TMDB movie → one Film → N full-film files told apart by edition; two TMDB movies → two Films
(D2's job). No Release/Version entity between Film and Video; if grouping ever matters, group by label.

Edition is a canonical field in `metadata-mappings.yaml`, `replace`, single-value, manual allowed:

```yaml
- canonical: edition
  sources:
    - Edition                 # container tag — baseline, wins
    - filename:edition        # {edition-X}, strict Plex grammar (F48 candidate)
```

List order is precedence, exactly as `release_date` already does it. The filename source is an F48
candidate routed by the **existing** auto-apply flag and confidence threshold (ADR-066/067) — an exact
`{edition-X}` is high-confidence; no special rule for one field. No provider source: TMDB has no
edition concept, and inventing one on the sidecar contract (ADR-033) would be a new subsystem.

It inherits ADR-051 curation and is writeback-eligible through the existing dialog. The
`formatMap` rows are `"edition": "Edition"` (Matroska/WebM — `EDITION` SimpleTag in GENERAL via
mkvpropedit) and `"edition": "XMP-prism:Edition"` (MP4/MOV — an XMP packet, the only writable
destination exiftool offers). Both read back as `Edition`, which is the mapping's `file:` key, so
ADR-093's tri-state `in_sync` and its startup WARN cover it with no new mechanism. The MP4 tag's only
consumer is Holodex itself — Plex and Jellyfin read editions from filenames alone — so XMP is
acceptable; `Subtitle` was rejected because it is already the tagline's key (`tags.go:101`).

**Completeness — revisits ADR-081 D1 (2026-09-14).** ADR-081 gives `FieldDef.Criticality` three
values: `""` (excluded, not listed), `critical`, `nice_to_have`. Edition needs a fourth,
**`optional`**: the facet is *listed* — tier, label, and a `curatable` flag (true for a plain-text
replace field) — but carries no weight, never counts as missing, and never enters the remediation
queue or the breakdown panel. Two reasons. First, on the requesting library editions sit on media
rather than films and most files have none; a `nice_to_have` facet would have marked most of the
library incomplete and filled the queue with rows no provider can answer. Second, the media page's
`#field-<canonical>` landing needs *something* that says the field exists and is curatable when the
resolver has dropped it for being empty and undecided — the facet is the only payload that does,
so it must stay listed. ADR-081 D1's "everything else stays `""`" rule is otherwise unchanged;
`optional` is reserved for fields that are legitimately empty on most entities.

Two UI consequences, both reuse: the media page renders edition through the generic field row and
`SourceBadge`; and `SourceBadge` must render its badge for **any curatable field, not only
multi-source ones** — a filename-only edition would otherwise have no affordance for a custom value.
That one-condition change applies to every single-source curatable field and is recorded in
`curation/CLAUDE.md`. The film page's Full-film list renders the resolved value as a pill and, when
empty, a deep link to `/media/{id}#field-edition` with the badge pre-expanded — one curation mount,
one navigation (design OQ1, ratified).

### D5 — Display name is a decision on `name` for Person, Studio and Film; Tag stays out

ADR-051's model never covered `name` — every kind rejects the decision in code with "rename instead".
D5 admits it: `name` becomes an ordinary resolved field whose sources are the **file baseline (the
canonical column)**, each provider's spelling, and custom. The *rendered* name is the resolved value.
The *canonical* column is untouched by any decision and remains the file / writeback / identity /
alias-routing / MCP truth — nothing but headers, cards, search-result rows and the search index reads
the resolved value. No `display_name` column: display is a decision, not a fact about the entity.

Two verbs map onto two affordances that already exist — no new buttons: **Display as** is the name
field's `SourceBadge`, on an "In files as `<canonical>`" line rendered only when resolved ≠ canonical
(so the at-rest header is unchanged, per HOLODEX-268); **Rename in files** is `NameEditControl`'s
pencil, unchanged, prefilled with the canonical value. Design review ratified that the split reads at a
glance (OQ2); the kill criterion — a first-time reader picks the wrong verb — stays live at QA.

**Tag is excluded.** The spec's first draft included it; this ADR upholds ADR-061's scoping call
instead: tags carry the identity spine and **not** the field-resolution model, because a tag's name
*is* the entity and the decision machinery would be ceremony. And the one tag-side case that was
floated as a motivation — casing, lowercased in storage since migration 0034 — is **policy, not a
regret**: the owner's rule is that tags are always lowercase. A display-name decision on a tag would
be a way to defeat that rule one tag at a time. HOLODEX-379 (which proposed reversing 0034) is closed
Won't Do.

Film's provider spelling is the sidecar's `title` key (ADR-086 §3), so a film `name` candidate is
`<provider>:title`. [ADR-089](ADR-089-film-enrichment-field-vocabulary.md) D3 kept film `name`
baseline-only because a provider source then meant an ungated rename of half the `(name, year)` key;
D5 admits the provider spelling as a *display* candidate only — the column is never written by a
decision — so D3's rule now binds the column, not the candidate list.

## Options Considered

### D1 — the handle

| Option | Complexity | Stability | Ambiguity | Verdict |
|---|---|---|---|---|
| **A. `kind:id` reference (chosen)** | Low — one parser, additive payload field | Stable across rename | None — id is the PK | ✅ |
| B. Slug column + route | Med — redirect table on rename or broken links | Breaks on rename | Reappears the moment two entities share a name | ✗ |
| C. Opaque UUID | Low | Stable | None | ✗ — no benefit over the PK; unreadable in chat, which is the use case |
| D. Nothing (status quo) | — | — | The problem | ✗ |

### D2 — where provider ids live

| Option | Complexity | Consistency | Verdict |
|---|---|---|---|
| **A. One polymorphic table, drop the memo column (chosen)** | Med — one migration, one fold, drop two tables | One fact, one place; all four kinds | ✅ |
| B. Keep per-kind tables, add `film_external_ids` + `tag_external_ids` | Med — two new tables, `externalIDTable()` grows | Four tables saying the same thing; memo column still a second copy | ✗ |
| C. Keep the memo column as a per-provider "last enriched" cache | Low | Two copies drift — the confusion this ADR exists to remove | ✗ |

### D3 — Film identity

| Option | Fidelity to TMDB | Complexity | Verdict |
|---|---|---|---|
| **A. Composite `(nameKey, year)` key + year-aware alias routing (chosen)** | Two same-name films are two rows, as the provider says | Low — one index, one routing rule | ✅ |
| B. Name-only key, year as a disambiguating alias | Forces Superman II 1980/2006 into one row or an alias tangle | Med | ✗ — wrong by construction |
| C. Separate film-identity subsystem outside the spine | — | High — a second alias/merge/queue | ✗ — the owner's "never completed" veto |

### D4 — where edition lives

| Option | Model fit | Complexity | Round-trip | Verdict |
|---|---|---|---|---|
| **A. Canonical video field: tag > filename, curation, writeback via tag (chosen)** | Edition is a file fact | Low — mapping row, formatMap row, one `SourceBadge` condition | Yes, both containers (verified) | ✅ |
| B. Release/Version sub-entity between Film and Video | Over-models; every film gets a layer it doesn't need | High — new table, new pages | — | ✗ |
| C. `edition` column on `videos`, owner-set only | Loses the file source; DB rebuild loses it | Low | No | ✗ |
| D. Rename the file to carry `{edition-X}` | Other players would read it | High — Holodex has never renamed files; atomicity + security surface | Yes | ✗ |
| E. Reuse `Subtitle` as the tag key | — | Low | Collides with `tagline → Subtitle` | ✗ |

### D5 — display name

| Option | New concepts | Provider spelling free | Verdict |
|---|---|---|---|
| **A. Decision on `name` (chosen)** — file / provider / custom sources | None — ADR-051 as-is | Yes — pick `tmdb` in the chip row | ✅ |
| B. `display_name` column + owner text box | A third name (canonical / display / key) with its own writeback/search/alias questions | No — retype it | ✗ |
| C. Curation-only (custom source, no provider chips) | None | No | ✗ — half of A for the same cost |
| D. Include Tag | Gives tags the field-resolution model ADR-061 deliberately withheld; would let per-tag display defeat the always-lowercase rule | — | ✗ |

## Trade-off Analysis

- **Reuse over reach.** Every decision is an extension of a seam that exists (the alias table's shape,
  the mapping's list-order precedence, `SourceBadge`, `RenameEntity`, `formatMap`). The price is that
  a few of those seams grow a case — `resolveOrCreateByName` gains a kind, `SourceBadge` gains a
  condition, `nameKey` gains a composite form. That is cheaper than any parallel mechanism and it is
  what keeps a five-story epic finishable.
- **One fact, one place, at migration cost.** D2 is the only decision that *removes* something. The
  fold is mechanical (two `INSERT … SELECT`s), but the reader diff touches identity, enrichment and
  the ADR-083 projection at once — so the F23 precedence test is written first, and the memo-column
  drop may trail in its own migration if the reader diff is large.
- **Film's composite key buys correctness and costs one routing rule.** The "no year and exactly one
  candidate" clause is the only place the spine's behaviour differs by kind; it is queued, not
  guessed, when ambiguous — consistent with the homonym posture elsewhere.
- **XMP for MP4 is the honest compromise.** It round-trips and it is the only writable target;
  nothing external reads it, and nothing external *would* read a QuickTime atom either. Recorded so
  no one re-litigates it with the same flawed `-listw` check.
- **Display-as reuses the chip row, which means `name` is now Tier-2-editable on a Tier-1 field.**
  The "In files as" line only appears when a decision stands, so the at-rest surface is unchanged;
  the kill criterion guards the rest.
- **Upholding ADR-061 on tags narrows the spec.** Deliberate: the alternative is the first crack in
  "same identity spine, not the same field model," and the tag need has a better home.

## Consequences

**Easier**
- An owner or agent names an entity once — `film:42` — and every surface (chat, Jira, API, MCP) agrees.
- A film matched to a provider *stays* that film across rescan and re-enrich; the Superman II
  coin-flip is gone by construction.
- Films get alias / rename / keep-separate / near-miss review with no new UI.
- Two files of one film are distinguishable on the film page, from the file or from the owner, and the
  distinction survives a database rebuild via the tag.
- A provider's spelling can be shown without touching the file.

**Harder / to watch**
- One migration folds two tables and (eventually) drops a column; the resolve precedence test is
  the guard — write it first.
- `SourceBadge`'s single-source badge now appears on every curatable field, not just edition; a
  visual pass across the media page is part of 377's QA.
- The composite film key means film-side alias APIs carry a year where person/studio ones don't;
  the shared `entity_identity.go` route config needs a film branch rather than a copy.
- MP4 edition lives in XMP; if a future player ever reads an MP4 edition atom, a second `formatMap`
  row is a one-liner — but the read-back key must stay `Edition`.

**Revisit when**
- Tags become enrichable (D2 already holds the row; D5 would need its own ADR — do not fold it in).
- A second provider disagrees with TMDB on whether a cut is a separate work — then "film = the work
  as the provider defines it" needs a provider-precedence rule, which ADR-051 can express.

## Action Items

1. [x] **374** `internal/api/ref.go` parser + `ref` on every entity payload + MCP arg decoder; `RefChip`
   mounted per the handoff §1a; tests for kind mismatch (400) and byte-identical bodies.
2. [x] **375** migration (number assigned against `origin/main` at commit time — 0045 was latest on
   2026-09-12): `entity_external_ids`, fold, drop; generalise `resolveOrCreateByName` /
   `identityQueryByType`; `GetFilmByExternalID`; ADR-083 projection reads the new table. **The F23
   precedence test first.** Decide in-PR whether `entity_enrichment.external_id` drops in the same
   migration.
3. [x] **376** `ux_films_namekey`; film in the 0022 type set + trigger; `canonicalTable('film')`;
   year-aware alias routing + queue on ambiguity; `AliasPanel` + `NameEditControl` on the film page.
   (Shipped 2026-09-14, migration 0047. The "film-side alias APIs carry a year" consequence did
   not materialise: aliases stay year-less and the year rule is applied where a title is routed —
   `CreateFilm` — so the route config needed only a film branch, no new request shape.)
4. [x] **377** mapping row (+ `.example`); extractor surfaces `Edition` for MKV/WebM/MP4/MOV; F48
   `{edition-X}` parser; `formatMap` rows (`Edition` / `XMP-prism:Edition`); `SourceBadge` renders for
   curatable single-source fields (+ `curation/CLAUDE.md`); video summary carries `edition`; film-page
   pill + deep link. Round-trip test on **both** containers. (Shipped 2026-09-14. Two things the
   design did not foresee: the F48 marker has to be *lifted* out of the stem before the `^…$`
   patterns run, or `{title}` swallows it; and a file with neither tag nor marker has no `edition`
   row for the deep link to land on — the media page now renders a deep-linked missing field as an
   empty curatable row from its completeness facet, gated by a new `curatable` flag so image /
   long-text / merge fields are never synthesised that way. Owner ruling 2026-09-14: edition is a
   `CriticalityOptional` facet — listed, never scored/queued — because on the requesting library
   most media legitimately has none.)
5. [x] **378** lift the three rejections for person/studio/film; "In files as" line + badge; pencil
   prefills canonical; writeback payload carries canonical; search indexes resolved + canonical + aliases.
   (Shipped 2026-09-15. Scope ruling: the resolved name renders on the three detail headings and the
   search row only — cards, tiles and pickers keep the canonical column; search rows carry
   `display_name` *beside* `name` because the pickers read the same endpoint and send `name` back.
   Search matches the display spelling through a narrow SQL mirror of the decided-replace rule rather
   than an index. Film's provider spelling is the sidecar's `title` key. The handoff QA 4.3 kill
   criterion is still the owner's to run.)
6. [x] Spec RD9 / P0 §378 and the design handoff §4d: strike Tag (this ADR's D5).
7. [x] Add the ADR-096 row to `README.md`; note in ADR-061's index row that its entity set is revisited here.
8. [x] `/testing-strategy` per slice; `/security-review` on the implementation diff (new writeback key,
   new mutation surface on `name`, film identity routes).
