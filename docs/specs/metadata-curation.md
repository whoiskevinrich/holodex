# Spec: Granular Metadata Curation & Merge (F30)

**Status**: Draft — scope decisions locked 2026-06-27 (see [Resolved Decisions](#resolved-decisions)); architecture recorded in [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md) (Proposed); pending `/security-review`
**Feature block**: F30
**Phase**: 3 (Enrichment) — follow-on to F22/F27/F28
**Date**: 2026-06-27
**Depends on**:
[F27 unified field resolution](metadata-plugins.md#f27--unified-field-resolution-implements-f223) (`internal/resolver`, `internal/registry`) ·
[F28 metadata writeback](metadata-plugins.md#f28-metadata-writeback) / [ADR-041](../architecture/ADR-041-metadata-writeback.md) (`internal/writeback`, `WriteBatch`) ·
[F22 metadata source plugins](metadata-plugins.md) / [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) (shadow store, `entity_enrichment`) ·
[ADR-028](../architecture/ADR-028-activity-surface-and-job-history.md) (job history / activity surface) ·
[ADR-030](../architecture/ADR-030-access-control-gating-seam.md) (owner gating)
**Routing**: touches data model + outbound file writes + a job queue → architecture recorded in **[ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)** (curation/merge resolution model + queued batch-write pipeline; see [Architecture impact](#architecture-impact)); **`/security-review`** required before merge (modifies library files, owner-gated); **`/testing-strategy`** must gain merge/dedupe/suppression + queued-write cases; frontend uses semantic tokens and is QA'd in all three skins.

---

## Problem Statement

Enrichment today is **all-or-nothing per field**. The F27 resolver walks a field's
configured `sources` in precedence order and the **first non-empty source wins the entire
field** — for a multi-value field like `genres`, the winning source supplies *all* the
values and every other source is discarded. There is no way to combine the file's own tags
with one or more enrichment providers, no deduplication *across* sources, no manual
addition or correction, and writeback ([F28](metadata-plugins.md#f28-metadata-writeback))
commits the whole winning array. An owner who wants the file's `Sci-Fi` **and** TMDB's
`Drama`, minus a genre they disagree with, plus one they typed themselves, cannot express
that. The result is that owners either accept a provider's field wholesale or keep their
file's value and lose the enrichment entirely — and curated intent is never captured.

---

## Goals

1. **Merge, don't replace.** For set-valued fields, the resolved value is the **union** of
   every configured source (file + manual + one or more providers), not just the
   highest-precedence source's array.
2. **Deduplicate across sources.** A value present in more than one source (e.g. file and
   TMDB both say `Science Fiction`) appears — and is written — **exactly once**.
3. **Capture manual intent durably.** Owner additions, edits, and removals persist as a
   first-class **`manual` source** that survives re-scan and re-enrich, and participates in
   resolution like any other source.
4. **Selective, owner-controlled writeback.** The owner chooses, per value, what is written
   to the file; the curated set (not the raw winning array) is what `WriteBatch` embeds.
5. **Safe at library scale.** All curated metadata for a file is written **atomically in one
   pass**, and writes are **queued** with bounded concurrency so a burst of writes does not
   overload the filesystem; every write is recorded in the activity history.

### Success Metrics

**Leading**

- Merge + dedupe verified end-to-end in CI: a video whose file tags and the fake provider
  both supply overlapping `genres` resolves to a single deduplicated union (unit +
  integration). Binary pass/fail; gate on merge.
- A manual add / edit / remove round-trips: applied, persisted, re-displayed after restart,
  and reflected in the written file — verified against the fake provider, no network.
- A queued batch write embeds all selected fields in **one** tool invocation (assert a
  single `WriteBatch` call per file) and appears in the activity feed with `kind=writeback`.

**Lagging**

- The `browse:true` shadow-store read-path (F27.4) demonstrably shrinks over time as owners
  curate-and-write libraries (carried from [ADR-041 Consequences](../architecture/ADR-041-metadata-writeback.md#consequences)).
- The merge/curation model generalizes to **person** entities (bio aliases, etc.) with no
  resolver change — only a new `entity_type` — validating the keystone claim of F22/F27.

---

## Non-Goals

- **Synonym / fuzzy reconciliation.** `Sci-Fi` and `Science Fiction` are treated as
  *different* values and both survive the union. Dedup is exact-after-normalization
  (trim + Unicode-casefold), not semantic. A synonym/alias map is a separate future item —
  conflating distinct strings risks silently dropping an owner's value.
- **Automatic / rule-based writeback.** Writes remain **owner-initiated** (ADR-041 keeps
  explicit-only; the `writeback: if_empty|overwrite` rules DSL stays on the backlog). This
  spec adds *queuing and batching* of owner-triggered writes, not a scheduler or crawler.
- **Prior-value capture / undo of the file's pre-write tags.** Still deferred (ADR-041). The
  owner reviews the curated set before committing; the `file_writebacks` audit records what
  was written. (Note: the **curation store is itself reversible** — suppressions and manual
  adds can be cleared — but the file's *previous on-disk tag* is not snapshotted.)
- **Reordering values within a field.** Curation is set membership (add / keep / remove /
  edit), not ordering. Write order follows source precedence then insertion order.
- **Multi-user curation / per-user overrides.** Single owner gate (ADR-030); curation is
  global to the instance.

---

## User Stories

Grouped by the owner persona (the only actor — enrichment and writeback are owner-gated).

**Merging & dedup**

- *As the owner, I want a field's value to combine my file's own tags with one or more
  enrichment providers so that I keep what the file already knew and gain what the provider
  adds.*
- *As the owner, when the file and a provider both supply `Science Fiction`, I want to see
  and write it only once so that my file isn't littered with duplicate tags.*
- *As the owner, I want to see which source(s) each value came from so that I can judge
  whether to keep it.*

**Manual control**

- *As the owner, I want to add a tag the machine didn't find so that my curation isn't
  limited to what providers return.*
- *As the owner, I want to edit a value (fix a typo, normalize casing) so that the written
  tag matches my library's conventions.*
- *As the owner, I want to remove a value contributed by the file or a provider, and have it
  stay removed across re-scan and re-enrich, so that I correct bad data once.*
- *As the owner, I want some values to show in Holodex but not be written to the file so that
  I can keep a note without polluting the file's tags.*

**Writeback**

- *As the owner, I want one "Write to file" action to embed all my curated fields at once so
  that I don't click through every field separately.*
- *As the owner curating a large library, I want writes to queue and run a few at a time so
  that my disk and the app stay responsive.*
- *As the owner, I want each write to appear in the activity history with its outcome so that
  I can confirm what changed and diagnose failures.*

---

## Concepts & Model

### Resolution modes (per field)

The resolver gains a per-field **merge mode**, derived from the existing `multi` flag and a
new optional `merge` flag in `metadata-mappings.yaml`:

| Mode | Applies to | Behavior |
|---|---|---|
| **precedence** (today) | scalar fields (`multi: false`) | First non-empty source wins a single value; the owner may **override** which value via a manual edit, but the field holds one value. |
| **merge** | set fields (`multi: true`, or explicit `merge: true`) | Resolved value is the **deduplicated union** of all configured sources plus the `manual` source; each value carries its contributing source(s). |

All fields are **curatable** (the owner can edit/remove/add and choose what is written),
per the scoping decision. Cross-source *union* is specific to merge-mode fields; a scalar
field's curation is a single override value plus a keep/remove/write decision.

### The `manual` source

A new persistent source namespace, `manual:<field>`, sits alongside `file:` and provider
namespaces in the resolver. It is backed by a curation store (below) and is always
consulted in merge — its values join the union; for precedence fields a manual value, when
present, is the override winner.

### Value-level curation actions

Each resolved value exposes owner actions; each is recorded in the curation store:

| Action | Effect on display | Effect on file write | Persistence |
|---|---|---|---|
| **Keep** (default) | shown | written (if `write` on) | implicit |
| **Add** | new value appears in the union | written | `action='add'` row in curation store, `source='manual'` |
| **Edit** | replaces a value's display string | edited string written | modeled as `suppress(original) + add(new)` |
| **Remove** (suppress / tombstone) | hidden everywhere | not written | `action='suppress'` row keyed by normalized value |
| **Don't write** | shown in Holodex | excluded from the file | `action='nowrite'` row keyed by normalized value |

Suppression and "don't write" are keyed by the **normalized value**, so they apply
regardless of which source re-supplies the value on a later scan/enrich (the tombstone
survives re-fetch — answer: persistent suppression).

### Deduplication & casing rule

1. **Normalize for comparison** each candidate value: trim surrounding whitespace + Unicode
   case-fold. This is the dedup match key only — it never changes what is displayed/written.
2. **Group** by normalized key so a value present in multiple sources survives once.
3. **Apply the field's configured output casing** to the surviving display/write value
   (decision #4): casing is a **per-field property**, not source-derived. Each field declares
   `casing: preserve | lower | upper | title` in `metadata-mappings.yaml` (see
   [Configuration](#configuration)). Example: `genres`/`tags` → `lower`, `title` and person
   `name` → `title`. Default `preserve` (first occurrence in source-precedence order, then a
   manual edit always wins over a source value).
4. **Provenance** for a surviving value is the set of all sources whose normalized value
   matched (UI may show the primary + a count, e.g. "TMDB + file").
5. Normalization, dedup, casing, suppression, and "don't write" are **pure re-interpretation**
   of stored data (resolver does no I/O — F27 invariant). Changing config or curation re-runs
   resolution without re-fetching providers or re-scanning files.

---

## Functional Requirements

### F30.1 — Cross-source merge & dedup (resolver)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.1a | Merge-mode fields resolve to the **union** of all configured sources, not the first non-empty source | A field with `sources: [tmdb:genres, file:genre]` where the file has `[Sci-Fi]` and TMDB has `[Drama, Sci-Fi]` resolves to `[Drama, Sci-Fi]` (3 inputs → 2 outputs) |
| F30.1b | Union is deduplicated by normalized value (trim + casefold); surviving value's casing is set by the field's configured `casing` (decision #4), not by source | File `Science Fiction` + TMDB `science fiction` → one value; with `casing: lower` it resolves+writes as `science fiction`; with `casing: title`, `Science Fiction` |
| F30.1c | `ResolvedField.Values` entries carry per-value provenance (contributing sources) | API exposes, per value, its source set; `WinningSource` (F27) is retained for back-compat / precedence fields |
| F30.1d | Precedence (scalar) fields are unchanged unless a manual override exists | A `multi:false` field with no manual edit resolves exactly as F27 does today (regression guard) |
| F30.1e | Resolution remains pure (no I/O); config/curation changes take effect without re-fetch/re-scan | F27.3d invariant preserved; covered by a resolver unit test with in-memory inputs |

### F30.2 — Curation store (manual source + tombstones)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.2a | A `metadata_curation` table persists value-level decisions keyed by `(entity_type, entity_id, field_key, norm_value, action)` | New migration; `entity_type='video'` in v1 (generalizes to `person` later) |
| F30.2b | `action ∈ {add, suppress, nowrite}`; `add` rows also store the display value and `source='manual'` | Round-trips: an added value survives restart and re-displays without any provider call |
| F30.2c | Suppression and `nowrite` are matched by normalized value, independent of contributing source | A suppressed `Sci-Fi` stays hidden after a re-enrich re-supplies it from TMDB |
| F30.2d | The resolver loads curation alongside enrichment and applies add→union, suppress→drop, nowrite→flag | One pre-loaded `Curation` map mirrors the `Enrichment` map shape; no extra per-field query |
| F30.2e | Clearing a curation action restores the underlying source value | Removing a `suppress` row makes the file/provider value reappear; removing an `add` row deletes only the manual value |

### F30.3 — Curation UI (inline per-value controls)

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.3a | Each resolved value renders inline as a chip/row with: provenance, a write toggle, and edit/remove affordances (owner only) | Hidden entirely for non-owner clients (ADR-030) |
| F30.3b | An "add value" affordance per field lets the owner type a new value | New value appears immediately as a `manual` chip; persisted via F30.2 |
| F30.3c | Edit replaces a value in place; remove tombstones it; both reflect immediately and persist | After page reload the curated state is identical |
| F30.3d | A per-value "don't write" toggle excludes a shown value from the file write | Toggling off marks `nowrite`; the value stays visible but is excluded from the next write |
| F30.3e | All controls use semantic tokens; QA'd in Cinémathèque, Broadcast, Brutalist | No `zinc-*`/hex/`rounded-(lg\|md\|sm\|xl)` in the components; chips, toggles, and provenance read correctly in all three skins |
| F30.3f | Keyboard-accessible: chips and the add-input are reachable and operable by keyboard | Roving-tabindex / focus rules consistent with existing pickers |

### F30.4 — Queued, atomic, batch writeback

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.4a | A single "Write to file" action for a video enqueues **one** write job carrying the full curated, write-enabled set across all fields | The job embeds all fields via a single `WriteBatch` call (one tool invocation per file — ADR-041 atomicity) |
| F30.4b | Suppressed and `nowrite` values are excluded; duplicates are collapsed before the write | The written file contains each surviving value exactly once; suppressed values absent |
| F30.4c | Writes run through a **bounded-concurrency queue** so concurrent/bulk writes do not thrash the filesystem | Worker limit defaults to **1** (fully serialized) and is overridable via `WRITEBACK_CONCURRENCY` (decision #2); queue depth is observable |
| F30.4d | The queue is **durable**: enqueued writes persist and replay after a restart (decision #1) | Pending jobs survive process exit; on boot the worker resumes them; an owner does not have to re-trigger |
| F30.4e | **Crash-safe mid-write**: a write interrupted by shutdown/crash leaves the original file intact and is safely retried | ADR-041 copy→write→rename guarantees the original is untouched until the atomic rename; on boot, orphan `.holodex-tmp`/`.holodex-new` files are cleaned up and the job is re-run or marked failed (never half-applied) |
| F30.4f | Each write is recorded as a `job_runs` row with `kind=writeback` (ADR-028) and surfaced in the activity feed | A completed/failed write shows in the 30-day history with video, fields, outcome |
| F30.4g | Per-field audit rows continue to be written to `file_writebacks` (F28) for each field in the batch | One audit row per field written; multi-value joined with `\n` as today |
| F30.4h | Atomicity & failure semantics from ADR-041 are preserved | On any failure the original file is byte-for-byte untouched; temp files cleaned up; no audit/`job_runs` success row |
| F30.4i | A field with no tag mapping for the container is skipped with a clear per-field reason, not a whole-batch failure | Batch writes the mappable fields; unmappable fields reported (e.g. `.avi` → field-level 422-equivalent in the job result) |

### F30.5 — API

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.5a | Curation CRUD endpoints (owner-gated): add / edit / remove (suppress) / nowrite a value for `(video, field)` | `requireOwner`; non-owner → `401`; invalid field/value → `400` |
| F30.5b | The detail response returns, per resolved value, provenance + curation state (`written?`, `suppressed?`, `manual?`) | SPA renders chips and toggles directly from this; no client-side re-resolution |
| F30.5c | `POST /api/media/{id}/writeback` evolves to accept the **curated batch** (all fields) and enqueue a job, returning `202 Accepted` + a job handle | Back-compat: the single-field body (F28) still accepted and enqueued as a one-field batch |
| F30.5d | A queued/running/failed write is queryable (job status) and visible in activity | Status reachable via the activity surface (ADR-028) |

### F30.6 — Security

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.6a | All curation + writeback endpoints behind `requireOwner` (ADR-030) | Token-less/non-owner client gets `401`; controls hidden in the SPA |
| F30.6b | Manual values are untrusted input: length-capped and sanitized before storage/write | Over-long or control-character input rejected/sanitized; consistent with F22.9b provider-value handling |
| F30.6c | No new outbound network surface introduced by curation | Curation is local DB only; image writeback continues to use the existing https-only, size-capped downloader (ADR-039 asset-host allowlist applies to image fields) |

### F30.7 — Testability / CI

| ID | Requirement | Acceptance Criteria |
|----|-------------|---------------------|
| F30.7a | Resolver merge/dedup/suppression covered by table-driven unit tests (no I/O) | Cases: overlap dedup, casing precedence, suppress-survives-reenrich, manual add, nowrite exclusion |
| F30.7b | End-to-end curate→write against the **fake provider** (F22.10) in CI | Curated union written via a single `WriteBatch`; `file_writebacks` + `job_runs` rows asserted; no network |
| F30.7c | Queue behavior tested: bounded concurrency, ordering, failure isolation | A failing write does not block or fail siblings; original untouched |

---

## Data Model (additions)

```text
metadata_curation                 -- value-level owner decisions (F30.2)
  id           PK
  entity_type  string             -- v1: "video"  (later: "person")
  entity_id    int64              -- FK into videos (v1)
  field_key    string             -- canonical field, e.g. "genres", "title"
  norm_value   string             -- normalized (trim+casefold) match key
  value        string             -- display value (for action='add'); else "" or original
  action       string             -- "add" | "suppress" | "nowrite"
  source       string             -- "manual" for adds; provenance note otherwise
  created_at   timestamp
  UNIQUE (entity_type, entity_id, field_key, norm_value, action)
  INDEX (entity_type, entity_id)
```

```text
writeback_queue                   -- durable write queue (F30.4d/F30.4e)
  id           PK
  video_id     int64              -- FK into videos
  payload      string             -- JSON: curated, write-enabled FieldWrite set for this file
  status       string             -- "pending" | "running" | "failed" | "done"
  attempts     int                -- retry count (bounded; surfaced on the job result)
  enqueued_at  timestamp
  updated_at   timestamp
  INDEX (status, enqueued_at)
```

On boot the worker (a) re-reads `status IN ('pending','running')` rows and resumes them —
a `running` row implies an interrupted attempt, so its file is verified intact and the job
re-run (F30.4e) — and (b) sweeps the media tree for orphan `.holodex-tmp` / `.holodex-new`
files left by a crash and removes them. `done`/`failed` rows are pruned on the same
retention basis as `job_runs`.

Reuses the existing `file_writebacks` audit table (F28, migration 0011) unchanged — one row
per field per successful write. Reuses `job_runs` (ADR-028) with a new `kind=writeback`
value; no schema change to `job_runs` (it already carries `kind`, `trigger`, counts,
`detail`).

---

## Configuration

New operator config introduced by this feature (document in
`docs/reference/configuration.md` on landing):

| Setting | Where | Default | Meaning |
|---|---|---|---|
| `WRITEBACK_CONCURRENCY` | env var | `1` | Number of write-queue workers (decision #2). `1` fully serializes file writes to protect the filesystem; raise only on fast storage. |
| `casing:` (per field) | `metadata-mappings.yaml` field entry | `preserve` | Output casing applied to a field's resolved values: `preserve` \| `lower` \| `upper` \| `title` (decision #4). Casing is per **field/property**, independent of source. |

```yaml
# metadata-mappings.yaml — per-field casing (decision #4)
fields:
  - canonical: genres
    multi: true
    merge: true
    casing: lower          # tags/genres normalized to lower case on write
    sources: [tmdb:genres, file:genre]
  - canonical: title
    casing: title          # Title Cased
    sources: [tmdb:title, file:title]
  - canonical: name        # person name (fast-follow generalization)
    casing: title
```

The resolver gains a `Curation` input mirroring `Enrichment`'s shape so the merge step has
all data pre-loaded (no per-field query):

```text
Curation = map[field_key] -> { add: []string, suppress: set[norm], nowrite: set[norm] }
```

---

## Architecture impact ([ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md))

Two cross-cutting decisions are recorded in [ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md) (Proposed):

1. **Merge resolution model** — generalizing F27's "first source wins" to a deduplicated
   union with per-value provenance and a persistent `manual` source + tombstones. This
   changes the central resolution contract and the `ResolvedField` shape, so it is an
   architecture decision, not just a spec.
2. **Queued batch writeback pipeline** — introducing a bounded-concurrency write queue and
   `kind=writeback` job runs on top of the existing synchronous `WriteBatch`. This is the
   first internal job *queue* (scans are triggered, not queued), so the concurrency model,
   backpressure, and crash/restart behavior warrant a recorded decision. It also partially
   realizes ADR-041's deferred **batch writeback (Option C)** — the ADR should note it
   supersedes that backlog item's "all fields at once" portion while leaving rule-based
   (auto) writeback deferred.

---

## Resolved Decisions

All five open questions were resolved with the owner (2026-06-27):

1. **Write-queue durability — PERSIST.** The queue is durable (`writeback_queue` table,
   F30.4d): writes survive restart and replay on boot. Mid-write interruptions are handled
   gracefully — the copy→write→rename model keeps the original intact until the atomic
   rename, and boot recovery cleans orphan temp files and re-runs or fails interrupted jobs
   without ever half-applying a write (F30.4e).
2. **Worker concurrency — DEFAULT 1, CONFIGURABLE.** Default `1` (fully serialized) with an
   operator override via `WRITEBACK_CONCURRENCY`; documented in
   [Configuration](#configuration) and `docs/reference/configuration.md`.
3. **Toggle states — KEEP BOTH.** Ship both `suppress` (hidden + never written) and
   `nowrite` (shown but excluded from the file) in v1 (F30.3d/F30.4b). Owner will revisit in
   a future session if the UI feels too busy.
4. **Casing — PER-FIELD CONFIG.** Output casing is configured **by field/property, not by
   source** (`casing: preserve|lower|upper|title` in `metadata-mappings.yaml`). E.g. tags
   lower-cased, title and person name Title Cased. Dedup comparison stays case-insensitive;
   the configured casing sets the written/displayed form. See
   [Deduplication & casing rule](#deduplication--casing-rule) and [Configuration](#configuration).
5. **Scope — VIDEO-ONLY V1 + person fast-follow.** Ship video curation first; person-entity
   generalization (aliases, bio, name) is a tracked **fast-follow task** (the model is
   already entity-typed, so it needs only a new `entity_type` — no resolver change).

---

## Phasing

1. **Resolver merge + dedup + provenance** (F30.1) — pure, in-memory, fully unit-tested. The
   architectural risk lives here; ship behind the existing `multi`/new `merge` flag with no
   UI yet.
2. **Curation store + manual source + tombstones** (F30.2) — migration, repo methods, resolver
   wiring; round-trip tested against the fake provider.
3. **Inline curation UI** (F30.3) — chips, add/edit/remove, write toggle; QA all three skins.
4. **Queued atomic batch writeback** (F30.4/F30.5) — the write queue, `kind=writeback` job
   runs, evolved endpoint; end-to-end CI against the fake provider.
5. **Generalize to person** (fast-follow task per decision #5) — new `entity_type`, no
   resolver change; person aliases/bio/name curation.

---

## Cross-references to update on landing

- `docs/reference/canonical-fields.md` — document merge mode and the `manual:` namespace.
- `docs/architecture/README.md` — index the new ADR.
- `docs/testing-strategy.md` — add merge/dedupe/suppression + queued-write cases.
- `metadata-plugins.md` §F28 backlog — mark "batch writeback" partially realized by F30.4.
- A `qa-metadata-curation.md` checklist (mirroring [qa-writeback.md](qa-writeback.md)) with
  numbered, tag-grouped items per the repo's QA conventions.
