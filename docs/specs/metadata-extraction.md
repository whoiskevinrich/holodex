# Spec: On-demand metadata extraction from filenames & tags (F48)

**Status**: Draft — pending `/architecture` (new ADR needed: auto-apply confidence gating +
writeback backup/rollback, which amends [ADR-041](../architecture/ADR-041-metadata-writeback.md)'s
deferred "prior-value capture" non-goal) and `/security-review` before merge
**Feature block**: F48
**Phase**: 3 (Enrichment)
**Date**: 2026-07-14
**Depends on**:
[F22 metadata source plugins](metadata-plugins.md) / [ADR-033](../architecture/ADR-033-metadata-source-plugins.md)
(`entity_enrichment` shadow store, namespaced sources — **this spec adds a `filename:` namespace
alongside `file:`/`tmdb:`, unchanged shape**) ·
[F27 unified field resolution](metadata-plugins.md#f27--unified-field-resolution-implements-f223)
(`internal/resolver`, precedence/merge over `orderedSources` — **a new namespace slots in with no
resolver change**) ·
[F30 metadata curation & write queue](metadata-curation.md) / [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)
(`internal/writeback.WriteBatch`, the durable `internal/writequeue`, per-file atomic writes —
**shipped; this spec's extraction and merge writes ride this queue unchanged, no new write
mechanism**) ·
[F36 field source-of-truth](field-source-of-truth.md) / ADR-051 (per-field decision/provenance
model that a queued extraction candidate ultimately resolves into) ·
[F37/F38 People/Studio source-of-truth](field-source-of-truth.md) / ADR-052 (entity-generic
resolver already treats Person/Studio/Video on one `BaselineSource` model) ·
[F43 entity name-identity](entity-identity.md) / [ADR-061](../architecture/ADR-061-unified-entity-name-identity.md)
(loose-key near-miss detection, `entity_type` merge, `keep_separate` — **this spec's exact-match
entity-resolution tier reuses the loose-key detector unchanged**; person merge itself is
[F23.9](person-aliases.md)) ·
[F47 enrichment review workflow](enrichment-review-workflow.md) / [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md)
(`Candidate.Confidence float64`, `StrongMatchThreshold`-style auto-apply-vs-review-queue routing —
**this spec's routing model directly mirrors it**, extraction is a second, non-provider source
of candidates) ·
[F35 owner tooling hub](owner-tooling-hub.md) (hosts the new Extraction tab, alongside Duplicates
and Enrichment) ·
[F41 runtime owner settings](runtime-settings.md) / ADR-060 (owner-editable filename-pattern
config at runtime, no redeploy)
**Routing**: touches the data model, introduces a new **automatic** (scan-time) file-write
trigger, and adds a backup/rollback mechanism that amends ADR-041's stated deferred non-goal →
**`/architecture`** required (new ADR, next number after 066); **`/security-review`** required
(owner-gated, new write-trigger surface, filenames are untrusted string input); **`/testing-strategy`**
must gain extraction/confidence/rollback cases; frontend uses semantic tokens, QA'd in all three
skins.

---

## Problem Statement

Filenames frequently encode metadata the embedded file tags are missing or have wrong — a
library convention like `[Studio] Title (Person, Person, Year) Resolution.mp4` carries Title,
People, Studio, and Release Date — but Holodex never reads it. Today the only filename-derived
signal is the scanner's bare fallback (using the filename stem as `Title` when tags are empty);
there is no structured parsing, and nothing reconciles filename data against tag data when they
disagree. Separately, when the owner corrects data *inside* Holodex — merging two Person records,
renaming a Studio — that decision never propagates back to the file: person merge today is
DB-only ([person-aliases.md](person-aliases.md) F23.9 moves videos and drops the loser, but never
touches a tag). The result is two one-way gaps: good data trapped in filenames never reaches the
app, and good decisions made in the app never reach the files.

At the volume the owner is operating at (thousands of files), neither gap can be closed by manual
review of every file. This spec adds filename parsing as a new resolver source, a confidence
model that auto-applies the unambiguous cases and queues only the genuinely uncertain ones, and a
writeback path for merge/rename decisions — all riding the existing F30 write queue rather than
introducing a new write mechanism.

---

## Goals

1. **Filenames become a real source.** A configurable, owner-editable filename pattern parses
   Title/People/Studio/Release Date out of the filename into a new `filename:` namespace,
   available to the resolver exactly like `file:`/`tmdb:` today.
2. **The unambiguous majority requires zero clicks.** A field whose filename and tag data agree
   (or where only one has data) at high confidence writes automatically; only genuine conflicts
   or low-confidence extractions reach the owner.
3. **Review doesn't scale with library size.** The owner works a bounded queue of actual
   ambiguities, not a per-file gate — reusing the F47 queue pattern (Owner Hub tab, per-row
   resolve-on-click, durable dismissal).
4. **Person/Studio merges reach the files.** Merging two People (or two Studios) enqueues a
   tag-rewrite writeback for every affected video, automatically, with no second confirmation step
   beyond the merge's own informed-confirm.
5. **Every batch of writes is reversible.** A pre-write snapshot plus an inverse-operation revert
   lets the owner undo a bad sync — closing the gap ADR-041 explicitly left open.

### Success Metrics

**Leading**

- A fixture library with known filename/tag agreement, single-source, and conflicting cases
  routes each field to the correct auto-apply/queue outcome per the confidence model below
  (unit-tested, no network — extraction is pure local parsing).
- On-demand ("Extract from filename"), batch ("Extract all"), and import-time (scan) triggers all
  produce identical extraction results for the same file (one code path, three call sites).
- A person merge on a fixture library with N affected videos enqueues exactly N writeback jobs and
  every affected file's tag reflects the canonical name after the queue drains.
- A revert restores the exact prior tag value for every field in a batch (byte-for-byte on the
  written field, verified against the pre-write snapshot).

**Lagging**

- The extraction review queue's backlog trends toward zero as the owner works it once per library,
  not per file (queue depth stays proportional to genuine ambiguity, not to library size).
- Fewer manually-retyped fields over time as filename extraction absorbs the "fill in the blanks"
  work the owner is doing today with a separate tool.

---

## Non-Goals

- **Filename rewriting / a rename schema.** This spec treats the filename as **read-only input**.
  Writing a new filename (e.g. after a person merge, or to reflect a curated title) is a distinct,
  more complex feature (resolution-string composition, missing/optional-field handling) — tracked
  as its own follow-up (see [Cross-references](#cross-references-to-update-on-landing)).
- **Phonetic (Soundex/Metaphone) matching.** Consistent with F43's own deferred P2 near-miss
  refinement — not reopened here.
- **Fuzzy matching as an auto-apply signal.** Jaro-Winkler similarity (below) is advisory only —
  it ranks candidate suggestions for the *owner* to confirm in the review queue. It never
  contributes to crossing the auto-apply threshold; only an exact loose-key match
  ([F43](entity-identity.md)) can do that. This is a deliberate, explicit decision (see
  [Resolved Decisions](#resolved-decisions) #3).
- **Poster extraction.** Filenames don't carry image data. Poster/cover-art continues to use the
  existing embedded-artwork detection (exiftool `Cover Art`/`Artwork`, MKV attachments — already
  shipped). Not touched by this spec.
- **Movie / Scene Number / Genre as first-class entities.** Tracked separately in
  [HOLODEX-191](https://whoiskevinrich.atlassian.net/browse/HOLODEX-191); this spec scores them
  as plain fields (Movie on the high-stakes non-entity tier, Scene Number/Genre on low-stakes)
  until that lands.
- **Owner-configurable confidence thresholds.** Hardcoded for v1 (see Resolved Decisions #4) —
  revisit only if the fixed thresholds prove wrong in practice.

---

## User Stories

Grouped by the owner persona (the only actor — extraction and writeback are owner-gated).

**Extraction**

- *As the owner, I want to click "Extract from filename" on a single video so that I can quickly
  fill in metadata a new file is missing.*
- *As the owner, I want to run extraction across a whole folder or the whole library at once so
  that I don't have to open every file individually.*
- *As the owner, I want extraction to run automatically when a file is scanned in so that new
  files start with as much metadata as the filename already encodes.*

**Confidence routing**

- *As the owner, I want an unambiguous extraction (filename and tags agree, or the entity already
  exists) to write itself so that I'm not clicking through thousands of obvious cases.*
- *As the owner, I want a genuine conflict (filename says one person, tags say another) to stop
  and ask me, so that I don't silently get the wrong data.*
- *As the owner, when a candidate name is close to but not exactly an existing Person/Studio, I
  want to see that name suggested in the review queue so that I don't have to type it from
  scratch — but I still want to be the one who confirms it.*

**Merge / sync-back**

- *As the owner, when I merge two Person (or Studio) records, I want every affected file's tags to
  update to the canonical name automatically so that I don't have to separately track down and
  re-sync each file.*

**Preview & rollback**

- *As the owner, before a batch of extracted values writes to a file, I want to see exactly what
  will change so that I can catch a mistake before it's on disk.*
- *As the owner, if a sync turns out wrong, I want to revert it so that I'm not manually fixing
  file tags one by one.*

---

## Concepts & Model

### Filename pattern parsing

A filename pattern is a small token grammar, owner-editable at runtime (via
[F41](runtime-settings.md)/ADR-060 — a DB override, no redeploy):

```
[{studio}] {title} ({people}, {year}) {resolution}
```

- `{studio}`, `{title}`, `{year}` are scalar tokens; `{people}` is a multi-value token
  (comma-split); `{resolution}` (and any bracketed token not mapped to a canonical field) is
  **consumed but ignored** — it participates in matching but produces no field value.
- Multiple patterns may be configured (`filename_patterns: []`, ordered). Each candidate filename
  is tried against patterns in order; the **first full match** wins. No match means no
  filename-derived data for that file — it falls through to tag-only resolution, unchanged from
  today. This is what makes the filename convention "one among many" safe: files that don't follow
  it simply don't produce a `filename:` source.
- Token → canonical field mapping: `{studio}`→`studio`, `{title}`→`title`, `{people}`→`people`,
  `{year}`→`release_date` (year granularity).
- Pattern compilation to a matching regex, and the token vocabulary, are pure and unit-tested with
  no I/O — same posture as the F27 resolver.

### The `filename:` source

Parsed values are written into the existing `entity_enrichment` shadow store (migration 0005,
[F22](metadata-plugins.md)) under a new `filename` namespace — identical shape to `file:`/`tmdb:`,
so the resolver's `orderedSources` iteration ([F27](metadata-plugins.md)) requires **no change** to
pick it up once configured into a field's `sources` list.

### Confidence model

Extraction confidence is a `float64` in `[0, 1]`, matching the existing convention
([F47](enrichment-review-workflow.md), `internal/enrich.Candidate.Confidence`) — a **second**
source of scored candidates, alongside provider enrichment, feeding the same auto-apply-vs-review
posture.

**Entity fields** (People, Studio) — three weighted components, entity resolution weighted
heaviest since it's the strongest signal the value is *known*, not just plausible:

| Component | Weight | Scoring |
|---|---|---|
| Source agreement | 0–0.30 | Filename + tag exact match: 0.30 · Only one has data: 0.20 · Fuzzy agreement: 0.10 · Conflict: 0 |
| Value specificity | 0–0.20 | Multi-word ("Alice Smith"): 0.20 · Single word ("Alice"): 0.07 · Garbled/unparseable: 0 |
| Entity resolution | 0–0.50 | Exact loose-key match to an existing entity ([F43](entity-identity.md)): 0.50 · Jaro-Winkler fuzzy match (**advisory only**, see below): 0.20 · No match (would create new): 0.05 |

**Non-entity fields** (Title, Release Date, Comment, Genre/Tags, Movie, Scene Number) have no
entity to resolve against, so the weight redistributes into two components:

| Component | Weight | Scoring |
|---|---|---|
| Source agreement | 0–0.50 | Exact: 0.50 · Single source: 0.30 · Fuzzy: 0.20 · Conflict: 0 |
| Value specificity | 0–0.50 | Structured/complete: 0.50 · Partial: 0.25 · Garbled: 0 |

### Field tiers and thresholds

| Tier | Fields | `AutoApplyThreshold` |
|---|---|---|
| High-stakes | People, Studio, Movie | 0.80 |
| Medium-stakes | Title, Release Date | 0.70 |
| Low-stakes | Comment, Genre/Tags, Scene Number | 0.40 |

Movie stays on the non-entity rubric (no Movie entity yet, [HOLODEX-191](https://whoiskevinrich.atlassian.net/browse/HOLODEX-191))
but inherits the high-stakes threshold — misfiling a scene under the wrong movie is a real
categorization error even without an entity table backing it.

### The exact-match gate (hard rule, not just a score)

For entity fields, **`AutoApply` requires the entity-resolution component to have come from an
exact loose-key match** — never from the Jaro-Winkler advisory tier — regardless of the aggregate
score. A candidate that totals above threshold only because of a fuzzy entity match still routes
to the review queue. Jaro-Winkler's only role is to **rank and pre-fill a suggested candidate** in
the review queue UI when no exact match exists, so the owner isn't typing a name from scratch —
it never itself authorizes a write. This mirrors F43's own invariant that a near-miss never
auto-merges.

### Manual-edit precedence

Extraction is **one-time import per field**: once a field carries a `manual:` source
([F30](metadata-curation.md)), a later extraction run (re-triggered on-demand) that disagrees with
the manual value always routes to the review queue, never auto-applies over it — a manual edit is
treated as the owner having already made the call.

---

## Functional Requirements

### F48.1 — Filename pattern parsing

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.1a | Owner-configurable ordered list of filename patterns, editable at runtime (F41/ADR-060) | Changing the pattern list takes effect without redeploy; validated on save (rejects unparseable token grammar) |
| F48.1b | A filename is tried against patterns in order; first full match wins; no match yields no filename-derived data | A file that matches no pattern falls through to tag-only resolution unchanged |
| F48.1c | Unmapped bracketed tokens (e.g. `{resolution}`) are consumed for matching but produce no field value | Pattern `[{studio}] {title} ({people}, {year}) {resolution}` against a real filename extracts exactly 4 fields, not 5 |
| F48.1d | Multi-value tokens (`{people}`) split on a configurable delimiter (default `, `) | `(Alice, Bob)` extracts two people, not one string |
| F48.1e | Pattern compilation and matching are pure, no I/O | Table-driven unit tests over a fixture set of filenames × patterns, no network/DB |

### F48.2 — `filename:` shadow-store integration

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.2a | Parsed values write into `entity_enrichment` under a `filename` namespace, same shape as `file:`/`tmdb:` | No migration needed for storage; existing table reused |
| F48.2b | Fields configure `filename:<field>` in their `sources` list (`metadata-mappings.yaml`) alongside `file:`/`tmdb:` | Resolver's existing `orderedSources` iteration picks it up with no resolver code change (regression guard) |

### F48.3 — Confidence scoring

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.3a | Entity-field scoring implements the 3-component weighted rubric above | Table-driven tests reproduce each named scenario (exact+entity-exists, exact+no-entity, fuzzy, garbled, conflict) at their specified score |
| F48.3b | Non-entity-field scoring implements the 2-component rubric above | Same table-driven coverage for Title/Release Date/Comment/Genre/Movie/Scene Number |
| F48.3c | Entity resolution reuses F43's loose-key detector for the exact tier — no new normalization logic | Same `nameKey` function, imported not reimplemented |
| F48.3d | Jaro-Winkler similarity computes the fuzzy-tier score and the queue's suggested candidate, but never satisfies the exact-match gate | A fuzzy match scoring above a tier's `AutoApplyThreshold` still routes to review (unit test asserts routing, not just score) |
| F48.3e | A field carrying an existing `manual:` source always routes to review on re-extraction, regardless of score | Unit test: manual override present + high-confidence conflicting extraction → queued, not applied |

### F48.4 — Auto-apply / review-queue routing

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.4a | A candidate scoring at or above its tier's `AutoApplyThreshold` **and** passing the exact-match gate (F48.3d) enqueues a write via the existing F30 write queue | No new write mechanism — same `WriteBatch`/`writequeue` call path as manual curation |
| F48.4b | A candidate below threshold, or failing the exact-match gate, creates a `metadata_extraction_review` row (pending) | One row per `(video, field)`; re-running extraction on an already-pending field updates the row in place, doesn't duplicate |
| F48.4c | Resolving a review row (owner picks filename value / tag value / existing entity / edits manually / dismisses) enqueues the resulting write the same way F48.4a does, and marks the row resolved | Resolved rows leave the queue without a refetch (F43/F47 pattern) |
| F48.4d | Dismissing a review row is durable — it doesn't resurface until the owner re-triggers extraction for that file | Mirrors F47 RD4's "not matched" dismissal semantics |

### F48.5 — Extraction triggers

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.5a | On-demand: a single-video "Extract from filename" action runs F48.1–F48.4 for that video only | Result reflected immediately in the video's resolved fields / review queue |
| F48.5b | Batch: an "Extract all" action runs the same pipeline across a folder or the whole library | Progress observable via the existing System Activity surface (ADR-028), `kind=extraction` |
| F48.5c | Import-time: scanning a new file runs extraction automatically as part of ingest | A freshly scanned file with a matching filename pattern has its high-confidence fields populated with no owner action |
| F48.5d | All three triggers share one extraction code path | No behavior drift between on-demand/batch/import-time (regression guard: same fixture produces the same result via all three entry points) |

### F48.6 — Extraction review queue UI

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.6a | A new **Extraction** tab in the Owner Hub ([F35](owner-tooling-hub.md)), parallel to Duplicates (F43) and Enrichment (F47) | Owner-gated; hidden for non-owner clients (ADR-030) |
| F48.6b | Each row shows the field, filename value, tag value, and (for entity fields) the Jaro-Winkler-suggested candidate when no exact match exists | Suggested candidate is clearly marked as a suggestion, not an applied value |
| F48.6c | Row actions: accept filename value / accept tag value / pick suggested entity / edit manually / dismiss | Matches F48.4c's resolution paths |
| F48.6d | All controls use semantic tokens; QA'd in Cinémathèque, Broadcast, Brutalist | No hardcoded colors/hex in the new components |
| F48.6e | Keyboard-accessible row navigation | Roving-tabindex, consistent with existing pickers (`EnrichPicker`) |

### F48.7 — Preview before sync

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.7a | Before a batch of auto-applied or owner-resolved extraction writes commits, the owner sees a diff of what will be written per file | Shows old value → new value per field; no write happens until confirmed |
| F48.7b | Preview is skippable once the owner has established trust (contextual, not mandatory) — matches the earlier confidence-gating intent | High-confidence auto-apply batches surface a lightweight preview; owner can proceed without per-file confirmation |

### F48.8 — Merge → writeback propagation

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.8a | Completing a Person merge ([F23.9](person-aliases.md)) enqueues one writeback job per affected video, rewriting the loser's name to the canonical name in the People tag | N affected videos → N writeback jobs; same writequeue as F48.4a |
| F48.8b | Completing a Studio merge does the same for the Studio tag | Symmetric with F48.8a once studio merge exists ([studio-entity.md](studio-entity.md) P2) |
| F48.8c | Merge-triggered writes do **not** require a second preview/confirm — the merge's own informed-confirm (F43 RD8: shows video count before committing) is the authorization | No additional dialog between merge-confirm and the writeback jobs enqueuing |
| F48.8d | Merge-triggered writes are snapshotted the same as any other write (F48.9) | A bad merge is revertible via F48.9, not just via un-merging in the DB |
| F48.8e | Filenames are **not** rewritten by a merge (Non-Goals) | Only embedded tags change; the filename retains the loser's name until the (future, separate) rename feature ships |

### F48.9 — Rollback

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.9a | Every write through the F30 queue snapshots the field's prior on-disk value before writing | `file_writeback_snapshots` row per field per write, grouped by `batch_id` |
| F48.9b | A "Revert" action on a completed batch restores every snapshotted field to its prior value via an inverse write | Byte-for-byte match to the pre-write value on the affected field(s) |
| F48.9c | Revert itself is a normal writeback job (goes through the same queue, is itself snapshotted) | A revert can be re-reverted (redo), no special-cased write path |
| F48.9d | Revert is available from the activity history entry for the original write | Owner doesn't need to hunt for the batch id manually |

### F48.10 — Security

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.10a | All extraction/review/revert endpoints are owner-gated (`requireOwner`, ADR-030) | Non-owner → `401`; controls hidden in the SPA |
| F48.10b | Filename-derived values are untrusted input: length-capped and sanitized before storage/write, consistent with F30.6b's manual-value handling | Over-long or control-character filename tokens rejected/sanitized |
| F48.10c | No new outbound network surface — extraction is local parsing only | Confirmed no HTTP calls introduced by F48.1–F48.5 |
| F48.10d | The new scan-time auto-trigger (F48.5c) cannot cause unbounded write pressure | Bounded by the existing `WRITEBACK_CONCURRENCY` queue limit (F30) — no new concurrency knob |

### F48.11 — Testability / CI

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F48.11a | Pattern parsing, confidence scoring, and routing are table-driven unit tests, no I/O | Covers every named scenario in [Concepts & Model](#concepts--model) |
| F48.11b | End-to-end extract→auto-apply and extract→queue→resolve→write against the fixture corpus in CI | Asserts single `WriteBatch` call per file (F30 invariant preserved) |
| F48.11c | Merge→writeback and revert are covered end-to-end | Fixture library merge produces N writeback jobs; revert restores original bytes |

---

## Data Model (additions)

```text
metadata_extraction_review        -- pending/dismissed field-level extraction conflicts (F48.4)
  id            PK
  video_id      int64             -- FK into videos
  field_key     string            -- canonical field, e.g. "title", "people"
  filename_value string           -- raw extracted value from filename (nullable)
  tag_value      string           -- raw value from embedded tags (nullable)
  confidence     real             -- 0.0-1.0, per Concepts & Model
  suggested_entity_id  int64      -- nullable; Jaro-Winkler-suggested Person/Studio, advisory only
  status        string            -- "pending" | "dismissed" | "resolved"
  created_at    timestamp
  resolved_at   timestamp nullable
  UNIQUE (video_id, field_key) WHERE status = 'pending'
  INDEX (status)
```

```text
file_writeback_snapshots          -- pre-write backup for rollback (F48.9, amends ADR-041)
  id            PK
  video_id      int64             -- FK into videos
  batch_id      string            -- groups snapshots taken in the same write operation
  field_key     string
  prior_value   string            -- raw tag value before this write ("" if previously absent)
  written_at    timestamp
  INDEX (batch_id)
  INDEX (video_id, written_at)
```

Both are new migrations (next available: **0025**, **0026**). No changes to `entity_enrichment`
(F48.2 reuses it via a new namespace value only) or to `writeback_queue`/`job_runs` (F48.4/F48.8
enqueue through them unchanged).

---

## Configuration

New operator/owner-editable config (F41/ADR-060 — DB override at runtime, no redeploy):

| Setting | Where | Default | Meaning |
|---|---|---|---|
| `filename_patterns` | runtime setting | `[]` (extraction disabled until configured) | Ordered list of filename token patterns (F48.1) |
| `filename_token_delimiter` | runtime setting | `", "` | Delimiter for multi-value tokens like `{people}` |

```yaml
# example filename_patterns value
filename_patterns:
  - "[{studio}] {title} ({people}, {year}) {resolution}"
  - "{title} ({year})"
```

---

## Architecture impact (new ADR, pending `/architecture`)

Two cross-cutting decisions need to be recorded:

1. **Confidence-gated auto-apply for a second candidate source (extraction, not just provider
   enrichment).** F47/ADR-066 established the auto-apply-vs-review pattern for provider matches;
   this generalizes it to filename/tag extraction with a different (weighted, multi-component)
   scoring model and a hard exact-match gate that provider matching didn't need. Whether this
   should be a shared `AutoApplyThreshold` abstraction or two independent implementations is an
   architectural call.
2. **Writeback backup/rollback.** ADR-041 explicitly deferred "prior-value capture / undo" as a
   non-goal. This spec reopens that decision to support safe auto-apply at scale. The ADR should
   record why the snapshot model (append-only `file_writeback_snapshots`, revert-as-forward-write)
   was chosen over alternatives (full git-like history, backup-file-on-disk) and confirm it
   composes with F30's existing crash-safe copy→write→rename model.

---

## Resolved Decisions

*(Captured across the product-brainstorming session, 2026-07-14.)*

1. **Extraction triggers — all three.** On-demand single-file, batch "Extract all," and
   automatic at import/scan time are all in scope (F48.5).
2. **Confidence model — weighted per-field rubric.** Entity fields weight entity resolution
   highest (0.50 of 1.0); non-entity fields split evenly between source agreement and specificity.
   See [Concepts & Model](#concepts--model).
3. **Fuzzy matching — Jaro-Winkler, advisory only, human-supervised.** Used to rank/suggest
   candidates in the review queue; never itself satisfies the auto-apply gate. Exact-match
   auto-apply reuses F43's existing loose-key detector rather than introducing a new algorithm for
   that tier.
4. **Thresholds — hardcoded for v1.** High 0.80 (People/Studio/Movie), Medium 0.70 (Title/Release
   Date), Low 0.40 (Comment/Genre/Scene Number). Not owner-configurable; revisit only if wrong in
   practice.
5. **Manual-edit precedence — one-time import.** A field with an existing manual override always
   queues on re-extraction conflict, never auto-applies over it.
6. **Batching/atomicity — per-file, via the existing F30 queue.** No new write mechanism.
7. **Merge → writeback — automatic, no second confirm.** Tag rewrite only; filename itself is
   never rewritten by a merge (that's the deferred rename-schema feature).
8. **Rollback — backup-on-write + inverse-operation revert.** Chosen over full versioning for
   simplicity; reopens ADR-041's deferred non-goal (see Architecture impact).
9. **Review queue placement — new "Extraction" tab**, parallel to Duplicates and Enrichment, not
   folded into either.
10. **Field tiers** — High: People, Studio, Movie. Medium: Title, Release Date. Low: Comment,
    Genre/Tags, Scene Number.
11. **Movie/Scene Number/Genre as entities — deferred**, tracked in
    [HOLODEX-191](https://whoiskevinrich.atlassian.net/browse/HOLODEX-191).

---

## Phasing

1. **Filename parsing + `filename:` shadow source** (F48.1/F48.2) — pure, unit-tested, no UI, no
   auto-apply yet.
2. **Confidence scoring + routing** (F48.3/F48.4) — the architectural risk lives here; ship behind
   a flag with auto-apply disabled (log-only) until the ADR lands.
3. **Rollback foundation** (F48.9) — snapshot-on-write, since F48.4's auto-apply shouldn't ship
   without it.
4. **Triggers** (F48.5) — on-demand first, then batch, then import-time (increasing blast radius,
   in order of confidence in the model).
5. **Extraction review queue UI + preview** (F48.6/F48.7).
6. **Merge → writeback propagation** (F48.8) — depends on F48.9 (snapshotting) being live.

---

## Cross-references to update on landing

- `docs/reference/canonical-fields.md` — document the `filename:` namespace.
- `docs/architecture/README.md` — index the new ADR.
- `docs/testing-strategy.md` — add extraction/confidence/rollback cases.
- `docs/specs/metadata-curation.md` — its **Status: Draft** header is stale (F30 is shipped, PR
  #55); fix independently of this spec.
- The **filename rename-schema** follow-up (Non-Goals) is tracked in
  [HOLODEX-192](https://whoiskevinrich.atlassian.net/browse/HOLODEX-192).
- A `qa-metadata-extraction.md` checklist (mirroring `qa-writeback.md`) with numbered,
  tag-grouped items per the repo's QA conventions.
