---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-193                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note:                # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
---

# HOLODEX-193 · On-demand metadata extraction from filenames & tags (F48)

Filenames become a real, structured metadata source (parsing + confidence-gated auto-apply/review),
and Person/Studio merges propagate to file tags automatically, with a snapshot/revert safety net.
Done means F48.1–F48.11 all shipped in code (docs already landed pre-branch) and the ADR-067 Action
Items closed.

**Design package:** [spec](../specs/metadata-extraction.md) ·
[ADR-067](../architecture/ADR-067-filename-extraction-confidence-and-rollback.md) ·
[design handoff](../design/metadata-extraction-handoff.md) ·
[testing-strategy](../testing-strategy.md) (§4/§5/Phase 3)

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [ ] spec `write-spec` → `docs/specs/**`
- [ ] architecture `architecture` → `docs/architecture/ADR-*`
- [ ] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [/] security `security-review`

<!-- Deferred gate example — carry the un-defer trigger so it isn't lost:
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [backend] Phase 1 — filename pattern parsing (F48.1) + `filename:` shadow-store write path
   (F48.2), pure/unit-tested — `internal/extract/`
2. [x] [backend] Phase 2 — confidence scoring (F48.3) + auto-apply/review routing (F48.4), behind a
   flag with auto-apply log-only until ADR-067 is Accepted — new `metadata_extraction_review` table
   (migration 0025)
3. [x] [backend] Phase 3 — rollback foundation (F48.9), `file_writeback_snapshots` (migration 0026)
4. [x] [backend] Phase 4 — extraction triggers (F48.5): on-demand → batch → import-time, in that order
5. [x] [frontend] Phase 5 — Extraction tab UI + preview (F48.6/F48.7)
6. [x] [backend] Phase 6 — merge → writeback propagation (F48.8)

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-15 · session
- skills: simplify, security-review

### 2026-07-14 · session
- skills: simplify, security-review

### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-07-16 · Phase 6 — merge → writeback propagation
- `internal/api/merge_writeback.go` (new): `propagateMerge` writes every affected
  video's full, current (post-merge) People/Studio name list to the embedded tag via
  the existing F30 write queue, sourced from the DB (`PeopleForVideos`/`StudiosForVideos`)
  rather than a partial string-patch of the on-disk value — correct even when a merged
  person/studio co-starred with someone else on a video (that name is preserved, not
  clobbered).
- `internal/repo/identity_ops.go`: `MergeEntities` split into a public wrapper plus a
  private `mergeEntities` core; a new `MergeEntitiesWithAffectedVideos` (and
  `MergePersonsWithAffectedVideos` in `aliases.go`) returns the affected-video list
  captured *inside* the merge's own transaction, before the move-associations step
  repoints them — atomic with the merge, not a separate racy pre-read. Every existing
  caller of `MergeEntities`/`MergePersons` (12+ call sites across 6 test files) is
  untouched; only the two merge HTTP handlers call the new variant.
- `internal/api/entity_identity.go`: `identityRoutes` gained `writebackField` (set to
  `"studio"` at the studio mount, empty for tag) so `mergeEntity`'s propagation decision
  is config-driven like every other per-entity specific, not a hardcoded
  `entityType == EnrichEntityStudio` check.
- F48.9d's migration-0026 comment and the revert route's own doc comment both explicitly
  anticipated this: multiple videos' writeback jobs sharing one `batch_id` so a single
  Revert restores all of them. Implemented it — migration 0027 adds `batch_id` to
  `writeback_queue`; `Queue.EnqueueBatch`/`EnqueueMany` (new) accept a caller-supplied
  batch id; `snapshotBeforeWrite`'s own-job idempotency check moved from
  `SnapshotsForBatch` (whole batch) to a new `SnapshotExistsForVideo` (scoped to
  batch+video) so one video's already-taken snapshot can't make a sibling video's job in
  the same shared batch skip taking its own — verified by a real-file (ffmpeg/exiftool)
  end-to-end test asserting both videos' snapshots land and one Revert restores both.
- Efficiency: `propagateMerge` enqueues all of a merge's affected-video jobs in one
  transaction (`EnqueueWritebackBatch`/`EnqueueMany`) rather than one `writeMu`
  acquisition + commit per video — matters at the spec's stated scale ("thousands of
  files"); mirrors `InsertWritebackSnapshots`'s existing single-prepared-statement
  pattern.
- Confirmed the writeback tag-mapping table (`internal/writeback/tags.go`) key for the
  People field is `"actors"`, not `"people"` — `internal/extract`'s own field vocabulary
  uses `"people"` throughout (matching the `{people}` filename token), so Phase 6 uses
  `"actors"` explicitly rather than copying that literal. Flagged as a likely-latent bug
  in the already-shipped Phase 2/4 extraction auto-apply path for `people` (not touched
  this session — out of scope for F48.8, but worth a follow-up: HOLODEX-193 comment or a
  new issue before `EXTRACTION_AUTO_APPLY_ENABLED` ever flips on for real).
- `/simplify` (4-agent review): fixed a real efficiency issue (the sequential-enqueue
  loop above), a duplicated `SnapshotsForBatchVideo`/`SnapshotsForBatch` scan loop
  (collapsed to an existence-only query), two near-identical name-flatten helpers
  (collapsed to one generic `namesByVideo[T]`), and the studio-specific hardcoded type
  check in the generic `mergeEntity` handler (`identityRoutes.writebackField`). The
  altitude review's most substantive finding — the affected-video pre-read racing the
  merge transaction — is the `MergeEntitiesWithAffectedVideos` split above.
- `/security-review`: no vulnerabilities found. Owner-gating unchanged (new code adds no
  routes, rides the existing `requireOwner`-gated merge endpoints); all new queries
  parameterized; tag content written comes from DB columns (`Person.Name`/`Studio.Name`),
  never from the merge request body (which only supplies numeric ids).
- skills: simplify, security-review
- handoff: F48.1–F48.9 (Phases 1–6) all shipped; only remaining spec-level TODOs are the
  cross-reference doc updates ("Cross-references to update on landing" in
  `docs/specs/metadata-extraction.md`: `canonical-fields.md`, `docs/architecture/README.md`
  ADR index entry — check both are current — and `qa-metadata-extraction.md`, which does
  not appear to exist yet) and the flagged `"people"`/`"actors"` field-key mismatch in
  Phase 2/4's extraction write path.

### 2026-07-15 · Phase 2 — confidence scoring + auto-apply/review routing
- Migration 0025 (`metadata_extraction_review`, partial unique index on
  `(video_id, field_key) WHERE status='pending'`).
- `internal/extract`: `confidence.go` (F48.3a/b weighted rubrics, tier/threshold table),
  `jarowinkler.go` (pure Jaro-Winkler + agreement/specificity classifiers — no existing
  fuzzy-match utility in the repo, so this is new), `routing.go` (`Route`: manual-override
  precedence + the entity exact-match hard gate, F48.3d/e), `process.go` (`Process`
  orchestration tying scoring → routing → review persistence → F30 write-queue enqueue).
- `internal/repo`: `ExactEntityMatch`/`EntityNames` (identity.go, reuse `nameKeyExpr` per
  F48.3c, no reimplementation), `HasManualSource` (decisions.go), `extraction_review.go`
  CRUD (Upsert in-place-on-pending, List, Resolve, Dismiss).
- `config.ExtractionAutoApplyEnabled` (env `EXTRACTION_AUTO_APPLY_ENABLED`, default false)
  is the ADR-067 Action Item 2 flag — ADR-060's generic runtime-settings mechanism doesn't
  exist yet (checked; all its own Action Items are unchecked), so this is a plain boot-time
  Config field, not a DB-backed owner setting. Gates only the *auto-apply* write path;
  owner-resolved review rows (F48.4c, once Phase 5 UI calls it) are human-supervised and
  not gated by this flag.
- No HTTP/API routes or triggers yet (Phase 4/5) — `Process` is fully unit/integration
  tested via fakes + a real sqlite repo, same posture as Phase 1's untriggered `Store`.
- Known v1 simplifications, flagged for empirical revisit (ADR-067 Action Item 4): the
  Jaro-Winkler fuzzy-match/agreement cutoff (0.85, borrowed from ADR-066's
  `StrongMatchThreshold` for consistency) and the non-entity "structured/complete" value-
  specificity heuristic (rune-length ≥3) are both underspecified by the spec's rubric
  tables and were chosen defensibly rather than derived from data.
- `/simplify` (4-agent review): factored `ExactEntityMatch`'s canonical→alias lookup into
  a shared `lookupByNameKey` helper also used by `resolveOrCreateByName`; `EntityNames` now
  reuses `enrichQueueEntities` instead of a second hand-written query; removed the unused
  exported `FieldTier`; added a cross-reference comment on `extraction_review.go`'s
  `ON CONFLICT ... WHERE` upsert noting it diverges from `person_images.go`'s
  delete-then-insert idiom for the same partial-unique-index shape, and why. One efficiency
  finding (`resolveEntityMatch` re-fetching `EntityNames` per call) was deliberately left
  for Phase 4 — no batch caller exists yet to hoist it for.
- skills: simplify
- handoff: Phase 3 (rollback foundation, F48.9, migration 0026) must land before Phase 4
  wires any trigger that could flip `EXTRACTION_AUTO_APPLY_ENABLED` on for real.
