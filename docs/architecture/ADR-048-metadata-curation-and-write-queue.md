# ADR-048: Granular Metadata Curation — cross-source merge + durable batch-writeback queue

**Status:** Proposed
**Date:** 2026-06-27
**Deciders:** Project owner
**Relates to:** [ADR-013](ADR-013-metadata-field-mapping.md) (field-mapping precedence — the model this generalizes) · [ADR-033](ADR-033-metadata-source-plugins.md) (source plugins / `entity_enrichment` shadow store) · [ADR-041](ADR-041-metadata-writeback.md) (metadata writeback — **partially realizes its deferred Option C**) · [ADR-028](ADR-028-activity-surface-and-job-history.md) (job history) · [ADR-030](ADR-030-access-control-gating-seam.md) (owner gating). **Spec:** [F30 Granular Metadata Curation & Merge](../specs/metadata-curation.md).

---

## Context

F27 (unified field resolution, under [ADR-013](ADR-013-metadata-field-mapping.md)/[ADR-033](ADR-033-metadata-source-plugins.md)) resolves each canonical field by walking its configured `sources` in order and taking the **first non-empty source as the entire field value** (`internal/resolver/resolver.go`). For a multi-value field such as `genres`, the winning source supplies *all* values and every other source is discarded; deduplication happens only *within* that one winning array. There is no `manual` source, and writeback ([ADR-041](ADR-041-metadata-writeback.md)) commits the whole winning array per field.

The result is **all-or-nothing metadata**: an owner cannot keep the file's own `Sci-Fi` *and* add TMDB's `Drama`, cannot dedup a value both sources supply, cannot type a value neither source knows, cannot remove a bad value durably, and cannot choose which values reach the file. The [F30 spec](../specs/metadata-curation.md) defines the desired behavior; this ADR records the two coupled architectural decisions it requires. They are coupled because the curated set produced by the resolution change is exactly the input to the write change.

### Forces / constraints (locked in the spec's [Resolved Decisions](../specs/metadata-curation.md#resolved-decisions))

- **Files are precious.** The write path must keep [ADR-041](ADR-041-metadata-writeback.md)'s atomic copy→write→rename guarantee — the original is byte-for-byte untouched until an atomic rename, and survives crash/shutdown mid-write.
- **Resolution must stay pure.** [ADR-013](ADR-013-metadata-field-mapping.md)'s invariant — resolution is re-interpretation of stored data with no I/O, so a config/curation change takes effect without re-fetching providers or re-scanning files — must hold.
- **Durable, throttled writes.** Writes persist across restart and run at bounded concurrency (default 1) so bulk curation cannot overload the filesystem.
- **Owner-gated.** Curation and writeback modify library files; both stay behind `requireOwner` ([ADR-030](ADR-030-access-control-gating-seam.md)).
- **Video-only v1**, but the model must generalize to `person` with only a new `entity_type` (no resolver change).

---

## Decision

Adopt two changes, delivered together.

### Decision 1 — Cross-source merge + persistent curation layer

Generalize the resolver from "first source wins the field" to a **deduplicated union across all configured sources plus a new `manual` source**, with value-level owner curation.

- **Merge mode (per field).** A field is resolved in `merge` mode when `multi: true` (or an explicit `merge: true`) in `metadata-mappings.yaml`; otherwise it keeps today's `precedence` (single-winner) behavior. Merge mode returns the **union** of every configured source's values, deduplicated.
- **`manual` source.** A new resolver namespace `manual:<field>`, backed by a new `metadata_curation` table, joins resolution like `file:` and provider namespaces. It is pre-loaded into the resolver alongside the enrichment map — no new per-field query, the [ADR-013](ADR-013-metadata-field-mapping.md) purity invariant is preserved.
- **Value-level actions, keyed by normalized value.** `add` (manual value), `suppress` (tombstone — hidden everywhere, never written), and `nowrite` (shown in Holodex, excluded from the file). Suppress/nowrite match the **normalized** value, so they survive a later re-scan/re-enrich that re-supplies the same value from any source. An `edit` is modeled as `suppress(original) + add(new)`.
- **Dedup + casing.** Comparison key is `trim + Unicode case-fold`; the *output* casing of a surviving value is set by a **per-field** `casing: preserve | lower | upper | title` property (not by source precedence). Default `preserve` (first occurrence in precedence order; a manual edit always wins).
- **Per-value provenance.** Each surviving value carries the set of sources whose normalized value matched, so the SPA can badge it (`TMDB + file`). `ResolvedField` is extended to carry per-value provenance and curation flags (`written?`, `suppressed?`, `manual?`) alongside the existing `WinningSource` (kept for precedence fields and back-compat).

### Decision 2 — Durable, bounded-concurrency batch-writeback queue

Layer a persistent write queue on top of the existing `internal/writeback.WriteBatch` (which already embeds all of a file's fields in a **single atomic tool invocation**).

- **One job per file.** A "Write to file" action enqueues a single durable job carrying the curated, write-enabled `FieldWrite` set across all fields (suppressed/`nowrite` excluded, duplicates collapsed). The worker calls `WriteBatch` once per file.
- **Durable queue.** A new `writeback_queue` table (`status ∈ pending|running|failed|done`, `attempts`, JSON `payload`) persists enqueued writes; they replay after restart.
- **Bounded concurrency.** A worker pool sized by `WRITEBACK_CONCURRENCY` (default **1**, fully serialized) drains the queue; default protects the filesystem, operator may raise on fast storage.
- **Crash-safe recovery.** On boot the worker resumes `pending`/`running` rows (a `running` row implies an interrupted attempt → its file is intact per the copy→write→rename model, so the job re-runs) and sweeps orphan `.holodex-tmp`/`.holodex-new` files. A write is never half-applied.
- **Observability.** Each completed/failed write is recorded as a `job_runs` row with a new `kind=writeback` ([ADR-028](ADR-028-activity-surface-and-job-history.md), no schema change) and surfaced in the 30-day activity feed; per-field `file_writebacks` audit rows ([ADR-041](ADR-041-metadata-writeback.md), migration 0011) continue unchanged.
- **Scope vs ADR-041.** This **partially realizes [ADR-041](ADR-041-metadata-writeback.md)'s deferred Option C** (batch writeback — "all fields at once") but keeps it **owner-triggered**; rule-based / automatic writeback (ADR-041 Option B) stays deferred.

New migrations: **0013** `metadata_curation`, **0014** `writeback_queue` (0012 is person_image_suppressions; sequence ends at 0012).

---

## Options Considered

### Decision 1 — resolution model

#### A — Cross-source dedup union + persistent curation store (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — resolver change + one table + `ResolvedField` shape change |
| Cost | Low runtime (curation pre-loaded; resolution stays pure) |
| Scalability | Good — set-membership ops, no per-field query |
| Team familiarity | High — mirrors the existing `Enrichment` pre-load + ADR-013 precedence |

**Pros:** Delivers merge + dedup + manual + durable removal exactly as F30 needs; keeps resolution pure; generalizes to `person` via `entity_type`. **Cons:** `ResolvedField` shape change ripples to SPA + MCP; Unicode normalization/casing edge cases to get right.

#### B — Keep first-source-wins; manual edits as a high-precedence override only

**Pros:** Smallest change — no union logic. **Cons:** Cannot combine file + provider value *sets* (the core ask); dedup-across-sources impossible. Rejected — does not solve the problem.

#### C — Materialize merged values into a real column / write-through on edit

**Pros:** Reads are trivial (no merge at read time). **Cons:** Breaks the [ADR-013](ADR-013-metadata-field-mapping.md) "pure re-interpretation" invariant, loses per-value provenance, and couples display state to disk writes. Rejected.

### Decision 2 — write pipeline

#### A — Durable DB-backed queue + bounded worker pool (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — first internal queue; needs restart + crash semantics |
| Cost | Low — SQLite table + a goroutine pool; reuses `WriteBatch` |
| Scalability | Good — concurrency cap is the backpressure knob |
| Team familiarity | Medium — `job_runs`/scan-mutex patterns exist, but no prior queue |

**Pros:** Survives restart (locked decision); throttles filesystem load; batches atomically via the existing `WriteBatch`; observable via `job_runs`. **Cons:** Introduces the project's first durable job queue — restart/crash/idempotency semantics become a maintained invariant.

#### B — In-memory queue (replay nothing on restart)

**Pros:** Simplest. **Cons:** Drops queued writes on restart — directly contradicts the locked durability decision. Rejected.

#### C — Synchronous per-field writes (status quo, no queue)

**Pros:** No new infra. **Cons:** No atomic all-fields batch, no backpressure, bulk curation thrashes disk and blocks the request. Rejected.

---

## Trade-off Analysis

The central tension is **read-time merge (purity) vs write-time materialization (simplicity)**. Choosing read-time merge (1A) keeps the resolver a pure function of stored data — provenance, casing, and precedence all stay re-derivable when config or curation changes, with no re-fetch — at the cost of a richer `ResolvedField` and a merge step on every detail render. Given that detail-page resolution is already per-request and the curation map is pre-loaded like enrichment, the runtime cost is negligible and the architectural cleanliness (one place where "what is true about this field" is decided) is worth it.

For writes, the tension is **durability/throttling vs simplicity**. The owner's explicit ask for a persistent, crash-safe, throttled queue (2A) rules out the in-memory shortcut (2B). The real cost is conceptual: this is the first durable queue in a codebase that has so far used a scan-mutex + best-effort `job_runs` recording. We bound that cost by keeping the queue deliberately small — one job per file, idempotent re-run, the file-safety guarantee borrowed wholesale from [ADR-041](ADR-041-metadata-writeback.md) so "resume on boot" needs no extra integrity machinery.

---

## Consequences

**What becomes easier**

- Owners curate metadata at the value level: merge file + manual + provider(s), dedup, edit, durably remove, and choose what reaches the file.
- Curated values become portable once written (any external tool reads the file's tags), and the `browse:true` shadow-store read-path (F27.4) shrinks as libraries are curated-and-written (carried from [ADR-041 Consequences](ADR-041-metadata-writeback.md#consequences)).
- Generalizing to `person` curation needs only a new `entity_type` — no resolver change.

**What becomes harder**

- `ResolvedField` gains per-value provenance + curation flags → coordinated changes in the SPA detail view and MCP field output.
- Two new tables + migrations (0013/0014) and the project's **first durable job queue**, whose concurrency / backpressure / restart-and-crash semantics are now a maintained invariant.
- Unicode normalization + per-field casing introduce edge cases (locale-insensitive case-fold, multi-script values) that need test coverage.

**What we'll need to revisit**

- **Rule-based / automatic writeback** ([ADR-041](ADR-041-metadata-writeback.md) Option B) — still deferred; this ADR keeps writes owner-triggered.
- **Synonym / fuzzy dedup** (`Sci-Fi` ≡ `Science Fiction`) — explicitly out of scope; an alias/synonym map is a separate decision.
- **Prior-value capture / undo** of the file's pre-write tag — still deferred ([ADR-041](ADR-041-metadata-writeback.md)); the curation store is reversible but the on-disk previous value is not snapshotted.
- **Queue durability tuning** — retry/backoff bounds and `done`/`failed` retention once bulk curation is exercised at scale.

---

## Action Items

1. [ ] Add migrations **0013 `metadata_curation`** and **0014 `writeback_queue`**.
2. [ ] Extend the resolver: merge-mode union, dedup (trim+casefold), per-field `casing`, `manual` source, suppress/nowrite application, per-value provenance; keep `precedence` path a regression-guarded no-op for scalar fields.
3. [ ] Extend `ResolvedField` + the detail API and MCP output to carry per-value provenance + curation flags.
4. [ ] Add owner-gated curation CRUD endpoints (add/edit/suppress/nowrite) — [ADR-030](ADR-030-access-control-gating-seam.md).
5. [ ] Build the durable write queue: enqueue-one-job-per-file, worker pool (`WRITEBACK_CONCURRENCY`, default 1), `kind=writeback` `job_runs`, boot recovery + orphan-temp sweep; reuse `WriteBatch`.
6. [ ] Evolve `POST /api/media/{id}/writeback` to accept the curated batch and return `202` + job handle (keep the single-field body back-compat).
7. [ ] Document `WRITEBACK_CONCURRENCY` and per-field `casing` in `docs/reference/configuration.md` and `docs/reference/canonical-fields.md`.
8. [ ] `/testing-strategy`: merge/dedup/suppression/casing unit cases + end-to-end curate→queued-write against the fake provider + queue concurrency/failure-isolation/restart tests.
9. [ ] `/security-review` before merge (file writes, untrusted manual input, owner gate).
10. [ ] Add the ADR-048 row to `docs/architecture/README.md` and note in [ADR-041](ADR-041-metadata-writeback.md) that Option C (batch) is partially realized here.
