# Studio entity (F38) — implementation design

**Status**: Design — implementation guide
**Spec**: [studio-entity.md](../specs/studio-entity.md) (F38, HOLODEX-11) · **ADR**: [ADR-053](../architecture/ADR-053-studio-entity-and-resolved-link-derivation.md)
**Date**: 2026-07-02

This is the component/flow-level design that turns ADR-053's decisions into an implementation
plan. It assumes the decisions are settled (data model, resolved-value derivation, deferred
identity ops) and answers *how* — package touchpoints, the reconcile algorithm, per-trigger
sequences, transaction boundaries, and the API↔handler mapping. It is grounded in the real
code: the F37 person slice is the template throughout.

---

## 1. Component map

```
                          ┌────────────────────────────────────────────────┐
   scan ──────────────►   │            RelinkVideoStudios(videoID)          │
   enrich apply ───────►   │  (internal/repo/studios.go — sole writer of     │
   decision PUT/DELETE ►   │   video_studios; own write tx; prune-on-empty)  │
   curation change ────►   │                                                 │
   startup backfill ───►   └───────────────┬─────────────────────────────────┘
                                           │ resolves via
                                           ▼
                      resolver.ResolveFields(NewVideoBaseline(v,extra),
                                             enrichment, curation, fields,
                                             opts(decisions))     ← pure, unchanged
                                           │ take resolved "studio" value(s)
                                           ▼
                      resolveOrCreateStudio(name) ─► studios ─► video_studios
                                                        │
                                                  studios_fts (triggers)

   reads:  GET /studios, GET /studios/{id}, facet, global search  ──► video_studios / studios (direct)
   detail: GET /studios/{id}.resolved[]  ──► ResolveFields(NewStudioBaseline(s), …) + studioize()
```

**New files**

| File | Contents | Template |
|---|---|---|
| `internal/db/migrations/0017_studios.up.sql` / `.down.sql` | `studios`, `video_studios`, `studios_fts` + triggers | 0001 people block |
| `internal/repo/studios.go` | `resolveOrCreateStudio`, `RelinkVideoStudios`, `ListStudios`, `GetStudio`, `StudioVideos`, `BackfillStudioLinks` | `internal/repo/aliases.go` + `repo.go` |
| `internal/resolver/studio_baseline.go` | `studioBaseline` / `NewStudioBaseline` | `person_baseline.go` (near-verbatim) |
| `internal/api/studios.go` | list/detail read handlers, `studioResolved`, `studioize` | `person_fields.go` + `getPerson` |
| `internal/api/studio_fields.go` | decision + curation owner-gated handlers, `mountStudioDecisions` | `decisions.go` / `curation.go` / `person_fields.go` |
| `web/src/routes/studios/+page.svelte`, `studios/[id]/+page.svelte` | list + detail pages | `people/` routes |

**Touched files**

| File | Change |
|---|---|
| `internal/repo/repo.go` | call `RelinkVideoStudios` after `UpsertVideo` commits (§4.1) |
| `internal/api/enrich.go` | call relink after `enrichVideoApply` / `enrichVideoClear` (§4.2) |
| `internal/api/decisions.go`, `curation.go` | call relink when the mutated field is `studio` (§4.3) |
| `internal/api/handlers.go` | mount studio read routes (public) + `mountStudioDecisions` (owner); global search + facet wiring |
| `internal/model` | `Studio` struct; `EnrichEntityStudio = "studio"` constant |
| `cmd/holodex` (or scanner bootstrap) | invoke `BackfillStudioLinks` once at startup as a job run |

---

## 2. Data model (migration 0017)

```sql
CREATE TABLE studios (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE video_studios (
    video_id  INTEGER NOT NULL REFERENCES videos(id)  ON DELETE CASCADE,
    studio_id INTEGER NOT NULL REFERENCES studios(id) ON DELETE CASCADE,
    PRIMARY KEY (video_id, studio_id)
);
CREATE INDEX idx_video_studios_studio ON video_studios(studio_id);

CREATE VIRTUAL TABLE studios_fts USING fts5(
    name, content='studios', content_rowid='id',
    tokenize="unicode61 remove_diacritics 2"
);
CREATE TRIGGER studios_ai AFTER INSERT ON studios BEGIN
    INSERT INTO studios_fts(rowid, name) VALUES (new.id, new.name);
END;
CREATE TRIGGER studios_ad AFTER DELETE ON studios BEGIN
    INSERT INTO studios_fts(studios_fts, rowid, name) VALUES('delete', old.id, old.name);
END;
CREATE TRIGGER studios_au AFTER UPDATE ON studios BEGIN
    INSERT INTO studios_fts(studios_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO studios_fts(rowid, name) VALUES (new.id, new.name);
END;
```

Down migration drops the three tables (and implicitly the triggers). No data preserved — links
re-derive from resolution.

---

## 3. The reconcile algorithm — `RelinkVideoStudios(ctx, videoID)`

Sole writer of `video_studios`. Idempotent; safe to call redundantly.

```
lock writeMu; BEGIN
  # 1. Resolve the studio field for this video (reuse the media-detail preload).
  v, extra   := GetVideo(videoID)                     # if soft-deleted → desired = ∅ (step 4)
  enrichment := EnrichmentForEntity(video, videoID)
  curation   := CurationForEntity(video, videoID)
  decisions  := DecisionsForEntity(video, videoID)
  resolved   := ResolveFields(NewVideoBaseline(v, extra), enrichment, curation,
                              [studioField], opts(decisions))
  desired    := trimmed, non-empty, de-duped set of resolved "studio" value(s)

  # 2. Reconcile to exactly `desired`.
  current    := SELECT studio_id,name FROM video_studios JOIN studios … WHERE video_id=?
  for name in desired without a current row: sid := resolveOrCreateStudio(name); INSERT OR IGNORE
  for (sid) in current whose name ∉ desired: DELETE FROM video_studios WHERE video_id=? AND studio_id=?

  # 3. Prune-on-empty: any studio touched by a DELETE above with zero remaining links.
  DELETE FROM studios WHERE id IN (<removed sids>) AND NOT EXISTS
         (SELECT 1 FROM video_studios WHERE studio_id = studios.id)
COMMIT; unlock
```

Notes:
- **`desired` for a single-valued mapping** is 0 or 1 name; for an operator-multi `studio` field it
  is the full resolved set (RD2). The algorithm is identical either way.
- **Soft-deleted / missing video** → `desired = ∅`, so all its links drop and any now-empty studio
  is pruned. This gives the "video stops counting toward its studio on soft delete" behavior
  (spec P0-2) *for the counts* — but see §6 for the read-side count filter, which is the primary
  mechanism; relink-on-delete is the belt-and-suspenders that also prunes.
- **`resolveOrCreateStudio`** mirrors `resolveOrCreatePerson` minus the alias lookup (no
  `studio_aliases` in v1): exact-name `SELECT` then `INSERT`. Runs inside the caller's tx.
- **Concurrency**: shares `repo.writeMu` with every other writer, so a relink and a scan upsert
  never interleave. Reads (list/detail/facet) are lock-free against `video_studios`.

---

## 4. Trigger sequences (ADR-053 §2 call sites)

All four converge on `RelinkVideoStudios`; each fires it **after** its own write commits, in a
separate transaction (ADR-053 Trade-off Analysis). The decision/curation hooks are field-scoped.

### 4.1 Scan upsert
```
scanner.persist → repo.UpsertVideo(v, extra)         # commits raw people/tag/metadata links
                → RelinkVideoStudios(id)              # separate tx; derives studio links
```
`UpsertVideo` returns the id; the scanner (or a thin `repo.UpsertVideoAndRelink` wrapper) calls
relink next. Not folded into `UpsertVideo`'s tx: relink needs enrichment/decision reads that would
widen that tx's lock scope, and scan is the highest-volume path.

### 4.2 Enrich apply / clear
```
enrichVideoApply → enrich.Enrich(video, id, provider, extID)   # writes entity_enrichment
                 → RelinkVideoStudios(id)
enrichVideoClear → enrich.Clear(video, id, provider)
                 → RelinkVideoStudios(id)
```
A re-enrich can change the candidate that an undecided (or provider-pinned) `studio` field resolves
to, so both apply and clear relink.

### 4.3 Decision + curation (field-scoped)
```
setFieldDecision / clearFieldDecision (media):
    after repo.SetDecision/ClearDecision commits:
        if canonical == "studio": RelinkVideoStudios(id)

setCuration / clearCuration (media):
    after the curation write commits:
        if field == "studio": RelinkVideoStudios(id)
```
The guard keeps the 99% case (a `title`/`actors`/… decision) from paying for a resolve. Implement
as a small helper `h.relinkIfStudio(ctx, videoID, canonical)` called from both media handlers.

### 4.4 Startup backfill — `BackfillStudioLinks`
```
cmd/holodex bootstrap (after migrations, before/around first scan):
    job := activity.Start(kind="studio-backfill")
    for each active, non-deleted video id:
        RelinkVideoStudios(id)
    job.Finish(linked=n, studios=m)
```
Idempotent: a second run reconciles to the same set (no-op). Recorded as an F21 job run so it is
observable. Batches over ids to avoid holding one giant transaction (each relink is its own tx).
Q2 (spec): a dedicated pass is the lean choice; piggybacking the first scan also works — the pass
is chosen for observability + not coupling backfill to a scan trigger.

---

## 5. Resolution for the studio detail page — `studioBaseline` + `studioize`

Near-verbatim copies of the F37 person code:

- **`studio_baseline.go`**: `studioBaseline{name}`; `Baseline(src)` returns `{name}` for
  `file:name`, `(nil,true)` for any other `file:` key, `(nil,false)` for providers — identical to
  `personBaseline`.
- **`studios.go` / `studioResolved`**: mirrors `personResolved` — preload
  `EnrichmentForEntity/CurationForEntity/DecisionsForEntity` with `EnrichEntityStudio`, build the
  studio field list, `ResolveFields(NewStudioBaseline(s), …)`, then `studioize()`.
- **`studioize`** = `personizeResolved` renamed: strip `InSync`, map internal `file` → payload
  `record` on decision/candidate/winning/per-value sources. The `record` constant and the
  `recordize` helper are entity-generic; **extract them once** (e.g. to a shared
  `recordVocabulary` helper) rather than copy — this is the one place to de-dupe F37↔F38 (flag in
  `/simplify`).

**Studio fields**: `name` (baseline + provider candidates, replace, **decisions rejected**),
plus the enrichment scalars from the P1 slice (`description`/`country`/`website` — Q1 picks
registry naming). Until enrichment lands, the field list is effectively just `name`, so the Details
section renders empty and the page is name + video grid (spec P0-6).

---

## 6. Read side — list, detail, facet, search

| Endpoint | Query shape | Notes |
|---|---|---|
| `GET /studios` | `SELECT s.id,s.name,COUNT(vs.video_id) FROM studios s JOIN video_studios vs … JOIN videos v ON v.id=vs.video_id AND v.active=1 AND v.deleted_at IS NULL GROUP BY s.id ORDER BY s.name` | active, non-deleted counts; empty studios already pruned so none show |
| `GET /studios/{id}` | studio row + `studioResolved` + paged videos (`StudioVideos`, same paging as person videos) | public; no `in_sync` |
| facet | studio block = the `GET /studios` counts; filter `?studio_id=` = `JOIN video_studios WHERE studio_id=?` | replaces `FacetValues(["Studio"…])` block |
| global search | add `studios_fts MATCH ?` to the mixed-entity union (ADR-017), new entity group | mirrors people/tags |

**Soft-delete is enforced on the read side** (the `v.active=1 AND v.deleted_at IS NULL` join
predicate), which is the primary mechanism; relink-on-soft-delete (§4, if wired) is a secondary
prune. At minimum the read predicate is required; wiring relink into the delete path is optional
polish (decide in S1 — the read predicate alone satisfies P0-2's counts).

**Back-compat**: the legacy `?studio=Acme` string filter on `GET /media` and the MCP `fields`
filter are **untouched** — they keep matching raw `video_metadata` values. Only the *facet UI*
stops generating `?studio=` and switches to `?studio_id=`. Document the intended divergence.

---

## 7. API surface → handler mapping

```
GET    /studios                                    studios.go        listStudios       public
GET    /studios/{id}                               studios.go        getStudio         public
GET    /media?studio_id={id}                        handlers.go       listMedia (+filter) public
PUT    /studios/{id}/fields/{canonical}/decision   studio_fields.go  setStudioDecision  requireOwner; name→400
DELETE /studios/{id}/fields/{canonical}/decision   studio_fields.go  clearStudioDecision requireOwner
POST   /studios/{id}/curation                      studio_fields.go  setStudioCuration  requireOwner
POST   /studios/{id}/curation/clear                studio_fields.go  clearStudioCuration requireOwner
POST   /studios/{id}/enrich/resolve|…              enrich.go (P1)    studio enrich      requireOwner
```

Decision/curation handlers are the media ones parameterized by entity type — reuse
`studioDecisionSource` (== `personDecisionSource`: `record`→`file`, reject literal `file`),
`personFieldByCanonical`-style validation against the studio schema, matched-provider precondition,
`manual_value` sanitize. `mountStudioDecisions` registered beside `mountPersonDecisions` in the
owner-gated group (`handlers.go` ~L249).

---

## 8. Edge cases / invariants (→ testing-strategy)

- **Adopt provider studio → link moves without rescan** (decision hook, §4.3).
- **Blank-pin an empty-baseline studio → no link** (`desired=∅`).
- **Fix last misspelled video → bogus studio pruned** (step 3).
- **Two videos, same resolved studio → one studio row, count=2** (resolve-or-create + PK).
- **Operator maps `studio` multi → n links per video** (RD2).
- **Soft-deleted video excluded from counts + listing** (read predicate).
- **Backfill idempotent** (second run no-ops).
- **`studioBaseline` additivity** — undecided enrichment field still resolves to provider (RD6).
- **`name` decision → 400**; **relink NOT fired** for non-studio field decisions.
- **Zero resolver-core diffs** — assert `ResolveFields` unchanged (the ADR-052 property).

---

## 9. Slice → file plan

| Slice | Files | Gate |
|---|---|---|
| **S1 backend** | 0017 migration, `repo/studios.go`, `resolver/studio_baseline.go`, `api/studios.go` + `studio_fields.go`, relink hooks, backfill, model consts, route mounts | `go test ./...`; derivation matrix green; zero resolver-core diff |
| **S2 frontend** | `studios/` routes, facet block, media-detail studio link, global-search group; chips reuse `SourceSelect`/`CurationFieldRow` | svelte-check, token-guard, 3-skin QA (design-handoff) |
| **S3 enrichment** (cuttable) | TMDB `studio` entity (`/search/company`+`/company/{id}`), registry fields, `/studios/{id}/enrich/*`, activity | provider tests; QA |
| **S4 QA + security** | integration, `/security-review`, 3-skin checklist | merge gate |

S1 ∥ S2 against the frozen payload (studio `resolved[]` == person shape minus rename); S3 after;
S4 gates merge. Effort L (per HOLODEX-11).
