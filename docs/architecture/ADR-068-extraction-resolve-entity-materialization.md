# ADR-068: Extraction resolve materializes entities via post-write re-extract

**Status:** Proposed
**Date:** 2026-07-20
**Deciders:** Project owner

**Extends:** [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md) (filename
extraction — this ADR closes a gap its resolve flow left: a resolved "create new" value never
became a DB entity) · [ADR-041](ADR-041-metadata-writeback.md)/[ADR-048](ADR-048-metadata-curation-and-write-queue.md)
(the durable write-queue whose post-write hook this ADR extends) · [ADR-047](ADR-047-per-item-metadata-refresh.md)
(the forced re-extract seam reused here). **Relates to:** [ADR-033](ADR-033-metadata-source-plugins.md)/[ADR-051](ADR-051-per-field-source-of-truth-decisions.md)/[ADR-052](ADR-052-baseline-source-contract.md)
(file-layer-is-baseline model — the reason materialization goes through the file, not an inline
insert). **Spec:** [metadata-extraction.md](../specs/metadata-extraction.md) (F48). **Issue:**
[HOLODEX-196](https://whoiskevinrich.atlassian.net/browse/HOLODEX-196) (QA of the ADR-067 rollout,
[HOLODEX-195](https://whoiskevinrich.atlassian.net/browse/HOLODEX-195)).

---

## Context

F48 QA (HOLODEX-195 step 4) surfaced two defects whose fixes are architectural, not incidental:

1. **A resolved "create new Person/Studio" never created the entity.** `resolveExtractionReview`
   enqueues a writeback that writes the name into the file's cast/studio tag, then marks the row
   resolved. But the write-queue worker's only post-write hook re-extracts embedded cover art
   ([ADR-048](ADR-048-metadata-curation-and-write-queue.md)); nothing re-reads the file's *metadata*. A
   Person/Studio row is created only by `resolveOrCreatePerson`/`resolveOrCreateStudio` inside
   `UpsertVideo`, which runs on a scan re-extract (size/mtime change) or an explicit Refresh
   ([ADR-047](ADR-047-per-item-metadata-refresh.md)). So after resolving, the tag was on disk but the DB
   had no entity and the video wasn't linked — the owner saw nothing happen. The ADR-067 worklog
   already flagged this ("no code path in the resolve handler that inserts a Person/Studio inline").
2. **A multi-person filename collapsed to one entity in the review UI.** The parser correctly
   splits `{people}` into N names ([ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
   F48.1d), and `Process` stores them ", "-joined in the single `metadata_extraction_review` row
   for `(video, field)`, but the review row carries one advisory `suggested_entity_id` and the
   frontend rendered one scalar value with a single-entity picker — so accepting via the picker
   wrote **one** name, silently dropping the rest of the cast.

### Forces

- **The file layer is the baseline truth; the resolver is the sole merge point**
  ([ADR-033](ADR-033-metadata-source-plugins.md)/[ADR-051](ADR-051-per-field-source-of-truth-decisions.md)/[ADR-052](ADR-052-baseline-source-contract.md)).
  An entity that exists in the DB but not in the file it was "extracted from" inverts that model
  and can diverge if the async file write later fails.
- **Entity creation already has one canonical path.** `UpsertVideo → replaceAssociations →
  resolveOrCreatePerson` (and `ReconcileVideoStudios` for studios) is how *every* Person/Studio is
  created from a file today. A second, resolve-time insert path would duplicate and could drift
  from it (normalization, alias routing, studio-link derivation).
- **Merge propagation must not regain a per-video cost.** [ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
  F48.8 already writes post-merge names to every affected video's tag. Those DB entities are
  already correct (the merge is a DB operation); re-extracting each one would double a large
  merge's I/O for no benefit.
- **No new schema for the multi-person case if avoidable.** A rollout-blocking QA fix should not
  require a migration unless the scalar model genuinely can't represent the data.

---

## Decision

### D1 — Resolve enqueues the file write; a post-write re-extract materializes the entity

The extraction resolve path is unchanged in shape: it enqueues a writeback of the chosen
value(s) and marks the row resolved. Entity creation is **not** done inline. Instead, the
write-queue's post-write hook — which already fires after a successful write — is extended: for a
write that could introduce a not-yet-in-DB entity, it calls a new file-only re-extract
(`refresh.Service.ReExtract`) on that video, which runs `BuildVideoFromFile → UpsertVideo →
resolveOrCreatePerson`/studio-relink. The just-written tag is read back through the *same*
canonical path a scan uses, so the entity is created and linked exactly as if the file had always
carried that tag.

`ReExtract` is the **file half of a Refresh only** — no provider re-enrich and no activity-history
row ([ADR-047](ADR-047-per-item-metadata-refresh.md)'s `Refresh` does both; those are wrong side effects for
a tag-sync). The post-write hook's `PostWriteFunc` gains the job's written `fields` so it can gate
the re-extract: it fires only for an entity field (`actors`/`studio`) whose write `source` can
carry an outside-the-DB value (`filename`/`manual`), and **skips** `merge` and `revert` (their DB
entities are already current). This keeps merge propagation ([ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md)
F48.8) at one write per video, no re-extract.

### D2 — Multi-person review stays a scalar row carrying per-value candidates (no migration)

The `metadata_extraction_review` row remains one per `(video, field)`. The queue *read* is
enriched: for an entity field it splits the ", "-joined value into per-value **candidates**, each
resolved against the identity spine ([ADR-061](ADR-061-unified-entity-name-identity.md)'s
`ExactEntityMatch`) to an existing entity (with its canonical name) or "new". The UI renders one
chip per candidate; the *resolve* path splits a multi-value field's edited manual value back into
the full list, so editing or swapping one person can no longer collapse the cast. People is the
only multi-value field; Studio is a single chip (which also gives the owner a one-click fix for a
mistyped studio). No schema change — the fix is entirely in the read projection and the
resolve-time split.

---

## Options Considered

### D1 — how a resolved value becomes a DB entity

#### A — Post-write re-extract through the canonical scan path (chosen)

**Pros:** Reuses the one existing create path (`UpsertVideo`), so normalization/alias-routing/studio
relink are identical to a scan; keeps the file as the single source of truth; the entity only
exists once the write actually landed on disk. **Cons:** Entity appears a moment *after* the async
write completes, not instantly on resolve; a re-extract re-reads the file (ffprobe/exiftool). Both
accepted: resolves are owner-initiated and low-volume, and the source gate keeps the cost off the
merge path.

#### B — Insert + link the entity inline in the resolve handler

**Pros:** Instant feedback. **Cons:** Duplicates `resolveOrCreate*`/`ReconcileVideoStudios` logic; a
DB entity can exist while its backing file write is still queued or has failed — a split-brain the
baseline model (ADR-033/051/052) is designed to prevent. Rejected: the speed win isn't worth a
second entity-creation path that can disagree with the file.

#### C — Nothing; rely on the next scheduled scan

**Pros:** No code. **Cons:** This is the bug — the owner resolves and sees nothing until an
unrelated scan happens to re-extract. Rejected.

### D2 — representing a multi-person review

#### A — Scalar row + per-value candidates in the read (chosen)

**Pros:** No migration; "Accept cast" already wrote all N via `splitJoined`; the fix is a read
projection plus a resolve-time split. Ships inside a QA pass. **Cons:** The advisory single
`suggested_entity_id` stays scalar (per-person fuzzy suggestion isn't surfaced); per-person
dismissal isn't possible (the whole field resolves or dismisses together). Accepted as the v1 the
owner chose.

#### B — One review row per person (migration)

**Pros:** Independent match/suggest/create/dismiss per person; reuses the single-entity picker
per row. **Cons:** Requires relaxing the `(video, field)` partial-unique index to admit a
per-value discriminator (migration), plus DTO/resolve/frontend rework — a spec-level change, too
heavy for a rollout-blocking fix. Deferred; revisit if per-person dismissal or per-person fuzzy
suggestions are wanted.

---

## Consequences

**What becomes easier**
- A resolved "create new" now actually creates and links the entity, through the same path a scan
  uses — the resolve UI's "will create new {Person|Studio}" promise now holds.
- Multi-person and mistyped-studio review both work with per-chip, click-to-edit control and no
  schema change.

**What becomes harder**
- The write-queue post-write hook is no longer purely cosmetic (cover art) — it can trigger a
  metadata re-extract with a DB write. The source/field gate is load-bearing: mis-gating it would
  re-extract every merge-propagated video. Covered by a unit test on the predicate.
- `ReExtract` is a second, narrower entry into the refresh service beside `Refresh`; the two must
  stay distinct (no re-enrich / no activity row on the post-write path).

**What we'll need to revisit**
- **Per-person review rows** (D2-Option B) — only if per-person dismissal or per-person fuzzy
  suggestion is wanted; needs a migration, explicitly deferred.
- **A kill switch for merge→writeback propagation** — noted in the HOLODEX-195 runbook rollback
  section as the one piece of F48 with no runtime off-switch; orthogonal to this ADR but adjacent.

---

## Action Items

1. [x] Add this ADR to `docs/architecture/README.md`.
2. [x] `refresh.Service.ReExtract` — file-only re-extract + studio relink, no re-enrich, no
   activity row (`internal/refresh/refresh.go`).
3. [x] `PostWriteFunc` gains the written `fields`; `cmd/holodex` wiring re-extracts for
   entity-field writes whose source is not `merge`/`revert` (`writeCanCreateEntity`).
4. [x] Per-value candidates in the extraction-queue read
   (`internal/repo/extraction_review.go`) + resolve-time split of a multi-value field
   (`internal/extract/resolve.go`, `IsMultiValueField`).
5. [x] Drop a bare 4-digit year misparsed into the `{people}` position (`internal/extract/pattern.go`).
6. [x] `/design-handoff` (spec-tracked): per-person chips with click-to-edit; Extract-all
   running feedback + Refresh.
7. [x] `/testing-strategy` (spec-tracked): parser year-drop, per-value candidate resolution,
   multi-value manual resolve split, `ReExtract` file-only invariant, the re-extract source gate.
8. [x] `/security-review` before merge: the resolve endpoint and merge writeback are unchanged and
   still `requireOwner`-gated; confirm the new post-write re-extract introduces no
   externally-influenced input (it re-reads a file already written from DB-sourced values) and no
   new route. Reviewed 2026-07-20 — clean, no findings: `ReExtract`'s path comes from a
   parameterized `RefreshTarget` and is passed as an arg (not a shell string) to ffprobe/exiftool;
   the per-candidate `ExactEntityMatch` is fully parameterized; the multi-value resolve split still
   flows through `enrich.SanitizeValues`; no new route, gating unchanged; no frontend `{@html}`/XSS.
