# ADR-067: Filename-based metadata extraction with confidence-gated auto-apply and rollback

**Status:** Proposed  
**Date:** 2026-07-14  
**Deciders:** Kevin Rich  
**Supersedes:** ADR-041 (in scope: deferred "prior-value capture / undo" non-goal)  
**Related:** ADR-033 (metadata source plugins), ADR-041 (metadata writeback), ADR-066 (enrichment auto-apply)

## Context

The Holodex data ingestion model presently has two active metadata sources:

1. **File-embedded tags** (`file:` namespace) — owner-scannable via exiftool/ffprobe.
2. **Provider enrichment** (`tmdb:` namespace, [ADR-033](ADR-033-metadata-source-plugins.md)) — HTTP-fetched, curated, scored by confidence threshold (0.70–0.85, [ADR-066](ADR-066-enrichment-auto-apply-and-dismissal.md)), auto-applied above threshold to reduce owner review volume.

**Gap 1: Filename encoding.** Filenames frequently encode metadata (e.g., `[Studio] Title (Person, Year) Resolution.mp4`) that neither embedded tags nor provider enrichment has access to. At the owner's library scale (thousands of files), manually typing this data is not feasible. Today, the only filename signal is the scanner's fallback title (filename stem when tags are empty); structured parsing is absent.

**Gap 2: Merge propagation.** When the owner merges two People or Studio records (renaming the loser to the canonical name in the DB), that decision never reaches the files. Person merge is DB-only; the embedded tag retains the loser's name. Re-syncing N affected files by hand defeats the benefit of a merge.

**Gap 3: No undo.** [ADR-041](ADR-041-metadata-writeback.md) explicitly deferred "prior-value capture / undo" to avoid scope creep. At the scale of auto-applied extraction and merge-triggered rewrites, a single bad sync or misconfigured pattern can corrupt tag data across hundreds of files with no recovery path.

This ADR records two cross-cutting architectural decisions required to close Gaps 1 and 2 safely:

1. How to generalize confidence-gating (currently provider-specific in ADR-066) to a second, different-scoring candidate source (filename extraction).
2. How to add crash-safe, owner-controlled rollback without introducing complex versioning overhead.

## Decision

### 1. Confidence-Gated Auto-Apply for Extraction (Generalizes ADR-066)

Introduce a **second source of scored candidates** — filename-derived extraction — that feeds the same auto-apply-vs-review-queue model as provider enrichment, but with a **distinct, multi-component scoring rubric** tailored to extraction's different risk profile.

**Scoring architecture:**
- **Provider enrichment** (ADR-066): single-tier, one threshold (0.70–0.85 per field). Sourced from HTTP APIs; trust is bounded by provider correctness.
- **Filename extraction**: two-tier rubric with field-dependent scoring.
  - **Entity fields** (People, Studio): three weighted components — source agreement (0.30), value specificity (0.20), entity resolution (0.50). Entity resolution is weighted highest because exact loose-key match ([ADR-061](ADR-061-unified-entity-name-identity.md)) is the strongest signal that the value is *known*, not just plausible.
  - **Non-entity fields** (Title, Release Date, Comment, Genre, Movie, Scene Number): two weighted components — source agreement (0.50), value specificity (0.50). No entity tier because there's nothing to match against.
- **Three field tiers + tier-specific thresholds** (hardcoded for v1, subject to empirical validation):
  - High-stakes (People, Studio, Movie): 0.80 threshold
  - Medium-stakes (Title, Release Date): 0.70 threshold
  - Low-stakes (Comment, Genre, Scene Number): 0.40 threshold

**Critical hard rule:** For entity fields, auto-apply requires an **exact loose-key match** to an existing entity (entity-resolution component scores 0.50 from that match). Fuzzy matches (Jaro-Winkler, scores 0.20 in the entity-resolution tier) never satisfy the auto-apply gate, regardless of aggregate score. Fuzzy candidates still appear in the review queue *ranked* by similarity and *pre-filled* as a suggestion, reducing owner typing—but the owner always confirms.

This mirrors [ADR-061](ADR-061-unified-entity-name-identity.md)'s own principle: near-misses (fuzzy/loose) are human-reviewed, never auto-merged.

**Why this design (vs. alternatives):**

| Alternative | Trade-off | Why rejected |
|---|---|---|
| Unified confidence model (one threshold for both provider + extraction) | Simpler: single tuning knob. Cost: extraction's multi-component rubric would need to flatten into provider's single-tier model, losing the entity-resolution signal. | Extraction risks are materially different (filenames are untrusted, local only, no network latency); field stakes vary more (Movie merge is high-risk; comment is not). A unified model would either be too permissive on low-stakes fields or block too many high-confidence extractions. |
| Fuzzy matching contributes to auto-apply (no exact-match gate) | Simpler: no special case in code. Cost: auto-apply a person named "Al Smith" extracted from a filename against a fuzzy match to existing "Alice Smith". Homonym risk is real at thousands-of-files scale. | The exact-match gate is load-bearing for safety. Fuzzy suggestions in the queue let the owner confirm quickly (not typing from scratch) without risking silent homonyms. |
| Two independent implementations (no shared abstraction) | Low coupling. Cost: drift risk if thresholds or semantics diverge. | Acceptable for v1. If extraction and provider scoring both become complex enough to justify factoring, a second ADR can introduce a shared `CandidateAutoApply` interface. Premature abstraction here adds complexity now for savings we haven't earned yet. |

### 2. Snapshot-Based Rollback (Amends ADR-041)

Reopen ADR-041's deferred "prior-value capture / undo" non-goal: **add a new table `file_writeback_snapshots` that records the pre-write value of every field before the write lands on disk.** Revert is a forward write (same queue, same snapshotting) that restores the prior value.

**Model:**
```sql
file_writeback_snapshots
  batch_id              -- groups snapshots from one write operation
  video_id, field_key
  prior_value           -- raw tag value before write ("" if previously absent)
  written_at
  INDEX (batch_id)      -- for bulk revert
```

A "Revert" action on a batch (observable in activity history) creates a new writeback job whose target is the snapshotted value. The revert itself is snapshotted, so it can be re-reverted (undo of undo).

**Why this design (vs. alternatives):**

| Alternative | Trade-off | Why chosen over |
|---|---|---|
| **Snapshot + forward-write revert** (this ADR) | Storage: one row per field per write. Complexity: moderate (snapshot on every write, inverse write on revert). Cost: no time-travel—only undo the *last* write per batch, not arbitrary history. | **Chosen.** Snapshot overhead is <1KB per write; inverse-write simplicity lets us reuse existing F30 queue + crash-safety model. No new write path = high confidence in correctness. At owner-scale (<100K files), history depth is moot; undo-last is the 90% case. |
| **Full git-like versioning** (per-field history, pick any prior value) | Storage: O(versions) per field. Complexity: high (value comparison, diff logic, history queries). Benefit: arbitrary time-travel; owner can undo to any prior state. | **Rejected.** Overkill for extraction's use case (owner typically reverts last bad batch, not picks a random state from 3 weeks ago). Storage and code complexity not justified by the 10% case. |
| **Backup file on disk** (rename current file.tag to file.tag.bak before write) | Simplicity: one file operation per write. Cost: disk sprawl (doubles tag storage), manual cleanup burden, no unified history surface. Doesn't integrate with activity tracking. | **Rejected.** Disk sprawl at scale is real (every extracted file has a .bak). No clear way to manage retention or present history to the owner in UI. Snapshot in DB stays bounded and visible. |
| **No rollback** (accept ADR-041's non-goal) | Simplicity: no snapshot table, no revert logic. Cost: owner must manually fix tag corruption if a bad pattern or merge sync goes wrong. At thousands-of-files scale, manual recovery is not feasible. | **Rejected.** Auto-apply at scale (Gaps 1 & 2 solving) *requires* rollback; the gap would remain. |

**Composition with F30:**
- F30 ([ADR-048](ADR-048-metadata-curation-and-write-queue.md)) already owns crash-safe writes: copy → write → rename. Each job is atomic on disk.
- Snapshots are taken **before** the write starts, same transaction as the job record (durable in the same DB).
- Revert is a new writeback job; it queues normally and inherits F30's atomicity + concurrency limit (`WRITEBACK_CONCURRENCY`).
- No new write path = no new failure modes.

## Options Considered

### Auto-apply design options

**Option A: Unified threshold (single confidence model for all sources)**
- One threshold per field, applies to both provider and extraction.
- Simpler configuration, easier to reason about in aggregate.
- **Rejected:** Extraction's entity-resolution signal (exact match vs. fuzzy) is critical for safety; provider enrichment never needed it. Flattening loses that differentiation.

**Option B: Two independent implementations (this ADR's choice)**
- Provider: single-tier threshold per field (ADR-066).
- Extraction: multi-component rubric per field-type + tier.
- Encode the hard exact-match gate in extraction's routing logic only.
- Accepted: clear separation of concerns; staged rollout (can disable extraction behind a flag while provider works); future consolidation possible if both models converge.

**Option C: Shared `AutoApplyDecider` abstraction**
- Define an interface that both provider and extraction implement.
- Pros: future-proofs against drift, centralizes policy.
- Cons: adds indirection now for a 1-to-2 case; extraction's scoring logic is different enough that shared abstraction wouldn't significantly reduce code.
- **Rejected for v1:** Prefer explicit, field-specific implementations. If both models grow complex enough to justify abstraction, a follow-up ADR can introduce it without rework.

### Rollback design options

**Option A: Snapshot + forward-write revert (this ADR's choice)**
- Storage: 1 row per field per write.
- Complexity: moderate.
- Undo depth: last-write only.
- Accepted: minimal overhead, reuses F30 crash-safety, integrates with activity history.

**Option B: Full versioning (git-like history)**
- Store all prior values for every field; owner can pick any prior state.
- Pros: maximum flexibility; owner never has to worry about "how far back can I go?"
- Cons: unbounded storage, complex value diffing, no clear retention policy at scale.
- **Rejected:** Overkill for owner's actual workflow (reverts last bad batch 90% of the time). Storage cost not justified.

**Option C: Disk-side backup (file.tag.bak)**
- Simplicity: one file-copy operation per write.
- Pros: backup exists on-disk independently of DB.
- Cons: disk sprawl (N tag files → N + N backup files), no unified history UI, manual cleanup burden.
- **Rejected:** At thousands-of-files scale, disk overhead compounds. No clear retention model. DB snapshot stays bounded and visible in activity history.

**Option D: No rollback (accept ADR-041's deferred non-goal)**
- Simplicity: no snapshot table, no revert mechanism.
- Cons: owner has no recovery if a bad merge or misconfigured pattern corrupts tags across 100s of files.
- **Rejected:** Auto-apply at scale *requires* undo. The entire Gap 3 remains unsolved.

## Trade-off Analysis

### Confidence model trade-offs

**Permissiveness vs. safety:**
- High thresholds (0.80 for entity fields) reduce false positives but may hold back good extractions. Empirically, exact-match entity resolution should be rare (most people/studios extracted from filenames won't pre-exist); low confidence is expected.
- Low thresholds (0.40 for non-entity fields) assume comment/tag corruption is low-risk. True if filename tokens are mostly clean; fails if the owner's naming is inconsistent.
- **Mitigation:** Thresholds are hardcoded for v1 but subject to empirical validation. If real usage shows systematic misclassification, a follow-up can make thresholds runtime-tunable (per [ADR-060](ADR-060-runtime-settings.md)).

**Hard exact-match gate trade-off:**
- Blocks fuzzy entity matches from auto-applying, even if they score above threshold.
- Cost: owner must click to confirm "did you mean 'Alice Smith'?" for 10+ candidate suggestions.
- Benefit: homonym safety (no silent "Al Smith" → "Alice Smith" merge by accident).
- **Accepted:** The gate mirrors [ADR-061](ADR-061-unified-entity-name-identity.md)'s principle. Fuzzy suggestions in the queue still speed up confirmation (pre-filled, ranked by similarity) without risking silent data corruption.

### Snapshot-based rollback trade-offs

**Storage cost:**
- One row per field per write. Typical write: 1–5 fields affected per video. Typical batch: 10–100 videos.
- Snapshot table grows O(fields × batch_size) per write operation.
- At 1000 writes/year (rough estimate for owner's manually-triggered extractions + merges), snapshot table is ~50K–100K rows. Negligible against a DB already storing millions of media records.
- **Accepted:** Storage overhead is negligible at owner-scale.

**Undo depth:**
- Can undo only the most recent write per batch, not arbitrary history.
- If owner applies batch A (100 writes), then batch B (50 writes), only batch B can be reverted directly without first undoing the entire batch A.
- **Mitigation:** This matches owner's actual workflow: "I ran the wrong pattern and corrupted 50 files; undo that batch" is the common case, not "I want to go back to the state from 3 weeks ago."
- **Accepted:** Single-level undo is sufficient for the use case.

**Code complexity:**
- New table, new revert endpoint, new queue job type (inverse write).
- All reuse existing crash-safe F30 primitives; no new write mechanism.
- **Accepted:** Moderate complexity, high confidence (riding proven F30 model).

## Consequences

### Positive consequences
- **Gap 1 closed:** Filenames become a structured metadata source, parsed at scan-time and on-demand, reducing owner data-entry burden.
- **Gap 2 closed:** Merge decisions propagate to files automatically, keeping DB and tags in sync.
- **Gap 3 closed:** Owner can safely undo bad syncs without manual tag repair.
- **Scale:** With confidence-gating + rollback, auto-apply can run at library scale (all files) without requiring per-file review.
- **Safety:** Exact-match gate for entities prevents fuzzy-match homonyms from auto-applying; fuzzy suggestions still accelerate manual confirmation.
- **Composition:** No new write mechanism; revert reuses F30 queue, inheriting crash-safety and concurrency limits.

### Negative consequences / limitations
- **Thresholds are frozen for v1:** Empirical misclassification may surface only in production. A follow-up (runtime-tunable thresholds per ADR-060) needed if the hardcoded tiers prove wrong.
- **No arbitrary time-travel:** Owner can revert the last write batch, but not cherry-pick a state from week 1. This is acceptable for the use case, but differs from full versioning systems.
- **Filename patterns are a configuration dependency:** A bad regex pattern is a configuration error, not a code bug. Owner education is critical (pattern validation on save, clear documentation).

## Action Items

1. [ ] Create the two new migrations (0025: `metadata_extraction_review`, 0026: `file_writeback_snapshots`).
2. [ ] Implement F48.3–F48.4 (confidence scoring + routing) behind a feature flag; disable auto-apply (log-only) until this ADR lands.
3. [ ] Implement F48.9 (rollback foundation) before enabling extraction auto-apply.
4. [ ] Post-merge: collect empirical data on extraction misclassification (log threshold violations); propose threshold tuning if needed via a follow-up ADR or ADR-060 update.
5. [ ] Document the filename namespace in `docs/reference/canonical-fields.md`.
6. [ ] Add extraction/confidence/rollback test cases to `docs/testing-strategy.md`.

## References

- [Spec: On-demand metadata extraction (F48)](../specs/metadata-extraction.md)
- [ADR-033: Metadata source plugins](ADR-033-metadata-source-plugins.md)
- [ADR-041: Metadata writeback](ADR-041-metadata-writeback.md)
- [ADR-048: Metadata curation and write queue](ADR-048-metadata-curation-and-write-queue.md)
- [ADR-060: Runtime settings](ADR-060-runtime-settings.md)
- [ADR-061: Unified entity name identity](ADR-061-unified-entity-name-identity.md)
- [ADR-066: Enrichment auto-apply and dismissal](ADR-066-enrichment-auto-apply-and-dismissal.md)
