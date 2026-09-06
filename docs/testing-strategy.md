# Holodex Testing Strategy

**Status**: Draft (plan); Phase-1 implementation status below  
**Date**: 2026-06-05 (plan) · updated 2026-06-14 (Quick Wins batch: ADR-031/032) · 2026-06-29 (Owner tooling hub F35) · 2026-07-12 (F47 enrichment review workflow, ADR-066) · 2026-07-14 (F48 on-demand metadata extraction, ADR-067) · 2026-07-28 (F49 claimed provider keys, ADR-074) · 2026-07-29 (F50 tag governance & video enrichment, ADR-075) · 2026-07-31 (tag writeback exclusion, ADR-077, HOLODEX-239; tag categories, ADR-078, HOLODEX-240) · 2026-08-01 (tag & category create affordance, HOLODEX-243) · 2026-08-04 (unified nav search live filter, HOLODEX-249, pre-implementation) · 2026-08-05 (F52 owner-mode video editing: commentary field, poster upload, studio placement, file-metadata gating; F40 implementation begins) · 2026-08-05 (F53 two-tier video poster resolution, HOLODEX-253, pre-implementation) · 2026-08-05 (F54 configurable provider search patterns, ADR-080, HOLODEX-254, pre-implementation) · 2026-08-05 (F55 Poster View for the People list page, HOLODEX-255, [design handoff](design/people-poster-view-handoff.md), pre-implementation) · 2026-08-07 (tag detail hierarchy & categories, HOLODEX-259, epic HOLODEX-240) · 2026-08-09 (Entity Completeness Score, F55/HOLODEX-260, ADR-081/082, [design handoff](design/entity-completeness-handoff.md)) · 2026-08-10 (Two-tier field editing model, F56/HOLODEX-268, epic HOLODEX-267, [design handoff](design/two-tier-field-editing-handoff.md), pre-implementation) · 2026-08-11 (Video composite-key collision check, F56.3/HOLODEX-270, [spec](specs/video-composite-key-collision.md), [design handoff](design/video-collision-verdict-handoff.md)) · 2026-08-11 (Studio relationship-edit popover, F56.4/HOLODEX-271, [spec](specs/studio-relationship-popover.md), [design handoff](design/studio-picker-handoff.md)) · 2026-08-13 (Writeback hides the target file tag, HOLODEX-216, parent epic HOLODEX-167, [design handoff](design/writeback-target-visibility-handoff.md)) · 2026-08-16 (Core file-metadata fields manually editable + writable, HOLODEX-115, epic HOLODEX-167: `overview`'s `long_text` branch now renders `SourceBadge` like every other replace field; the bespoke `commentary` field (F52) is retired — a system owner maps whatever they want the file's Comment tag to mean via `metadata-mappings.yaml`, not a hardcoded facet) · 2026-09-02 (alias collapse, ADR-088/HOLODEX-306 — provider `also_known_as` values become real `entity_aliases` rows; plan written ahead of implementation) · 2026-09-04 (F59 film provider enrichment, ADR-089) · 2026-09-06 (Fire-and-forget writeback, ADR-091/HOLODEX-323, [spec](specs/fire-and-forget-writeback.md), pre-implementation — supersedes **ADR-073 D4 only**; the D1/D2/D3 rows below stand)  
**Scope**: Phases 1–3. Grounded in the ADRs (`docs/architecture/`) and phase specs (`docs/specs/`).

---

## 0. Implementation status — Phase 1 (2026-06-10)

The plan below is the target. What actually exists today:

**Automated tests (Go, `go test ./...`, real SQLite + temp-FS, no external binaries needed):**
- `internal/config` — precedence + **CLI overrides** (CLI > env > yaml > default), DB-path derivation.
- `internal/metadata` — resolution classifier boundaries; **pure exiftool/ffprobe mappers** (title/people/tags/date/raw-capture, dimension/duration) via inline fixture JSON.
- `internal/repo` — upsert idempotency, filtered list + person filter, FTS prefix search, deactivate-missing, and **concurrent-writer safety** (no `SQLITE_BUSY`).
- `internal/scanner` — walk + media filter, add/skip/remove reconciliation, change detection, and the **fs-watcher** indexing a newly created file.
- `internal/api` — list/detail/search/people/tags handlers, 404s, and **Range (206)** streaming via `httptest`.
- `internal/repo`/`internal/api` — **Tag Categories** (HOLODEX-240, ADR-078): CRUD, cross-table name collision with tags (both directions, create + rename), bulk assign/unassign idempotency, delete's cascade to `category_tags` (tags survive), the browse-facet expansion, `ListCategories`' `tag_count`/`tag_ids` fields, and `ResolveOrCreateTag`/`POST /tags` (owner-gated, no video attach, sharing the deny-list/length-cap/category-collision checks every other tag-creation path enforces).

**Verified by driven browser QA (not yet automated):** the SvelteKit UI end-to-end (scan → browse → filter → detail → Range playback), the facet autocomplete + pagination, and **all three theme skins** (per `.claude/CLAUDE.md`). The frontend builds on **Tailwind v4** (CSS-first config, ADR-025); because the theme is a runtime `[data-theme]` swap, skin QA also covers computed-token verification (`--accent`/`--radius`/`--font-*` resolve per skin) — the migration was checked this way, not just by eye.

**Deferred from the plan (known gaps, not silent):**
- The full **golden-fixture corpus** (`testdata/gen.sh` + `*.golden.json`) and `//go:build integration` tests that exercise *real* `exiftool`/`ffprobe`/`mkvpropedit` — the mapping logic is currently unit-tested against captured JSON instead. MKV multi-target-level precedence (ADR-010) is **not yet** covered by a real-file test.
- **Cache** tests — the in-process backend is deferred (ADR-022), so only Noop passthrough applies.
- Frontend **Vitest** component tests and the **Playwright E2E** suite — covered manually for Phase 1; to be automated.
- **Perf** (50k dataset, p95 budget) and **a11y/axe** automation — WCAG AA contrast was verified by computation across all three skins (F8.3), not yet wired into CI.

---

## 1. Principles

1. **Metadata correctness is the product.** Holodex's promise is "the file's tags are the source of truth" (goal + ADR-004). A silently wrong title or mis-bucketed resolution is the worst class of bug — it corrupts the user's trust invisibly. Extraction, precedence, and classification get the deepest, most adversarial testing.
2. **Test behavior against acceptance criteria, not implementation.** Every spec requirement (`F*`) has a Given/When/Then; tests map to those.
3. **Fast feedback by default, fidelity on demand.** Pure logic runs in milliseconds with no binaries; tests needing `exiftool`/`ffprobe`/SQLite/Docker are separated so the inner loop stays fast.
4. **Real dependencies over mocks where they're cheap and deterministic.** Use real SQLite (it's embedded) and real media fixtures (they're tiny). Mock only slow/non-deterministic externals (Phase 3 IMDB/TMDB HTTP).
5. **Don't chase a single coverage number.** Set per-layer expectations; weight effort by risk and blast radius.

---

## 2. The Pyramid for Holodex

```
            ┌─────────────────────┐
            │   E2E (Playwright)   │   ~10 flows: compose up → browse → search → play
            ├─────────────────────┤
            │  Integration         │   real SQLite + real media fixtures + real binaries
            │  (Go + testcontainer)│   scanner, extraction, API, MCP, migrations, cache
            ├─────────────────────┤
            │   Unit / component   │   pure logic (Go) + Svelte components (Vitest)
            └─────────────────────┘
```

- **Unit (many, ms):** merge/precedence logic, resolution classifier, filter→SQL builder, cache keys, mapping resolver, config precedence, Svelte components.
- **Integration (some, sub-second each):** anything touching SQLite, the filesystem, or `exiftool`/`ffprobe`. The bulk of *confidence* lives here for a data app.
- **E2E (few, seconds):** the real container serving the real UI, driven by a browser.

---

## 3. Test Fixtures — the critical enabler

Because extraction spans MP4/MKV and encoder variance (ADR-004) and MKV target-level precedence (ADR-010), a **deterministic fixture corpus is the foundation of the whole suite.** Fixtures are tiny synthetic media generated by a script (committed as a generator + small outputs), each paired with a **golden JSON** of expected extraction output.

### 3.1 Generation (CI-reproducible)
A `testdata/gen.sh` produces 1-second synthetic clips and writes tags via *different* tools to simulate the real fragmentation Holodex must absorb:

```bash
# MP4 with iTunes atoms via ffmpeg
ffmpeg -f lavfi -i testsrc=duration=1:size=1920x1080:rate=1 \
  -metadata title="Blade Runner" -metadata artist="Harrison Ford" \
  -metadata genre="Sci-Fi" -metadata date="2019-10-04" \
  testdata/mp4/fhd_full.mp4

# MP4 with atoms written by exiftool (different key surface)
exiftool -Title="Studio Test" -Publisher="Acme Pictures" testdata/mp4/publisher.mp4

# Cinematic 4K scope (height < 2160 — must classify 4K+, ADR-012)
ffmpeg -f lavfi -i testsrc=duration=1:size=3840x1606:rate=1 testdata/mp4/scope4k.mp4

# Near-miss FHD (1888 wide — must classify FHD via 10% tolerance, ADR-012)
ffmpeg -f lavfi -i testsrc=duration=1:size=1888x1062:rate=1 testdata/mp4/nearmiss_fhd.mp4

# MKV with tags at MULTIPLE target levels via mkvpropedit (ADR-010)
ffmpeg -f lavfi -i testsrc=duration=1:size=1280x720:rate=1 testdata/mkv/multilevel.mkv
mkvpropedit testdata/mkv/multilevel.mkv --tags global:testdata/mkv/multilevel_tags.xml
# tags.xml carries TITLE at level 50 (episode), a different TITLE at level 70 (collection),
# ACTOR at level 50, and a TITLE on a track (level 30) that must be IGNORED.

# Unicode / diacritics (search folding, ADR-017)
ffmpeg ... -metadata title="Amélie" testdata/mp4/unicode.mp4
```

### 3.2 Required fixture matrix
| Fixture | Exercises |
|---------|-----------|
| Full MP4 (all 6 core fields) | Happy-path extraction (F2.1–F2.6) |
| MP4 with `Publisher` only | Mapping source key (ADR-013), extended capture (F2.9) |
| MP4 no metadata | Filename/mtime fallback (F2.7) |
| MKV multi-target-level | Level-50 precedence; ignore track/collection (ADR-010) |
| MKV with cover art / MP4 without | Tier-1 embedded art vs. generate (ADR-009) |
| 4K scope (3840×1606) | Width-based classification (ADR-012) |
| Near-miss FHD (1888w) / HD (1152w) | 10% tolerance boundaries (ADR-012) |
| Unicode title "Amélie" | FTS diacritic folding (ADR-017) |
| Corrupt / truncated file | Skipped + logged, scan continues (NFR) |
| Zero-byte / zero-duration | No divide-by-zero; graceful skip |
| Filename with spaces/quotes/unicode | Path handling, no shell injection into subprocess |

### 3.3 Golden-file pattern
Each fixture has `*.golden.json`. Extraction tests compare output to golden; a `-update` flag regenerates goldens on intentional changes:
```go
if *update { os.WriteFile(goldenPath, got, 0644) }
want, _ := os.ReadFile(goldenPath)
assert.JSONEq(t, string(want), string(got))
```

---

## 4. Backend Strategy by Component

| Area | Type | Critical things to cover | Target |
|------|------|--------------------------|--------|
| **Metadata merge/precedence** (ADR-004) | Unit | exiftool value wins over ffprobe; ffprobe fills gaps; per-field fallback order | ~95% |
| **MKV target precedence** (ADR-010) | Unit + Integration | Level-50/untargeted authoritative; track(30) ignored; people/genre NOT inherited from 60/70; title MAY fall back | ~95% |
| **Resolution classifier** (ADR-012) | Unit (table-driven) | Every boundary: 1151/1152, 1727/1728, 3455/3456; scope content; portrait video | 100% |
| **Scanner — incremental** (ADR-018) | Integration | New→insert; unchanged→skip (zero extraction calls); size/mtime change→re-extract; missing→active=false; mid-copy (young mtime) skipped | behavior |
| **Scanner — reactivation & empty-walk guard** (ADR-018, [issue #26](https://github.com/whoiskevinrich/holodex/issues/26)) | Integration | A deactivated row whose file reappears **unchanged** is reactivated via the fast-path with **no re-extract** (counts as `updated`, not `skipped`); a walk that sees **zero media files** skips `DeactivateExcept` entirely so a transiently empty/unreadable media root never mass-hides the library; `StatByPath` surfaces `active`; repo `Reactivate(id)` flips one row | behavior |
| **Scanner — symlinks** (ADR-011) | Integration | Follow + canonical dedup (one card for 2 symlinks); loop doesn't hang; target outside MEDIA_PATH indexed; FOLLOW_SYMLINKS=false skips | behavior |
| **Repository / SQLite** | Integration | Real DB w/ migrations; junction integrity; WAL concurrent read during write; pagination envelope | ~85% |
| **Search / FTS5** (ADR-017) | Integration | Title FTS; diacritic fold ("amelie"→"Amélie"); global mixed-entity grouping incl. films gated on `films_enabled` (HOLODEX-283); bm25 ordering sanity | ~85% |
| **Filter→SQL builder** (ADR-006) | Unit | Each param; combinations; tag order-independence (`tags=1,2`==`2,1`); injection-safe params | ~90% |
| **Cache** (ADR-008) | Unit + Integration | Hit/miss; key normalization; **invalidation on reindex** (no stale list/detail); prefix invalidation; NoopCache passthrough | ~90% |
| **Metadata mapping** (ADR-013) | Unit + Integration | Many-to-one normalize; source precedence order; `multi:true` split; facet distinct values; reload endpoint | ~90% |
| **Config precedence** (ADR-014) | Unit | CLI > env > yaml > default; DATA_PATH-derived paths | ~90% |
| **Migrations** (ADR-016) | Integration | Up from empty; up/down round-trip; **data preserved** across a representative migration; abort on failure | behavior |
| **API handlers** (ADR-006) | Integration | Each endpoint happy+error; 404 unknown id; pagination; OpenAPI contract conformance | ~85% |
| **Range serving** (ADR-015) | Integration | Full 200; `Range:` → 206 + correct `Content-Range`; open-ended/suffix ranges; bad range → 416; serve-by-ID only (no path input) | ~90% |
| **Thumbnail pipeline** (ADR-009) | Unit + Integration | Queue dedup + high-priority-first ordering; state machine (NULL→embedded/generated/failed, failed retried by sweep only); scanner hook (art→ExtractEmbedded, else Enqueue); serve 404→200 contract; regenerate 202 + reset + enqueue; Tier-3 enqueues only NULL-state visible items; disabled → no-op; `nice` skipped on Windows; **real-ffmpeg frame gen** (`-tags integration` — catches argv/muxer breakage the stubbed seam can't); **two-tier poster resolution** (F53, HOLODEX-253, planned): `extractCoverArt`/`generateFrame` each produce `{id}.jpg` (ThumbnailWidth) and `{id}-poster.jpg` (PosterWidth) from a single extraction/seek — Tier 2 must not double-seek; `GET .../poster` falls back to thumbnail-tier bytes when the poster tier doesn't exist yet (lazy-backfill safety net, RD6) | ~90% |
| **MCP tools** (ADR-005) | Integration | 4 tools return schema-valid output; **parity with REST** (same filter → same ids); unknown id error; mapped-field params (Phase 2) | ~90% |
| **Observability** (ADR-019) | Unit | /healthz always 200; /readyz 503→200 after bootstrap; scan summary log shape; graceful-shutdown drains | smoke |
| **Activity read-model** (ADR-028, F21.1–F21.3) | Unit + Integration | `GET /admin/activity` shape; scanner `Status()` reflects idle/running + correct `trigger` + last-run counts (no hot-path lock); `job_runs` insert per pass + **30-day prune** + history survives restart; library counts via cache seam; **no-secrets invariant** (no paths/env/tokens — incl. history `error_message`) | ~90% |
| **Job-run attribution** (ADR-071, HOLODEX-207) | Unit + Integration | Migration 0028 **up and down** against a table that already holds rows (pre-migration rows default to unattributed; down drops the index and all three columns but **no data**); `entity_type`/`entity_id` recorded at each attributing path (writeback → its video, refresh → `report.VideoID`, enrich → its entity pair); library-wide kinds stay **zero-valued**, not sentinel; `batch_id` round-trips a **non-numeric** `merge-person-N-M` id — the shape the retired `/· batch (\d+)/` regex could not match, which left Revert unavailable for merge propagation; **no foreign key** (a run outlives the entity it names, so `entity_id` may dangle) | ~90% |
| **Job-history digest** (ADR-071 D3, HOLODEX-210) | Unit + Integration | `GROUP BY kind` roll-up (run count, error count, most-recent `last_run`); **`last_status` is the newest run's status** (SQLite bare-column-with-MAX), asserted against a kind whose older run succeeded but newest failed; **shape is invariant to run count** — seeding 500 more clean runs leaves `kinds` and `failures` lengths unchanged; a **clean window** returns an empty (non-nil) `failures`; failure list **capped** at `digestFailureCap` while each kind still reports its true error count; digest included in the **no-secrets** invariant (its `failures` carry full rows incl. `error_message`) | ~90% |
| **Owner gating seam** (ADR-030, F21.7) | Unit + Integration | Open (no token) vs gated (token set) on every owner route; **constant-time** token compare; **fail-loud** on non-loopback bind + no token; **CSRF** rejection of cross-site admin POST; frontend capability-flag toggle hides controls | ~95% |
| **Owner session cookie** (ADR-046) | Unit + Integration | `POST /session` exchange sets cookie on valid token / **401 + no cookie** on wrong; cookie value is **signed, not the raw token**; cookie carries `HttpOnly` + `SameSite=Strict`; gated route + `/capabilities owner` authorized **by cookie** *and* (no regression) **by header**; **tampered/expired cookie → 401 + expiring `Set-Cookie`**; `DELETE /session` sign-out (idempotent); **"trust this device" longer `Max-Age`** (server-set, not client-forgeable); sliding renewal same-class never resurrecting expired; gate-open exchange is a no-op (no cookie); `Secure` set except plain-HTTP loopback | ~90% |
| **Related-media endpoint** (ADR-031, QW2/QW3) | Unit + Integration | Person key = highest **global** video count (tie-break lowest id); tag key = **most distinctive** by `c·(1−c/N)` (a **near-universal tag is demoted** below a mid-frequency one; tie-break higher `c`, then lowest id); items **exclude current item**, **active-only**, **≤5**; `items:[]` valid when no siblings; `person`/`tag` null when the item has none; **404** unknown/inactive id; attached people/tags present (**no N+1**). Selection is **deterministic** (pin the chosen key); only the item *draw* is `RANDOM()` — assert **set membership / exclusion / count**, never order. **Stability** (client fetch-once-per-view) is a page-level test (§5), not the endpoint's | ~90% |
| **Provider client + contract** (ADR-033, F22.1) | Unit + Integration | `ProviderClient` against the **in-process fake**: `/describe` capability parse + `protocol_version` mismatch rejected; `/resolve` candidate ranking; `/enrich` field+asset shape; timeout/5xx/garbage → single fetch fails, server survives; **mocked, never live** | ~90% |
| **Provider registry / config** (ADR-033, F22.2) | Unit | Load `metadata-sources.yaml` (missing file → empty, no error); enable/disable; **atomic reload** (mirrors mapping store); disabled/unreachable provider skipped not fatal | ~90% |
| **Unified field resolution** (ADR-033, F22.3) | Unit | `sources` list **interleaving `file:` keys and providers** resolves first-present-wins; provider never overwrites a file-extracted first-class field unless ordered ahead of `file:`; pure re-interpretation (precedence change needs no re-fetch) | ~95% |
| **Shadow enrichment store** (ADR-033, F22.4) | Integration | `entity_enrichment` upsert keyed by (entity_type,entity_id,provider,field_key); confirmed `external_id` persists → re-enrich skips identity; **re-scan does not touch shadow rows**; clearing a provider removes only its rows | ~90% |
| **Matching paths** (ADR-033, F22.5b) | Unit + Integration | Embedded-ID present → deterministic auto-resolve; absent → name-search candidates returned for manual confirm; ambiguous/no-result handled; confidence surfaced advisory (never auto-applied in v1 — **superseded by F47/ADR-066's threshold-gated auto-apply, see below**) | ~90% |
| **Enrichment security** (ADR-033, F22.9) | Unit + Integration | Enrich endpoints behind `requireOwner` (401 without token); **SSRF allowlist** — core calls only configured `base_url`s, ignores provider-supplied redirect hosts; **untrusted response** values length-capped/sanitized, asset downloads size+content-type limited; **no upstream API key** in config/logs/read-model | ~95% |
| **MCP enriched fields** (ADR-033, F22.5f) | Integration | `get_person`/`list_people` return enriched fields **with provenance**; parity with REST | ~85% |
| **Enrichment dismissals** (ADR-066, F47 S1/P0-4) | Integration | `enrichment_dismissals` upsert keyed `(entity_type, entity_id, provider)`; dismiss is idempotent (re-dismiss = no-op, not a duplicate row); undismiss ("Try again") deletes the row and nothing else; a dismissed pair **blocks** `/resolve` from being called again for it until cleared; **`ON DELETE CASCADE`** removes dismissals with the entity, mirroring `entity_enrichment`/`entity_aliases` cleanup | ~90% |
| **Auto-apply routing** (ADR-066 D1, F47 P0-2/P0-3) | Unit + Integration | A `/resolve` returning **exactly one** `confidence >= 0.85` candidate routes straight to `apply()` (no picker, same code path as a manual pick); **two-or-more** strong, or only possible/weak candidates, never auto-applies; an auto-applied field is **indistinguishable** from a manual one in `field_source_decisions` (same `provider:<name>` provenance) — Keep/Revert (F36) works unmodified, no new undo table | ~95% |
| **Refresh / Refresh-all** (ADR-066, F47 P0-5/P1-2) | Unit + Integration | `refresh` calls `apply()` directly with the **stored** `external_id` — **zero** `/resolve` calls; **400** if the provider isn't linked; `refresh-all` fans out over only that entity's configured providers (small fixed N, not the catalog), each independently refreshed/auto-applied/surfaced-for-review per D1's routing — an ambiguous provider is **never silently dropped** from the response; one provider's failure/unreachable state doesn't abort the others' results | ~90% |
| **Enrich-queue listing** (ADR-066, F47 P0-1/P0-6) | Integration | `GET /owner/enrich-queue` membership = missing an `entity_enrichment` row for ≥1 provider whose `entity_types` includes the entity's type, **excluding** dismissed `(entity, provider)` pairs; **zero provider calls** on load (assert fake provider call-count stays 0); rows carry **one state per provider** (`unreviewed`/`auto_applied`/`needs_review`/`not_matched`), never a single collapsed flag; `requireOwner`-gated (401 without token) | ~90% |
| **`profile_url` scheme validation** (ADR-066, F47 P1-1) | Unit | A candidate's `profile_url` is served only when `http`/`https`; a hostile scheme (`javascript:`, `data:`) is **dropped server-side** before the response reaches the client — never forwarded as-is; absent/malformed URL → field omitted, not an error | ~95% |
| **Filename pattern parsing** (ADR-067, F48.1) | Unit | Table-driven parse over a fixture filenames × patterns matrix, no I/O; first-full-match-wins ordering, no match falls through to tag-only resolution unchanged; unmapped bracketed tokens (`{resolution}`) consumed for matching but produce no field value; multi-value `{people}` splits on the configurable delimiter; **a bare 4-digit year matched into the `{people}` position is dropped, not surfaced as a person** (F48.1f, HOLODEX-196 #3 — a name containing digits is kept); owner-edited pattern list validated on save (rejects unparseable token grammar), takes effect without redeploy (F41/ADR-060) | ~95% |
| **`filename:` shadow-store integration** (ADR-067, F48.2) | Unit + Integration | Parsed values write into the existing `entity_enrichment` under a new `filename` namespace, identical shape to `file:`/`tmdb:`, no migration needed; `filename:<field>` slotted into a field's `sources` list is picked up by the **unchanged** F27 `orderedSources` iteration (regression guard: no resolver code touched) | ~90% |
| **Extraction confidence scoring** (ADR-067, F48.3) | Unit | Entity-field 3-component rubric (source agreement/value specificity/entity resolution) and non-entity 2-component rubric each reproduce every named scenario (exact+entity-exists, exact+no-entity, fuzzy, garbled, conflict) at their specified score; entity resolution reuses F43's `nameKey` loose-key detector (imported, not reimplemented); a field carrying an existing `manual:` source always routes to review on re-extraction, regardless of score | ~95% |
| **Exact-match auto-apply gate** (ADR-067 D1, F48.3d/F48.4a) | Unit | A candidate scoring at/above its tier's `AutoApplyThreshold` **and** passing the exact-loose-key-match gate on the entity-resolution component enqueues a write via the existing F30 `WriteBatch`; a fuzzy-only entity match scoring above threshold still routes to review (routing asserted, not just the score) — mirrors F43/ADR-061's "near-miss never auto-merges" invariant for a second candidate source | ~95% |
| **Auto-apply / review-queue routing** (F48.4) | Integration | `metadata_extraction_review` carries one row per `(video_id, field_key)` (`UNIQUE … WHERE status='pending'`); re-running extraction on an already-pending field updates it in place, never duplicates; resolving a row (accept filename/accept tag/pick suggested entity/edit manually/dismiss) enqueues the write the same way F48.4a does and marks the row resolved without a refetch; dismissal is durable (F48.4d, mirrors F47 RD4) — it doesn't resurface until extraction is re-triggered for that file | ~90% |
| **Extraction triggers — one code path** (F48.5) | Integration | On-demand (single video), batch ("Extract all", `kind=extraction` observable via System Activity/ADR-028), and scan-time (import) triggers all call the same extraction function; the fixture-corpus regression guard asserts identical input produces an identical routing outcome via all three entry points; the scan-time trigger stays bounded by the existing `WRITEBACK_CONCURRENCY` queue limit — no new concurrency knob (F48.10d) | ~90% |
| **Rollback — snapshot + revert** (ADR-067 §2, F48.9) | Integration | Every write through the F30 queue snapshots the field's prior on-disk value into `file_writeback_snapshots` (grouped by `batch_id`) **before** the write lands; a "Revert" restores every snapshotted field to its prior value byte-for-byte via a normal writeback job — the revert is itself snapshotted, so it can be re-reverted with no special-cased write path; snapshot + job record share the write's transaction (crash-safety, composes with F30's copy→write→rename) | ~95% |
| **Merge → writeback propagation** (F48.8) | Integration | Completing a Person (or Studio) merge enqueues exactly one writeback job per affected video, rewriting the loser's name to the canonical name in the tag; **N** affected videos → **N** writeback jobs; no second confirm beyond the merge's own informed-confirm (F43 RD8); merge-triggered writes are snapshotted the same as any other write (revertible via F48.9); the filename itself is **not** rewritten by a merge (Non-Goals boundary) | ~90% |
| **Extraction resolve materializes entities** (ADR-068 D1 — gate superseded by ADR-073, F48.6g, HOLODEX-196 #4) | Unit + Integration | `refresh.ReExtract` is the **file half of a refresh only** — re-extracts + relinks studios, but does **not** re-enrich providers and records **no** activity row (asserted via fakes: zero provider calls, zero `RecordJobRun`); a missing/soft-deleted target errors before any write. The post-write *gate* is gone (see the ADR-073 row) — the hook now fires for every write | ~90% |
| **Post-write baseline resync + queued-write status** (ADR-073, HOLODEX-214) | Unit + Integration | The write-queue post-write hook fires after **every** successful write — table-driven over a plain replace field, an entity field, a `merge`-sourced write, and a `revert` — so the DB's stored copy of the file's tags (the ADR-052 baseline `in_sync` is computed against) never describes the pre-write file; the synchronous `writebackMedia` branch does the same read-back, so neither write path can drift. `GET /writeback/jobs/{id}` is `requireOwner`-gated (**401** without owner auth, **200** with it, **400** bad id) and reports `pending`/`running` in flight, `failed` **carrying the queue's own error message**, and `done` for an absent row — the deleted-on-success case, asserted explicitly so the "absent means done" contract is deliberate rather than incidental. The SPA poll (`$lib/writebackJob`, pure + injected fetcher) keeps polling through a **failed status fetch** (unreadable ≠ failed write), throws only on a `failed` job, **resolves** rather than throwing at the timeout (a write still in flight is not a failure), and stops when cancelled | ~90% |
| **Fire-and-forget writeback — per-video status, retry, dismiss** (ADR-091, HOLODEX-323, [spec](specs/fire-and-forget-writeback.md), planned) | Unit + Integration | **This row supersedes the *SPA-poll* half of the ADR-073 row above (D4 only) — D1/D2/D3 and their tests stand unchanged, and D1's unconditional post-write read-back is the precondition this feature depends on, so its existing coverage becomes a load-bearing regression guard rather than incidental.** New per-video status query over `writeback_queue.video_id`, table-driven across the four states that matter: pending only, failed only, **both at once** (a retry queued while an older failure is undismissed), and neither. `TestRetryWriteback_ResetsFailedToPending` — the reset plus `kick()`; the gap it fills is real, not defensive: `ClaimNextWriteback` selects `WHERE status = 'pending'` and `RecoverRunningWritebacks` resets only `running`, so **nothing** moves `failed` → `pending` today and `attempts` never exceeds 1 in practice — a test asserting auto-retry would be asserting a behaviour that does not exist. `TestDismissWriteback_DeletesRow` — the row goes, the `job_runs` row stays (RD2: the audit trail lives there, so the queue stays a work queue, not a log). `TestEnqueueWriteback_ClearsPriorFailedForVideo` — RD5's supersede rule, scoped to **that video only**, asserted with a second video's failed row surviving untouched. Retry and Dismiss are `requireOwner`-gated (**401** without the token, matching every other mutation in this table); Retry against an absent or already-`pending` row is a **safe no-op, not a 404** — the same derived-aggregate posture as `GET /writeback/batches/{batchID}/status` returning 200 with zero counts (line 191) | ~90% |
| **Per-person review candidates + multi-value resolve** (ADR-068 D2, F48.6f, HOLODEX-196 #1) | Unit + Integration | `ExtractionQueue` splits an entity field's `", "`-joined value into per-value candidates, each resolved against the identity spine (existing entity → canonical name + id, case-insensitive; else new); a non-entity field carries no candidates even when its value contains `", "`; `ResolveReviewAction` on a multi-value field (People) splits the owner's edited manual value back into the full cast (editing one name never collapses the field), while a scalar field (Title/Studio) keeps a comma-bearing value intact | ~90% |
| **Extraction security** (F48.10) | Unit + Integration | Extraction/review/revert endpoints `requireOwner`-gated (401 without token, controls absent from the SPA DOM); filename-derived values are length-capped/sanitized before storage/write, same posture as F30.6b's manual-value handling; **no new outbound network surface** — extraction is local parsing only (asserted by a zero-HTTP-calls check over the extraction path); the scan-time auto-trigger is bounded by the existing queue concurrency limit, no unbounded write pressure | ~95% |
| **Person aliases — store** (ADR-036, F23.1–F23.3) | Integration | `person_aliases` CRUD: add (trim, non-empty, ≤200 chars); **per-person case-insensitive uniqueness** (`COLLATE NOCASE`, idempotent add — no dup, no error); same alias allowed on two people; delete by id scoped to person (404 on unknown/foreign id); **`ON DELETE CASCADE`** removes aliases with the person | ~90% |
| **Person aliases — search** (ADR-036, F23.5) | Integration | `person_aliases_fts` MATCH surfaces the person by any alias; **diacritic fold** ("beyonce"→alias "Beyoncé"); **dedup** — a person matching both its name and an alias appears **once**; per-group `LIMIT` respected; canonical-name first. **Search videos include the matched person's media** (via `VideoFilter.PersonIDsAny`, OR-semantics) so searching a name *or* alias returns their library even when no video title matches; title matches still included + video-id de-duped | ~90% |
| **Person aliases — scan-time resolution** (ADR-036, F23.8) | Integration | the scanner write path resolves an extracted name **name → alias → create**: a file tagged with an alias links to the **canonical** person (no duplicate created) and a **re-scan keeps it merged** (cardinal merge invariant); name-hit fast path unchanged | ~90% |
| **Person merge** (ADR-036, F23.9) | Integration | `MergePersons` moves `video_people` as a **de-duped union** (a file under both names collapses to one); merged name → alias; prior aliases re-point to canonical; shadow enrichment dropped; duplicate row deleted; **self-merge & unknown-id error** | ~90% |
| **Alias/merge collision** (ADR-036, F23.10) | Integration | `PersonConflict` detects a name already owned by a *different* person (by name or another's alias), returns it with context, ignores the self case; add endpoint returns **409 with the conflict person** (never a silent merge) | ~90% |
| **Person aliases/merge — endpoints** (ADR-036, F23.2–F23.4/F23.9–F23.11) | Integration | `GET /people/{id}` includes `aliases`; `POST …/aliases` + `DELETE …/{aliasId}` + `POST …/merge` **behind `requireOwner`** (401 without token when gated); `400` empty/over-long alias, missing `from_id`, self-merge; `404` unknown person/alias; `409` colliding alias | ~90% |
| **Soft-delete — visibility seam** (ADR-037, F24.1–F24.2) | Integration | After `SoftDelete`, the item is absent from **every** read surface in one sweep: `ListVideos`/count, `GetVideo`→`ErrNotFound`, `PathByID` (stream)→`ErrNotFound`, `Search`, `Related` (subject), `ListPeople`/`ListTags` counts, `FacetValues`, `MetadataKeys`, `LibraryCounts.VideosActive`, `VideoVisible`→false; `Trash` shows exactly it. Soft-delete idempotent (re-delete keeps `deleted_at`); unknown id → `ErrNotFound` | ~95% |
| **Soft-delete — scanner survival** (ADR-037, F24.3) | Integration | A soft-deleted row whose file is still on disk survives a re-scan: not reactivated (guards the #26 fast-path), not re-extracted (upsert count unchanged), not deactivated (recorded as seen), `deleted_at` not cleared by `UpsertVideo`'s `ON CONFLICT`; counted skipped not added/updated | ~95% |
| **Soft-delete — restore / purge** (ADR-037, F24.4–F24.6) | Integration | `Restore` clears `deleted_at` (item returns to all views), `ErrNotFound` on a live item; `ExpiredSoftDeleted(cutoff)` selects only rows past the grace cutoff; `HardDelete` removes the row and **cascades** junctions/FTS; `PurgePath` reads a soft-deleted row's path (the one read that ignores `deleted_at`) | ~90% |
| **Purge job** (ADR-037, F24.4/F24.8) | Unit | `Sweep` purges expired + records a `job_runs` (`kind=purge`) row (Removed/Errors counts); **grace=0 → no-op** (no expiry query, no run); `purgeItem` treats a **missing file as success** (finish row delete) but a **permission/read-only failure leaves the row** (no `HardDelete`, error run, retry); `RemoveFiles=false` purges DB-only and keeps the file; `PurgeNow` soft-deletes-then-removes and `ErrNotFound` propagates | ~90% |
| **Soft-delete — endpoints** (ADR-037, F24.9–F24.10) | Integration | `DELETE /media/{id}` (soft, 204 + idempotent), `?purge=true` (hard now), `POST …/restore` (200/404), `GET /admin/trash` (purge_at computed) **behind `requireOwner`** (401 without token, nothing mutated); `404` unknown id / restore-of-live; purge-now disk failure surfaces a path-free message and leaves the item in Trash | ~90% |
| **Derived-field engine — formulas + `Derive`** (ADR-063, F45) | Unit | `deriveAge`/`deriveAgeAtDeath` present/absent/**unparseable** input, deathdate-branch (⇒ no running age), `floor`, **leap-day boundary** (2000-02-29 crosses once); `Derive(resolved, now)` **purity** (fixed `now` → deterministic, no I/O), stamping (`Computed`, `computed:` source, **nil** Decision), **mutual exclusion** (exactly one of age/age-at-death), ordering (row adjacent to `birthdate`); **grep guard**: no `time.Now` in `internal/resolver/` (AC-8) | 100% (formulas) |
| **Computed provenance token + guard** (ADR-063, F45, §D3) | Unit + Integration | `fieldsource.ForComputed`/`IsComputed`; `computed` **excluded from `Valid()`/`ForNamespace()`** (`Valid("computed:age")==false`); decision endpoint **rejects 400** a `Computed` canonical / any `computed:` source, writes nothing to `field_source_decisions` | ~95% |
| **Derived person fields — API** (ADR-063, F45, FR5/FR7) | Integration | `personResolved` (with a fixed `Handlers.now`) emits derived rows in `resolved[]` for a birthdate-bearing person, **omits** otherwise; **owner == visitor** payload (D3); row carries `computed:true`, `winning_source:"computed:age"`, `derived_from:["Birthdate"]`, **no** decision/candidates/in_sync; **time-varying** — advancing `now` increments Age with **no DB write**; golden no-op when no birthdate | ~90% |
| **Tag provenance + rescan-safe sync** (ADR-075 D3, F50 P0-1) | Integration | `video_tags` gains `source TEXT NOT NULL DEFAULT 'file'`; `replaceAssociations()` narrows from unconditional `DELETE ... WHERE video_id=?` to `DELETE ... WHERE video_id=? AND source='file'`, reinserting only the file-derived set (`INSERT OR IGNORE`, so a file tag colliding with an existing manual/provider row on the same `tag_id` is a no-op, not an error); the single highest-value test in this spec: seed a video with one `file`, one `manual`, one `provider:tmdb` tag, rescan, assert only the `file` row was touched and the other two are byte-identical rows (same `created_at`) | ~95% |
| **Tag deny-list enforcement** (ADR-075 D2, F50 P0-2/P0-3) | Integration | `denied_tags(term_key PK, term, created_at)`; the guard sits *inside* `resolveOrCreateByName` gated on `entityType==tag`, so it's enforced identically at all three callers — table-driven over scanner (`replaceAssociations`, denied term **skipped silently**, scan continues), manual attach endpoint (`ErrTagDenied` → **422**), and the materialization pass (skipped silently, no partial failure of the enrich-apply). Exact-string case-insensitive **not substring**: denying `gnome` must not block `garden gnome` or `Gnomes`; denying `Gnome` must still catch `GNOME`/`gnome ` (nameKey fold) | ~95% |
| **Tag hierarchy** (ADR-075 D1, F50 P0-4/P0-5) | Unit + Integration | Cycle guard walks the proposed parent's full ancestor chain before commit and rejects if the tag being reparented appears in it (table-driven: direct parent-of-self, grandparent-of-self, and a same-tree sibling that is *not* an ancestor must succeed); `WITH RECURSIVE` descendant expansion for tag-filtered browse/search returns a tag **and every descendant**, computed fresh per query (no denormalized cache to go stale); merge-driven reparenting — merging a tag with children reparents the children onto the survivor **in the same transaction** as the existing alias-registration + `video_tags` move, asserted atomically (a failed merge leaves children on the original parent, never orphaned) | ~90% |
| **Enrichment tag materialization** (ADR-075 D4, F50 P0-6) | Integration | Resolved `genres` values materialize into real `tags` rows via `resolveOrCreateByName(tag, name, source='provider:<name>')`, `INSERT OR IGNORE` into `video_tags`; **idempotent** — re-enriching the same video N times leaves exactly the same `video_tags` rows (no duplicate inserts, no duplicate alias chains); **alias-canonicalizing** — a provider value that is an existing alias (e.g. `"azure"`) attaches under its canonical name (`"blue"`), asserted by inspecting the inserted row's `tag_id`, never a second `azure` tag; a denied term in the resolved `genres` set is skipped the same as any other deny-list path (not a special case) | ~90% |
| **Tag attach/detach endpoints** (F50 P0-7/P0-8) | Integration | New `POST/DELETE /api/v1/videos/{id}/tags`, `requireOwner`-gated (**401** unauth); attach routes through `resolveOrCreateByName(source='manual')` — same near-miss/collision surface `/tags`' own rename/alias flow already exercises, so this reuses that test fixture rather than duplicating it; detach removes only the one `(video_id, tag_id)` row (a tag shared by other videos is untouched); denied-term attach returns **422** with the term named, not a generic error | ~90% |
| **Genre writeback — ancestor chain + dual-filter** (F50 P0-9/P0-10) | Integration | Writeback assembles the full ancestor chain per tag (`"Animal; Dog; German Shepherd"`, ADR-041's existing `genres`→`Genre` container mapping unchanged — this only changes what feeds the field); the **union of curated tags and raw TMDB genres** is deny-list-filtered on **both** sides before assembly — the regression case is a denied term present only in the raw TMDB `genres` value (never materialized as a `tags` row) reaching the file if only the curated side were filtered; table-driven over "denied in curated only", "denied in TMDB-raw only", "denied in both" | ~90% |
| **Tag writeback exclusion — flat filter** (ADR-077 D1, migration 0033) | Integration | `TagNamesForVideo`'s final projection gains `WHERE t.writeback_enabled = 1`, the recursive ancestor walk itself untouched — `TestTagNamesForVideo_WritebackFlagFlat` seeds a 4-level chain (`Animal > Mammal > Dog > GermanShepherd`), disables the mid-chain "Dog", and asserts Dog alone drops while **both** further ancestors (`Animal`/`Mammal`) and the descendant leaf (`GermanShepherd`) survive — re-enabling restores all 4; `ListTags`/`GetTag` continue to return the disabled tag (it stays searchable, only Genre writeback is affected) | ~95% |
| **Tag writeback exclusion — manual sync + batch status** (ADR-077 D2/D3, HOLODEX-239) | Integration | `TestSyncTagWriteback_RecomputesFullUnion`: a sync enqueues the video's **current full** `genres` writeback value (every still-enabled tag on the video), not just the synced tag in isolation; `TestSyncTagWritebackBulk_DedupsSharedVideo`: a video carrying two selected tags is enqueued **once** via `VideoIDsForTags`' deduplicated union, not twice; `TestSetTagWriteback`/`TestSetTagsWritebackBulk_AppliesRegardlessOfPriorState`: the flag-flip endpoints (`PATCH /tags/{id}/writeback`, bulk) never enqueue (proved via a nil write queue that would panic on any accidental `Enqueue` call), only the sync endpoints do; `TestWritebackBatchStatusEndpoint`: `GET /writeback/batches/{batchID}/status` aggregates `writequeue`/`job_runs` rows sharing a `batch_id` into `pending`/`running`/`done`/`failed` counts end to end over HTTP, including the pending→done transition and an **unknown batch id returning 200 with zero counts** (not 404, since a batch is a derived aggregate, not a stored entity); all four endpoints are `requireOwner`-gated (401 without token) | ~95% |
| **Tag Categories — entity + cross-table collision** (HOLODEX-240, ADR-078 D1/D3) | Unit + Integration | `CreateCategory`/`RenameCategory`/`DeleteCategory` same-table collision (`ErrNameTaken`) and no-op self-rename; **cross-table collision both directions** — a category can't take an existing tag's name and a tag can't take an existing category's name, using the **tag-style fold** on both sides (`"Sci Fi"`==`"SciFi"`) so the comparison is meaningful (ADR-078 Forces); covered at both insert (`CreateCategory`, `AttachTagToVideo`) and rename (`RenameCategory`) call sites; the scanner path (`UpsertVideo`→`replaceAssociations`) **silently skips** a category-colliding tag rather than failing the whole upsert (mirrors the F50 deny-list precedent) | ~95% |
| **Tag Categories — junction + cascade + facet** (HOLODEX-240, ADR-078 D2) | Integration | `AssignTagsToCategory`/`UnassignTagsFromCategory` are **idempotent, batched** (`INSERT OR IGNORE` — re-assigning an already-member tag alongside a new one is a no-op for that tag, not an error); `DeleteCategory` cascades to `category_tags` via `ON DELETE CASCADE` with **zero application cleanup code** — the category's member tags are unaffected and still resolve; `ListCategories`' `TagCount`/`TagIDs` fields (S5 addition) derive from one pass over `category_tags`, asserted against a populated **and** an empty category; the browse-facet `VideoFilter.CategoryIDs` expansion matches every video tagged with **any** member tag, reusing `TagIDs`' existing `EXISTS(...)` clause shape with no new primitive | ~90% |
| **Tag detail — children & category-membership queries** (HOLODEX-259, extends ADR-075/ADR-078) | Integration | `ChildrenForTag` returns a tag's **direct children only** (one level, name-ordered) — deliberately narrower than the existing `WITH RECURSIVE` descendant expansion browse/search already use, not a duplicate of it; a childless tag returns an empty (non-nil) slice, never an error. `CategoriesForTag` returns a tag's category memberships (name-ordered) via the existing `category_tags` junction — the read-direction mirror of `AssignTagsToCategory`'s write direction, no new table or index. Both wired into `GetTag`'s previously-absent `Children`/`Categories` fields; covered by `TestChildrenForTag` (`internal/repo/tag_hierarchy_test.go`) and `TestCategoriesForTag` (`internal/repo/categories_test.go`) | ~90% |
| **Configurable search-query patterns** (F54, ADR-080, HOLODEX-254, planned) | Unit | `internal/enrich/query.go`'s `BuildQuery`: three-tier precedence (operator `Source.SearchPattern` > provider-advertised `Manifest.PreferredSearchPattern` > `fileConfig.DefaultSearchPattern` > raw-title floor) table-driven over every tier combination; `{name}` vs `{name?}` token rendering — an optional token with no resolved value is **omitted**, a required token with no value **fails the whole tier through** to the next-lower tier (not rendered with a gap); `performers` = top-3 actors **only** (no director, per the spec's resolved open question) space-joined; `year` derived from `release_date`'s year component; an unknown token name is **rejected at config-load time** (warning logged) without disabling the provider, falling to the next tier at render time; `sanitizeTitle` (unconditional, no config gate) strips `[](){},`, strips `\b\d{3,4}p\b`/`\b[48]k\b` word-bounded case-insensitive (asserting `Agent 007`/`Suite 1080` are **unaffected** — the false-positive guard), collapses whitespace, and **falls back to the raw unsanitized input when the stripped result is empty or whitespace-only** (spec FR4/AC-8a, the empty-sanitization gap found during design handoff); a golden test asserts `POST /resolve`'s request-body shape is byte-identical to pre-F54 for a video with no pattern configured — only `hint.query`'s *content* changes, never the wire contract (D1) | ~95% |
| **`ResolveOrCreateTag` / `POST /tags`** (HOLODEX-240, ADR-078, S5) | Unit + Integration | Backs `/categories/{id}`'s "+ Add tag" control and `/tags`' own "+ New" pill (HOLODEX-243) — resolve-or-create **with no video attach**, sharing `resolveOrCreateByName`'s deny-list (422)/length-cap (400)/category-collision (409) checks with `AttachTagToVideo`; idempotent on a case/whitespace-variant re-resolve (same id, no duplicate); owner-gated (401 without token) | ~90% |
| **`ListTags` left join / zero-video tag visibility** (HOLODEX-243, resolves a HOLODEX-240 known gap) | Unit + Integration | `namedCountQuery` gained an `includeZero` parameter — `ListTags` passes `true` (left join), `ListStudios` keeps `false` (inner join, unchanged; no empty-creation path exists for studios, so nothing exercises the difference there). A tag created bare (via `/categories/{id}`'s "+ Add tag" or `/tags`' "+ New" pill) now appears in `GET /tags` immediately with `VideoCount=0` (`video_count,omitempty` — a zero count is an *absent* JSON key, not a `0`), instead of being invisible until some video is tagged with it. `TestResolveOrCreateTag` (repo) and `TestResolveOrCreateTagEndpoint` (api) both flipped from asserting the old exclusion to asserting the new inclusion | ~90% |
| **`ListPeople` poster version** (F55, HOLODEX-255, [handoff](design/people-poster-view-handoff.md) Surface 6, planned) | Unit + Integration | `TestListPeoplePosterVersion` mirrors the existing `TestListPeopleHeadshotVersion` (`internal/repo/person_images_test.go`) exactly, for the new `poster_id` correlated subquery: `poster_version` is `0` for a freshly seeded person with no images; inserting a `role: model.PersonImageHeadshot` image leaves `poster_version` at `0` (**the specific regression P0-6 exists to prevent** — a naive implementation that read `headshot_id` for both fields would show `poster_version` flip on a headshot-only upload); inserting a separate `role: model.PersonImagePoster` image then sets `poster_version` to that image's id while `headshot_version` is unaffected by it. Table-driven over the 2×2 (headshot present/absent × poster present/absent) matrix in one test, not four | 100% |
| **Completeness score + facet model** (F55/HOLODEX-260, ADR-081 D3, `internal/resolver/complete_test.go`) | Unit | `Complete`'s scoring formula pinned against the spec's worked example (`TestComplete_WorkedExample`); zero missing scored facets → score 100, `Actionability` **nil** not `0` (undefined ratio, not a zero one — `TestComplete_NoMissingFacets`); a field with no `registry.FieldDef.Criticality` tag (e.g. `deathdate`) is skipped entirely — unscored, unlisted, never drags the score toward 0 (`TestComplete_AllExcludedYieldsZeroScore`); a `Computed:true` field (D1's invariant, e.g. `age`) is never scored/listed even if it somehow carries a `computed:` `WinningSource` (`TestComplete_ComputedFieldNeverScored`); a missing **merge** field (RD1 — merge fields carry no `Candidates`) is always non-actionable, since actionability only ever reads a **replace** field's cached candidate list (`TestComplete_MergeFieldMissingIsNeverActionable`) | 100% |
| **Completeness computation + asset-facet injection** (F55/HOLODEX-260, `internal/api/completeness_test.go`) | Integration | `completenessForVideos` scores critical facets correctly against a seeded video (`TestCompletenessForVideos_ScoresCriticalFacets`); a facet marked not-applicable is excluded from **both** the score's weight sum and actionability's missing count — not counted as resolved, not counted as missing (`TestCompletenessForVideos_NotApplicableExcluded`); person `photo` and studio `branding_image` have no row from the normal `ResolveFields` pass (they're asset-delivered, not field-mapped) and are synthesized via `injectAssetFacet` — asserted present with the correct tier for a person/studio image already on file (`TestCompletenessForPeople_PhotoInjection`, `TestCompletenessForStudios_BrandingImageInjection`) | ~90% |
| **Browse sort/filter + facet summary** (F55/HOLODEX-260, `internal/api/completeness_browse_test.go`) | Integration | `Completeness` sort param is silently ignored (falls back to default order) for a non-owner request, honored for an owner one, on **all three** entity lists (`TestListMedia/People/Studios_CompletenessSort_OwnerGated`); sort actually orders low→high or high→low correctly (`TestListMedia_CompletenessSort_Orders`); the browse "Missing {facet}" filter narrows to entities where that canonical is unresolved (`TestListMedia_MissingFacetFilter`); `GET /completeness/facets` returns the per-facet missing-count summary the filter chip's options are built from (`TestCompletenessFacets`) | ~90% |
| **Remediation queue** (F55/HOLODEX-260, `internal/api/completeness_queue_test.go`) | Integration | Facet groups sort critical-criticality-first, then by missing-count descending within a tier (`TestSortFacetGroups_CriticalFirstThenCountDesc`); the queue groups rows by missing facet across entity types (`TestRemediationQueue_GroupsByFacet`); each group splits candidate-ready (has a cached, unapplied provider value) from needs-research (`TestRemediationQueue_ActionableSplit`) — the split the frontend's Apply-vs-Search/Upload row rendering depends on | ~90% |
| **Detail-page `completeness` field** (F55/HOLODEX-260, `internal/api/completeness_detail_test.go`) | Integration | `getMedia`/`getPerson`/`getStudio` each expose an owner-gated `completeness` field on the detail response — present (non-nil, populated) for an owner request, **absent** (`omitempty`, not null/empty) for a visitor request, computed from the same resolve pass the handler already runs for `resolved[]` rather than a second query (`TestGetMedia/Person/Studio_Completeness`) | ~90% |
| **Not-applicable mutation** (F55/HOLODEX-260, DD8, `internal/api/facet_not_applicable_test.go` + `internal/repo/facet_not_applicable_test.go`) | Integration | `PUT`/`DELETE /media/{id}/fields/{canonical}/not-applicable` round-trips set-then-clear (`TestFacetNotApplicableAPI_SetThenClear`); an unknown canonical (`registry.IsKnown` false) or a request against a non-existent video is rejected, not silently accepted (`TestFacetNotApplicableAPI_Validation`); the mutation is `requireOwner`-gated — 401 without the token (`TestFacetNotApplicableAPI_OwnerGated`); the repo layer's set/get/clear and its **batch** read across multiple entities (used by `completenessForVideos`, not a per-entity N+1) are covered independently of the HTTP layer (`TestFacetNotApplicable_SetGetClear`, `TestFacetsNotApplicableForEntities_Batch`) | ~95% |
| **Provider-link badge projection** (HOLODEX-266, ADR-083, `internal/api/external_links_test.go`) | Integration | `externalLinksForEntity` (the HTTP-layer projection both `getPerson` and `getStudio` call) and `Service.BuildProviderLink`/`verifiedClient`'s link-template persistence are exercised together against a fake HTTP provider, since `persistLinkTemplates` only fires as a side effect of a real provider action (`Resolve`) — a direct repo write would skip the D2 wiring under test. `TestExternalLinks_MultiBadge`: two distinct namespaced ids attached to the same person (via two `ReconcileVideoPeople` calls, additive across namespaces per `attachExternalID`'s `INSERT OR IGNORE`) round-trip as **two** independently namespace-split, labeled, resolved badges (ADR-083 D3, "one badge per stored id, 0..N"). `TestExternalLinks_Studio` mirrors this on the studio wiring with a **mixed** resolved+degraded pair, proving both `externalLinksForEntity` call sites share one projection. `TestExternalLinks_TemplateMismatch` table-drives the degraded state (ADR-083 D2): no `link_templates` declared, a template declared for a different namespace, for a different entity kind, and enrichment disabled entirely (`h.enrich == nil`) — all four yield a **label-only** badge (`url` key omitted, not empty-string or an error), never a broken link. `TestExternalLinks_MalformedIDSkipped` proves a stored external id without a `namespace:id` separator (the column has no format constraint) is silently dropped from the response, not surfaced as a partial/broken entry | ~90% |
| **Locked curation-relink commit** (ADR-084, HOLODEX-277) | Integration | `SetCurationChecked`'s optional `commit func()` runs under the same `writeMu` lock as the curation write, immediately after it succeeds — closes a race where two concurrent edits to different person-typed fields (`actors`/`director`) on the same video could each pass `check()` before either's relink landed, then interleave their check-write-relink cycles so the second `ReconcileVideoPeople` full-replace silently dropped the first's link; `ReconcileVideoPeopleLocked` is the locked core `commit` calls directly (no locking of its own — callable only from inside a `SetCurationChecked` callback), `ReconcileVideoPeople` a thin locking wrapper for callers outside one. `TestCurationAPI_PeopleConcurrentDifferentFields_NoLostUpdate` (`internal/api/curation_concurrency_test.go`) runs 20 rounds of genuinely concurrent HTTP requests adding to `actors`+`director` on the same video, asserting both links survive `video_people` every round | ~90% |
| **Films entity — CRUD, attach/detach, scene numbering** (F56, HOLODEX-279, ADR-085) | Unit + Integration | `internal/repo/films_test.go`: `TestCreateFilm`/`TestListAndSearchFilms` (`UNIQUE(name, year)` identity, not a bare-name unique like `studios.name`); `TestAttachFilmVideoCollisions` — attaching at an already-taken scene number **rejects naming the current occupant**, no silent swap/auto-bump (RD5); `TestBulkAttachFilmVideos` — the film-side multi-select attach sequentially auto-numbers from a caller-supplied start; `TestFilmsForVideo` — a video's film memberships read back correctly for the video-side picker. `internal/api/films_test.go`: `TestFilmsDisabled_RoutesUnregistered` — `films_enabled=false` means the routes don't exist (404), not merely 403-gated (spec's "server-side and real" requirement); `TestCreateFilm_GetOrCreate`, `TestFilmVideoAttachDetach`, `TestFilmVideoSceneCollision` (HTTP-layer mirror of the repo collision test, asserting the 409 shape) | ~90% |
| **Films entity — baseline union-of-scenes** (F56 RD2, `internal/resolver/film_baseline_test.go`) | Unit | `filmBaseline` mirrors `studioBaseline`'s shape: `TestFilmBaseline_NilFilmIsEmptyBaseline` (no film → no baseline, not a panic or a zero-value struct); `TestFilmBaseline_NameResolvesFromRecord` (the film's own `name`/`year`/`description` feed the `record:` baseline directly, not derived from scenes); `TestFilmBaseline_RD6Additivity` — cast/tags are the **union** of the film's scenes' resolved values, set semantics (no double-counting a person credited on two scenes); `TestFilmBaseline_RecordBlankPinSuppressesProvider` — an owner-blanked record field stays blank even when a provider candidate exists, the same suppression semantics every other baseline source already honors | ~95% |
| **Films entity — resolver source injection, suspend, disambiguation** (F56 RD7, `internal/resolver/film_source_test.go` + `internal/api/film_injection_test.go` + `internal/api/film_links_test.go`) | Unit + Integration | `TestResolveDecided_FilmSourceWins`/`TestResolveUndecided_FilmSourceNeverAutoWins` — a film registers as an ordinary namespaced `provider:film:<id>` competitor for `album`/`title` under the existing `field_source_decisions` precedence, never auto-wins without a standing decision; `TestResolveDecided_MultiFilmDisambiguatesByNamespace` — two films attached to the same video are two distinct sources, not a collision; `TestResolveDecided_FilmSourceSuspendedDropsField` — **the RD7/ADR-085 §5 suspend mechanism**: with `films_enabled=false` the candidate is never injected, so a decision pointing at it resolves via the pre-existing "decided source currently unmatched → falls back to file chip" path (the same edge case `field-source-of-truth-handoff.md` already documents) rather than a bespoke "unavailable" state — the decision row itself is untouched, so re-enabling restores it automatically without a resolve/relink cycle. `TestFilmSourceInjection_SceneVsFullFilm` — a scene file writes `album` only, a full-film file writes `title`+`album` (writeback eligibility keyed per-file, not per-film). `TestFilmVideosSurviveFullRelinkCycle` — **the single highest-risk invariant (spec P0-2)**: `film_videos` links are user assertions, never derived; a full `RelinkVideoEntity` pass (the same cycle that prunes derived `video_studios`/`video_people` links on an empty resolve) leaves every film attachment untouched, attached or not | ~90% |
| **Film-studio cascade — `decideStudioForVideo` extraction** (ADR-087 D1, HOLODEX-285, planned) | Unit + Integration | `decideStudioForVideo` is a pure extraction of `setFieldDecision`'s existing Studio branch (`internal/api/decisions.go:124-155`) — behavior must be **bit-for-bit unchanged** for the single-video path (spec P0-1's own acceptance criterion): the existing Studio-decision test suite (composite-key collision gate, `SetDecisionChecked`'s TOCTOU re-check, `relinkStudiosWithContext`) must pass unmodified against the extracted function, proving the refactor introduced no behavior drift; a new `TestDecideStudioForVideo_OverrideBypassesCollisionGate`/`_NoOverrideBlocksOnCollision` pair pins the `override` bool's effect directly on the extracted signature (the single-video handler still honors `body.Override`; the cascade below always passes `false`, since RD4's unconditional overwrite applies to a video's *prior decision*, not to this safety gate) | ~90% |
| **Film-studio cascade — `cascadeFilmStudio` + `VideoIDsForFilm` + endpoint** (ADR-087 D2/D3, HOLODEX-285, planned) | Integration | `TestCascadeFilmStudio_BestEffortPerVideo` — **the P0 invariant this feature exists to prove**: seed a film with 3 attached videos, force video 2 into a composite-key collision, assert videos 1 and 3 both land in `results` as `"enqueued"` with jobs in the shared batch while video 2 alone reports `"collision"` — the deliberate opposite of ADR-077's abort-on-first-error posture (line 191 above), so the test's own doc comment states why (a decision-set is a commit, not a read, per ADR-087 Forces) or a future reader could "fix" it into aborting. `TestCascadeFilmStudio_ValueReuseNoRecompute` — the writeback job's `Values` are exactly the `names` `decideStudioForVideo` returned, not a second read of the freshly-set decision. `TestCascadeFilmStudio_EmptyBatchWhenAllFail` — every video collides/errors → `batch_id == ""`, **zero** `EnqueueMany` calls (a nil write queue that would panic on any call, mirroring `TestSetTagWriteback`'s existing no-enqueue proof pattern at line 191). `TestCascadeFilmStudio_ZeroAttachedVideosIsCleanNoOp` — a film with no `film_videos` rows returns `{batch_id: "", results: []}`, not an error (ADR-087 Action Item 7's explicit ask). `TestVideoIDsForFilm` mirrors `VideoIDsForTag`'s existing shape (active/non-deleted only). `TestCascadeFilmStudioEndpoint_OwnerGated` — 401 without the owner token, matching every other `requireOwner` mutation in this table | ~90% |
| **`StudiosForVideos`/`FilmStudios` icon + video-count extension** (HOLODEX-290, [design handoff](design/studio-link-card-handoff.md), done) | Unit + Integration | `IconURL` is a computed serving URL (F51/ADR-079), not a stored column, so both queries widen their `SELECT` with a shared `studioActiveVideoCountSubquery` scanned into `Studio.VideoCount`, and populate `Studio.ImageVersions` via the existing `studioImageVersions`/`attachStudioImages` batch helpers; both API call sites (`internal/api/handlers.go`, `internal/api/films.go`) then call `setStudioImageURLs()` before serializing, converting `ImageVersions` into `IconURL` — no schema/migration change. `TestStudiosForVideos_IncludesIconAndCount`/`TestFilmStudios_IncludesIconAndCount` assert both fields round-trip for a studio with an icon and a nonzero count, **and** that a studio with no icon scans to a nil `ImageVersions`/empty `IconURL` rather than erroring or panicking; the count reflects live `video_studios`/film-scene participation at query time, not a stored/stale counter. **Shared-query regression guard**: `StudiosForVideos` is also called by `mergeEntity`'s writeback propagation (`internal/api/entity_identity.go:185`), which reads only `.Name` off each result — the widened `SELECT` must not change that call's behavior, proven by re-running the existing merge-writeback test suite unmodified against the widened query rather than only adding new icon/count-specific assertions | ~90% |
| **Provider aliases → the identity spine** (ADR-088 D1/D3, HOLODEX-306, planned) | Integration | Enriching a person whose provider payload carries `also_known_as` writes real `entity_aliases` rows with `source='tmdb'` — asserted through the enrich apply path against a fake provider (next to `TestServiceResolveEnrichClear`, `internal/enrich/enrich_test.go:202`), never by calling the repo helper directly, since the write exists only as a side effect of apply; `resolvePeopleCredits` (`internal/enrich/service.go:648`) is the existing precedent for a side-effecting write off a provider payload and the shape to copy. **The payoff assertion is a pair, and both halves must hold or the collapse shipped the old bug in a new place**: after enrich, (a) `r.Search` surfaces the person by the provider name (the `TestSearchMatchesAlias` route, `internal/repo/aliases_test.go:230`) and (b) a file tagged with that name links to the **existing** person rather than creating a second one (the `TestScanResolvesAliasToCanonical` route, `:316`) — both existing tests already prove the spine does this for an owner-typed alias, so the new test's whole job is proving a provider-authored row is *the same kind of row*. Re-enrich is idempotent (`INSERT OR IGNORE` on the generated `alias_key` — N runs, same row ids; `entity_aliases` has no `created_at`, so row identity is the id); the entity's own `nameKey` is skipped, so a provider echoing the canonical name never self-aliases; a value trimming to empty or over the 200-char cap is dropped, not stored. **Spec RD6's near-duplicate filter is table-driven and its narrowness is the assertion**: against a canonical `Hayao Miyazaki`, the candidates `Hayao-Miyazaki` and `Hayao  Miyazaki` are dropped while `H. Miyazaki`, `Miyazaki, Hayao`, and `宮崎駿` are all **kept** — the false-positive half matters more than the true-positive half, since an over-eager filter silently costs the entity reach. The filter is computed in Go at import time and must never touch the stored `alias_key` (a generated column, `lower(trim(alias))`); a test asserting the two folds stay distinct guards the F43 uniqueness semantics this spec explicitly does not reopen. Spec RD5: a second enrich whose payload **omits** a previously supplied AKA leaves that row in place — provider input is additive, and only the owner or a merge removes an alias. Studio parity in the same test — `entity_aliases` is polymorphic and `AliasPanel` is reused verbatim, so a person-only test leaves half the shipped surface unproven. **Two existing tests must be rewritten, not merely kept green**, and their edits are the D1 regression guard: `TestEnrichmentShadowStore` (`internal/repo/enrichment_test.go:13`) currently seeds `fields = {"aliases": [...]}` and asserts the multi-value split, and `TestPersonFields_Synthesis` (`internal/api/person_fields_test.go:12`) asserts `aliases` is the person's merge field — after D1 neither statement is true, and `resolved[]` must carry no `aliases` entry at all | ~90% |
| **Alias suppression durability** (ADR-088 D4, HOLODEX-306, planned) | Integration | Deleting a provider-sourced alias writes an `entity_alias_suppressions` row and a **second full enrich pass does not resurrect it** — the user-trust invariant the table exists for, asserted across a real re-apply rather than by unit-testing the skip predicate. This is the single-store analogue of `TestResolve_SuppressSurvivesReenrich` (`internal/resolver/resolver_test.go:331`), which proves the same guarantee for the merge-field curation path being retired; the new test should say so, or a reader will assume one supersedes the other by accident. Three deliberate asymmetries, each asserted so a later reader cannot "fix" them: deleting an **owner-authored** alias (`source=''`) writes **no** suppression (nothing would re-add it); suppression is keyed `(entity_type, entity_id, alias_key)`, so suppressing a name on person 12 leaves person 40 free to receive or add it; and an owner **manually re-typing** a suppressed name succeeds, creating an owner row alongside the standing suppression — the suppression gates the enrich path only, never the owner | ~95% |
| **Provider-alias collision → review queue** (ADR-088 D5, HOLODEX-306, planned) | Integration | A provider name whose `alias_key` is already held by **another** entity cannot be inserted (`UNIQUE (entity_type, alias_key)`, ADR-061 RD1). Enrich **skips it and still succeeds** — asserted by checking the person's bio/birthdate/photo all landed in the same run, since the real failure mode is one awkward AKA costing an entity its entire enrichment — and inserts the pair into the existing `identity_review_queue` with `variation='provider-alias'`, `id_lo`/`id_hi` ordered so the same pair reached from either direction is one row, not two. The alias is **absent** from the enriching entity afterwards; no code path merges the two entities (extends F43's "near-miss never auto-merges" to a second, non-owner-initiated candidate source). A candidate colliding with the **same** entity's existing alias is a plain no-op — no queue row, no error — asserted alongside, so the two collision shapes can never be conflated. The skipped set surfaces on the person and studio detail reads as `skipped_aliases`, **derived from the queue rather than stored per-entity, and returned to the DENIED side only**. The queue row is a pair and reads from both ends, but the panel's sentence ("<name> already belongs to another {noun}") is only true for the entity that was *refused* the name — on the entity that owns it, the same line asserts the opposite of the truth. The side is not stored, so it is derived: the row belongs to the caller when the **other** entity holds `detail`, by canonical name or as an alias, the same two routes `entityConflict` walks. Phrasing it as "the other side holds it" rather than "this side does not" also makes a stale pair silent on both pages once the holder frees the name. `TestSkippedAliasesForEntity` covers both holder routes, the denied side, the holder's silence, and the stale-pair case and that resolving the pair clears the line with no extra bookkeeping, and that a near-miss pair with no `detail` never leaks into it. `TestPersonDetail_AliasSourceAndSkipped` covers the HTTP shape and the owner gate: for a visitor the key is **absent**, not null or empty, so a visitor never learns a collision exists. `TestAliasSourceRoundTrips` pins `source` on both the per-entity and the batch alias read, which are separate SELECTs that could otherwise drift — a dropped `Scan` target would make every provider alias look owner-typed and the badge would simply never render. **A guard the plan missed and the implementation added**: `FlagNearMiss` consults `entity_keep_separate` before queueing, and a provider-alias collision must too — otherwise every re-enrich re-proposes a pair the owner already dismissed, which is F43 RD5's "a kept-separate pair never nags" violated by a source that repeats on a schedule rather than on an owner action (`TestProviderAliasCollisionRespectsKeepSeparate`; the pair is still *skipped*, just not re-queued). **Delivered** as `internal/repo/provider_aliases_test.go` — `TestApplyProviderAliases_{LiveOnArrival,Filters,AdditiveAndIdempotent,Studio,UnknownEntity}`, `TestProviderAlias{SuppressionIsDurable,CollisionQueuesReview,CollisionRespectsKeepSeparate}` — plus `TestEnrich{WritesProviderAliasesToSpine,SurvivesAliasWriteFailure}` (`internal/enrich/enrich_test.go`) for the wiring and its best-effort posture | ~90% |
| **Curation → alias promotion migration** (ADR-088 D6, HOLODEX-306, planned) | Integration | Reuses `openAt` (`internal/db/fold_test.go:19`) — migrate to N-1, seed pre-migration rows, migrate up, assert on transformed **data**. `TestMigration0022FoldsCaseDuplicates` (`:67`) is the directly copyable template and the only other migration test in the repo that asserts on data rather than schema shape. `metadata_curation` rows with `field_key='aliases'`: `add` actions become owner alias rows (`source=''`, `INSERT OR IGNORE`), `suppress` actions become `entity_alias_suppressions` rows, and the curation rows are then deleted — an owner who curated the old display-only row keeps the result, now searchable. **The failure the `INSERT OR IGNORE` guards, seeded explicitly**: a curated `add` whose value collides with an alias another entity already holds must not abort the migration and take the whole upgrade down with it — assert the migration completes and the colliding value is simply absent. Pre-existing `entity_aliases` rows take `source=''` from the column default with no `UPDATE`, and their id/alias pair is unchanged afterwards. `down` is exercised for schema reversibility only; promoted data is **not** un-promoted, which is documented in the migration rather than silently lossy. **Two things the implementation pass added that the plan had missed**, both now covered: (a) `entity_alias_suppressions` is polymorphic like `entity_aliases`, so no FK can express "a suppression never outlives its entity" — it needs the same three `AFTER DELETE` cleanup triggers 0022 gave aliases (`TestMigration0044SuppressionsDieWithTheirEntity`, asserting both the cleanup and its per-entity scoping); (b) those triggers live on `people`/`studios`/`tags`, **not** on the dropped table, so the `down` migration must `DROP TRIGGER` them explicitly or the next `DELETE FROM people` fails on a missing table — `TestMigration0044Down` deletes a person after migrating down, and that assertion was verified non-vacuous by removing the `DROP TRIGGER` lines and watching it fail. **Delivered** as `internal/db/migrations/0044_alias_source_and_suppressions.{up,down}.sql` + `internal/db/alias_collapse_test.go` | ~95% |
| **Registry removal + synthetic alias facet** (ADR-088 D1/D7, HOLODEX-306, planned) | Unit + Integration | Deleting the `aliases` `FieldDef` removes a `CriticalityNiceToHave` scored facet, so its replacement is asserted the way the existing asset facets are — `injectAssetFacet` (`internal/api/completeness.go:59`) is the seam, and the `TestCompletenessForStudios_BrandingImageInjection` / `TestCompletenessForPeople_PhotoInjection` / detail-endpoint / browse-filter quartet (lines 200–203 above) is the pattern to copy wholesale. The facet is synthesized by querying `entity_aliases` directly — the resolver never produces it — and is present for an entity with **≥1 alias of any source**, absent for one with none. Provenance must not leak into scoring: a person whose only aliases are provider-sourced scores identically to one who typed them (`source` is provenance, not privilege, D2). Guard on the denominator: removing a registry field changes every other facet's share of the weight sum, so pinned arithmetic is re-derived deliberately, never nudged until green — in the event it was `TestCompletenessForStudios_BrandingImageInjection` (3 facets → 4, so 33 → 25), since studio gains a facet it never had while person swaps one for another. **The row as planned was wrong about what "delete the FieldDef" achieves**, and the correction is the most valuable thing here: a stored `entity_enrichment` row does not vanish when its canonical is retired, it is **demoted**, and F39 auto-registration then renders the now non-canonical key as a display-only "Aliases" row — the second list surviving the collapse through a different door. Three further tests follow from that: the enrich path must stop storing the key (`TestServiceResolveEnrichClear` inverted to assert `aliases` is *absent* from resolved fields), the hardcoded `personFields` synthesis must stop emitting it (`TestPersonFields_Synthesis`, which also now asserts **no** synthesized field is `Multi` — a stray one would put the second list back), and a one-time backfill must promote-and-clear rows written before the upgrade (`TestPromoteEnrichmentAliases`, asserting the guards apply to old data, unrelated keys survive, and a second run is a no-op). **Delivered** as `TestCompleteness_AlternateNamesFacet` + `TestCompletenessForStudios_AlternateNamesFacet` (both assert the facet is blind to `source`, and that retired `aliases` is not *also* still scored — two facets for one concept would double-count the very duplication being removed) | ~95% |
| **Film cast lands film-level, never on the videos** (ADR-089 D1, HOLODEX-310, planned) | Integration | **The invariant this feature exists to prove, and the one a future refactor is most likely to break.** Seed a film with N attached videos, apply a provider cast, then assert every attached video's `video_people`, `field_source_decisions` **and** `file_writebacks` rows are byte-identical to their pre-apply state — not merely "no new person link", since the plausible wrong implementation reuses ADR-087's `cascadeFilmStudio` and would write decisions *and* enqueue writebacks. The test's doc comment must state why films deliberately diverge from ADR-087 here (cast is many-valued and per-scene; cascading it makes the union report back what the film wrote, destroying the coverage signal D2 depends on) or a reader will "fix" the asymmetry into consistency. Provenance round-trip: clearing the provider deletes that provider's `film_people_roles` rows and leaves owner-entered rows standing, mirroring every other cleared-field path. `film_people_roles` uses the `''` role sentinel migration 0043 inherited from 0037's lesson (line 40) — a provider cast credit with no role must land as `''`, never NULL, or the PK silently permits duplicates | ~95% |
| **Cast difference is a set operation on identity, not strings** (ADR-089 D2, HOLODEX-310, planned) | Unit + Integration | With a scene union of 10 and a billed list of 14 sharing 10 names, the detail payload carries 10 union entries and **4** difference entries — never 24, and never a name in both collections. The arithmetic is the assertion; a naive implementation returns both lists in full and the bug stays invisible until a real film renders. Difference is computed by **resolved person identity**, so an alias (`entity_aliases`, ADR-088) or a case variant of a union member is *not* reported missing — the false-positive half matters more than the true-positive half, since a phantom "missing" entry tells the owner their complete rip is incomplete. A billed name with **no** Person row at all is a genuine miss and must appear. Degenerate shapes asserted alongside so a later reader cannot conflate them: empty difference → the group is omitted, not rendered empty; empty union with credits present → the difference is the full billed list; no credits at all → no difference collection and no coverage counts in the payload | ~90% |
| **Year is a gated identity write** (ADR-089 D3, HOLODEX-311, done) | Unit + Integration | `films.year` is half `UNIQUE(name, year)` (migration 0043, films-entity RD8), so filling it from a provider's `release_date` is an identity change, not a field write. **Delivered** as `internal/api/film_year_test.go` + `film_year_internal_test.go`. `TestFilmEnrichApply_FillsYearFromReleaseDate` — a yearless film picks up the parsed year (the fake cans `2001-07-20`, so the assertion is on 2001, not on the string being copied). `TestFilmEnrichApply_NeverOverwritesAnExistingYear` — pins the fill-only rule against an owner-asserted year the provider disagrees with; without it, an overwrite would be a one-way door, since no prior value is stored to restore on clear. `TestFilmEnrichApply_YearCollisionWithheldAndNamed` — **the load-bearing test, and it asserts two things that must both hold**: both films' identity columns are unchanged and the occupant is named, *and* the provider's `description` still resolves. Asserting only the first would pass against an implementation that discarded the whole enrich — the exact over-reach ADR-089 D3 was amended to remove. `TestFillFilmYear` covers the repo states the HTTP path cannot reach: non-positive years are a no-op rather than a write of 0; a second fill is silently idempotent (so a refresh-all cannot flip a year); a same-name film collides only on the *taken* year and is free to take another. `TestFilmReleaseYear` fails closed on every unparseable shape — a garbage parse would claim `(name, <nonsense>)` as an identity, so `20-07-2001` and `0000-01-01` must yield 0, not a plausible-looking number. `TestFilmFields_NameHasNoProviderSource` pins the other half of the pair: `filmFields` synthesizes `name` from the baseline alone, and is handed a non-empty provider list so the assertion cannot pass vacuously — a future provider source appended there is an ungated rename and must fail here. **Non-vacuity verified** by deleting the collision guard and watching `TestFilmEnrichApply_YearCollisionWithheldAndNamed` and `TestFillFilmYear` both fail | ~95% |
| **`banner` in, `thumb` out** (ADR-089 D4, HOLODEX-312, planned) | Unit + Integration | `assetRoleFor("film", "banner")` and `("film", "backdrop")` both map to `model.FilmImageBanner`; `assetRoleFor("film", "thumb")` returns `ok == false` after the retirement, and `model.FilmImageThumb` no longer exists — a compile-time guarantee reinforced by a role-validator test rejecting `"thumb"`. **No migration**: `film_images.role` is `TEXT NOT NULL` with a descriptive comment, not a `CHECK`, so the test asserts a `banner` row inserts and round-trips against the *unchanged* 0043 schema — if someone later adds a `CHECK`, this test is where it surfaces. `UNIQUE (film_id, role, source)` already lets an uploaded and a provider-sourced banner coexist; asserted explicitly, since that asymmetry with the studio single-slot model (0043 lines 65-67) is deliberate and easy to "correct". Provider side: `buildMovieEnrichResponse` emits `{kind: "banner"}` from `det.BackdropPath` **only** for `entity_type == "film"` — a video enrichment must still emit no banner asset, or ADR-086's fields-vs-assets split for videos is silently reopened. The download passes the ADR-039 perimeter unchanged, asserted by pointing a fake provider's banner at a non-allowlisted host and confirming the enrich still succeeds with the poster and without the banner | ~90% |
| **Owner year edit vs. provider year fill** (ADR-089 D3, HOLODEX-317, done) | Unit + Integration | `repo.SetFilmYear` and `repo.FillFilmYear` differ on exactly one axis — overwrite — and share the `(name, year)` probe. `TestSetFilmYear_OverwritesWhereFillDoesNot` pins **both halves in one test**, so a later consolidation of the two cannot quietly grant providers overwrite rights over owner-asserted identity. `TestSetFilmYearEndpoint` covers set, overwrite, a non-positive year as a 400 (the owner typed it, so it is a request error rather than the fill's silent no-op), an unchanged column after a rejected input, and 404 for a missing film. `TestSetFilmYearEndpoint_CollisionIsAConflictNotAnError` is the load-bearing one and its **status code is the assertion**: a clash must return `200 {conflict}` and carry no `year` key. `NameEditControl` routes a resolved conflict to its inline verdict card and a rejected promise to a red inline error, so a 409 here would silently restore the red-error presentation HOLODEX-317 exists to delete — a test asserting only "it failed" would have passed against exactly that regression | ~95% |
| **Film billed cast is the union's complement, and creates nobody** (ADR-089 D1/D2, HOLODEX-310, done) | Integration | **Delivered** as `internal/api/film_cast_test.go`. `TestFilmBilledCast_OnlyTheComplement` — a union of 2 against a billed list of 4 yields exactly the 2 absent, never all 4; the arithmetic IS the assertion, because the plausible wrong implementation returns the whole billed list and that bug is invisible until a real film renders. `_MatchesByIdentityNotString` — a case variant and an alias of a union member both count as COVERED, via `repo.LookupEntityIDByName` (canonical nameKey then alias). The false-positive direction is the one that matters: telling an owner their complete rip is incomplete is worse than omitting one genuinely missing name. It also pins that two spellings of one person are **one** billed credit, not two. `_LinksKnownPeopleAndCreatesNone` — a billed name already in the library carries its `person_id` so the chip links, an unknown name stays inert with no id, and **the people count is identical before and after the render**: the invariant D1 was amended for, since writing `film_people_roles` would have manufactured a Person per billed performer. `_DegenerateShapes` — no provider cast at all yields no group and a zero total (an unenriched film's Cast section is untouched); full coverage yields an empty difference with a non-zero total so the page can say "all N". **Non-vacuity verified** by disabling the coverage check: three of the four fail, each with its intended diagnostic | ~95% |

### Critical invariants (adversarial tests — break these and the app lies)
- **Precedence**: a track-level (30) `TITLE="Commentary"` must NEVER become the video title.
- **No stale cache**: after re-indexing a changed file, the next list/detail/facet read reflects new data.
- **Scan idempotency**: scanning the same tree twice yields identical records and zero duplicate cards.
- **Resolution tolerance**: 3456→4K+, 3455→FHD; exact boundary behavior is pinned.
- **Mapping precedence**: file with both `Publisher` and `Studio` resolves per `sources` list order.
- **Migration safety**: user-authored Phase 3 data (aliases, enrichment) survives an up-migration.
- **A merge survives re-scan**: the scanner *reads* `person_aliases` to route an extracted name to its canonical person but never *writes* the table; a full re-scan (rebuilding `people`/`video_people`) leaves aliases intact and re-links alias-named files to the canonical person — it must never re-create a merged-away duplicate. (Ties to migration safety; the scanner-writes-nothing half mirrors the enrichment shadow layer.)
- **Never auto-merge same-named people**: adding an alias that already names a *different* person returns a 409 for owner confirmation; no code path silently collapses two distinct person rows or silently routes a homonym's files.
- **Case/whitespace never forks identity** (F43/ADR-061): a per-entity `nameKey` is unique across canonical names **∪** aliases; `"fox"`/`"Fox"`, an edge-whitespace variant, and (tags only) an internal-whitespace variant all resolve to the **one** entity — a second can never be created at scan or in the editor. Per-entity scope is load-bearing: `"sci fi"`≡`"scifi"` for a **tag**, but `"Mary Jane"`≢`"MaryJane"` for a **person**.
- **A merge survives re-derivation** (F43/ADR-061 RD6 — the derived-link analogue of "merge survives re-scan"): merging B→A registers B's name as an alias, so the derivation reconcile re-routes B's resolved name to A on the next pass; a rescan / re-enrich / decision change **never resurrects** the merged-away entity. **Studio** proves it via `RelinkVideoStudios`; **person** joins it under ADR-072 (F40), which makes `video_people` derived via the generic `RelinkVideoEntity` and routes person names through the alias table at derivation time — so a person merge without the registered alias is undone exactly as a studio one would be. Break this and the derivation silently undoes every merge on a derived-link entity.
- **The identity backfill auto-folds only the provably-safe** (F43/RD10): the one-time pass merges the **pure-case hard pairs** (survivor = lower `id`, decisions/curation/enrichment **moved not dropped** where non-conflicting) and **never** auto-merges a fuzzy near-miss — near-misses only ever seed the review queue. Idempotent (second run = no-op); the unique-index build cannot fail on residual dupes because the fold precedes it.
- **A kept-separate pair never nags** (F43/RD5): once the owner dismisses a near-miss (or picks "keep separate"/"create anyway"), no scan-time flagging or detector pass re-proposes that pair — the negative decision is as durable as an alias.
- **Name-identity and provider-identity never cross** (F43/ADR-061 D5 × ADR-055): resolve is **id → name → create** — a provider-supplied record keys by `<namespace>:<id>` (name display-only), a file/owner-authored record keys by `nameKey`; neither path silently adopts the other's key.
- **A provider alias is a routing rule, not a label** (ADR-088 D3, HOLODEX-306): after enriching a person from a provider, that provider-supplied name must **both** find them in search **and** route a file tagged with it to them on the next scan. Either half passing alone is the pre-collapse bug wearing a new table — the entire point of the collapse is that an alias has one meaning regardless of who authored it, so the assertion is always the pair, never one side.
- **A deleted alias stays deleted** (ADR-088 D4): re-enrichment never resurrects a name the owner removed. The suppression is scoped per-entity precisely so it can do that without holding the globally-unique `alias_key` hostage — another entity must remain free to claim the same name.
- **Enrichment never fails, and never merges, on a name conflict** (ADR-088 D5): a provider name already held by another entity degrades to a skip plus an `identity_review_queue` row. *Enrich completing* is as load-bearing as the skip and must be asserted with it — one awkward AKA must never cost an entity its bio, birthdate, and photo.
- **Activity leaks no secrets**: `/admin/activity` and `/admin/activity/history` never serialize a filesystem path, env value, or token (incl. `job_runs.error_message`).
- **Gate is real and loud**: an owner-only route is unreachable without the token when `ADMIN_TOKEN` is set; when unset on a non-loopback bind, the server warns and flags it (never a silent open control surface).
- **Related never includes self**: `GET /media/{id}/related` must never return the current item in `person.items` or `tag.items` — the exclusion holds even when the item is the *only* sibling (→ empty `items:[]`, not itself). Selection (which person/tag is keyed) is fully deterministic; only the item *draw* is random, so tests pin the chosen key + membership and tolerate any order.
- **Enrichment never overwrites the file**: file-extracted first-class fields (title/people/tags/date) survive a re-scan even after a provider enriched the same canonical field, unless the owner explicitly ordered the provider ahead of `file:` (ADR-033 F22.3c). Shadow data is additive.
- **No outbound call without intent**: no provider HTTP request is issued except as a direct result of an owner-initiated enrich/resolve (no scheduler, no enrich-on-scan). Asserted by a fake provider whose call-count stays zero across a scan pass.
- **Soft-delete is orthogonal to disk presence** (F24/ADR-037): a soft-deleted item stays hidden across a re-scan of its still-present file — `deleted_at` is never cleared by the scanner, and the #26 reactivation fast-path can never resurrect it. The delete is undone only by an explicit `Restore`.
- **Disk and DB never desync on purge** (F24.8): a failed file unlink must leave the row soft-deleted (retried next sweep) — never a deleted row with a surviving file, nor a removed file with a live row. A *missing* file is the desired end state and finishes the row delete.
- **Enrichment can't be an SSRF vector**: core only ever connects to an allowlisted provider `base_url`; a `/resolve`/`/enrich` response cannot redirect it to another host, and no upstream API key is ever serialized into config dumps, logs, or the read-model.
- **A promoted key renders exactly once** (F44/ADR-062 FR3): promoting an auto-registered field materializes it as a `mapping.Field`, so `AutoRegisterFields` excludes it automatically — the field is either a display-only auto row **or** a curatable mapped row, never both. Break this and every promoted field doubles.
- **Promotion presentation is global; value curation is per-entity** (F44/ADR-062 D1): one `field_promotions` row shapes label/render/group/order for *every* entity of the type, while `field_source_decisions`/`metadata_curation` stay keyed by `(entity_id, field_key)` — a value pinned/added/suppressed on person A must never leak to person B, and a relabel on the shared row must apply to both.
- **De-/re-promote never loses curation** (F44/ADR-062 D-reversible): decisions/curation rows are keyed by `field_key`, independent of the promotion row, so `ClearPromotion` reverts the field to a display-only auto row **without touching** the shadow value or any prior decision/curation — which then re-apply verbatim on re-promotion. The delete is presentation-only.
- **Promotion outranks operator YAML but never the schema contract** (F44/ADR-062 D3): the in-app promotion is tier-0 (above `metadata-mappings.yaml`) **only** for a non-canonical key; a promotion can never target a canonical (`registry.IsKnown`) or `_`-prefixed key (⇒ 422), so tier-2 registry keys keep their contract and a promotion can't shadow `bio` or reach a reserved sidecar key.
- **Promotion is zero-impact when unused** (F44/ADR-062): with no `field_promotions` rows, resolved output and rendering are byte-identical to pre-F44 (the F39 baseline) — the golden no-op, on all three entities.
- **The resolver stays clock-free** (F45/ADR-063 AC-8): `internal/resolver/` reads **no** wall clock — `Derive` takes `now` as a parameter, injected at the `Handlers.now` edge. A grep guard fails the build on any `time.Now` in the package. Break this and ADR-051's determinism (and its test suite) silently rots; time-varying Age becomes wall-clock-flaky instead of a controllable test input.
- **A computed field is never adoptable** (F45/ADR-063 D3): a `computed:` row carries **nil** `Decision`/`Candidates` (so no SPA affordance, nothing to write) **and** the decision endpoint rejects a `Computed` canonical / `computed:` source with 400 — `computed` is deliberately absent from `Valid()`/`ForNamespace()`. Non-adoptability is structural *and* enforced at the API, never by convention alone.
- **Age and age-at-death are mutually exclusive** (F45/ADR-063 D4): the two formulas are one function branching on `deathdate` — a living person shows exactly `age`, a deceased person exactly `age_at_death`, **never both**. A change to either formula must preserve the "exactly one row" property.
- **Missing/unparseable input → no row, for everyone** (F45/spec D3): when a required input (`birthdate`, or `deathdate` for age-at-death) is absent or non-ISO, `computable=false` yields **no row** — no placeholder, no "—", no nudge — and owner and visitor payloads are **identical** (there is no owner-only branch on a computed row).
- **Compute-on-read, never stored** (F45/ADR-063): derived values touch **no** migration, column, or shadow row; Age is time-varying and must be recomputed live (a stored value would be stale the instant after it is written). Asserted by AC-2 — advancing the injected `now` increments Age with zero DB writes.
- **A dismissal is as durable as an acceptance** (F47/ADR-066 D2): once the owner records "None of these match" for `(entity, provider)`, that pair is excluded from queue/needs-review state and blocks `/resolve` from firing again until an explicit "Try again" clears it — no TTL, no background re-check, no code path silently re-prompts the same rejected candidates.
- **Auto-apply never invents a new confidence model** (F47/ADR-066 D1): the auto-apply trigger is the *existing* `>=0.85` "Strong match" cutoff (`EnrichPicker.matchLabel`, unchanged) applied only when **exactly one** candidate clears it — any ambiguity (2+ strong, or only possible/weak) still stops at the owner. `confidence` stays provider-native and non-normalized; no per-field weighting is introduced.
- **Refresh never re-searches** (F47/ADR-066 RD7/RD8): once a provider is linked (a stored `external_id` exists), Refresh/Refresh-all call `apply()` directly — asserted by a fake provider's `/resolve` call-count staying zero across a refresh of an already-linked entity.
- **Extraction auto-apply never bypasses the exact-match gate** (F48/ADR-067 D1): for People/Studio/Movie, a candidate's aggregate score crossing its tier's threshold is necessary but not sufficient — the entity-resolution component must have come from an exact loose-key match (F43), never the Jaro-Winkler advisory tier. A fuzzy match that would otherwise clear the aggregate threshold still routes to review; the suggested candidate it produces is display-only until the owner clicks. Break this and a filename typo ("Al Smith") can silently merge onto an unrelated existing person ("Alice Smith").
- **A manual edit is a one-time-import boundary for extraction** (F48/ADR-067, spec "Manual-edit precedence"): once a field carries a `manual:` source (F30), a later extraction that disagrees always queues for review, never auto-applies over it — extraction treats a prior manual edit as the owner having already made the call, the same precedence F36's decision short-circuit already gives a `manual` decision elsewhere.
- **A revert is byte-for-byte, and revertible itself** (F48.9/ADR-067 §2): reverting a completed batch restores every snapshotted field to its exact pre-write value (asserted against `file_writeback_snapshots.prior_value`, not a re-derived guess) — and the revert is itself a normal, re-snapshotted writeback job, so a bad revert can be undone the same way a bad extraction batch can.
- **Merge propagation never touches the filename** (F48.8e, spec Non-Goals): a Person/Studio merge rewrites only the embedded tag on affected files; the on-disk filename is untouched until the separate, not-yet-built rename-schema feature ([HOLODEX-192](https://whoiskevinrich.atlassian.net/browse/HOLODEX-192)) ships — a merge test asserting a changed *path* would be testing the wrong feature.
- **Rescan never destroys a non-file tag association** (F50/ADR-075 D3): `replaceAssociations()` deletes and reinserts only `video_tags` rows with `source='file'` — a manually-added or provider-materialized tag survives any number of rescans untouched. This is the one bug this whole spec exists to fix; a test asserting the old unconditional-delete behavior is asserting the bug.
- **The tag deny-list is exact-string, case-insensitive, never substring, and unbypassable** (F50/ADR-075 D2): the guard lives inside `resolveOrCreateByName` itself, not at each caller — so no future call site can create a denied tag by forgetting to check. `"Gnome"` blocks `"gnome"`/`"GNOME "` (nameKey fold) but never `"Garden Gnome"`.
- **A hierarchy edit can never create a cycle** (F50/ADR-075 D1): the reparent guard walks the *full* ancestor chain of the proposed parent before committing — a tag can never become its own ancestor, directly or transitively, and the rejection names the offending tag rather than failing silently.
- **Tag materialization is idempotent and alias-canonicalizing** (F50/ADR-075 D4): re-enriching a video any number of times produces the same `video_tags` rows as enriching it once, and a materialized alias term always attaches and writes back under its *canonical* name — an aliased provider value (`"azure"`) must never appear as a second, spelling-variant tag (`"azure"` alongside `"blue"`).
- **Genre writeback's TMDB-raw union side is deny-list-filtered too** (F50 RD9 follow-up): a denied term reaching the file via the *raw* TMDB `genres` union — never materialized as a `tags` row, so invisible to a curated-tags-only filter — is the specific gap the owner caught during spec review; both sides of the union pass through the same deny-list check before assembly.
- **Writeback exclusion never affects search or browse** (ADR-077 D1): `writeback_enabled=false` is a single flat `WHERE` clause scoped to `TagNamesForVideo`'s Genre-field projection — the tag itself, its `video_tags` rows, and every list/detail/search/facet read are completely unaffected; a disabled tag is indistinguishable from an enabled one anywhere except the written file's Genre field. A test asserting a disabled tag disappears from `/tags`, search, or a video's tag chips is testing the wrong feature.
- **A sync is a full recompute, never an append** (ADR-077 D2): `syncTagWriteback` recomputes each affected video's **entire** current `genres` writeback value from its live, currently-enabled tag set — it is not "add this one tag's name to whatever the file already has." A tag disabled *after* a file was last written and then synced must see its name **removed** from the file, not merely fail to have it re-added.
- **A tag and a category can never share a name** (HOLODEX-240/ADR-078 D3): enforced at **two independent layers** — an app-layer pre-flight check (the friendly `409`/error-copy path) *and* four DB triggers on `tags`/`categories` insert and rename (the correctness backstop, catching any insert path — a bulk-import script, a fixed-up test helper — that bypasses the pre-flight check). A test asserting only the pre-flight check, never the trigger, would miss a regression in any future insert path that forgets to call it.
- **Deleting a category never touches its member tags** (HOLODEX-240/ADR-078 D2): `DeleteCategory`'s cascade to `category_tags` is a DB-layer `ON DELETE CASCADE`, not application code — a test that only checks the category is gone, without also asserting every one of its former member tags still resolves by id/name, would miss a future regression that accidentally deletes the tags themselves alongside the junction rows.
- **Tag and category creation are not symmetric, and a test must not assume they are** (HOLODEX-243): tag creation via `POST /tags` resolves-or-creates silently on an exact-name match (no conflict, no error) and is followed by a non-blocking near-miss check; category creation via `POST /categories` hard-409s on an exact-name collision (against either an existing category or an existing tag, ADR-078 D3) and has no near-miss step at all. A single shared "create" handler that funnels both types through identical success/error branching is testing — and would ship — the wrong behavior for one of the two types.
- **A tag created with zero videos must still be listed** (HOLODEX-243): `namedCountQuery`'s `includeZero` left join is what makes `/tags`' "+ New" pill work at all — a regression back to an unconditional inner join (e.g. "simplifying" the shared helper, or a future caller passing the wrong flag) would silently resurrect the HOLODEX-240 bug where a bare-created tag vanishes from `GET /tags`/the merge picker/search until some video happens to be tagged with it. `ListStudios` deliberately keeps the inner join (`includeZero=false`) — a test must not assume the two entity lists behave identically here.
- **The search box is never seeded blank** (F54/ADR-080 D4, spec AC-8a): `sanitizeTitle` returns its **input unchanged** when stripping brackets/commas/resolution tokens would leave an empty or whitespace-only string — a degenerate filename like `[720p]` alone renders the raw, unsanitized title, never `""`. A test asserting an empty seed for any input is testing the wrong behavior; the owner staring at a blank box with no explanation is the exact failure mode this rule exists to prevent.
- **Operator override always wins, and a provider can never force its own query** (F54/ADR-080 D2): a `search_pattern` set in `metadata-sources.yaml` for a provider always outranks that same provider's `/describe`-advertised `preferred_search_pattern` — untrusted provider input can shape the *default* an operator hasn't configured, never override an operator's explicit choice. Precedence is asserted with both set simultaneously, not just tested independently.
- **The sanitizer has no config gate** (F54/ADR-080 D4): unlike the D2 pattern tiers (all operator-configurable), bracket/comma/resolution stripping on the raw-title floor and the `{title}` token applies unconditionally — there is no YAML flag to disable it, and a test must not add one. This is the one behavior change in F54 that ships with no opt-out.
- **`poster_version` and `headshot_version` are independent, never conflated** (F55/HOLODEX-255 RD3/P0-6): the two roles are separately, independently fillable — a person can have one without the other. `PersonPosterCard`'s conditional-border logic must read `poster_version`, and a test asserting the border disappears because a person has *any* image (using `headshot_version` as a stand-in) is asserting the exact bug P0-6 exists to prevent.
- **Not-applicable removes a facet from the pool entirely, in both directions** (F55/HOLODEX-260, ADR-081 D3): marking a facet not-applicable excludes it from **both** the score's weighted denominator and actionability's missing count — it is neither "resolved" (which would inflate the score) nor "missing" (which would depress actionability or count toward a remediation-queue row). A test asserting the score is unchanged after marking a *missing* facet not-applicable is asserting the wrong thing — the score legitimately moves, because the facet leaves the pool rather than converting to a hit.
- **A field with no criticality tag is invisible to completeness, not a zero** (F55/HOLODEX-260 D1 boundary): any field excluded from `registry.FieldDef.Criticality` (e.g. `deathdate`) and every `Computed:true` field (F45/ADR-063 D1) are skipped by `Complete` entirely — absent from `Facets`, never contributing 0 to the weighted sum. A test that adds such a field to a fixture and expects the score to *drop* is asserting a field that structurally cannot be scored.
- **Asset facets are synthesized, never resolved-and-forgotten** (F55/HOLODEX-260): `photo` (person) and `branding_image` (studio) never appear in `ResolveFields`' output — they're delivered as uploaded assets, not mapped fields — so `injectAssetFacet` is the *only* code path that puts them in the completeness breakdown at all. A future change to the asset-upload path that forgets to keep this synthesis in sync would silently drop these facets from the panel and the remediation queue with no test failure elsewhere to catch it.
- **The `completeness` field is owner-only, and cheaply so** (F55/HOLODEX-260): all three detail handlers gate `completeness` on the same owner check `resolved[]` already uses, and compute it from the *same* resolve pass rather than a second DB round-trip — a test asserting presence for an owner must also assert **absence** (not a null/zero-value stand-in) for a visitor, and a regression that reintroduces a duplicate resolve call is a performance bug this row exists to catch structurally (e.g. via a query-count assertion), not just a shape check.
- **A template mismatch degrades the badge, never breaks it** (HOLODEX-266/ADR-083 D2): a stored external id whose namespace/entity-kind has no matching `link_templates` entry — or enrichment disabled entirely — still renders as a label-only identity signal (`url` omitted), never an error response and never a badge with an empty/broken `href`. A malformed stored id (not `namespace:id`) is dropped from the list entirely, not surfaced as a partial entry. `externalLinksForEntity`'s best-effort lookup failure (best-effort by design, mirroring the completeness/resolved fields on the same handlers) logs and serves the rest of the detail page rather than 500ing it.
- **A film's video links are asserted, never derived** (F56/HOLODEX-279 spec P0-2, the epic's flagged single highest-risk gap): `film_videos` is written **only** by the explicit attach/detach endpoints — `RelinkVideoEntity`, the scanner, enrichment, and every decision/curation write path must never touch it, in either `films_enabled` state. Unlike `video_studios`/`video_people`, which `RelinkVideoEntity` derives from resolved field values and prunes on empty, no resolver output can ever reproduce "scene 6 of film Y" — a code path that ever calls `RelinkVideoEntity`-style pruning against `film_videos` would silently detach real user assertions. `TestFilmVideosSurviveFullRelinkCycle` is the regression guard.
- **A suspended film source is dropped, never left dangling** (F56 RD7/ADR-085 §5): when `films_enabled=false`, the film candidate is simply never injected into the resolve pass — a standing decision pointing at it hits the resolver's pre-existing "decided source currently unmatched → fall back to file chip" path, and the `field_source_decisions` row is left untouched. Re-enabling the flag restores the prior resolution automatically, with no re-derivation and no data loss. The alternative bug this guards against: dropping (rather than suspending) the decision on flag-off would re-resolve `album`/`title` to some other source while the file on disk still holds the film-written value, orphaning the write with no explanation on the next scan.
- **`films_enabled=false` is read-suppressing only, never write-destructive** (F56 spec, films_enabled semantics): turning the flag off never reverts Album/Title values already written to files, never prunes a `film_videos` link, and migrations run regardless of flag state — the flag gates read/route/resolver-injection behavior only. `TestFilmsDisabled_RoutesUnregistered` proves the routes vanish entirely (404) rather than merely gating access (403) — a stricter guarantee than most owner-only surfaces in this app, called out because a future refactor toward the standard `requireOwner` pattern would silently weaken it.
- **Film enrichment never touches a video (F59/ADR-089 D1)**: applying provider cast, year, description or images to a film leaves every attached video's `video_people`, `field_source_decisions` and `file_writebacks` unchanged. Films assert membership; they do not rewrite their members.
- **Absence means “nothing to report”, and that depends on delete-on-success** (ADR-091 D3, HOLODEX-323): the page renders no writeback badge when no `writeback_queue` row exists for the video — which conflates succeeded, swept and never-existed *on purpose*, because all three correctly mean nothing. This is the mitigation for the second-consumer hazard ADR-073 D3 flagged in its own consequences. It holds **only while `FinishWriteback` deletes on success**. A future change that retains completed rows would silently turn every finished write into a rendered badge, so the invariant test asserts the deletion directly, not just the empty render.
- **A failure never clears itself** (ADR-091 D4): a failed row persists across reloads, sessions and restarts until the owner retries, dismisses, or writes again (RD5). Anything that expires or auto-sweeps a failure breaks the invariant above — once silence can mean “we gave up quietly”, silence stops meaning success everywhere.
- **A pending write never suppresses `out of sync`** (ADR-091 RD6): until the job lands the file genuinely still differs, and a queued write can sit behind a large `EnqueueMany` batch (merge propagation, tag sync). An intermediate design hid the badge during pending; that was a workaround for two badges that looked identical, fixed in the design by weight rather than by suppression. A test asserting out-of-sync *disappears* while pending is pinning the bug, not the behaviour.
- **A writeback failure message never reaches a non-owner** (ADR-091 / spec R2.1a, security review 2026-09-06): the persisted error is `err.Error()` straight off `writeback.WriteBatch`, and **every** failure path there embeds absolute filesystem paths — `writeback copy: %w` / `writeback rename: %w` wrap `os` errors carrying both paths, and the exiftool/mkvpropedit/ffmpeg branches append raw tool stderr containing the `.holodex-tmp` path. `GET /media/{id}` is **not** owner-gated, and `redactFileMetadataForVisitor` already strips `FilePath`/codecs/container from that payload *precisely because* client-side gating is insufficient (its own doc comment says so). Attaching the raw error to the media payload would reintroduce path disclosure through a new field and defeat that control — the same shape as the `get_video` MCP leak. **The assertion must be made at the API, on a video whose write genuinely failed with a path-bearing error**: a test that only checks the badge does not render passes against a payload that still carries the path.
- **A film cast name renders exactly once (F59/ADR-089 D2)**: the billed-but-absent group is the scene union's *complement*, computed by resolved person identity. A name in both sources appears only in Cast, and an alias never manufactures a phantom absence.
- **A film identity collision changes no identity (F59/ADR-089 D3)**: a provider apply whose year would violate `UNIQUE(name, year)` leaves *both* films' `name`/`year` byte-identical and names the occupant, while the enrichment itself still lands (ADR-033's shadow store is additive and ungated). The year moves completely or not at all — never half-written. The fill is also fill-only: it never rewrites a year the owner asserted.
- **A film's billed cast writes nothing (F59/ADR-089 D1)**: rendering the billed-but-absent group creates no Person row, no `film_people_roles` row, and touches no attached video. It is a read-time complement over the enrichment shadow, so clearing the provider empties it by construction.

---

## 5. Frontend Strategy (SvelteKit, ADR-002)

| Area | Type | Tooling | Cover |
|------|------|---------|-------|
| Components (VideoCard, FilterBar, ResolutionBadge, RawTagPanel) | Component | Vitest + @testing-library/svelte | Props/state, empty/loading/error states |
| Filter→URL sync (F4.7) | Interaction | Vitest | Applying filters updates URL; loading URL restores state |
| Global search palette (F4.10) | Interaction | Vitest | Debounce; grouped results; keyboard select |
| Virtual scroll grid (F3.1) | Interaction/perf | Vitest + Playwright | 1k items no thrash; 60fps scroll (manual/trace) |
| Dark mode (F8) | Component + visual | Vitest + Playwright | Default dark, no white flash, persist toggle |
| Keyboard nav (F12.5) | Interaction | Playwright | `/` focus, arrows, Enter, Esc — no mouse |
| Accessibility (F8.3) | Automated | Playwright + axe-core | WCAG AA contrast; roles/labels |
| Responsive (F12.6) | Visual | Playwright | 375px/768px reflow |
| **Search-history store** (`searchHistory.ts`, QW1) | Unit | Vitest | Record-on-submit; **case-insensitive dedupe → move-to-top**; **cap 10** eviction; clear / single-remove; **malformed-JSON → reads empty** (defensive, never throws); whitespace-only not recorded |
| **Search-history dropdown** (QW1) | Component/Interaction | Vitest | Opens on focus of an **empty** box when history non-empty; **hides the instant the user types** (`!searchTerm.trim()` gate — no filter-as-you-type); reopens after clearing + re-focus; ↓/↑ highlight, Enter runs, Esc closes; **click runs query before blur closes** (`onmousedown`); **no network call**; 3-skin: square corners (Broadcast/Brutalist), **no `▮` caret on rows** |
| **Atmosphere overlay on playback** (overlay bugfix) | Component/Interaction | Vitest + Playwright | `onplay` toggles `body.is-playing` → `.app-atmosphere::after` hidden; `onpause`/`onended`/unmount restore it; **all 3 skins** (Broadcast scanlines+vignette fully cleared during play) |
| **RelatedShelf** (QW3) | Component | Vitest | Shelf **omitted** when block null or `items` empty; renders heading link (`/people|tags/{id}`) + **≤5 VideoCards**; cards wrapped in `.video-grid` so **Brutalist counter restarts per shelf**; loading/error never blocks primary detail content |
| **Related shelves — stable per view** (QW2/QW3) | Interaction | Vitest | The media page's `$effect` tracks **only `id`** — an incidental re-render (skin switch, thumbnail regenerate) **does not refetch** `/related` (shelves don't reshuffle); changing `id` triggers exactly one fresh fetch |
| **Browse-state preservation** (`browse.svelte.ts`, QW4) | Unit | Vitest | **Signature match → reuse** cached set (no refetch); **mismatch → fetch page 0**; **invalidate** on filter/sort change; scroll capture/restore round-trip; single-entry replace (no accumulation) |
| Enrich picker (F22.5b) | Interaction + a11y | Vitest + Playwright | Owner-only render; debounced search → candidates; **combobox/listbox keyboard nav** (↑/↓/Enter/Esc, `aria-activedescendant`); dialog focus-trap + focus-return; confirm → fields populate; empty/error states |
| Provenance badges (F22.7) | Component + visual | Vitest + Playwright | file=muted pill vs provider=outlined-accent pill; `aria-label` long-form; **legible in all 3 skins**, never uses `--warn`; confidence label is text not color-only |
| Person aliases panel (F23.6) | Component/Interaction + visual | Vitest + Playwright | Chips render from `person.aliases`; **owner-only** add field + per-chip ✕ (absent from DOM for non-owner); add clears input + keeps focus (multi-add); **optimistic delete** restores chip on failure; inline `text-warn` for invalid/over-long (words, not color-only); panel hidden when no aliases & not owner; **all 3 skins** (muted pill, `rounded-full`, ✕ accent-on-hover never `--warn`); CJK alias no tofu |
| Delete / Trash (F24) | Component/Interaction + visual | Vitest + Playwright | **owner-only** Manage block + header Trash link (absent from DOM for non-owner); `ConfirmDialog` a11y — `role=dialog`/`aria-modal`, **focus starts on Cancel**, trap, Esc/backdrop cancel, focus-return; soft-confirm copy reflects the grace days; **purge confirm names the file path**; Trash rows show "deleted X ago · purges in Y", Restore=accent (no confirm) vs Delete-permanently=warn (confirmed); empty/loading/error states themed; **all 3 skins** — `--warn` destructive vs `--accent` restore stays distinct (load-bearing on Brutalist), solid-warn CTA uses readable `--warn-ink` |
| **Owner hub + nav split** (F35) | Component/Interaction + visual | Vitest + Playwright | Content nav = **Media/People/Tags only** (Keys/Status/Trash links gone); **owner gear** absent from DOM for non-owner / Preview-OFF; gear active on `/owner` via `text-accent` + `aria-current` (never `bg-accent`); `/owner` shell renders `skin-title` "Owner" + tab row; **active tab `bg-surface-2 text-ink` + `aria-current`** (skin-picker idiom, NOT accent — that stays the single primary action); tab switch is in-group nav (no full reload); F29 toggle **relabeled "Owner view"** — no "Admin" string in `+layout.svelte`; gear label hides `<sm` (icon kept, `aria-label`); focus order `Activity → Owner-view → gear → skins`; **all 3 skins** (tokens-only, `rg` guard empty) |
| **Nationality flag** (HOLODEX-139) | Unit + Component + visual | Vitest + Playwright | `nationality.ts` value→country: **last comma segment** is the country (`"…, United Kingdom"` → gb), single-token **country name or demonym** (`"British"` → gb), synonym/diacritic folding (`USA`→us, `Türkiye`→tr), **dedupe by code**, empty/unknown → **null**; `NationalityFlags.svelte` renders primary flag + muted **`+N`** (aria-hidden; `alt`/`title` list all), **nothing when none resolve** (no flag, no gap); flag chrome is tokens-only (`rounded-theme`/`border-rule`); flags **self-hosted** (flag-icons, offline, not inlined into the page chunk); **all 3 skins** (square Broadcast/Brutalist, hairline reads on surface); **not owner-gated** (visitors see it) |
| **Computed field row + provenance** (F45, ADR-063) | Unit + Component + visual | Vitest + Playwright | `providerFromWinningSource("computed:age") === ""` (the handoff §3 gotcha — **no phantom "C" provider bubble**); `calculatedFrom(["Born"]) === "calculated from Born"` (serial-comma join); the person page compact loop renders a computed row **read-only** with the "calculated from Born" phrase as the value's `title` + `aria-label` and **no** icon/badge — **no** `SourceSelect`, **no** promote pill, **no** Custom entry — **identically for owner and visitor**; bare-integer value under **Born** in the primary `<dl>` (not the Additional-details block); tab **skips** the row (nothing focusable) while a screen reader reads the "calculated from Born" note; tokens-only (`rg` guard empty) — no skin-dependent styling (plain `text-ink` value) |
| **Enrichment queue tab** (F47, ADR-066) | Component/Interaction + visual | Vitest + Playwright | `owner/enrichment/+page.svelte` grouped People → Studios → Media, actionable rows (`needs_review`/`unreviewed`) sort above `auto_applied`/`not_matched`; `EnrichQueueRow` mirrors `DuplicatePairRow`'s rhythm; `ProviderStatusChip` is a non-interactive, labelled status (state readable as text, never color-only); **zero** `/resolve` network calls on tab load; owner-only (absent from DOM for non-owner); **all 3 skins** (token-guard clean) |
| **`EnrichPicker` — dismissal + view-source** (F47, ADR-066) | Component/Interaction + a11y | Vitest + Playwright | "None of these match" fires `dismiss`, closes the picker, emits `ondismissed` (caller flips to `not_matched` without a refetch); hidden once `candidates.length === 0`; a candidate's `profile_url` (scheme-valid) renders "view source ↗" with `stopPropagation` (opens new tab, **never** triggers apply/closes the picker); an accessible name beyond the "↗" glyph |
| **`EnrichProviderChips` — Refresh/Re-match/Clear** (F47, ADR-066) | Component/Interaction + visual | Vitest + Playwright | Unlinked provider: primary stays "Enrich" (opens picker), no overflow menu; linked provider: primary flips to **"Refresh"** (direct `apply()`, no `/resolve` network call, no picker), ⋯ overflow gains "Re-match…" (opens picker) + existing "Clear"; "Refresh all" fans out per configured provider, shows busy label while in flight, and an ambiguous partial result surfaces inline on that provider's own chip — **never** silently dropped; **needs-review/not-matched states read `text-ink`/`text-muted`, never `text-warn`** (only an actual request failure does) — re-verified on **Brutalist** per the F43 regression risk |
| **Extraction tab + `ExtractionQueueRow`** (F48.6, ADR-067) | Component/Interaction + visual | Vitest + Playwright | `owner/extraction/+page.svelte` groups pending rows **by video** (not by entity type, a deliberate divergence from Enrichment), sorted most-fields-pending-first (ties by filename); within a group, fields render People→Studio→Title→Release Date→other; **zero** network calls beyond the list fetch on load (mirrors Duplicates/Enrichment's zero-cost pattern); non-entity row actions (Accept filename/Edit…/Accept tag/Dismiss) render only when their underlying data exists; an empty-side value renders `— (empty)` in `text-muted italic`, never blank space; **entity fields render one chip per parsed name** (HOLODEX-196 #1/#5, ADR-068 D2) marked *exists*/*new* — clicking a chip opens `EntityPickerDialog` seeded on that name and swaps it (existing or new) without disturbing the others, `×` removes one, and **"Accept cast"** stages the whole edited list (verified: editing one of three chips still writes all three; a single studio chip is the mistyped-studio fix); **"Extract all"** shows a running notice + bounded auto-refresh and a manual **Refresh** reloads in place without the full-screen loading state (HOLODEX-196 #2); confidence shown as a tier label (Strong/Weak/Conflict), never a raw percentage; owner-only (absent from DOM for non-owner); **all 3 skins** (`rg` token guard clean, chip *exists*/*new* colors resolve in Cinémathèque/Broadcast/Brutalist) |
| **Preview-before-write dialog** (F48.7, `WritebackFormDialog` diff mode) | Component/Interaction + visual | Vitest + Playwright | Row body renders the old value struck through (`decoration-warn` on the strike, **not** `text-warn` on the text itself) → arrow → new value `text-accent font-medium`; the existing per-row checkbox is retained, an unchecked row is skipped at write time and the submit button stays disabled at `checkedCount===0` (existing `WritebackFormDialog` guard, unchanged); a contextual "skip preview next time" checkbox surfaces only for auto-applied batches (F48.7b), unchecked by default, and never appears for a manually-resolved batch (F48.7a) since the owner explicitly asked to review those |
| **Revert control (System Activity)** (F48.9d) | Component/Interaction + visual | Vitest + Playwright | Button renders only on an activity row carrying a `batch_id`; click → busy "Reverting…" (`aria-live="polite"`) → success shows an inline "Reverted" status line under the original entry (not a new row) → failure surfaces `text-warn` inline, same convention as every other activity-row failure state; a reverted batch's own Revert control disappears, but the new revert job's own activity row gets its own Revert button (F48.9c — no special-cased UI, it's just another `batch_id`-bearing row) |
| **Media-page tag chips** (F50 P0-8, [design handoff](design/tag-governance-and-video-enrichment-handoff.md) §1) | Component/Interaction + visual | Vitest + Playwright | Owner-only remove `×` revealed via the existing `.curation-actions` hover/focus-within class (absent from DOM entirely for a visitor — same chips as today); `·provenance` suffix renders for `file`/`provider:<name>` sources and is **suppressed** for `manual`; add-input reuses `/tags`' own inline-editor classes; a denied-term submission shows an inline `text-warn` rejection (not a silent no-op); a near-miss submission renders the **same** near-miss card `/tags` already has (component reuse, not a second implementation) |
| **Deny-list tab** (F50 P1-1, handoff §2) | Component/Interaction + visual | Vitest + Playwright | New `/owner/tags` ("Deny-list") tab appears in the Owner tab row only for an owner; add/remove rows round-trip against the list; empty-state copy matches `/tags`'/Duplicates' existing empty-state pattern; a non-owner hitting the route directly redirects home (same gate every other `/owner/*` page has) |
| **Hierarchy pill-menu action** (F50 P1-2/P1-3, handoff §3) | Component/Interaction + visual | Vitest + Playwright | `/tags`' existing pill ⋯ menu gains **Set parent…** (relabels to **Change parent…** + shows **Parent: {name}** + **Clear** once one is set); a cycle-rejecting server response surfaces through the menu's existing `actionError` slot (no new error UI); `/tags/{id}`'s optional ancestor breadcrumb (via `EntityVideos`' `hero` snippet) renders only when ancestors exist, absent for a root tag |
| **Writeback exclusion — Details card + bulk bar** (ADR-077, [handoff](design/tag-writeback-exclusion-handoff.md)) | Unit (shipped) + Component/Interaction + visual (target) | Vitest + Playwright | `writebackJob.ts`'s shared `pollUntilSettled` loop and `waitForWritebackBatch` are unit-tested today (`writebackJob.test.ts`, 6 cases: settles, resolves without throwing on `failed>0`, survives a transient fetch failure, rethrows a real `ApiError` status, gives up to last-known counts on timeout, stops on cancellation) — this is real, already-green automated coverage, not a target row. The component layer (`tags/{id}` Details card toggle + sync trigger, `/tags` Manage-mode bulk bar's 3 actions, `WritebackBatchDialog`'s phase transitions) has **no** Vitest/Playwright coverage yet, consistent with §0's standing frontend-automation gap — verified instead via [`tag-writeback-exclusion-qa-checklist.md`](design/tag-writeback-exclusion-qa-checklist.md) and a live end-to-end pass against `backend-amv` (toggle, single-tag sync incl. zero-value skip, bulk toggle on mixed prior state, bulk sync incl. zero-enqueued), all 3 skins |
| **`PickerShell`** (HOLODEX-240 follow-up) | Component + a11y | Vitest + Playwright | Shared dialog chrome extracted out of `EntityPicker`/`CategoryPicker`'s 100%-duplicated modal (§ split below) — `trapTab` wraps Tab/Shift+Tab at both DOM boundaries; trigger focus is captured on mount and restored on unmount (**known pre-existing gap, not a regression**: driven-browser QA this session found focus lands on `<body>` rather than the trigger on both the pre- and post-extraction code — the trigger element is already gone from the DOM by the time `onMount`'s cleanup reads `document.activeElement`, byte-identical before/after; tracked in §11, not silently fixed as a drive-by); Escape and backdrop-click both call `onclose`; the `merge-rise` animation scopes correctly (`getComputedStyle(dialog).animationName` resolves) in **all 3 skins** with 15–18:1 heading contrast measured this session |
| **`EntityPicker` / `CategoryPicker`** (F43 + HOLODEX-240, sharing `PickerShell`) | Component/Interaction + a11y | Vitest + Playwright | Manually driven-browser QA'd this session post-extraction (Cinémathèque/Broadcast/Brutalist): `EntityPicker`'s merge flow (step 1 search → step 2 informed confirm → Back returns to step 1, title unchanged across steps) and `CategoryPicker`'s `mode="add"` (roving-tabindex reaches the trailing `+ Create "{query}"` row, `optionCount` spans results-plus-create) render and function correctly through the shared shell; **not yet QA'd this session**: `CategoryPicker`'s `mode="remove"` (no create row, the Manage-bar's "None of the selected tags belong to a category yet." zero-state hint), and the tag pill ⋯ menu's single-tag "Add to category…" entry point |
| **`/tags` unified type filter + search** (HOLODEX-240 §1) | Component/Interaction | Vitest + Playwright | All/Tags/Categories segmented control (`SortToggle`'s shell reused, plain buttons no `radiogroup`) narrows the grid; the new search input (`aria-label="Search tags and categories"`) filters both types client-side over the unpaged lists, `aria-live="polite"` results-count line matches `EntityPicker`'s status-line copy pattern; **not yet driven-browser QA'd** (HOLODEX-240.md "Up next" #1) |
| **Category pill — plain + Manage-mode asymmetry** (HOLODEX-240 §2) | Component/Interaction + visual | Vitest + Playwright | Plain pill: accent border + `aria-hidden` tag-glyph icon + `tagCount()` badge (never a video count), visually distinguishable from a tag pill without relying on filter state; **Manage-mode asymmetry is the one place a reader could mistake it for a bug** — a category pill's body still **navigates** to `/categories/{id}` while `manage` is on (unlike a tag pill, which toggles selection), it is **never selectable**, and its ⋯ menu carries only **Rename**/**Delete** (no Merge/alias/parent); Delete opens `ConfirmDialog` naming the affected tag count, consistent with the trash "Delete permanently?" copy shape; **not yet driven-browser QA'd** |
| **`/categories/{id}` detail page** (HOLODEX-240 §3, new route) | Component/Interaction + visual | Vitest + Playwright | Deliberately sparse — no `EntityVideos`, no ancestor breadcrumb, no video-count hero line (all confirmed non-goals); member-tag chips mirror the media page's Tags-section idiom (owner-only add/remove, same near-miss nudge card); non-owner sees read-only chips, no rename button; rename reuses the tag ⋯-menu's exact inline `<form>`; **not yet driven-browser QA'd**, and the shared `TagChipList` extraction the handoff flagged (§3, "worthwhile... not a blocking requirement") is still open — today it's bespoke copy on both pages |
| **Browse-page "Categories" facet** (HOLODEX-240 §5) | Interaction | Vitest + Playwright | A fourth `FacetFilter`, zero component changes — `Option.video_count` is optional and already suppressed when falsy, so `categoryOptions` simply omits the field (categories have no aggregate count); selecting a category ORs in its member tag ids server-side, combining with other facets (Tags, Studios) the same way two Tags selections already do; **not yet driven-browser QA'd** |
| **Detail-page poster binding swap** (F53, HOLODEX-253, planned) | Component/Interaction | Vitest + Playwright | `media/[id]/+page.svelte`'s `<video poster>` binds to `video.poster_url` instead of `video.thumbnail_url` (single-line change, no new component/state); mtime-based `?v=` cache-busting inherited from the existing thumbnail-reload mechanism with no new plumbing; `VideoCard.svelte`'s list binding is asserted unchanged (regression guard for this feature's own Goal #2) |
| **Enrich-picker seed value swap** (F54, ADR-080, HOLODEX-254, [design handoff](design/configurable-provider-search-patterns-handoff.md), planned) | Component/Interaction | Vitest + Playwright | **Zero `EnrichPicker.svelte` diff** — confirmed at the file level in the design handoff, not just behaviorally; the only change is `media/[id]/+page.svelte`'s `entityName` prop *value* (`enrich_queries?.[provider.name] ?? sanitizedFallback` instead of the raw `video.title`), a data change with no new markup/token/CSS. `person/[id]` and `studios/[id]` keep `entityName={person.name}`/`entityName={studio?.name}` **unchanged** (F54 is video-only, spec Non-Goals) — asserted as an explicit regression guard, not left implicit. The picker's existing auto-search-on-open and single-strong-match auto-apply (F22.5b/RD1) fire against the new seeded value unchanged; a pattern- or sanitizer-seeded query is *more* likely to land the auto-apply path, the intended effect rather than a new code path. Optional P1 transparency caption (if built): renders only when the seed differs from the raw resolved title, copy `"Pre-filled from {search pattern\|filename cleanup}."`, hides on first `oninput` |
| **`/tags` create pill + inline form** (HOLODEX-243, [design handoff](design/tag-category-create-affordance-handoff.md)) | Component/Interaction + visual | Vitest + Playwright | Dashed `+ New` pill renders **first** in the grid, owner-only, survives the type filter and an active search query (it's a control, never a filtered result) and — the regression the handoff calls out by name — stays visible on the **zero-tags-zero-categories empty state**, which requires hoisting it above the existing `{#if loading}…{:else if empty}…{:else}` branch (today's empty branch skips the whole grid `<div>`); expanding renders the Tag/Category toggle (**always resets to Tag**, never sticky) + name input + accent submit + ghost cancel + `text-warn` error slot, reusing the exact rename/alias form shape; submit **branches by type** — tag creation calls `resolveOrCreateTag` (silent resolve-or-create; an exact-name match is a no-op success, not a conflict) then runs the existing post-success near-miss check (`api.nearMiss('tag', …)`, same card as the media page's `+ Add tag` flow), while category creation calls `createCategory` and surfaces `submitCatRename`'s verbatim 409 collision copy with **no** near-miss step; `createBusy` guards a rapid double-submit; cancel collapses to the dashed resting state without a request; **implemented and manually driven-browser QA'd this session** across all 3 skins (dashed-pill contrast, popover positioning, collision error, near-miss "Merge them in"/"Keep both", empty-search survival, owner-gating, Escape-closes-and-restores-focus) — **no automated Vitest/Playwright coverage yet**, tracked in §11 |
| **Tag detail hierarchy & categories card** (HOLODEX-259, [spec](specs/tag-detail-hierarchy-and-categories.md), [design handoff](design/tag-detail-hierarchy-reparent-confirm-handoff.md)) | Component/Interaction + visual | Vitest + Playwright | New "Hierarchy & categories" section on `tags/[id]/+page.svelte`: **Parent** (chip + existing-tags-only typeahead, `× Clear`, cycle-guard copy shared verbatim with `/tags`' Manage-mode control via the new `web/src/lib/tagHierarchy.ts` `findTagByName`/`cycleMessage`, extracted out of and now reused by both surfaces); **Children** (chip list + resolve-or-create "+ Add child" that **attaches immediately** for a brand-new name or a childless root, but interrupts with the design handoff's `ConfirmDialog` (`variant="destructive"`) when the resolved name already has a parent and/or children of its own — decision tree asserted: new/childless-root → no dialog; has-parent-only / has-children-only / has-both → each renders its own copy variant; Cancel **preserves the typed input** and returns focus to the add-child input, never clears it); **Categories** (chip list + `CategoryPicker` `mode="add"` single-tag). Read states (shared `entityChip`/`plainLink` snippets) render for every visitor; only the `+ Add`/`×` affordances are owner-gated — absent from the DOM, not merely disabled. **Manually driven-browser QA'd this session** across all 3 skins (Cinémathèque/Broadcast/Brutalist; via `javascript_tool` DOM manipulation, since `computer`'s click-by-coordinate and `screenshot`/`zoom` were unreliable in this environment): set/clear parent, immediate-attach child, all three reparent-confirm copy variants incl. cancel-preserves-input-text and refocus-to-input, remove child, add/remove category, and owner-view toggle hiding every mutation affordance while read states stay visible. **No automated Vitest/Playwright coverage yet**, consistent with §0's standing frontend-automation gap — tracked in §11 |
| **Unified nav search — `SearchResultsPanel` + `navSearch.svelte.ts`** (HOLODEX-249, [spec](specs/nav-search-live-filter.md), [handoff](design/nav-search-live-filter-handoff.md)) — **pre-implementation, target coverage** | Unit + Component/Interaction + a11y | Vitest + Playwright | `navSearch.svelte.ts`: NS2's scope-matching function is **pure and table-driven** — every (current-page-scope, selected-tab) pair maps to `in-place` or `overlay` with no DOM, so it gets a plain Vitest table test independent of any component; default-tab-on-load matches the page's declared scope. `SearchResultsPanel.svelte`: debounced query (fake timers, assert exactly one fetch per settled keystroke burst); **3-rows-then-"View all N"** grouping on the All tab vs. the flat 8-row cap on a single-scope tab (handoff Part B); empty-groups omitted, zero-match state, skeleton-then-real-rows with no layout shift; roving-tabindex over result rows mirrors `EnrichPicker.svelte`'s existing pattern (Arrow Up/Down, Enter activates, Escape closes **without** touching `searchTerm` or in-place filter state — NS5's explicit non-goal); tablist uses real `role="tablist"`/`role="tab"`/`aria-selected` (not the header's `role="group"` toggle-button idiom) with its own roving tabindex, Arrow-Left/Right cycling, automatic activation. **Regression guard (NS4)**: the Media (`/`) and Tags (`/tags`) pages each render **exactly one** text-search affordance in the DOM post-change — assert the old inline `#q` / `query` inputs are gone, not just that the nav box still works. **Per-page mechanics (NS3)**: Media's filter stays URL-synced server-side (`filtersToParams`/`paramsToFilters` round-trip unchanged); People/Studios/Tags filter client-side via the shared `filterByName` utility. **Detail-page video lists (NS6, promoted P1→P0 per the handoff) — implemented**: `pageScopeFor` maps `people/[id]`/`studios/[id]`/`tags/[id]` to the Videos scope (table-tested in `navSearch.test.ts`, incl. the negative case — `categories/[id]` has no video list and stays scopeless). The filtering itself lives once in the shared `EntityVideos.svelte` (not copy-pasted into the three `+page.svelte` callers, which keep passing their raw `videos`/`empty` unchanged) — it filters via `filterByTitle` (`format.ts`'s title-keyed twin of `filterByName`, unit-tested in `format.test.ts`) with **zero** network calls beyond the page's original fetch, and swaps the empty state to a query-aware `No videos match "…".` message. Verified live against `backend-amv` (person + tag detail pages, incl. tab-mismatch revert and clear-to-restore); no live studio fixture in that dataset, so the studio page is covered by the same `pageScopeFor` table test plus type-check, not a live click-through. **`/search` page reuse (handoff Part E)**: same `SearchResultsPanel` renders as the page body, unwrapped from `absolute`/`fixed` positioning, no "View all" cap, no dismiss-on-outside-click. **All 3 skins**: group-header `text-muted uppercase` treatment and skeleton bars verified against Broadcast's blue surface and Brutalist's near-black/lime combination; mobile `<640px` fixed full-sheet layout (no horizontal scroll, 5 tabs fit 375px) is the primary regression target per the spec's stated mobile pain point. **Status**: spec + design-handoff done (this row records target coverage before code exists — see §11 if implementation stalls) |
| **Poster View for People** — `PersonViewToggle`/`viewPreference.svelte.ts`, `PersonPosterCard`, `PersonPosterGrid` (F55, HOLODEX-255, [spec](specs/people-poster-view.md), [design handoff](design/people-poster-view-handoff.md)) — **pre-implementation, target coverage** | Unit + Component/Interaction + a11y | Vitest + Playwright | `viewPreference.svelte.ts`'s `readView`/`writeView` are **pure and table-driven**, mirroring the shape `sortPreference.svelte.ts`'s `readSort`/`writeSort` would need if it had its own test today (it doesn't — no `sortPreference.test.ts` exists yet, per §0's standing frontend-automation gap, so this is new coverage, not a mirrored regression guard): missing/unset key → `'list'`; a malformed/unknown stored value (`"grid"`, `""`, corrupted JSON) → `'list'`, never throws; a valid `'poster'` round-trips through write→read. `PersonPosterCard`: conditional border is driven by `person.poster_version`, **never** `person.headshot_version` (the P0-6 regression — assert with a fixture person carrying a headshot but no poster: card must still render **with** the placeholder border); `:focus-visible` ring renders on Tab focus but **not** on a mouse click (regular `:focus`) — the two must be visibly distinguished in the DOM/computed-style assertion, not just "some ring appears." `PersonPosterGrid`'s density formula (`min(mediaDensity.value * 2, viewportTierCap.value * 2)`) asserted **in lockstep** with `density.svelte.ts`'s real `TIERS` table at every breakpoint (1536/1280/1024/480px), not a hand-copied literal table that could silently drift from RD8's stated 2:1 ratio if `TIERS` changes. **Keyboard-focus adversarial check (RD4) — genuinely new coverage, no prior portrait-frame consumer has this affordance to mirror**: a full Tab pass through a populated grid reaches **every** `PersonPosterCard` (assert `document.activeElement` visits each card's `<a>` in DOM order, none skipped) and each shows a computed `outline` matching `--accent` — run in **all three skins** (Cinémathèque/Broadcast/Brutalist), since this is exactly the kind of regression that "looks right in the default skin" masks. Merge-button auto-switch-to-List-and-select-mode (handoff's Q1 resolution) is a one-line `onclick` — covered by a single interaction test, not a flow. List-view RD7 padding fix: a computed-style regression guard (`padding-top`/`padding-left`/`padding-bottom` all resolve to `0`) rather than a visual diff |
| **Completeness browse controls, remediation queue, breakdown panel** (F55/HOLODEX-260, [spec](specs/entity-completeness-score.md), [design handoff](design/entity-completeness-handoff.md)) | Component/Interaction + visual | Vitest + Playwright | Browse: a "Completeness" sort option and a "Missing {facet}" `FacetFilter` chip (media/people/studios) reusing `FacetFilter`'s generics widening — both owner-only, absent from the DOM for a visitor. `owner/completeness/+page.svelte` remediation queue: `CompletenessQueueRow` renders a candidate-ready row with a `ProvenanceBadge` + Apply button vs. a needs-research row with Search (and, for image facets, Upload) links — **individual apply/search/upload only, no bulk-apply** (HOLODEX-199 precedent, a deliberate v1 boundary a test must not assume away). `CompletenessPanel.svelte` per-entity breakdown card (video/person/studio detail pages, placed immediately after the page's Metadata/Details card per DD4): score bar (`font-display text-2xl` + `bg-surface-2`/`bg-accent`) + actionability line, falling back to "Fully complete" copy when `actionability` is `undefined`; facets grouped Critical/Nice-to-have; a four-state status pill — Curated (accent-outline pill), Provider (`ProvenanceBadge`, reused not reimplemented), Missing (dashed-muted pill), Not applicable (plain muted text) — **never `text-warn`** for any of the four, since none represents an error state; the DD8 not-applicable toggle renders only for `videoId && canonical === 'external_provider_id'` (video-only, single-facet v1 scope per the handoff, not a general per-facet affordance) and reuses the shared `.btn-accent`/`.btn-ghost` button treatments rather than a hand-forked variant. Manually driven-browser QA'd across all 3 skins this epic on every surface, including the not-applicable toggle end to end (score recalculates, pill flips to "Not applicable", `aria-pressed` tracks state, parent refetches via `onchanged`). **No automated Vitest/Playwright coverage yet**, consistent with §0/§11's standing frontend-automation gap for this project's velocity of shipped features |
| **Provider-link badge** — `ProviderLinkBadge.svelte`, `EntityVideoMeta.svelte`, person page hero (HOLODEX-266, ADR-083) | Unit (shipped) + Component/Interaction + visual (target) | Vitest + Playwright | `sortExternalLinks`/`isHttpUrl` (`format.ts`) are unit-tested today (`format.test.ts`): alphabetical-by-label ordering independent of input/backend row order, non-mutating, a degraded (no-`url`) badge sorts the same as a resolved one; `isHttpUrl` accepts `http(s)://` case-insensitively and rejects `javascript:`/`data:`/`mailto:`/non-URL strings — the same scheme gate `ProviderLinkBadge.svelte` applies before ever setting `href` (defense in depth: the backend only ever emits a validated http(s) template, but Svelte doesn't sanitize `href`). The component layer itself — `ProviderLinkBadge`'s linked-`<a>` vs. non-interactive-`<span>` branch, its brand-icon/monogram fallback, and `EntityVideoMeta`'s `·`-separated badge row on the person/studio detail pages — has **no** Vitest/Playwright coverage yet, consistent with §0/§11's standing frontend-automation gap; manually driven-browser QA target: multi-badge row renders one pill per external id in label-sorted order, a degraded badge is non-interactive (no `href`, `aria-label` reads "Known to {label}" not "View … on"), all 3 skins (`rounded-full` pill legible against `--surface`/`--accent` hover) |
| **Two-tier field editing — `SourceBadge.svelte`** (F56, HOLODEX-268, epic HOLODEX-267, [spec](specs/two-tier-field-editing.md), [design handoff](design/two-tier-field-editing-handoff.md)) | Unit + Component/Interaction + a11y | Vitest + Playwright | New sibling to `SourceSelect` (not an extension of it — the design handoff's explicit rationale is that `SourceSelect`'s select-commits-immediately model is structurally what the RD6 bug below exploits), built directly on `CurationChip`'s existing `radio` mode + `ProvenanceBadge` + `f36.ts`'s `sourceChips`/`resolveSelection`. Single-expansion (F56.9) coordinated by a new module-level singleton, `expandedField.svelte.ts`, mirroring `adminMode.svelte.ts`. Rolled out to every Tier-2 replace field across all three entity pages: Person's compact and long-text fields (`people/[id]/+page.svelte`), Studio's compact and long-text fields (`studios/[id]/+page.svelte`), and Video's generic Metadata `dl` replace-field rows (`media/[id]/+page.svelte`) — including its `long_text` branch (HOLODEX-115), which renders `SourceBadge` the same way the row's other display branches already did, rather than a bespoke per-field section. Per-page Tier-1 exclusions kept on `SourceSelect` per the design handoff's Overview line ("excluding Title/People/Studio on Video and name on Person/Studio"): Video's Studio field (own header-adjacent `SourceSelect`, unconverted pending HOLODEX-271's relationship popover) and Person's `onadopt`-intercepted name field; Studio's name has no chip control at all (`AliasPanel`'s `allowRename` owns it). **At-rest parity (F56.1/F56.5/F56.6)**: a Tier-2 field with one candidate source renders the bare value with no badge/click-target, identical DOM for owner and visitor; a field with 2+ candidates renders `ProvenanceBadge` for both, badge is a `<button aria-expanded>` for owner vs. a read-only `<span>` for visitor — byte-identical at rest until hover/click (the discoverability decision resolved to **hover-only reveal**, not persistent, per the design handoff). **Stage-then-confirm (F56.2/F56.3 — the core mechanism)**: clicking the badge expands a `role="radiogroup"` chip row + Confirm/Cancel with **zero** network calls; clicking a chip, or typing+committing the inline Custom draft, changes only local staged state (assert zero `PUT`/`DELETE` decision calls); Cancel/Escape/outside-click collapses with zero calls and the field unchanged; only Confirm calls `decide()`, exactly once. **F56.4 — the RD6 bug fix, the clearest acceptance criterion — verified live**: on `backend-films` (TMDB-enriched `Oscar Isaac`, `nationality`/`birthdate`/`website` fields all RD6-pending — empty record baseline, provider chip winning with `decision.standing: false`), expanding the badge rendered the pending chip already staged; clicking **Confirm alone**, no further chip click, issued exactly one `PUT /api/v1/people/{id}/fields/nationality/decision` (verified via `read_network_requests`) and the field's `decision` flipped to `{"source":"provider:tmdb","standing":true}` on refetch (verified via `curl`). This is the direct regression test for the bug this story exists to fix, confirmed end-to-end, not just at the code level. **Tier-1 unaffected (F56.6/F56.7)**: the Name field keeps its `SourceSelect` radiogroup (`onadopt`-intercepted, scoped down per `curation/CLAUDE.md`) — verified still present and untouched. **Single-expansion (F56.9) — verified live**: expanding Website while Born was expanded-but-unconfirmed collapsed Born and discarded its stage; clicking a theme-switcher button (any click outside the expanded region) also collapses via the shared `dismissable` action. **Escape — verified live**: closes the expanded row and returns focus to the badge button. **All 3 skins — verified live** via `javascript_tool` computed-style contrast checks (screenshots time out in this environment, per §11): Confirm/Cancel text-vs-background contrast ratios were Cinémathèque 9.2:1/6.3:1, Broadcast 12.1:1/4.9:1, Brutalist 17.2:1/5.7:1 — all comfortably above WCAG AA's 4.5:1. **No automated Vitest/Playwright coverage yet**, consistent with §0/§11's standing frontend-automation gap (this codebase has no `@testing-library/svelte`/`jsdom` in its toolchain — manual driven-browser QA is the established substitute for component-level features, see the HOLODEX-259/F55/HOLODEX-266 rows above). Full QA target: [two-tier-field-editing-qa-checklist.md](design/two-tier-field-editing-qa-checklist.md). |
| **Video composite-key collision check** — `NameEditControl` conflict generalization, `CollisionOfferCard.svelte` (F56.3, HOLODEX-270, [spec](specs/video-composite-key-collision.md), [design handoff](design/video-collision-verdict-handoff.md)) | Unit (shipped) + Component/Interaction + visual | Vitest + Playwright | Backend: `internal/repo/video_collision_test.go`'s `TestFindTitleCollision` covers the composite-key match itself (same normalized title/date/people-set/studio-set → collision; a distinct title, a different people set, or a soft-deleted candidate → no collision) and `TestFindTitleCollision_NoPeople` regression-guards a colliding video with zero linked people — `VideoCollision.People` must marshal as JSON `[]`, never `null` (a Go nil-slice default the frontend's unconditional `video.people.length` read cannot survive); `internal/api`'s decision-endpoint test covers the Title path's 409+`override` gate (no-override proposal that collides → 409 with the `VideoCollision` payload; the same proposal resubmitted with `override:true` → commits normally). Frontend: `NameEditControl` was generalized to be generic over the conflict-payload type (shared with HOLODEX-269's `MergeOfferCard` slot) so `CollisionOfferCard` — the video-flavored sibling, "View existing video" `.btn-accent` / "Save anyway, keep both" `.btn-ghost`, no merge verb, no third option per the spec — plugs into the same `verdict` snippet on the Video Title mount; no dedicated Vitest coverage yet for `CollisionOfferCard` itself, consistent with §0/§11's standing frontend-automation gap, but **fully driven-browser QA'd this session** in an isolated backend+frontend instance seeded with two fixture videos sharing a composite key: collision-card render (this pass is what caught the nil-people bug live, before the backend fix), "Save anyway" commits with override and updates the displayed title, "View existing video" navigates away and leaves the baseline `title` column + resolved decision untouched (confirmed via direct API inspection, not just UI appearance); all 3 skins pass WCAG AA (closest margin 4.71:1, Broadcast muted meta-text/ghost-button). Studio (HOLODEX-271) and People (HOLODEX-272) trigger points are explicitly out of scope for this story and untested here — they reuse `FindTitleCollision`'s sibling once those stories wire their own trigger points |
| **Studio relationship-edit popover** — `StudioPicker.svelte`, replacing `SourceSelect`'s Studio radiogroup (F56.4, HOLODEX-271, [spec](specs/studio-relationship-popover.md), [design handoff](design/studio-picker-handoff.md)) | Unit (shipped) + Component/Interaction + visual | Vitest + Playwright | Backend: `internal/repo/video_collision.go`'s `FindStudioCollision` is `FindTitleCollision`'s sibling (same composite-key match, proposed studio name substituted for title), covered by `TestFindStudioCollision`; `internal/api/decisions_collision_test.go`'s `TestDecisionAPI_StudioCollision` exercises the same 409+`override` gate as the Title path on `PUT /media/{id}/fields/studio/decision`, but fires on **any** manual studio assignment (chip pick, search-select, or create), not just a free-text edit — a picker selection changes the composite key exactly the same way a typed rename does; also asserts `resolveOrCreateByName` matches the existing "Acme" studio case-insensitively on override rather than creating a duplicate "ACME" row. Frontend: `StudioPicker` composes `NameEditControl`'s docked-pencil/busy/error/conflict state machine (verdict slot reused, `CollisionOfferCard` renders unmodified — no video-specific fork needed) with `EntityPickerDialog`'s debounced-search-plus-create-fallback body (inlined into `PickerShell`'s frame rather than nested); known-candidate chips (`sourceChips`, today's `SourceSelect` set minus the trailing Custom chip) are suppressed when there's only one candidate, per the spec's "don't show a one-item chip row" rule. `npm run check`: 0 errors. No dedicated Vitest coverage yet, consistent with §0/§11's standing frontend-automation gap; **manually driven-browser QA'd this session** in an isolated backend+frontend instance: pencil renders beside `NameEditControl`'s Title pencil for an owner; single-candidate chip-row suppression confirmed against a real fixture video; debounced search-select commits and closes the popover; create-fallback ("Use "…" as a new studio") commits and closes; all 3 skins pass WCAG AA (Broadcast/Brutalist dialog body 15.6–18.5:1, shared `text-muted` helper line 4.67–5.59:1, same token as every other component using it — not a regression). The 409/`CollisionOfferCard` path itself was **not** re-driven through the browser for Studio specifically — reproducing a live composite-key collision would require a second People-editing surface that doesn't exist yet (HOLODEX-272) — and is instead covered by `TestDecisionAPI_StudioCollision` above plus `CollisionOfferCard` being the exact same, already browser-QA'd component from the HOLODEX-270 row; flagged here rather than silently assumed |
| **People attach/detach + relationship picker** — `PersonPicker.svelte` (F56.5, HOLODEX-272, epic HOLODEX-267, [spec](specs/people-relationship-picker.md), [design handoff](design/people-relationship-picker-handoff.md)) | Unit (shipped) + Component/Interaction + visual | Vitest + Playwright | Backend: attach/detach rides the existing `POST /media/{id}/curation` `action=add`/`suppress` path on `actors`/`director` (no new endpoint), gated by `internal/repo/video_collision.go`'s `FindPeopleCollision` — `TestFindPeopleCollision` (`internal/repo/video_collision_test.go`) covers the composite-key match on a proposed person-name set (normalized/trimmed names, an unrelated video, and a candidate with a divergent people set all correctly report no collision); `internal/api/curation_collision_test.go`'s `TestCurationAPI_PeopleCollision`/`TestCurationAPI_PeopleCollision_Suppress` exercise the same 409+`override` gate for both add and suppress, and `TestCurationAPI_NonPersonFieldSkipsCollisionGate` regression-guards that the gate only fires for `actors`/`director`, not unrelated curation fields. `Person.Role` (`internal/model/model.go`) and the repo query resolving it (`internal/repo/repo.go`) are new plumbing the multi-role picker depends on. Frontend: `PersonPicker` is `StudioPicker`'s multi-select sibling — unlike Studio it has no `verdict` prop of its own (a conflict closes the popover; the page renders the shared `CollisionOfferCard` in the grid's place instead, since the grid's own remove control shares the same attach/detach calls with no picker instance to render into). **Fully driven-browser QA'd this session** against a fresh, unenriched `E:\TestCopy-Film` database (zero pre-existing people, so every case below exercises the create-fallback path, not a pre-seeded match): typing <2 chars shows the "type at least two characters" hint; a genuine no-match renders "No matches for "…"" plus the create-fallback row with an Actor/Director role toggle (Actor default); clicking it attaches immediately (no separate confirm step) and the popover **stays open** (multi-select, matching the design's explicit non-goal of closing on first commit) while the newly-attached chip appears in the picker's own attached-list with its own `×`, and the underlying grid gains a live `PersonPoster` card with its own remove control — confirmed synced without a page reload; re-searching the same name now finds the real person and the role toggle correctly **excludes the already-held role** (only "Director" offered after "Actor" was attached); attaching the second role renders the disabled "Already attached as Actor, Director" no-op row exactly per the design's states table; detaching from the picker's own chip `×` removes just that role and leaves the other/the grid entry intact; detaching from the grid's own remove control (with the picker dialog closed) empties the section back to the bare "Add person" tile with no error. Zero console errors across the full sequence. **All 3 skins** verified via `javascript_tool` computed-style reads (screenshots time out in this environment, per §11): dialog background/border/text and the role-pill pressed/unpressed treatment all resolved to distinct, theme-appropriate token values in Cinémathèque, Broadcast, and Brutalist — no hardcoded colors. **The 409/`CollisionOfferCard` path itself was not re-driven through the browser** — forcing an exact `{Title, Date, Studio}` match in this fresh dataset would need non-trivial fixture seeding — and is instead covered by `TestCurationAPI_PeopleCollision`/`_Suppress` above plus `CollisionOfferCard` being the same already-QA'd component from the HOLODEX-270/HOLODEX-271 rows; flagged here rather than silently assumed, same posture as the Studio row above. **Incidental fix, out of this story's scope**: live QA was initially blocked app-wide by an unrelated pre-existing bug — `MappedFacets.svelte`'s unconditional `facet.values.length` read crashed on the backend's nil-slice-to-`null` marshaling for a zero-value facet (`/api/v1/facets`, same bug class as the HOLODEX-270 `VideoCollision.People` nil-slice fix), breaking SvelteKit hydration on every route; patched with a minimal `facet.values?.length` guard (one-line, frontend-only) to unblock this session's QA — the backend-side root cause (nil slices marshaling as `null` instead of `[]`) is filed as a separate follow-up, not fixed here. No dedicated Vitest coverage yet for `PersonPicker.svelte`, consistent with §0/§11's standing frontend-automation gap. |
| **Films entity frontend** — `/films` list, `/films/{id}` detail, `FilmAttachDialog.svelte`, film-side bulk picker, films rows on person/studio/tag pages (F56, HOLODEX-279, [spec](specs/films-entity.md), [ADR-085](architecture/ADR-085-films-entity.md), [design handoff](design/films-entity-handoff.md)) — **pre-implementation, target coverage; backend already built and Go-test-covered (§4 rows above)** | Unit + Component/Interaction + a11y | Vitest + Playwright | `/films`: poster-forward grid renders a monogram fallback (reusing Studio's `logo-plate-ink` token, no new token) for a film with no poster. `/films/{id}`: full-film file **list** (RD4, not a single slot — asserted with 2+ full-film rows) is a visually and structurally separate region from the scenes list, ordered by scene number then unnumbered-last (muted `—` badge); the per-file writeback control is present **only** on rows in the full-film region, never on a scene row — the direct regression guard for the spec's "writeback iff represents the entire film" rule. **`FilmAttachDialog` (video→film, RD9's small-scale side)**: single-select, poster/name/year result rows over `EntityPickerDialog`'s roving-tabindex chrome (not `aria-activedescendant`, per this project's standing keyboard-list convention); the in-dialog second step (scene-number entry vs. full-film toggle) surfaces the RD6 subtractive-hide consequence inline before commit, never silently; a 409 scene-collision response is handled in place, naming the occupant, no dialog-closing error toast. **Film-side bulk picker (RD9's library-scale side, new component)**: defaults to unattached-videos-only scope; studio/people filter chips pre-populate from the film's own resolved cast/studio; multi-select + "Select all visible"; an already-attached-elsewhere badge renders per candidate (0..N is legal from this direction but worth flagging); the sticky commit footer's starting-scene-number input drives sequential auto-numbering server-side (mirrors `TestBulkAttachFilmVideos`); a mid-batch collision is all-or-nothing (no partial commit), per the handoff. **Films row on person/studio/tag pages (RD6's compensating surface)**: renders only when `films_enabled` is true **and** at least one film matches — never an empty "Films" heading (this project's standing no-dead-end convention, [[feedback-mockups-need-mechanisms]] precedent). **RD7/ADR-085 §5 — no new UI to test**: a video whose film-sourced Album/Title candidate is suspended (flag off) renders through the *existing*, already-covered "decided source unmatched → file chip" `SourceSelect`/`SourceBadge` fallback — this row explicitly does **not** need a bespoke "source unavailable" component test, since none was built (see the design handoff §6b resolution). **All 3 skins**, per the theming rule — poster aspect-ratio (`aspect-[2/3]`) and monogram-fallback contrast are the two genuinely new visual surfaces with no prior portrait-frame consumer to inherit a regression guard from, the same posture F55/RD4's poster-grid keyboard-focus check called out. **Status**: spec + ADR-085 + design handoff done; this row records target coverage ahead of implementation — see §11 if it stalls |
| **`FilmStudioCascadeDialog` + `WritebackBatchDialog` `autostart`** (F57, HOLODEX-285, [spec](specs/film-studio-cascade-writeback.md), [ADR-087](architecture/ADR-087-film-studio-cascade-decide-and-writeback.md), [design handoff](design/film-studio-cascade-writeback-handoff.md)) — **pre-implementation, target coverage** | Component/Interaction + a11y | Vitest + Playwright | Film page Studios row: pencil renders only when `isOwner`, unconditioned on `studios.length` (an owner with zero attached studios still gets the pencil — the regression a naive `{#if studios.length}`-nested gate would introduce); non-owner rendering is byte-identical to today's plain-links markup, asserted as a DOM diff, not just "no pencil visible." `FilmStudioCascadeDialog` step 1 (picker): subhead states the exact attached-video count and the unconditional-overwrite sentence (RD4 — the one place this behavior is disclosed, no confirm step later); candidate chips source from `FilmStudios(filmID)` **not** `sourceChips(field,'file')` (Media's source) — assert the two picker instances genuinely read different data, not just different copy; the zero-attached-videos edge case shows the "no attached videos yet" note and disables commit **client-side** with **zero** network calls (avoids round-tripping to an endpoint that would return an empty no-op). Step 2 (results, same dialog, in place — no second modal mount): status line fires once (`aria-live="polite"`) with the omit-zero-clause summary sentence; Collision/Error `<details>` groups **expanded** by default when non-empty, Enqueued **collapsed**; footer's "View writeback progress →" is **absent from the DOM** (not disabled) when `enqueued===0` — this project's standing no-dead-affordance convention (same posture as the Films entity row's RD6 films-row above). `WritebackBatchDialog`'s new `autostart` prop: with `autostart` the dialog skips `'confirm'`/`'starting'` and calls `trigger()` immediately on mount, landing straight in `'progress'` — asserted by mounting with `autostart` and confirming no `'confirm'`-phase markup ever renders; **every existing caller** (Tag sync, the already-shipped call sites) is asserted **unaffected** by omitting the prop (default `false`), a direct regression guard since this is an additive change to a shared component. Focus moves to the Results step's status line on mount (mirrors `NameEditControl`'s focus-into-verdict pattern). **All 3 skins**: the token guard (`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)'`) and the muted-disabled guard (`rg 'text-muted[^"]*disabled:opacity'`, since the zero-enqueued state must withdraw the button, never disable it) both stay empty, per the design handoff's own stated guards. **Status**: spec + ADR-087 + design handoff done; this row records target coverage ahead of implementation — see §11 if it stalls |
| **Fire-and-forget writeback — dialog close-on-ack + Metadata badges** (ADR-091, HOLODEX-323, [spec](specs/fire-and-forget-writeback.md), [design handoff](design/fire-and-forget-writeback-handoff.md)) — **pre-implementation, target coverage** | Unit + Component/Interaction + a11y | Vitest + Playwright | **`pollUntilSettled` already has real green coverage** (`writebackJob.test.ts`, 6 cases — line 331) and is **reused unchanged** by the new page-level poll; what changes is the *consumer*, so the existing module tests stay valid and the new work is asserting that `WritebackFormDialog` no longer calls `waitForWritebackJob` at all — a call-count-zero guard, since the plausible wrong implementation leaves the poll in place and merely unlocks the close button. Dialog: closes on the `202` (asserted by resolving the POST and confirming unmount **without** a job-status fetch ever firing); a **failed enqueue keeps it open** with the error inline and close affordances unlocked — the one path where fire-and-forget must not apply; no success toast renders. Focus returns to the trigger on the new close path (the existing `onMount` cleanup does this today and must not regress — note the **known pre-existing** focus-restore gap in `EntityPicker`/`CategoryPicker` recorded in §11, which this must not replicate). Gutter: the `isWriting`/`isError` icons are gone, and **no two remaining glyphs share a meaning** — the no-op row renders an equals glyph, not a second check, asserted explicitly because the prior draft used a muted check for “won’t write” and inverted the glyph’s conventional sense. Row order: writable → no-ops → unwritable. Metadata header: the four badge states, with **out-of-sync co-rendering alongside pending and failed** (the RD6 regression guard — assert presence, since the tempting wrong test asserts absence); badges sit beside the write action, not the section label; **zero badges** when clean. Disclosures: overview and poster open on **hover, keyboard focus, and tap**, and dismiss on `Escape` **without** closing the dialog — the keyboard and touch paths are the ones a hover-only implementation silently drops, so they are the assertions that matter, not the hover one. Visitor renders no badges and no Retry/Dismiss, gated on `canWriteback` rather than a blanket owner check ([[feedback-holodex-visitor-owner-field-gating]]). **All 3 skins**: the two filled badge variants and the outline variant are new surfaces; in skin 1 `warn` and `accent` are both warm oranges, so the glyph and copy — not hue — carry the pending/failed distinction, and a contrast pass alone will not catch a regression that drops the glyph. **Status**: spec + ADR-091 + design handoff done; this row records target coverage ahead of implementation — see §11 if it stalls |
| **`StudioLinkCard.svelte`** — icon + linked name + video count, replacing the ad hoc Studio links on Media and Film detail pages (HOLODEX-290, [design handoff](design/studio-link-card-handoff.md)) — **done** | Component/Interaction + visual | Vitest + Playwright | Icon-set path renders `icon_url` via `object-contain` inside a solid `border-rule` frame; no-icon path falls back to `monogram(studio.name)` inside a **dashed** frame, mirroring `EntityImageSlot`'s existing empty-state convention — assert the dashed/solid split is driven purely by `icon_url` presence, not `logo_url`/`poster_url`. Video-count line renders via the shared `videoCount()` helper (not a literal `Videos: {n}`), including the `0`/singular/plural boundaries `videoCount()` already covers elsewhere. Whole-card link: one `<a href="/studios/{id}">` wraps icon+text (assert a single tab stop per card, not two), icon `img` carries `alt=""` (redundant with the adjacent visible name) rather than a repeated `alt={studio.name}`. Multi-studio: N studios render as N cards in a `flex flex-wrap` row with no "+N more" cap (F38 precedent). **Call-site regression guard**: Media's visitor-only resolved-but-unlinked-studio text fallback and Film's "No studio set" empty state are both unchanged by this swap — assert byte-identical DOM for those two branches, since neither has a `Studio` object to hand the new component. **All 3 skins**: `rg 'zinc-|sky-|rounded-(lg|md|sm|xl)'` over the new file stays empty (token-only, per this section's standing theming rule); focus ring uses the browser's default link treatment, no custom `:focus-visible`, consistent with the plain links it replaces. **Status**: implemented and manually driven-browser QA'd across all 3 skins (icon-set and monogram-fallback states, both call sites) this session; no dedicated Vitest/Playwright coverage yet, consistent with §0/§11's standing frontend-automation gap |
| **`PeopleGrid.svelte`** — reusable People/Cast poster-tile grid, extracted from the Media detail page's People section and reused for the Film detail page's Cast section (HOLODEX-294, [design handoff](design/people-grid-handoff.md)) — **done** | Component/Interaction + visual | Vitest + Playwright | `editable` derives from `isOwner && !!attach && !!detach` — assert Media (owner) renders the hover-reveal × badge per tile plus the docked `PersonPicker` add-tile, while Film's Cast (no `attach`/`detach` passed) renders neither, even though both share the exact same tile markup. Tile size: `grid-cols-3 sm:grid-cols-4 md:grid-cols-6`, matching Films' own grid exactly (a visual-regression assert against the Films section's classes, not just a snapshot). Empty-state split: `editable && !people.length` renders the bare `+ Add person` text CTA (`PersonPicker`'s `hasPeople={false}` branch) with **no** grid and **no** dashed box; `!editable && !people.length` renders nothing at all (`{#if editable || people.length}` gate) — assert both, not just the populated case. Each-key uses `personKey()` (`id:role`) — assert a dual-role person (actor **and** director on the same video) renders as two independent tiles, each with its own working remove badge (ADR-072 composite-key regression guard, same shape as the pre-extraction markup it replaced). **Call-site regression guard**: `removeGridPerson`'s parameter widened `ResolvedPerson`→`Person` to satisfy the component's `onRemove?: (p: Person) => void` — assert its runtime role-validation guard still fires correctly given the looser static type. **All 3 skins**: `rg 'zinc-|sky-|rounded-(lg|md|sm|xl)'` over the new file stays empty; computed-style contrast on the section heading/tile name vs. body background checked at 5.7–6.3:1 across Cinémathèque/Broadcast/Brutalist (all above 4.5:1 AA), tile aspect ratio (`0.619`) consistent across all three. **Status**: implemented and manually driven-browser QA'd this session (Media owner populated+empty states, Film Cast read-only with real attached-film data, all 3 skins via `javascript_tool` computed-style reads per §11's screenshot-timeout note); no dedicated Vitest/Playwright coverage yet, consistent with §0/§11's standing frontend-automation gap |
| **`AliasPanel` provider badge + collision review line** (ADR-088, HOLODEX-306, [design handoff](design/alias-collapse-handoff.md), planned) | Component/Interaction + visual | Vitest + Playwright (aspirational — see the tooling note) + manual 3-skin pass | Extends the F23.6 row above rather than replacing it; everything that row covers still holds, since the panel's merge button, add form, `MergeOfferCard`, and near-miss nudge are untouched. New: a chip with `source: 'tmdb'` renders the provider's **display name** from the registry (`TMDB`, never the raw namespace) and a chip with `source: ''` renders **no badge and no placeholder gap**; ordering stays one case-insensitive list with provider and owner chips **mixed** — grouping them would re-introduce the two-tier reading the collapse removes, so assert the interleave explicitly, not merely that both render. The badge is a plain span and **not** `ProvenanceBadge` (which expands to a source breakdown that is meaningless for a single-origin alias) — assert the breakdown affordance is *absent*, since the wrong component would otherwise look right. `aria-label={`Remove alias ${a.alias}`}` must **not** absorb the provider name. The review line renders only when `skipped_aliases` is non-empty, inside the panel's existing `aria-live="polite"` region, singular-vs-plural copy table-driven, and stays hidden for a visitor along with the rest of the owner controls. **3-skin QA by computed contrast, not eyeballing** — `text-muted` on `bg-surface-2` is the exact pairing this codebase has shipped unreadable before, and the badge is the smallest text on the panel. **Tooling reality check**: `web/package.json` has no `@testing-library/svelte`, `jsdom`, or `happy-dom`, so no Svelte component can be mounted today (all 12 existing frontend tests are plain-TS modules under `web/src/lib/`). What is genuinely automatable this epic is `addPersonAlias`/`deletePersonAlias` in `web/src/lib/api.test.ts` — currently untested, unlike their `curatePerson`/`renameEntity` neighbours at `:77`/`:97` — plus the Go API tests covering `source` and `skipped_aliases` on the detail payload. Everything else above is a manual QA checklist until that dependency gap closes. **Delivered**: six `api.test.ts` cases (`addEntityAlias` path/verb, `source` round-trip, 409→conflict, non-409 throws, `deleteEntityAlias`, studio base mapping) plus a live three-skin pass on a seeded studio page — badge text on the chip measures **5.88 / 4.71 / 5.47** (Cinémathèque / Broadcast / Brutalist), all above AA 4.5:1 for its 10px size, with Broadcast the tightest and worth re-measuring if that token moves. The badge's `border-rule` sits at 1.23–1.49:1 against the chip, i.e. barely visible — deliberate, not a defect: `bg-surface` on `bg-surface-2` measures **1.01:1**, so no fill would read either, and the badge is distinguished by size, casing, and muted color rather than by its edge |
| **"Also known as" row removed from the person page** (ADR-088 D1, HOLODEX-306, planned) | Component | Vitest (via the resolved-payload fixture) + manual | The person page's `mergeFields` loop must render **nothing** for `aliases` — two lists of alternate names reappearing is the precise regression this epic exists to prevent, so the assertion is scoped to "no `aliases` row", **not** "no `mergeFields` block". The loop deliberately stays (an operator can map another merge field), so a test asserting the block is gone would pass today and wrongly fail the moment someone maps one. Cheapest durable form given the tooling gap: assert server-side that a person's `resolved[]` carries no `aliases` entry (the §4 D1 regression guard), since the loop is data-driven and renders nothing without one |
| **Film detail header poster; Images section removed** (HOLODEX-307, mechanical reuse — no dedicated design handoff, same posture HOLODEX-280 itself took) — **done** | Component/Interaction + visual | Vitest (type-check + `EntityImageSlot`'s existing row-variant callers) + manual 3-skin pass | `EntityImageSlot.svelte` gained `variant="frame"` alongside the existing `variant="row"` (still exercised unchanged by Studio's Images section, HOLODEX-286) — assert the row variant's DOM is byte-identical pre/post this change, since frame mode is additive, not a rewrite of the shared component's default path. Frame mode: empty+visitor → monogram fallback (not a blank dashed box — the poster role's old row-variant empty state, which reads as broken in a hero); empty+owner → the same "+" click-to-upload affordance poster already had; filled → the image plus two owner-only corner-overlay buttons (pencil = replace, × = remove, the latter two-step confirm/cancel, mirroring the row variant's own confirm state machine). Film detail page: the header's poster box (left of title/studio/tags) is now this frame-variant slot bound to `film.poster_url` in place of the old static monogram-only placeholder `<div>`; the dedicated "Images" section (which held poster **and** thumb) is removed entirely. `thumb` is dropped from the frontend only (`Film.thumb_url`, `FilmImageRole`'s `"thumb"` member) — it had no UI consumer per HOLODEX-280's own worklog; the backend `film_images` schema/API (§4's "Film poster/thumb self-hosted images" row above) is untouched and still generically role-keyed. `npm run check` (0 errors) and `npm run test` (152 tests) both clean. **Manually driven-browser QA'd** on `backend-films` (seeded `Dune` film): owner view shows the "+" upload affordance in the header (no Images section below); toggling Owner view off shows the monogram fallback correctly; verified across Cinémathèque, Broadcast, and Brutalist skins via screenshot. Upload/replace/remove's network round-trip itself was **not** re-driven live in this session (no local file-picker automation in this environment) — `EntityImageSlot`'s `upload`/`remove` props are unchanged in shape from the row variant's already-shipped, already-tested call (`api.uploadFilmImage`/`api.deleteFilmImage`), so this is a display/wiring change riding an existing, working mutation path, not new mutation logic; flagged here rather than silently assumed, same posture as the collision-path notes on the Studio/People picker rows above. **Follow-up noted, not fixed here**: a `/simplify` altitude pass flagged that the new frame-variant overlay buttons duplicate the Person hero's `editBtn` pattern (`people/[id]/+page.svelte`) rather than converging on one shared component — three near-identical pencil-overlay implementations now exist across `NameEditControl.svelte`, the Person hero, and `EntityImageSlot.svelte`; consolidating them would touch the currently-working Person hero, out of scope for this change |
| **Film enrichment wiring** (ADR-089 D5, HOLODEX-309) | Unit + Interaction | Vitest | `EnrichEntityKind` includes `'film'` and `ENRICH_ENTITY_BASE.film` resolves to `/films`; `runEnrichRefresh`/`runEnrichRefreshAll` hit `/films/{id}/enrich/refresh*` **with no film-specific branch in `enrichRefresh.ts`** — the widening is the whole change, and a branch there means the generic path was bypassed. Chips and picker render on the film Details section for an owner and **not at all** when `films_enabled` is off (a rendered chip would 404 against unregistered routes). Visitor sees the section-level "Enriched from X" note and no controls, matching studio. **Guard**: the epic's diff leaves `web/src/lib/components/enrichment/**` unchanged — those components take no entity id and no entity kind, so a modification means the reuse was misunderstood |
| **Film year edit client** (ADR-089 D3, HOLODEX-317) | Unit | Vitest | `setFilmYear` PUTs `/films/{id}/year`; a collision resolves as `{conflict}` and **must not throw** — a throw renders red inline-error text where the verdict card belongs; a genuine 400 still rejects with `ApiError`, which is what the control's error slot is for |
| **Film cast difference rendering** (ADR-089 D2, HOLODEX-310, planned) | Component | Vitest + @testing-library/svelte | The union renders in the primary `PeopleGrid`; the difference renders in a second, separately-headed instance — a real heading, not a styled `div`, since colour and dashed borders alone do not carry "billed but absent" to a screen reader. No name appears in both grids. Empty difference → the second group is absent from the DOM, not present-and-empty. No provider credits → the Cast section matches its pre-F59 render exactly, including the absence of the coverage counts line |

---

## 6. E2E Flows (Playwright against `docker compose up`)

A seeded fixture library mounted as `MEDIA_PATH`; assert end-to-end:
1. Cold start → `/readyz` green → browse grid populated.
2. Title search returns expected video; URL reflects query.
3. Combine filters (4K + tag + date) → correct subset.
4. Navigate People → person page → only their videos.
5. Open detail → raw-tag panel shows captured tags (F7.4).
6. Player loads; **seek** issues a Range request (verify 206 in network log).
7. Dark-mode default; toggle persists across reload.
8. (Phase 2) MCP `search_videos` over HTTP returns same ids as the UI filter.
9. (Phase 2) Add a `studio` mapping → reload-config → Studio facet appears & filters.
10. Add a file to the watched dir → appears within scan interval (F1.2).
11. **(Quick Wins · QW1)** Run a search → open another → focus the search box → the prior query appears in history → click it re-runs the search (URL `/search?q=…`); remove + clear empty the list.
12. **(Quick Wins · QW3)** Open a detail page with shared people/tags → a "More with `<person>`" and/or "More with `<tag>`" shelf renders ≤5 cards → click a card navigates onward; an item with no siblings shows no empty rail.
13. **(Quick Wins · QW4)** Scroll the grid, "Load more" ×2 (150 items), open an item, press **Back** → **same scroll position** (±a few px) **and** all 150 items still present **and** the opened item on screen **and** **zero** `GET /api/v1/media` fired **and** no `Loading…` flash. Then change a filter → resets to top + refetches; hard-reload → rebuilds page 0 at top.
14. (Phase 3, F22) With the **fake provider** wired in compose and `ADMIN_TOKEN` set: open a person → Enrich → name-search → pick a candidate → fields populate with "from <provider>" provenance; re-enrich skips the picker; clearing the provider falls the field back. Assert no provider call fires without the click, and a token-less client sees no Enrich control.
15. **(Phase 3, F23)** With `ADMIN_TOKEN` set: open a person → add alias "Ziggy" (chip appears) → type "zig" in the global search box → the person appears → click → lands on the same person; delete the alias → "zig" no longer surfaces them. A token-less client sees the chips read-only (no add field, no ✕).
16. **(Phase 3, F23 — merge)** Library has duplicate people "Jennifer Lawrence" and "J Law". As owner, on Jennifer's page click **Merge a person in…** → pick "J Law" → confirm (both video counts shown) → Jennifer's video list grows to the union, "J Law" appears as an alias, the standalone "J Law" page is gone (404). Searching either name lands on Jennifer. **Trigger a re-scan** → still merged (no "J Law" person reappears). Also: typing "J Law" into the add-alias field of a *different* person surfaces the collision prompt (never a silent merge); and the `/people` list **Merge people…** multi-select → "Keep which name?" achieves the same merge.
17. **(F24 — delete/restore)** With `ADMIN_TOKEN` set, as owner: open a detail page → **Move to Trash** → confirm → land back on the grid with the item **gone**; it is also absent from search and its people/tag pages, and `/media/{id}` 404s. Open **/trash** → the item shows "deleted … · purges …" → **Restore** → it returns to the library and Trash empties. A token-less client sees **no** Manage block, **no** Trash link, and `/trash` shows "Owner only." (Purge-now is exercised only against a throwaway fixture file — never the seed corpus — asserting the file is gone and the row hard-deleted; a read-only-mount variant asserts the item stays in Trash on a failed unlink.)
18. **(F35 — owner hub + nav split)** As owner with **Owner view on**: the top nav shows only **Media · People · Tags**; click the **Owner** gear (near the skin picker) → land on `/owner` with tabs **Status · Metadata keys · Trash** → switch tabs (content swaps with **no** full-app reload / no jump to top). Paste the old `/status`, `/keys`, `/trash` URLs → each **redirects** to its `/owner/*` tab. Toggle **Owner view off** → the gear **disappears** and the bar is the clean visitor surface (no Keys/Status link leak); with it off, paste `/owner/trash` → it **auto-reveals** owner view and renders the page fully (announces "Owner view on."). A **token-less** client sees no gear and any `/owner*` URL redirects home with no owner data. Run the gear, tabs, and active-tab read in **all three skins**.
19. **(F44 — promote / override)** With the fake/TMDB provider wired and `ADMIN_TOKEN` set, enrich a person so a non-canonical field (e.g. `measurements`) shows under **Additional details**. As owner: the row shows a **Promote** pill → open the inline editor → set a label + `render:text` + group + order → Save → the field **leaves** Additional details and appears above as a curatable **`SourceSelect`** row (baseline `·record` chip + `provider` chip), rendered **once** (no doubled row). Pin a source, then open **Edit** (same editor, pre-filled) → **Remove promotion** → the field returns to a display-only auto row; **re-promote** → the prior source pin re-applies. Promote a `chips` field → it becomes a **`CurationFieldRow`** merge row (per-value ✕ / ＋ Add) instead. Confirm the label change shows on **another** person with the same key while a value suppressed on the first does **not**. A **token-less** client sees **no** Promote/Edit controls — just the curated label/value. Repeat the promote/edit/de-promote round-trip on `/studios/{id}` and `/media/{id}`. Run the affordance, the inline editor, and the post-promotion curatable row in **all three skins**.
20. **(F45 — derived person fields)** On the `backend-films` testbed (enriched people carry a clean ISO `birthdate`): open a **living** enriched person → an **Age** line shows a bare integer directly under **Born**, with **no** icon/badge; hovering the number shows the tooltip **"calculated from Born"** — and **no** source-select / promote / Custom controls even as owner (identical to the visitor view). Open a **deceased** person (birth + death date) → an **Age at death** line with a number and **no** running Age line. Open a person with **no birthdate** → **no** Age line at all (no dash, no gap), as owner **and** visitor. Inspect the person detail response → the computed row carries `computed:true`, `winning_source:"computed:age"`, `derived_from:["Born"]`, and no `decision`/`candidates`/`in_sync`; a decision `POST` naming `age`/`computed:age` returns **400**. The row has no skin-dependent styling (plain `text-ink` value), so the three-skin check is trivial.
21. **(ADR-077 — tag writeback exclusion)** As owner, on a tag's page with already-written files: toggle **File writeback** off (button flips, no confirm, no network beyond the `PATCH`) → click **Sync writeback now** → the batch dialog progresses confirm → progress → done, and the file's Genre field no longer contains that tag's name while the tag itself still shows on the video in Holodex. Toggle it back on and sync again → the name reappears in Genre. On `/tags` in Manage mode, select 2+ tags spanning mixed writeback states → **Turn off writeback for selected** flips all of them (no confirm) → **Sync writeback now for selected** opens the same batch dialog scoped to the deduplicated video union across the selection, and a video carrying two of the selected tags is written exactly once. A token-less client sees none of these controls.
22. **(HOLODEX-240 — tag categories)** With `ADMIN_TOKEN` set, as owner on `/tags`: switch the type filter to **Categories** (empty grid, P1 empty-state copy) → use the search box, still empty → select 2+ tag pills in Manage mode → click **"Add to category…"** → in the picker, type a brand-new name and pick **"+ Create"** (no second confirm — assign is additive, unlike Merge) → the tags now carry the new category; switch the type filter to **All** → the new category pill appears (accent border + tag-glyph icon, count badge = 2), its body **navigates** to `/categories/{id}` even in Manage mode. On `/categories/{id}`: rename via the pencil button's inline form → both member-tag chips (linking back to `/tags/{id}`) → **+ Add tag** a third, existing tag by name → remove one via its chip's ✕. Back on `/tags`, select the same 2 tags again → **"Remove from category…"** → the picker lists only categories they're actually in (skips opening entirely with an inline hint if none) → remove → re-select and confirm the Manage-bar bulk actions are gone (< 2 selected). On the main library page, the new **Categories** facet lists the category → selecting it returns exactly the videos tagged with either member tag, combining correctly with an additional Tags-facet selection. A **token-less** client sees no category ⋯ menu (Rename/Delete absent), no `/categories/{id}` add/remove controls (read-only chips), and no Manage-bar bulk category actions. Attempt creating a category or tag whose name collides with the other type (fold-equivalent, e.g. an existing tag "Sci Fi" vs. a new category "SciFi") → a clear inline error, not a silent merge. Run the picker, the pill Manage-mode asymmetry, and the detail page in **all three skins**.
23. **(HOLODEX-249 — unified nav search, target)** On `/people` (no filter today): the "People" tab is pre-selected on load; type a name → the People grid filters **in place**, no dropdown card, **zero** network calls beyond the page's original list fetch. Tap the **Videos** tab without navigating away → the People grid freezes and a grouped panel opens showing Videos matches with a "View all N in Videos" row; click it → lands on `/search?q=…&type=videos` with the query pre-filled. Tap back to **People** → in-place filtering resumes on `/people` itself. Clear the box → the panel/tab state closes and (if history exists) the recent-history dropdown reappears exactly as before this feature. On `/` (Media): confirm the URL still reflects the query (`filtersToParams` round-trip unchanged) and the page's own inline search input is **gone** — same for `/tags`. On a person/studio/tag detail page, confirm the "Videos" tab is pre-selected on load and the embedded video list filters in place (NS6) — **verified live** for person and tag (no studio fixture in the `backend-amv` testbed). Resize to a 375px viewport → the panel becomes a full-width fixed sheet with no horizontal scroll and all 5 tabs visible unwrapped. Run the tab row, the panel, and the mobile sheet in **all three skins**. *(Marked target — spec + design-handoff exist; NS1–NS6 implemented and manually driven-browser verified, no automated Playwright E2E coverage yet.)*

---

## 7. Non-Functional Testing

### Performance (the "snappy" goals)
- **Seeded 50k-record dataset** generator (DB rows + minimal fixtures) for realistic load.
- **Search API p95 ≤ 300ms** (F4.9): k6 or Go benchmark hitting `/api/v1/media` with varied filter combos against 50k rows on a 4-core/8GB runner. Fail CI if regressed > threshold.
- **Route transitions ≤ 150ms** (NFR): Playwright + Performance API / Lighthouse CI budget.
- **Scan throughput**: benchmark full extraction (N files, M workers) and steady-state incremental (stat-only) — assert incremental is dominated by stat, not extraction (ADR-018).
- **Cache effect**: measure hit-rate and latency delta with cache on/off.

### Resilience / failure injection
- Corrupt file mid-library → scan completes, file skipped + logged.
- DB locked / busy under concurrent write+read → no errors (WAL, ADR-003).
- ffmpeg/exiftool missing → clear startup behavior; `THUMBNAIL_ENABLED=false` path works.
- SIGTERM mid-scan → graceful shutdown, clean WAL, fast restart (ADR-019).
- Network-share latency simulation for mid-copy quiet-period (ADR-018).

---

## 8. Tooling & CI Pipeline

| Stage | Runs | Gate |
|-------|------|------|
| **lint** | `golangci-lint`, `svelte-check`, `eslint`, `prettier` | every push |
| **unit** | `go test -short ./...`, `vitest run` (no binaries/docker) | every push, < 60s |
| **scripts** | `make test-scripts` (`node --test "scripts/**/*.test.mjs"`, zero deps) | every push, < 5s |
| **integration** | `go test ./...` in the Debian image (exiftool+ffprobe+sqlite present); generates fixtures first | every PR |
| **e2e** | `docker compose up` + Playwright | every PR |
| **a11y + visual** | Playwright + axe + screenshot diff | every PR |
| **perf** | k6/bench against seeded 50k; Lighthouse budget | nightly + label `perf` |

Conventions:
- Go: standard `testing`, table-driven, `testify/require` for assertions, golden files with `-update`.
- Integration tests gated by build tag `//go:build integration` (or `-short` skip) so the inner loop stays binary-free.
- Frontend: Vitest (jsdom) for unit/component; Playwright for browser.
- **Automation scripts** (`scripts/**`): `node:test` + `node:assert/strict`, no dependencies. I/O is
  injected (`exec`) so the logic is testable without a registry or a Jira. `scripts/resolve-release-digest.mjs`
  decides which image digest gets published as a release (ADR-070), so its **negative** cases are the
  point — a mismatched revision label, a non-ancestor commit, and a registry error that must abort rather
  than be mistaken for "no image here" each have a test. Treat that file as critical-invariant.
- Fixtures generated deterministically in CI; goldens committed.
- Coverage reported per-package; PRs surface deltas (informational, not a hard gate except on the critical-invariant packages).

---

## 9. Phasing — what lands when

### Phase 1 (MVP) — foundation tests
- Fixture corpus + generator + goldens (the long-pole; build first).
- Extraction + merge + MKV precedence (ADR-004/010).
- Resolution classifier boundaries (ADR-012).
- Scanner: incremental, symlinks, mid-copy, soft-delete (ADR-011/018).
- Repository + migrations + WAL concurrency (ADR-003/016).
- Title FTS + diacritics + filter builder (ADR-017).
- Range serving (ADR-015); core REST endpoints (ADR-006).
- `video_metadata` capture (F2.9); raw-tag panel (F7.4).
- Health/readiness/shutdown smoke (ADR-019).
- Component tests for VideoCard/FilterBar; dark-mode default; a11y AA.
- E2E flows 1–7; perf baseline for search.

### Phase 2 — MCP, thumbnails, mapping
- MCP tool tests + **REST/MCP parity** (ADR-005).
- Thumbnail pipeline (ADR-009): Tier-1 embedded-art extraction vs Tier-2 generated frame; bounded worker pool + high-priority bump; single-column state machine with sweep-retried `failed`; serve 404-while-pending → 200; `POST .../thumbnail` regenerate; `thumbnail_queue_depth` on `/admin/status` (full Prometheus deferred); a `-tags integration` test runs real ffmpeg so the argv/muxer is exercised.
- Mapping: normalization, precedence, `multi`, facets, reload endpoint, key-discovery view (ADR-013, F20).
- Sort, keyboard nav, responsive, Prometheus `/metrics` shape.
- E2E flows 8–10.

### System Activity "Under the Hood" (F21) — post–Phase 2
- **Activity read-model** (ADR-028): scanner `Status()` states/triggers + last-run counts; `job_runs` insert + 30-day prune + restart survival; library-count caching; no-secrets invariant (incl. history `error_message`).
- **Owner gating seam** (ADR-030): open-vs-gated across all owner routes; constant-time compare; fail-loud on non-loopback + no token; CSRF rejection of cross-site admin POST; frontend capability-flag toggle.
- **Owner session cookie** (ADR-046): token-exchange sets a signed `HttpOnly`/`SameSite=Strict` cookie (never the raw token); reload-persistence — cookie alone authorizes gated routes and flips `capabilities.owner`; header path unchanged; tampered/expired cookie rejected + cleared; sign-out idempotent; "trust this device" longer (server-set) lifetime; sliding renewal bounded by an absolute cap; `Secure` except plain-HTTP loopback. Frontend: stays signed in across reload, "Trust this device" + "Sign out" controls, graceful drop to prompt on 401 — token never in web storage; **all 3 skins**.
- **Activity page + header indicator**: polled refresh reflects state within one interval; loading/empty/error states; **all 3 skins** (tokens-only, per CLAUDE.md). SSE (F21.8) tests land with ADR-029.
- **Poll resilience + ForwardAuth re-auth** (HOLODEX-127): an upstream Authentik ForwardAuth expiry 302-redirects an authed `fetch` cross-origin; `redirect: 'manual'` turns that into an opaque redirect the client detects (`ReauthError`) instead of a CORS-blocked `TypeError`, then recovers via a single top-level reload (`window.location.assign`) that follows the 302 through the still-valid SSO cookie. Unit (`api.test.ts`): authed reads/writes carry `redirect: 'manual'`; an opaque redirect raises `ReauthError`; concurrent authed failures reload exactly once; a 200 is unaffected. Unit (`activity.test.ts`): a single blip keeps last-good `data` and shows no error (surfaced only after `FAIL_GRACE` consecutive failures), the poll backs off to `MAX_POLL_MS` and resets on success, a `ReauthError` never flashes an error, and the ADR-046 **401 owner-expiry** drop-to-read-only path is unchanged (the two expiries are deliberately distinct).

### Quick Wins batch — post–Phase 2
- **Overlay bugfix**: `.app-atmosphere.is-playing::after` suppression toggled by the media-page `<video>` (`onplay`/`onpause`/`onended`), restored on unmount; verified in **all 3 skins** (Broadcast is the load-bearing case).
- **Search history** (QW1): `searchHistory.ts` store unit tests (dedupe/cap/defensive-parse/clear) + dropdown interaction (keyboard, click-before-blur, no network); 3-skin dropdown render.
- **Related-media** (ADR-031, QW2/QW3): repo random-select methods + `GET /media/{id}/related` handler — selection determinism, self-exclusion, active-only, ≤5, null/empty blocks, 404, no-N+1; **assert membership/count not order** (RANDOM()). `RelatedShelf` component tests incl. per-shelf Brutalist counter reset.
- **Fluid Back** (ADR-032, QW4): `browse.svelte.ts` store unit tests (signature reuse/invalidate, scroll round-trip) + the **E2E scroll+pages restoration** flow (no refetch, no flash). This is the first Playwright assertion on scroll restoration — establishes the pattern for future paginated views.

### Admin Mode toggle (F29) — post–Phase 2

Frontend-only, presentation layer over the owner gate (no backend, no migration). Spec
[`admin-mode.md`](specs/admin-mode.md), handoff + QA checklist under `docs/design/`.
- **Store** (`adminMode.svelte.ts`, **done** — `adminMode.test.ts`): defaults **ON**; `set`/`toggle`
  persist the boolean to `localStorage['holodex-admin-mode']`; `init` restores an explicit stored `"false"`
  and keeps the default ON for an absent/garbage value; no-throw without `localStorage`; `reveal()` forces
  ON + announces (auto-reveal), no-op when already on.
- **Effective gate**: every owner-gated surface reads `activity.isOwner && adminMode.enabled` (the per-page
  `isOwner` derived now folds in `adminMode`). With Admin mode **OFF**, the full hide-set (header Trash link,
  home "Recently Added", media enrich/writeback/delete/regenerate + provenance + the raw "Enrichment data:
  File Extraction"/"…: {provider}" disclosures (relocated under Manage, now owner-gated), person aliases/
  merge/images/enrich, people-list merge, status rescan/reload) is **absent from the DOM** — agent-asserted,
  not just
  visually hidden. The `/status` token-unlock UI stays `needToken`-gated so visitor view never locks the
  owner out.
- **Auto-reveal (P0-6)**: landing on an owner-only route (`/trash`) while OFF flips Admin mode ON, renders
  the page, and announces via the layout `aria-live` region; a public route does **not** flip it.
- **Security invariant** (ties to ADR-030): the toggle is **presentation only** — it changes no token,
  capability, or server authorization. An owner-only API still authorizes with Admin mode OFF; a true
  non-owner is still rejected. (`/security-review` before merge.)
- **3 skins / a11y**: `role="switch"` + `aria-checked`, accent-fill ON vs muted-outline OFF, open-eye/
  eye-slash icon (meaning not color-only), label hides `<sm`; tokens-only (`rg` guard empty). Vitest/
  Playwright UI coverage tracked with the broader frontend-suite debt (§9 Quick Wins / F21).

### Owner tooling hub + nav split (F35) — post–Phase 2

Information-architecture + visibility change over the owner gate — the consolidated owner area F29 parked.
Spec [`owner-tooling-hub.md`](specs/owner-tooling-hub.md), handoff + QA checklist under `docs/design/`.
Frontend-routing-led; **no new backend** beyond closing any ungated `/keys`/`/status` API exposure (verify
in `/security-review`); the internal `adminMode` store/key are **unchanged** (the rename is deferred — P2).
- **Routes & redirects**: the three pages move under `/owner/*` behind a shared `owner/+layout` shell;
  `/status`, `/keys`, `/trash` **permanently redirect** to their `/owner/*` equivalents (old bookmarks and
  F29's `/trash` references still resolve). `svelte-check` + a redirect loader test.
- **Group gate + single auto-reveal (P0-6)**: `owner/+layout` enforces `effectiveOwner` (non-owner →
  redirect home, **no owner data rendered**) and fires auto-reveal **once at the group level** (a Preview-ON
  owner landing on any `/owner` route flips to owner view and announces "Owner view on."); nested children no
  longer each re-implement it — consolidating F29's per-route behavior.
- **Header rework**: content nav reduced to Media/People/Tags; an **Owner gear** in the owner-chrome cluster
  (gated on `effectiveOwner`; active state `text-accent` + `aria-current`, **not** a fill; label hides
  `<sm`); the F29 toggle **relabeled "Owner view"** — **string-only** (the word "Admin" leaves the header
  UI — asserted by an `rg` over `+layout.svelte`), no behavior change.
- **Hub shell + tabs**: `skin-title` "Owner" heading + tab row (Status/Metadata keys/Trash); **active tab
  `bg-surface-2 text-ink` + `aria-current`** (the skin-picker idiom — never `bg-accent`, which stays
  reserved for a page's single primary action like Status's Rescan); tab switch is in-group client nav (no
  full reload); inner page content relocated **unchanged**.
- **Visitor-leak invariant** (the bug this closes): with Preview OFF / as a non-owner, the gear and all
  `/owner` content are **absent from the DOM** and the bar is exactly Media/People/Tags + search + skins —
  closing the pre-F35 exposure of the `/keys` and `/status` nav links. Agent-asserted.
- **Security** (ties to ADR-030): the relocation/relabel changes **no** client authorization. The one
  server change is gating **`GET /metadata-keys`** behind `requireOwner` — it backs the now-owner-only Keys
  tab, closing an F20-era public exposure the nav split surfaced (spec P0-4). Asserted in `auth_test.go`
  (`TestGateRequiresTokenWhenSet`: 401 without token / 200 with, when a token is set); activity + trash were
  already gated.
- **3 skins / a11y**: gear + tabs tokens-only (`rg` guard empty); `aria-current`/`aria-label`; focus order
  `ActivityIndicator → Owner-view toggle → Owner gear → skin picker`. Vitest/Playwright UI coverage tracked
  with the broader frontend-suite debt.

### Phase 3 — enrichment

**Metadata source plugins — People (F22, ADR-033)** — the first slice, fully CI-testable with **no network or API keys**:
- **Provider contract** conformance via an **in-process fake** implementing `/describe` `/resolve` `/enrich` `/healthz`; `protocol_version` mismatch rejected; timeout/5xx/garbage tolerated (fetch fails, server lives). Real TMDB/IMDB containers are exercised only by manual/staging checks, **never live in CI**.
- **Unified resolution**: `sources` interleaving `file:` keys + providers, first-present-wins, file-first-class fields preserved on re-scan (the cardinal enrichment invariant).
- **Shadow store** `entity_enrichment`: upsert, persisted `external_id` (re-enrich skips identity), re-scan non-destructiveness, clear-provider isolation, **migration preserves user enrichment** (ties to the migration-safety invariant).
- **Matching**: embedded-ID auto vs name-search-confirm paths.
- **Security**: `requireOwner` 401s, SSRF allowlist + no-redirect, untrusted-response sanitization/size limits, no-keys-in-core.
- **Frontend**: enrich picker (combobox/listbox a11y, focus trap), provenance badges in **all 3 skins** (tokens-only). MCP enriched-field parity.

**Person aliases & merge (F23, ADR-036)** — the first People slice, fully CI-testable, no network:
- **Store**: `person_aliases` CRUD; per-person case-insensitive uniqueness (idempotent add); same alias on two people allowed; delete scoped to person; `ON DELETE CASCADE`.
- **Search**: `person_aliases_fts` MATCH surfaces the person by any alias (diacritic-folded), deduped with name matches (person appears once), per-group limit respected.
- **Scan-time resolution**: extracted name routes name → alias → create; alias-tagged file links to the canonical person; **merge survives re-scan** (cardinal invariant).
- **Merge**: de-duped video union; merged name → alias; prior aliases re-pointed; enrichment dropped; duplicate deleted; self-merge/unknown-id guarded.
- **Collision**: same-name belonging to a different person → 409 for confirmation, never auto-merge (homonyms).
- **Endpoints**: alias add/delete + merge owner-gated (401), 400 invalid/self-merge, 404 unknown, 409 collision; `GET /people/{id}` carries `aliases`.
- **Frontend**: aliases panel + `PersonPicker` merge + collision prompt + `/people` multi-select merge — owner-only controls, optimistic delete, inline validation, **all 3 skins** (tokens-only, `accent-accent` checkboxes).

**People images (F25, ADR-038)** — faces across the app; fully CI-testable, **no network** (the one external fetch is exercised against the in-process provider fake / a local test server, never a real host):
- **Ingest normalization** (the security spine — `internal/personimage`): a valid JPEG/PNG/GIF/WebP decodes → re-encodes to a single safe format; **all metadata stripped** (round-trip an image with planted EXIF/GPS → output has none); a renamed-non-image / truncated / polyglot byte stream is **rejected, nothing written**; an oversized-dimension (decompression-bomb) input is rejected **before** full decode; output bytes/dimensions are bounded. Identical pipeline for upload, enrichment-download, and promote-crop inputs.
- **Placeholder resolution** (pure function): `(skin × role × gender) → asset` is deterministic; `male/female` map to their bucket, **`nonbinary` + unknown + absent → `neutral`**; unknown skin/role guarded; a placeholder is never persisted nor counted against the cap. Snapshot/golden the generated SVG per cell (27 cells) so skin/token drift is caught.
- **Repo `person_images`**: insert/get/list/delete CRUD; **core-slot uniqueness** (second `headshot` upsert *replaces*, never two); **gallery cap** enforced transactionally (over-cap `extra` rejected; **core slots never blocked by the cap**); `sort_order` reorder; **`ON DELETE CASCADE`** with the person; the row-`id`-as-version invariant (replace ⇒ new id).
- **Gallery cap, override & suppression (F25.23–25, ADR-043)**: `SetGalleryCap` changes the effective bound (`PERSON_GALLERY_MAX`) and an `OverCap` insert bypasses it; **delete-suppression** records an enrichment `extra`'s `source_url` (and *not* an upload's empty URL, nor a core role's) so a re-enrich **skips the suppressed asset URL** (enrich orchestration test, fail-open on lookup error). API: a **core-role upload at a full gallery → 201** (the F25.8 bug-fix regression guard), an over-cap `extra` → 409, and `allow_over_cap=true` → 201; `/capabilities` advertises `person_gallery_max`.
- **Content-hash dedup (F34, ADR-050)**: every ingest threads `content_hash = sha256(normalized bytes)` (the sink hashes the *normalized* output, not the raw download — proven by a `Hash(Normalize(raw))` equality test). **Repo**: an **enrichment** `extra` whose hash already exists for the person — under **any** role (cross-role: matching the headshot) — is rejected with **`ErrDuplicateImage`**; a re-enrich offering the same hash is rejected; an **owner upload** duplicating a hash is **allowed** (deliberate, never deduped); a **core** insert duplicating a hash is **allowed** (single-slot replace + the F25.29 poster seed legitimately repeat a hash). `ExistingPersonImageURLs` returns every stored non-empty `source_url` (any role). **Backfill collapse** (`CollapseDuplicateGalleryExtras`): keeps the earliest `extra` of a hash, drops an `extra` matching a **core** image (core wins), **never deletes a core image**, and is **idempotent** (second pass removes 0); `PersonImagesMissingHash`/`SetPersonImageHash` round-trip. **Sink**: an `ErrDuplicateImage` from the repo is a **silent skip** — `StoreAsset` returns nil and **nothing is written to disk**. **Enrich orchestration**: a gallery asset whose URL is in `ExistingAssetURLs` is **skipped before fetch** (URL fast-path, gallery-scoped, fail-open) while a fresh gallery URL still flows. **Backfill pass** (`personimage.Backfill`): hashes rows from on-disk bytes, **skips a missing file** (not fatal, retried next boot), removes collapse victims' files; idempotent.
- **Owner/admin cap bypass + gallery/viewer modals (F25.32–34, HOLODEX-174)**: `Sink.StoreAsset`'s `overCap` param threads to `PersonImageInsert.OverCap` (repo test: `overCap=true` on an insert asserts `OverCap` reaches the repo call — `TestSinkThreadsOverCap`); the three `enrich`-apply HTTP handlers and `ReEnrich` pass `h.auth.authorized(r)` rather than an asserted literal, so the bypass can't diverge from the actual owner check even though every call site is already behind `requireOwner`. **Frontend** (Vitest + a11y): `PersonGalleryModal` opens on the "Gallery (N)" trigger with the first grid item focused (not the header's own Close button — a real regression caught during manual QA), traps Tab, and returns focus to the trigger on close; `PersonImageViewer` opens from either the row or the grid, fits an image without upscaling past native resolution, and its prev/next `aria-disabled` at the ends match the row's existing move-left/right convention; stacked-modal Escape closes only the topmost layer (verified: first Escape from the viewer-over-grid state leaves the grid open with focus back on the originating grid item, second Escape closes the grid) rather than both at once; the owner overlay's `pointer-events-none`/`-auto` layering means a background click opens the viewer while a button click (promote/move/delete) does not.
- **API**: serving route returns the **real image when present, else the resolved placeholder**, never 404 for a valid `(person, role)` (unknown role → 400, unknown person → 404); real images carry `?v=` + `immutable` cache headers and a replace emits a **new `?v=`**; **multipart upload** validation (missing field, wrong role, bad/oversized file → 400; `MaxBytesReader` enforced); **owner-gating** — upload/delete/reorder/promote/override → 401 without token when gated, 200/201/204 with it; viewing is public. No client-supplied path reaches the filesystem (traversal attempt → still server-assigned path).
- **Enrichment asset download** (extends F22): a provider-returned asset URL is fetched **through the existing SSRF guards** (allowlist, cross-host-redirect refusal, response-size cap, timeout — reuse/extend the F22 tests), then run through the **same normalization** and stored with provenance (`source=enrichment`, provider/external_id); a hostile asset URL (internal address, redirect-to-internal, oversized body, non-image) is refused with nothing written.
- **Frontend** (Vitest + a11y): `personImageURL` builds the `?v=` cache-bust correctly (and omits it when absent); `PersonAvatar`/`PersonBanner`/`PersonPoster`/`PersonGallery` render real-or-placeholder, fall back to the placeholder on `img` error (**never a broken-image box**), reserve the box (no layout shift); owner-only upload/delete/promote controls **absent from the DOM** for non-owners; the promote crop editor traps+returns focus; **all 3 skins** (tokens-only — the `rg 'zinc-|sky-|…'` guard stays empty); placeholder glyph contrast ≥3:1.

**Later Phase-3 slices (future specs/ADRs):**
- Tag aliases / tag-graph DAG traversal (incl. cycle handling); person-merge (folding distinct extracted people); promoting provider-sourced aliases into searchable `person_aliases`.
- **Writeback** (consumes the F22 shadow layer): round-trip (write → re-scan → read-back equal); **backup created before write**; atomic-failure leaves original intact; `WRITEBACK_ENABLED` gating.
- Preview-trailer generation gating + storage budget.

**Refresh Metadata (F31, ADR-047)** — per-item forced re-extract + re-enrich; fully CI-testable, **no network** (the provider half runs against fakes / the in-process provider fake):
- **Refresh service** (`internal/refresh`): the forced re-extract **persists unconditionally** — no `(size, mtime)` change-detection on this path (the property that catches an external edit which preserved the file's mtime); a missing/soft-deleted target (`repo.ErrNotFound`/`ErrDeleted`) **short-circuits before any extract or write** and propagates unwrapped for the 404-vs-409 split; a file-read failure aborts **before** persistence (**no row mutation** on a bad/unmounted file); **per-source isolation** — a provider failure becomes that source's `ok:false`+error and never undoes the file commit or fails the refresh; no-match / nil-enricher is a clean file-only refresh; the file-layer diff drives the `changed` signal (unchanged file ⇒ no change).
- **Activity recording** (F31.6): **exactly one** `job_runs` row per refresh (`kind=refresh`, `trigger=manual`); a partial/failed run marks the row errored; `detail` references the item as `#id` and carries **no path** (the ADR-028 no-secrets invariant); a rejected request (404/409) records **nothing**.
- **Scanner seam** (`BuildVideoFromFile`): re-extracts and builds the `Video` **without touching the DB** (read-only — persistence is the caller's apply phase) and without change-detection.
- **Repo** (`RefreshTarget`): resolves a live id → path and distinguishes **missing (`ErrNotFound`) vs soft-deleted (`ErrDeleted`)**, so a refresh never re-reads or reactivates a trashed item (ADR-037 #26 guard).
- **Enrich additions**: `ProviderMatches` enumerates an entity's linked providers from the shadow store (dedup to one match per provider, persisted `external_id`, entity-scoped — a different entity's rows don't leak in); `ReEnrich` re-fetches **without** recording its own job row (so the refresh owns the single combined row) and writes **only** the provider's shadow layer.
- **API** (`POST /media/{id}/refresh`): owner-gated (**401** without owner auth, **202** with it), **404** unknown, **409** soft-deleted, **400** bad id, **503** when the service is unwired; per-item path mirrors `/media/{id}/enrich` + `/writeback`. Covered by an `httptest` handler test over a real repo + stub extractor (`internal/api/refresh_test.go`).
- **Frontend** (Vitest + a11y): `summarizeRefresh` maps a report → the inline status line (synced / already-in-sync / partial-warn); the ghost **Refresh** control + `aria-live` status are owner-only (**absent from the DOM** for non-owners), the spinner honors reduced-motion, and the always-available header renders for owners even with no resolved/file metadata; refetch-after-mutate updates `resolved[]` and busts the cover; **all 3 skins** (tokens-only — the `rg 'zinc-|sky-|…'` guard stays empty). **Live 3-skin visual QA** (Cinémathèque/Broadcast/Brutalist) per the design handoff's `[human]` checklist remains the manual gate (needs a running backend + media + owner session + a matched provider).
- **Non-destructive layering invariant** (cardinal): a re-extract never drops enrichment data and a re-enrich never drops file fields — the two layers are written independently and merged only by the resolver.

**Per-field source-of-truth (F36, ADR-051)** — standing per-item, per-field source decision over precedence; fully CI-testable, **no network** (provider half against the in-process fake). Maps to the QA checklist §2 smoke items; the cardinal invariants are file-first default, source-pin-not-value, and atomic-batched writes.
- **Resolver — decision short-circuit (replace)** (`internal/resolver`, pure, no I/O): a decision pins display to the decided source (`file` / `provider:<name>` / `manual`) **over** mapping order; with **no** decision the field resolves **file-first** under `default_source: file` (the bug fix — a provider no longer masks the file), and `default_source: mapping` restores today's first-non-empty. The decision map is pre-loaded alongside enrichment/curation (the ADR-013 purity invariant — a decision change re-renders with no re-fetch/re-scan). *(QA 2.3)*
- **Resolver — merge untouched (RD1 regression guard)**: merge-field resolution (union + per-value F30 curation, combined provenance) is **byte-identical** with and without F36 — the source decision is **replace-only** and must not perturb set fields. *(QA 2.4)*
- **Source-pin, not value-pin** (cardinal): a `file` decision reflects a later re-extracted file value (drive via the forced re-extract seam); an `adopt provider` decision reflects a re-enriched value; only a `manual` decision is a frozen literal. This is the property that makes "I edited the file and it now shows" true for a decided field. *(QA 2.5)*
- **Writeback uses the decided value, ONE atomic batch per file (RD5/P0-4 — NON-NEGOTIABLE)**: with N decided replace fields (+ merge fields' curated sets), a write produces **exactly one** `WriteBatch` invocation for the file (assert call-count = 1 — the per-field-regression guard) carrying the decided values, through the existing F30 durable queue (one `kind=writeback` job per file); the copy→write→rename atomicity (ADR-041) and crash-replay (ADR-048) are unchanged. *(QA 2.6)*
- **Decision mutations are DB-only (RD5)**: `PUT`/`DELETE …/decision` performs **no** file write — no `WriteBatch` call, no `.holodex-tmp`/`.holodex-new`. The file is touched solely by the explicit write action. *(QA 2.7)*
- **Sync recompute**: a field is *out of sync* iff the decided value ≠ the value embedded in the file; after a successful write it reads in-sync **without** mutating the decision (the convergence rule — no silent flip from `provider` to `file`). *(QA 2.9)*
- **Multi-provider (§8)**: the control offers one `Adopt` per **matched** provider (a provider with no value for the field is omitted); two providers disagreeing on a replace field is **self-evident** from the two distinct value chips — the old informational "providers differ" hint was removed in the chip redesign (HOLODEX-112), so only the out-of-sync pill is ever `text-warn`. The `provider_trust_order` config (HOLODEX-118) decides the *undecided* winner **among providers** (first-listed wins; unlisted keep mapping order behind them), with the file layer still ahead of all providers under `default_source: file` and any per-field decision overriding it. Covered by two in-process providers disagreeing on a replace field: (1) first-listed wins when the file is empty; (2) file still wins when present; (3) an unlisted provider ranks behind a listed one; (4) a per-field decision short-circuits the order; (5) absent config keeps mapping order (`internal/resolver` `TestResolve_ProviderTrustOrder_*`). The **per-provider match/enrich/clear UI** (HOLODEX-119) widens the SPA's single-provider assumption (`provider = sources.find(…)`) to a per-provider list on both detail pages — a **frontend-only** change over the already-per-provider backend, so it carries no new Go test; it is verified end-to-end via QA §3.9a–d against two enabled providers (a second `providers/tmdb` sidecar), across all 3 skins. Note the two-tier rule: the Enrich/Clear **buttons** come from the provider **registry** (`/enrich/sources`), while a provider's **chip** additionally requires it to be a configured `source:` for that field in `metadata-mappings.yaml`.
- **API** (`PUT`/`DELETE /media/{id}/fields/{canonical}/decision`): owner-gated (**401/403** without owner), **400** bad source/canonical, **404** unknown id/field, **409** soft-deleted, **200/204** happy path; untrusted `manual_value` sanitized on the same path as F30 manual add. `httptest` over a real repo. *(QA 2.8)*
- **Frontend** (Vitest + a11y): the new `SourceSelect` is a `role="radiogroup"` with `aria-checked` segments and **roving tabindex** (one Tab stop, arrow-key select — cf. F22 `EnrichPicker`); per-segment `aria-label` names the value; the control + candidates + out-of-sync pill are **owner-only** (absent from the DOM for non-owners — the field renders read-only as today); selecting a source shows **no** file-write spinner (DB-only); **all 3 skins** (tokens-only — the `rg 'zinc-|sky-|…'` guard stays empty; the selected-segment treatment and the warn-vs-muted signal separation eyeballed per skin via the `[human]` checklist).
- **RD6 implicit-winner legibility (HOLODEX-245)**: `resolveSelection()` (`web/src/lib/f36.ts`) is the shared core `selectedChipKey`/`isPendingSelection` project from — one walk of the branch, so the two can't independently drift on when an implicit (non-`standing`) winner fires. `f36.test.ts` pins it against **both** payload shapes: the simplified test-fixture convention (`decision` omitted) and the **real API shape** (`decision` always populated, `standing:false` for an implicit winner — the resolver's `replaceMarkers()` never returns a nil marker) after a first pass keyed off `!field.decision` compiled, passed the fixture-shaped tests, and was a no-op against live data, caught only by fetching a real enriched video. `SourceSelect`/`CurationChip`'s `pending` chip variant (dashed ring, hollow dot, `·{provider}, pending`) is presentational only — no change to `needsWriteback`/`in_sync`/the resolver's precedence order; the writeback dialog's decided/undecided split (HOLODEX-213) is unaffected. The writeback dialog's `image_url` row now also renders a read-only file-vs-enriched thumbnail comparison (mirrors the existing `was:` idiom); `matchesFile` was reordered to gate before the `image_url` branch so the "already in file, nothing to write" case applies uniformly across display types. **Live 3-skin QA** confirmed AA+ contrast (text 15.7–18.1:1, dashed border 8.5–16.4:1) against a real TMDB-enriched video.
- **Writeback dialog commits a decision (HOLODEX-273)**: checking a box in `WritebackFormDialog` — individually or via **Select all** on the undecided group — is itself the commit action (the dialog's equivalent of `SourceSelect`'s explicit Confirm), so `submit()` creates a standing `field_source_decisions` row for every newly-checked **replace** field before the file write, closing the gap where a bulk write left the DB reporting the field undecided/RD6-pending even after the value landed in the file. `ensureDecision` is a no-op for a row already `decision.standing` (no redundant `PUT`), for `image_url` display fields (RD5 — a candidate pick there stays `SourceSelect`-only), and for merge/multi fields (RD1 — `Genres`/`Actors`/`Director` keep F30 per-value curation and never carry a source decision, even though the dialog's `fields` prop is the full unfiltered `resolved[]` array and lists them as rows too). An edited row (live value ≠ its pre-edit seed) commits `manual` with the edited value rather than pinning the original winning provider, so the new decision never disagrees with what was actually written. **Live-verified** end to end against a real TMDB-enriched video (`backend-films`, video id 8): select-all over 13 undecided fields → submit → independently `curl`'d `GET /api/v1/media/8` confirmed `overview`/`release_date`/`runtime`/`status`/`original_language`/`homepage` flipped to `{source: "provider:tmdb", standing: true}`, an edited `external_provider_id` landed as `{source: "manual", manual_value: "…", standing: true}`, `genres`/`actors`/`director` stayed `decision: null`, `poster_url` stayed `standing: false`, and an already-decided `tagline` row fired no extra `PUT`. Frontend-only change over the existing owner-gated `PUT .../fields/{canonical}/decision` endpoint (F36) — `decide` in `+page.svelte` calls the same `api.setFieldDecision` `SourceSelect` already uses, with `canonical` sourced from the resolved field (not user-typed) and `manual_value` on the same accepted path as F30 manual add; `/security-review` confirmed no new mutation surface and no findings.
- **Film bulk-attach dialog: default search term + optional starting scene number (HOLODEX-300)**: `FilmBulkAttachDialog.svelte`'s search input now seeds from the film's title (`filmName` prop) instead of starting empty — a one-time seed, not a live prop sync (mirrors `WritebackBatchDialog`'s `initialBatch` pattern; the film name can't change while this dialog is open). The starting-scene-number field is now optional per design handoff §4c ("omitted means every selected video attaches unnumbered"): a nullable `*int64`/`number | null` threads end to end (Svelte state -> `api.ts` -> JSON body -> `bulkAttachFilmVideos` handler -> `BulkAttachFilmVideos`), reusing `insertFilmVideo`'s pre-existing nil-safe "unnumbered, never collides" handling rather than adding new logic -- the nil path also skips the per-video collision query entirely. `TestBulkAttachFilmVideosUnnumbered` (`internal/repo/films_test.go` and `internal/api/films_test.go`) covers the omitted-field path at both layers, alongside the pre-existing `TestBulkAttachFilmVideos` (sequential numbering, mid-batch collision rollback) and `TestFilmVideoSceneCollision`. **Live-verified** against `backend-films`/a real film ("Dune"): default search populates the film title, a blank starting-scene-number attach produces unnumbered (`—`) scene cards, all 3 skins render the changed elements with correct token contrast. Caught by this review pass: `commit()`'s blank-check originally called `.trim()` on `startingSceneNumber`, but `bind:value` on the `type="number"` input coerces it to a `Number` once any digit is typed (documented precedent: `FilmAttachDialog.svelte`'s `confirm()`), so `.trim()` threw and broke the numbered-attach path entirely -- fixed to check `startingSceneNumber !== ''` directly, matching the sibling component.
- **Entity-generic seam (`BaselineSource`, ADR-052 — built)**: the resolver merge core (`ResolveFields`) is parameterized by a `BaselineSource`; `Resolve` is the video wrapper (`NewVideoBaseline` = file layer). A unit drives `ResolveFields` with a **non-video** `BaselineSource` to prove entity-agnosticism, and a guard asserts `Resolve == ResolveFields(NewVideoBaseline(…))` — so the deferred People/Studio fast-follows ride the same tests with a different `BaselineSource`. The decision store + primitive still key by `(entity_type, entity_id, canonical_field)`; video-only is exercised now.

**People on the unified model (F37, fast-follow ② / HOLODEX-10)** — persons through `ResolveFields` + decisions + curation; fully CI-testable, no network (fake provider). Maps to the [F37 QA checklist](design/people-source-of-truth-qa-checklist.md) §2 smoke items; the cardinal invariants are **RD6 additivity**, **name-materializes-not-pins (RD1)**, and **merge cleanup (RD5)**.
- **`personBaseline`** (`internal/resolver`, pure): `name` resolves from the record; every other canonical person field resolves an empty baseline; person fields flow through `ResolveFields` with **zero resolver-core changes** (the ADR-052 seam pays off — reuse the existing non-video-baseline unit shape). *(QA 2.1)*
- **RD6 additivity (cardinal)**: an enriched person with **no** decisions resolves every field to exactly what the raw-enrichment view showed — the refactor changes nothing until the owner decides. Snapshot-style equality test. *(QA 2.2)*
- **Decision short-circuit for persons**: `record` (blank-pin), `provider:<name>` (source-pinned — a re-enrich flows through), and `manual` (frozen literal) each override the default; the person payload vocabulary is `record` (mapped or stored per spec Open Q1 — pin whichever with tests). **No `in_sync`** on person resolved fields; `enriched[]` retired from `GET /people/{id}`. *(QA 2.3, 2.7)*
- **Name is not a decision (RD1)**: `PUT /people/{id}/fields/name/decision` → **400**; the rename path is `POST /people/{id}/rename` — one transaction: `people.name` updated, old name inserted as an F23 alias, FTS matches the old name afterwards; rename onto another person's name → **409** (with id/name/video count) and **no mutation** — never an auto-merge (the F23 homonym rule). *(QA 2.4, 2.5)*
- **Merge cleanup (RD5)**: `mergePersons` drops the duplicate's `field_source_decisions` + `metadata_curation` rows in the merge transaction; the canonical person's rows are untouched (extend the existing F23 merge tests). *(QA 2.6)*
- **API parity**: person decision/curation endpoints mirror the media ones — owner-gated (401/403), 400 bad source/canonical/unmatched-provider, 404 unknown person/field; `manual_value` sanitized on the F30 path. `httptest` over a real repo. *(QA 2.4)*
- **Frontend** (Vitest + a11y): `f36.ts` helpers gain a `baselineKey` param — `record` chips anchor/fold/select exactly as `file` chips do, and the **default keeps the media page byte-identical** (existing tests untouched); `CurationChip` treats `record` as muted baseline provenance; the rename confirm dialog is focus-trapped, Escape returns focus to the opening chip, and activating a non-record name chip fires **no** decision call; no Write button / out-of-sync pill in the person DOM; 3-skin `[human]` eyeball per QA §4. *(QA 2.8, §3–§4)*

**Studio as an entity (F38, fast-follow ③ / HOLODEX-11, ADR-053)** — the third entity on the decision model; the new axis vs. person is **derived links**: `video_studios` follows the *resolved* `studio` field, not raw extraction. Fully CI-testable, no network. Cardinal invariants: **link-follows-resolved-value (RD1)**, **prune-on-empty**, **RD6 additivity**, and **zero resolver-core diffs**. Maps to the [F38 QA checklist](design/studio-entity-handoff.md#qa-checklist-3-skin).
- **`studioBaseline`** (`internal/resolver`, pure): `name` from the record, every other field an empty baseline; RD6 additivity (undecided enrichment resolves to the provider value) and the record blank-pin both hold; asserted with **zero resolver-core changes** (`internal/resolver/studio_baseline_test.go`). *(§2.2)*
- **Derivation matrix — `ReconcileVideoStudios` (repo, sole writer)**: create / idempotent-repeat / replace / empty(blank-pin/soft-delete) each reconcile `video_studios` to exactly the resolved names; a studio shared by two videos is **not** pruned when only one is fixed; a multi-mapped field yields one link per value; empty names dropped (`internal/repo/studios_test.go`). *(derivation matrix + prune-on-empty)*
- **Derivation via real endpoints (RD1)**: `PUT /media/{id}/fields/studio/decision` adopting a provider **moves** the derived link with no rescan (`GET /studios` reflects it); clearing reverts to the file-first value and **prunes** the adopted studio; `?studio_id=` filters media by the link (`internal/api/studios_test.go`). The relink also fires on video enrich apply/clear, refresh, and scan upsert (best-effort; startup backfill is the one-time catch-up, gated on an empty `video_studios`).
- **Studio entity endpoints (RD5)**: `/studios/{id}/fields/{canonical}/decision` + `/curation` mirror the person shapes — `name` → **400** (read-only identity), unknown field → 404, unknown studio → 404, visitor → 401/403; resolved payload carries **no `in_sync`** (studios have no file). Reuse the shared `record` vocabulary (extracted to `record_vocab.go`, exercised by the existing F37 person tests).
- **Frontend** (verified live, 3 skins): `/studios` list + `/studios/{id}` detail (Details hidden until a field beyond `name` resolves), media-detail studio→entity link (target from `video_studios`, always matches the displayed value), search Studios group, nav link; tokens-only (token-guard clean, tokens react across Cinémathèque/Broadcast/Brutalist). *(§3–§4)*

**TMDB company enrichment (HOLODEX-121, F38 S3)** — the studio page's curatable fields; provider-side + registry + endpoint work over the *already entity-generic* enrich service, resolver, and chips (zero core diffs, the F37/RD5 property proven a third time). Untrusted provider company data flows through the **existing** sanitize + host perimeter; **no file writes, no new SSRF/asset surface** (the logo is a plain `image_url` field on the same `image.tmdb.org` host as posters, never downloaded — the F25 image store is not generalized, spec Non-Goal / P2-3). Fully CI-testable, no network (in-process `Fake` studio).
- **Provider (`providers/tmdb/tmdb_test.go`)**: `/describe` advertises `studio` + `description`/`country`/`logo` (and `logo` is **not** an asset kind); `/search/company` resolve maps name + `origin_country` disambiguation (id-path returns confidence 1.0); `/company/{id}` enrich maps `description`/`country`/`website`/`logo`, with `website` falling back to the durable TMDB company page when `homepage` is absent and `logo` omitted when `logo_path` is null; **no `assets[]`**.
- **Service (`internal/enrich`)**: the entity-generic `Enrich` runs studio resolve→enrich→provenance→clear against the `Fake` studio; the `logo` field resolves as `image_url` and never touches the person-image asset-download path (which stays gated to `person`).
- **Endpoints (`internal/api/enrich_test.go`)**: owner-gated `/studios/{id}/enrich/{resolve,,clear}` mirror the person trio — full apply flow surfaces `fake:description` provenance on the studio detail `resolved[]`; a token gate returns 401 without the header, 200 with. No relink on studio-entity enrich (it changes the studio's own fields, not the video→studio links).
- **Registry**: `description` (long_text), `country`, and `logo` (image_url) added; `studioScalarFields` gains `logo`. Operators enable it by adding `studio` to the provider's `entity_types` (the `Supports` gate).

**Self-hosted studio logo (HOLODEX-130, ADR-057)** — **supersedes** the S3 "logo is never downloaded / F25-store-not-generalized" note above (§F38 S3): the studio logo is now a downloaded, normalized, self-hosted **derived cache of the resolved `logo` field**, served from our own origin instead of the hotlinked provider CDN. The scope stays *minimal* (one logo per studio; no upload/gallery/suppression), so it reuses the person normalize spine and the ADR-039 asset perimeter rather than cloning the person-image subsystem. Fully CI-testable, no network (a live `httptest` "CDN" on the provider's base host serves a real JPEG).
- **Disk layout (`internal/studioimage/studioimage_test.go`)**: `ImagePath` builds `{dir}/{studioID}/{id}.jpg` from server-assigned integer ids only (no request-value path component, the ADR-038 traversal invariant); `Store`/`Remove` round-trip atomically (temp+rename, no leftover `.tmp`), and removing an absent file is not an error.
- **Cache index (`internal/repo/studio_logos_test.go`)**: `ReplaceStudioLogo` is single-slot (`UNIQUE(studio_id)` — count stays 1 across replaces) and **advances the id** on each replace (the `?v=` cache-buster); `GetStudioLogo` round-trips the row and 404s when absent; `GetStudio`/`ListStudios` attach `LogoVersion` (the API turns it into the served URL). The prior HOLODEX-126 "lowest-provider-wins over the enrichment field" test is retired — the cache is a single row keyed by studio, tracking the *resolved* logo.
- **Relink + serve + SSRF (`internal/api/studio_logo_test.go`)**: end-to-end `RelinkStudioLogo` fetches through the winning provider's SSRF-guarded client, normalizes, stores, and serves `GET /studios/{id}/logo` as **200 `image/jpeg` + `immutable`** (a decodable image); the `/studios` list carries the **self-hosted served URL**, not the CDN; the relink is **idempotent** (unchanged resolved URL → no re-download, stable id); a **blank-pin** decision (via the owner endpoint, exercising the trigger) clears the cache → 404; a resolved logo URL on an **off-allowlist host is refused** by `FetchAsset` and nothing is cached (the ADR-039 perimeter holds for studios, not just person portraits); a studio with no cached logo serves **404** (the SPA renders the monogram, no placeholder route).
- **Provider unchanged**: TMDB still emits `logo` as an `image_url` **field** (`providers/tmdb/tmdb_test.go` unchanged) — the download is entirely core-side, so there is **no provider/contract change** and no new outbound host beyond the already-allowlisted `image.tmdb.org`.
- **Frontend** (verified live, 3 skins + visitor): `/studios/{id}` Details gains per-provider Enrich/Clear (owner) reusing `EnrichPicker`, and an `image_url` logo branch (img preview + source chip for owner, img + provenance badge for visitor); tokens-only, QA'd across Cinémathèque/Broadcast/Brutalist. *(§3–§4)*

**Provider brand icon (HOLODEX-134, ADR-059)** — the **per-provider** analogue of the self-hosted studio logo: a provider advertises an optional `brand_icon` in its `/describe`, and Holodex downloads, normalizes, and self-hosts it so the SPA can render a provider glyph in place of the repeated "from `<provider>`" provenance text. Unlike the studio logo (a per-entity *field*), the icon is a property of the provider, so it is the first **additive `/describe` change** (a top-level `brand_icon`, no protocol bump) — but the download/normalize/serve/monogram spine is ADR-057 reused verbatim. Triggered at **boot + config-reload only** (a brand mark is static), not per enrich. Fully CI-testable, no network (a live `httptest` provider serves both `/describe` and the icon JPEG on its base host).
- **Disk layout (`internal/providericon/providericon_test.go`)**: `ImagePath` builds `{dir}/{id}.jpg` from the server-assigned row id only — the provider **name never touches the path** (ADR-038 traversal invariant); `Store`/`Remove` round-trip atomically (temp+rename, no leftover `.tmp`); removing an absent file is not an error.
- **Cache index (`internal/repo/provider_icons_test.go`)**: `ReplaceProviderIcon` is single-slot per provider (`UNIQUE(provider)`) and **advances the id** on replace (the `?v=` cache-buster); two providers are independent; `GetProviderIcon` round-trips and 404s (`ErrNotFound`) when absent; `ListProviderIcons` reflects the set; delete is idempotent.
- **Relink + serve + directory + SSRF + reconcile (`internal/api/provider_icon_test.go`)**: end-to-end `RelinkProviderIcon` reads `/describe` via `DescribeProvider`, fetches the `brand_icon` through the SSRF-guarded client, normalizes, stores, and serves `GET /providers/{name}/icon` as **200 `image/jpeg` + `immutable`** (a decodable image); the public `GET /providers` directory carries the **self-hosted served URL** cache-busted by row id; the relink is **idempotent** (unchanged advertised URL → no re-download, stable id); a provider advertising **no `brand_icon`** caches nothing and 404s (directory omits `icon_url` → SPA monogram); a `brand_icon` on an **off-allowlist host is refused** by `FetchAsset`, nothing cached; `RefreshProviderIcons` **prunes the orphan** icon of a provider removed from the registry (no FK cascade).
- **Provider (`providers/tmdb`)**: the TMDB sidecar advertises `brand_icon` only when `TMDB_BRAND_ICON_URL` is set (its own logo is SVG, which the raster ingest rejects) — env-gated so no brittle default ships; the `/describe` struct gains the field (existing `providers/tmdb/tmdb_test.go` still green).
- **Frontend scaffolding** (type-checks clean; live QA lands with the consumers HOLODEX-135/136/137): `EnrichSource` gains `icon_url`; a `providers` store lazily loads the public directory; `ProviderIcon.svelte` renders icon-or-monogram on the shared `bg-logo-plate` (tokens-only, token-guard clean). Not yet mounted on a route — the provenance badge / enrich chip / website label wire it in.

**Studio logos in the list (HOLODEX-126, F38 follow-up)** — surfaces the S3 `logo` in the `/studios` list as a leading well. The list payload gains `logo_url` via **one extra batch query**, leaving the shared `namedCountQuery` untouched and avoiding an N-way per-studio resolve. Fully CI-testable, no network.
- **List payload attach (`ListStudios` / `attachStudioLogos`, repo)**: after the shared list query, a single `SELECT entity_id, value FROM entity_enrichment WHERE entity_type='studio' AND field_key='logo' AND entity_id IN (…)` fills `Studio.LogoURL`; a studio with a `logo` row carries it, a studio with **none** carries empty (not another studio's logo), and when two providers store a logo the **lowest provider name wins** deterministically (`internal/repo/studios_test.go`). Explicitly the **stored** provider logo, not the resolved one (a blank-pinned logo still shows in the list; the detail page stays authoritative) — an accepted, documented list-only edge.
- **Fallback render (frontend, verified live, 3 skins)**: each row opens with a fixed ~40×26 `bg-logo-plate` well — `logo_url` present → real logo (`object-contain`, `alt="{name} logo"`); absent → a monogram (name's first glyph, upper-cased, `text-logo-plate-ink`, `aria-hidden`). Logo rows and monogram rows are the **same height** (the alignment constraint). New `--logo-plate-ink` token; tokens-only (token-guard clean; reacts across Cinémathèque/Broadcast/Brutalist). *(§3.1b)*

**Studio external-id de-dup (HOLODEX-122, ADR-054)** — the deterministic counterpart to name-based aliases; converges same-company spellings by the TMDB company id, fully CI-testable, no network. Cardinal invariants: **external-id-first resolve precedence**, **dedup-by-id across two spellings**, **name fallback when the id is absent**, and **the sidecar never displays/resolves**.
- **Resolve precedence + dedup + back-fill (`resolveOrCreateStudio` / `ReconcileVideoStudios`, repo)**: two videos whose resolved studio names are *different* spellings sharing `tmdb:174` converge to **one** studio (count 2, first-seen name wins, RD6); a name with no id resolves by name only; an id supplied later back-fills onto a name-created studio and a third spelling then converges; **prune-on-empty cascades the `studio_external_ids` row** (proven by re-creating after clearing all links) (`internal/repo/studios_test.go`). *(external-id resolve precedence + dedup + name fallback)*
- **Capture (TMDB provider)**: the movie mapping emits `_studio_external_ids` = `"tmdb:<id> <name>"` per named, positive-id company, aligned with `studio` (`providers/tmdb/tmdb_test.go`).
- **Side-map parse (`studioExternalIDsFromRows`, api, white-box)**: only `_studio_external_ids` rows contribute; `"<id> <name>"` splits on the first space (names keep internal spaces); malformed entries skipped; no rows → nil (name-only) (`internal/api/studios_internal_test.go`).
- **Sidecar is inert to display/resolve**: `FieldsFromRows` skips `InternalFieldPrefix` (`_`) keys so `_studio_external_ids` never reaches the media-detail `enriched[]` list (`internal/enrich/enrich_test.go`); it is not a mapped canonical field, so the resolver ignores it (unchanged `ResolveFields`). **No new provider host/asset/SSRF surface; no media-file write.**
- **Ingest-time shape guard (HOLODEX-258, `sanitizeStudioExternalIDs`, enrich)**: unlike `_person_external_ids` (core-synthesized, F32), `_studio_external_ids` is provider-authored end-to-end as one self-describing `"<id> <name>"` string, so `sanitizeFields`'s generic `SanitizeValue` pass alone never validated that the id token is a well-formed `namespace:id` pair. `sanitizeStudioExternalIDs` runs right after `sanitizeFields` and drops any value whose id token isn't `ns:id` with both halves non-empty — mirrors `sanitizePeople`'s ADR-055 guard for the person channel. Pure-function table test plus an end-to-end fake-provider test proving the call-site wiring drops a malformed value while a well-formed one survives (`internal/enrich/enrich_test.go`).
- **Deferred (tracked)**: the browse **facet** entity-switch (HOLODEX-120: `?studio_id` + a Studios dropzone replacing the legacy `?studio=` string facet) — the legacy `?studio=` string filter stays working meanwhile. *(The TMDB company enrichment slice, HOLODEX-121/S3, has landed — see above.)*

**Universal enrichment unique-key invariant (HOLODEX-123, ADR-055)** — generalizes the studio dedup above
into a cross-entity rule: **every provider record carries a namespaced id `<namespace>:<id>` and it is the
sole identity/de-dup key; no name fallback for provider-supplied identity**, and a namespace is a **shared
identity space** (two providers emitting the same id converge). This ADR is docs-only; the concrete tests
land with the implementation issues and are enumerated there:
- **Perimeter enforcement (HOLODEX-124)** — a `/resolve` candidate, `/enrich`, or `people[]` credit with an
  empty or non-`<namespace>:<id>` id is **refused**; a namespace not advertised in `/describe.id_namespaces`
  is rejected; a single `ParseExternalID` helper is the one place the grammar is enforced (adversarial:
  bare id, empty namespace, embedded space, spoofed namespace).
- **Person id-first identity (HOLODEX-125)** — `person_external_ids(external_id PK)` + external-id-first
  resolve mirrors the studio precedence tests (dedup two spellings by shared id; cross-provider convergence;
  a homonym with a *different* id stays a **separate** person — the collision the name path caused). The
  **owner-curated** name-alias/merge path (F23) stays name-based and is unaffected — its tests are unchanged.
- Note the studio "**name fallback when the id is absent**" (above) is the **owner-decided/custom** studio
  value path (human intent), not a provider-supplied identity — the invariant leaves it intact.

**Owner person & studio media linking + writeback (F40 / HOLODEX-114, ADR-072)** — the fourth entity slice:
the owner *authors* a person/studio link and *persists it to the file*, and `video_people` migrates from
scan-time raw-extraction to **resolved-value derivation** (the studio pattern, generalized). Fully
CI-testable, no network (provider half against the in-process fake). Cardinal invariants: **one writer of
`video_people`**, **lossless cutover** (derivation sources ⊇ extraction sources), **canonical-name
round-trip closes**, **role is derived-from-field, never authored**, and the **person orphan-grace +
authored-identity guard**. Maps to the [F40 design handoff](design/person-media-linking-handoff.md) 3-skin QA.
- **Migration (P0-1)**: `video_people.role TEXT NOT NULL DEFAULT ''`, PK → `(video_id, person_id, role)`,
  `people.orphaned_at TIMESTAMP NULL`; the **down** migration collapses duplicate-role rows to
  `(video_id, person_id)` and drops the columns; **existing user links survive** the up→down→up cycle
  (ties the migration-safety invariant). A person in two roles on one video is **two rows, distinct by
  role**; a role-less link stores `''` and **re-derives stable** (no spurious second row — the
  `NULL`-in-composite-PK hazard the sentinel avoids).
- **Generalized reconcile — `RelinkVideoEntity` (repo, sole writer of `video_people` *and* `video_studios`)**:
  the derivation matrix from F38 is re-run per person-typed field **and** studio — create / idempotent-repeat
  / replace / empty each reconcile links to exactly the resolved names; a person/studio shared by two videos
  is **not** pruned when only one is fixed; a multi-valued field yields one link per value; empty names
  dropped. **Role assignment**: a name resolved from `actors` links `role='actor'`, from `director`
  `role='director'`, from a role-less person-typed field `role=''` (`internal/repo`). *(derivation matrix
  incl. unset role)*
- **Studio regression under the generalization (P0-8, cardinal)**: the **entire existing
  `internal/repo/studios_test.go` derivation + dedup suite passes byte-for-byte** against `RelinkVideoEntity`
  — F38 behavior (link-follows-resolved-value, prune-on-empty, external-id precedence) is unchanged. This is
  the regression guard that lets one function replace two.
- **Single-writer guard (P0-3, cardinal)**: the raw-extraction people branch is **removed** from
  `replaceAssociations` — a repo-level test asserts a scan upsert leaves `video_people` untouched **until**
  the post-commit reconcile runs, and a **CI grep guard** (sibling of the token guard) asserts exactly one
  `INSERT … INTO video_people` site in `internal/repo` (the reconcile). A curated actor not in the file
  **appears** after the curation reconcile with no rescan (the display-vs-truth split is dead for people).
- **Lossless cutover loss-guard (P0-4/RD9, cardinal)**: a fixture library whose people come from `Artist`,
  `Cast`, `Performer`, **and** `Director` tags is migrated + backfilled; **post-backfill active link count
  ≥ pre-migration** (the job records both and **fails loudly** on unexplained shrinkage). Adversarial:
  removing `director` from the marked set (or a `peopleKeys` tag with no mapped field) trips the guard —
  proving it catches a dropped source. Backfill is **idempotent** (second pass = 0 changes) and ordered
  migrate → backfill → serve (the `''` default never surfaces a wrong role).
- **Canonical-name round-trip (P0-7/P0-11, cardinal)**: writeback flattens resolved `actors` → `Artist`
  (comma-delimited) and `studio` → `Publisher` using each entity's **canonical** name; a re-scan
  `splitMulti`-splits `Artist` back and the reconcile re-links **exactly the same** person set — **no
  duplicate person, no "Alice, Bob" single person**, studio re-links to the same entity. Adversarial: a
  linked person whose file spelling was an **alias** is written as the **canonical** name, and the re-scan
  still resolves to the same entity via `resolveOrCreatePerson` (the property that keeps the round-trip
  stable). Reuses the F28 extractor round-trip fixture (`Artist="Audrey Tautou, Mathieu Kassovitz"` → 2).
- **Orphan grace + sweep + authored-identity guard (P0-2/P0-9/RD8 — person only)**: clearing a person's
  last link **stamps `orphaned_at`** (does **not** delete); a link returning **before** the sweep clears the
  stamp. The sweep deletes `orphaned_at < now()−30d` **only** for people with **no authored identity** — an
  orphaned person with an alias / merge history / curated headshot / manual field-edit or decision is
  **kept and reported** past 40 days; a plain orphan is deleted past 30 days, kept before. **Studio keeps
  immediate prune** (the F38 prune-on-empty test is unchanged — the grace is person-only). Enumerate the
  "authored identity" predicate once and assert the sweep and its tests agree (spec Q7).
- **Homonym safety (F23, cardinal)**: linking/deriving a person by a name that collides with a *different*
  real person **never auto-merges** — `resolveOrCreatePerson` name routing is reused, so two same-name people
  stay distinct (the existing F23 collision tests cover the seam; the picker surfaces disambiguation, it does
  not merge).
- **API / owner-gating (P0-6/P0-7/P0-10/P0-11)**: the link picker writes through the **existing** curation
  `add` endpoint (no new route) and writeback through the existing `POST /media/{id}/writeback` — both
  `requireOwner` (**401/403** without owner; **200/202** with). The reconcile fires post-commit on curation
  add/suppress/clear and decision set/clear for a person-typed **or** studio field. Untrusted picker/curation
  values sanitized on the F30 `manual` path; no name injection into the exiftool/mkv write (the
  `/security-review` seam).
- **Frontend** (Vitest + a11y): the new `LinkPicker` is a `role="combobox"` + `role="listbox"` popover with
  **roving tabindex** (cf. `EnrichPicker`), Esc/click-outside close + **focus-return** to "+ Add", a
  persistent **Create "<query>"** row (inline bare-create, RD10), and loading/empty/error `aria-live` status;
  a name matching no entity creates name-only and links in one step; owner-only (**absent from the DOM** for
  non-owners). The **role badge** on the person page tags videos by derived role (set = `text-accent`, unset
  = "Appears in"), two tags when one person holds two roles on one video. **All 3 skins** (tokens-only — the
  `rg 'zinc-|sky-|…'` guard stays empty; the accent left-border active-row and the role tags vs. the
  active-state accent eyeballed per skin via the handoff `[human]` checklist across Cinémathèque / Broadcast
  / Brutalist).

**Owner-mode video editing — poster upload, studio placement, file-metadata gating (F52, HOLODEX-251/HOLODEX-252)** —
rounds out owner-mode video editing alongside F40 in the same branch. Fully CI-testable, no network.
Cardinal invariants: **an uploaded poster is immune to automatic replacement**, and **file metadata
never reaches a non-owner response**.
> **Retired 2026-08-16 (HOLODEX-115)**: F52 originally shipped a bespoke, zero-source `commentary`
> field (an owner note with no mapped sources) alongside this row's other cardinal invariants below.
> The owner determined the file's `Comment` tag should be whatever the *system owner* maps it to via
> `metadata-mappings.yaml` (ADR-013), not a hardcoded application facet — `overview` (TMDB plot
> synopsis) already had exactly that mapping in `internal/writeback/tags.go`'s `formatMap`, and was
> simply stuck read-only in the UI. `commentary` was removed entirely (`internal/registry/registry.go`,
> the local mapping YAMLs, and `+page.svelte`'s dedicated section/derived/filter clause), and the
> generic Metadata `dl`'s `long_text` branch was extended to render `SourceBadge` for any long_text
> replace field an operator maps — `overview` picks this up automatically, same as any other replace
> field. The **mechanism** the two bullets below originally described (a zero-source mapping field
> loads and decisions round-trip generically) is still real and still covered — see
> `TestDecisionAPI_OverviewReplaceField` (`internal/api/decisions_test.go`) for the current exemplar
> (manual override + adopt-provider + clear, on `overview`) — only the concrete `commentary` field
> itself is gone. `registry_test.go`'s `TestCriticality` and `resolver/complete_test.go`'s
> `TestComplete_AllExcludedYieldsZeroScore` swapped their zero-criticality exemplar to `deathdate`.
- **Zero-source mapping field (P0-2)**: `mapping.parse` no longer skips a YAML field entry whose
  `sources` list is empty — only a blank `canonical` is dropped. A field entry with no `sources:`
  loads; `GET /media/{id}` omits that field's row until a manual decision exists, then includes it
  with `candidates: []` (no file/provider candidates ever compete) — `internal/mapping`,
  `internal/resolver`. Regression: every existing multi-source field's precedence is unchanged
  (existing mapping tests re-run clean).
- **Replace-field decision round-trip generalizes to long_text (HOLODEX-115)**:
  `PUT/DELETE /media/{id}/fields/{canonical}/decision` behaves identically for a `long_text`
  replace field (`overview`) as for a plain-text one (`title`/`studio`) — owner-gated, sanitized
  manual value, no file write at decision time — via `TestDecisionAPI_OverviewReplaceField`, which
  reuses the same `setFieldDecision`/`clearFieldDecision`-shaped harness `title`/`studio` already use.
- **Poster upload tier (P0-5/P0-6, cardinal)**: `model.ThumbnailUploaded` is a new terminal state;
  `POST /media/{id}/poster` (multipart, size-capped, decode+normalize, mirrors the
  `person_images.go` upload test shape) writes to the existing `thumbnail.ThumbPath` and sets state
  `uploaded`; an oversized/undecodable image → 400, no partial write; storage unconfigured → 503.
- **Uploaded posters are sweep-immune (P0-8, cardinal, adversarial)**: the startup-sweep query
  (`thumbnail_state IS NULL OR = 'failed'`, `internal/repo/repo.go`) is asserted **not** to match
  `'uploaded'` — a fixture library with an uploaded poster, after a full sweep + rescan cycle, has an
  **unchanged** thumbnail file (byte-identical). Adversarial: a test asserting the sweep query's `IN`/`OR`
  clause explicitly excludes `uploaded` fails loudly if a future edit widens the clause.
- **Revert (P0-7)**: `DELETE /media/{id}/poster` removes the uploaded file, resets state, and
  triggers the same extract/enqueue path `regenerateThumbnail` already uses — a subsequent
  `GET` serves the file-derived poster again, not a 404.
- **File-metadata gating (P0-11, cardinal, data-exposure)**: not just a frontend toggle — verified at
  the point a visitor (no session / non-owner) actually can't observe codec/container/bitrate/path
  data through any response path the page uses. If that data already ships in the `GET /media/{id}`
  JSON payload regardless of caller (gated only by frontend render), this is a **security-review
  finding**, not a passing test — flag explicitly rather than assume the UI gate is sufficient.
- **Studio-near-title relocation (P0-10)**: pure template/layout change — the resolved `studio`
  field's value, decision control, and entity link are unchanged in substance; a Vitest render check
  confirms the studio row is a sibling of `<h1>`, not a descendant of the metadata `dl`, and that it
  is excluded from the generic canonical-field loop (no duplicate render).
- **Frontend** (manual driven-browser QA, no automated harness — §0/§11's standing gap): the generic
  Metadata `dl`'s `long_text` branch renders `SourceBadge` (not a read-only paragraph) for a `long_text`
  replace field when `isOwner`, matching the pattern the row's other display branches already used
  (HOLODEX-115) — verified end-to-end on `overview` against `backend-films`: expand/stage/confirm via
  `SourceBadge`, `PUT /media/{id}/fields/overview/decision`, "Write decisions to file" →
  `mkvpropedit`-written `Comment` tag confirmed via `exiftool -Comment`; the poster upload trigger is a
  labeled button with a visually-hidden file input (never `display:none`), disabled + spinning while in
  flight; the Remove control appears only when `thumbnail_state === 'uploaded'`. Tokens-only (`rg
  'zinc-|sky-|#'` clean);
  **all 3 skins**.

**Provider render hints + non-canonical field auto-registration (F39, HOLODEX-128, ADR-056)** — a provider
advertises per-field render hints in `/describe.field_hints`; Holodex persists them and **auto-registers**
any stored **non-canonical** shadow field as a **display-only** row on video/person/studio, with zero
per-operator mapping config. Fully CI-testable, **no network** (in-process fake advertising hints). Cardinal
invariants: **the four-tier ladder** (operator mapping > code registry > provider hint > title-case; a hint
never shadows a canonical or `_`-key), **presence-driven** (no value → no row), **display-only** (no
decision/curation coupling), **`image_url` allowlist-gated**, and **backward-compat** (no hints/values →
byte-identical to pre-F39). Maps to the [F39 QA checklist](design/provider-render-hints-qa-checklist.md).
- **Contract decode (`internal/enrich`)**: `Manifest.FieldHints` parses a `field_hints` map; a manifest with
  **no** `field_hints` decodes unchanged; hints are sanitized/validated on ingest — over-long `label` capped
  and control-chars stripped, unknown `render`→`text`, unknown `group`→`extended`; a hint on a canonical key
  or a `_`-prefixed key is dropped. *(QA 2.1)*
- **Ladder precedence (pure)**: a small overlay resolves `(label,render,order)` for a key top-down (mapping ▸
  registry ▸ persisted hint ▸ title-case); a provider hint governs **only** a key the code registry does not
  define; canonical keys ignore hints. *(QA 2.2)*
- **Persistence (`provider_field_hints`, repo)**: reading `/describe` in an owner action replaces that
  provider's rows in one write txn (delete-where-provider + insert) under `writeMu`; the read path resolves
  hints from the table with **no** provider call (migration `00NN` up/down applies cleanly). *(QA 2.3)*
- **Auto-registration predicate + ordering (rides `ResolveFields`, all 3 entities)**: a shadow key is
  surfaced iff present **and** non-`_` **and** non-canonical **and** not already mapped/synthesized; an
  unmapped canonical key and a `_studio_external_ids` row are **not** surfaced; the presence gate drops
  valueless keys; auto-registered fields sort after canonical ones by (group, order, key). Extends the
  ADR-052 non-video-baseline unit to prove the append works for video, person, and studio with the resolver
  core unchanged. *(QA 2.4, 2.5, 2.6, 2.7)*
- **`mapping.Field.Display` propagation**: `ResolveFields` sets `Display = f.Display` if non-empty else
  `registry.Lookup(...).Display`; empty `Display` reproduces today's output (regression guard). *(QA 2.9)*
- **Security — `image_url` gate + display-only**: a hinted `image_url` value on an allowlisted host
  (ADR-039 `asset_hosts` / `base_url`) resolves as an image; a non-allowlisted host resolves as `text` (the
  field is marked non-image), so a provider cannot make the browser beacon an arbitrary host; `url` non-http
  → text; an auto-registered field carries **no** `Decision`/`Candidates`/curation `Items`, and the
  decision/curation endpoints reject its (unmapped) canonical key. *(QA 2.8, 2.10)*
- **Backward-compat golden (cardinal)**: a provider with no `field_hints` and an entity with no non-canonical
  values → resolved output **byte-identical** to pre-F39 (snapshot equality). *(QA 2.11)*
- **Frontend** (Vitest + a11y): a read-only `ChipValueList` renders one `border-rule` pill per value (no
  ✕/＋); the auto-registered read-only branch switches on `display` (text/long_text/chips/url/image_url) and
  shows a single `ProvenanceBadge`, **no** owner controls for owner or visitor; the "Additional details"
  group + divider render only when ≥1 field is present; tokens-only, QA'd across **all 3 skins** (token-guard
  clean; badge-vs-chip collision eyeballed per skin). *(QA 2.12, §3–§4)*
- **Promotion path (live QA)**: adding a `metadata-mappings.yaml` entry for an auto-registered key +
  reload-config moves it into the curatable set with source chips — the provider hint no longer governs it
  (frontend-observable; no new Go test beyond the ladder unit). *(QA 3.6)*

**Unified entity name-identity (F43, ADR-061)** — the **name** companion to ADR-055's provider-id invariant:
one per-entity normalized `nameKey` (unique across canonical ∪ aliases) + a shared alias/merge/rename/
keep-separate spine across **person / studio / tag**. Fixes the `"fox"`/`"Fox"` split and gives studios/tags
the merge+alias people already have. Generalizes F23's tests to three entities; fully CI-testable, no network.
Maps to the [F43 QA checklist](design/entity-identity-qa-checklist.md). Cardinal invariants (§4): **case/whitespace
never forks identity**, **studio merge survives re-derivation**, **backfill auto-folds only the provably-safe**,
**kept-separate never nags**, **id→name never cross**. Production probe: **14 hard pure-case pairs, 56 near-misses (41 tags)**.
- **Name-identity core (`resolveOrCreateByName` + a per-entity `normalize` registry, repo)**: resolve order
  **id→name→create** (external-id first per ADR-054/055, then `nameKey` over canonical ∪ aliases, then create);
  a `nameKey` owned by a **canonical name or an alias** routes to the same entity. **Per-entity normalize**:
  person/studio fold **case + edge whitespace** only, tag folds **case + all whitespace** — table-driven cases
  assert `"fox"≡"Fox"≡" fox "` for all three, `"sci fi"≡"scifi"` for tag, and `"Mary Jane"≢"MaryJane"` for
  person/studio. A change to a `normalize` fn is treated as a migration (index rebuild).
- **Collision matrix — all three modes × scan/editor × entity** (`internal/repo` + `internal/api`): (1)
  **canonical↔canonical case** — scan routes silently to the existing row (no second entity); an editor rename
  onto another entity's `nameKey` → **409**, no mutation. (2) **canonical↔alias** — adding an alias equal to
  another entity's canonical name → **409**; scanning that string routes to the alias's canonical. (3)
  **alias↔alias** — two entities cannot both own the same alias `nameKey` (unique per type). No path auto-merges
  (the homonym rule). Parameterized across person/studio/tag off a shared table.
- **Merge (per entity, shared helper)**: de-duped association union; decisions/curation/enrichment **moved not
  dropped** where non-conflicting (survivor's win on conflict); loser's name → alias; loser deleted;
  self-merge/unknown-id → 400/404. **Derived-link entities (studio, and person under ADR-072)**: after the
  merge, a derivation pass (`RelinkVideoStudios`, or the generic `RelinkVideoEntity` for person — scan/enrich/
  decision) does **not** recreate the loser — the registered alias re-routes it (the cardinal re-derivation
  invariant). When F43 and ADR-072/F40 both land, the person case runs against `RelinkVideoEntity`; until then it
  is the re-scan case (F23 raw-extraction).
- **One-time backfill (`internal/repo` + a `cmd/holodex` job, ADR-028)**: over the current library it auto-folds
  the pure-case hard pairs (survivor = lower id, move-not-drop) and inserts `identity_review_queue` rows for the
  near-misses — **never** merging a near-miss; recorded as one observable job run (no path/secret in `detail`);
  **idempotent** (second pass folds/queues nothing); the fold ordering lets the unique `nameKey` index build clean.
- **Review queue + keep-separate (`internal/repo` + `internal/api`)**: the loose-key detector flags a pair only
  when it is a fuzzy near-miss (different `nameKey`), excluding exact matches (already collapsed) **and**
  `entity_keep_separate` pairs; scan-time flagging is idempotent and inside the scan transaction (no prompt, no
  merge). `GET /owner/duplicates` returns pairs grouped by type with counts; `dismiss` writes keep-separate and
  the pair **never re-surfaces** on a subsequent scan/detector pass.
- **Search / FTS parity (RD11)**: `person_aliases` migrates into the shared `entity_aliases` + `entity_aliases_fts`
  (entity_type-filtered); the **F23 search-by-alias behavior is byte-identical** pre/post (existing F23 search
  tests pass unmodified) before the old table is dropped; **studio + tag** aliases now surface their entity in
  global search (diacritic-folded, deduped with name matches, per-group limit) — the F23.5 guarantee, three entities.
- **Person conformance (RD11)**: the person `nameKey` becomes case-insensitive; all F23 endpoints/scan-routing/
  search behavior are preserved (the F23 suite is the regression guard); the 6 person hard pairs fold in the backfill.
- **Endpoints**: `alias add/delete`, `merge`, `rename` for **studio + tag** mirror the person shapes — owner-gated
  (**401**), **400** invalid/self-merge/empty, **404** unknown, **409** cross-entity name collision; `GET
  /owner/duplicates` + `POST …/dismiss` owner-gated. `httptest` over a real repo, parameterized by entity.
- **Identity is DB-only (no media-file writes)**: no alias/merge/rename/dismiss path issues a `WriteBatch` or
  touches `.holodex-tmp`/`.holodex-new`; zero `/writeback` calls on any identity surface (the F37/ADR-053 precedent,
  asserted).
- **Frontend** (Vitest + a11y, then live 3-skin): `AliasPanel` on person + studio (not tag); `EntityPicker`
  (generalized `PersonPicker`, roving-tabindex/focus-trap/Esc/focus-return); `/tags` **manage mode** (selectable
  pills + per-pill rename/alias/merge menu); the **exact-collision card** (bordered, blocking) vs the **near-miss
  soft-warning** (muted, non-blocking, "create anyway"→keep-separate) render distinctly; the "N possible duplicates"
  **banner** (`--warn`, owner-only) deep-links the tab; the `/owner/duplicates` tab (Option-A dense rows, tags
  first, Merge/Keep-separate). Owner controls **absent from the DOM** for non-owners; tokens-only (`rg` guard empty);
  the `--warn`/`--accent` separation holds on **Brutalist**; **all 3 skins**.

**In-app promote / override affordance (F44, HOLODEX-171, ADR-062)** — an owner-gated, DB-backed **tier-0**
override (`field_promotions`) that materializes an auto-registered (F39) non-canonical field into a synthetic
`mapping.Field`, making it a first-class **curatable** field via the *existing* F36 decision + F30 curation code
paths, on all three entities — with **zero** `metadata-mappings.yaml` editing. Rides F39's auto-registration and
F36/F30's resolver seams, so the diff is a synthetic field + a tier-0 fold, not a new editing model. Fully
CI-testable, **no network** (in-process fake advertising the non-canonical values). Cardinal invariants (§4):
**renders exactly once**, **presentation-global / curation-per-entity**, **de-/re-promote never loses curation**,
**tier-0 outranks YAML but never a canonical key**, and **zero-impact when unused**. Maps to the
[F44 QA checklist](design/promote-override-fields-qa-checklist.md) §2 smoke items.
- **Store (`field_promotions`, `internal/repo/promotions.go`)**: `SetPromotion` upserts a
  `(entity_type, field_key)` row under `writeMu` (stamping `created_at`/`updated_at` in `timeLayout`); a second
  `SetPromotion` on the same key updates in place (no duplicate row); `ClearPromotion` deletes and is a **no-op on
  a missing row**; `PromotionsForEntityType` returns only that type's rows. Mirrors `decisions.go`. Migration
  `0023_field_promotions` up/down applies cleanly and **preserves user promotions** across the round-trip (ties
  the migration-safety invariant). *(QA 2.1)*
- **Ladder tier-0 (pure)**: for a promoted key `(label,render,group,order)` resolves **promotion ▸ operator YAML ▸
  registry ▸ provider hint ▸ title-case** — first tier with an answer wins; a promotion's **empty** presentation
  column falls through to the lower tiers **for that column only** (empty `label` inherits the hint/title-case
  label while a non-empty `render` still wins). Extends the F39 four-tier overlay with tier-0. *(QA 2.2)*
- **Tier-0 outranks YAML (D3, cardinal)**: a **video** key that has **both** a `metadata-mappings.yaml` mapping
  **and** a promotion resolves the promotion's label/render/order **and** curatable status; the promotion
  `mapping.Field` **replaces** the YAML one on the `canonical` collision, so the field renders **once**. Every
  un-promoted key resolves exactly as ADR-056 defines (regression guard). *(QA 2.3)*
- **Non-canonical guard (cardinal)**: `SetPromotion` and the `PUT` handler **reject** a canonical key
  (`registry.IsKnown ⇒ 422`) and a `_`-prefixed key (`⇒ 422`); no row is created. The registry/schema contract
  and reserved sidecar keys stay inviolate — you cannot promote `bio` or `_studio_external_ids`. *(QA 2.4)*
- **Materialization + candidate derivation (D-candidate)**: a promoted key produces a synthetic
  `mapping.Field{Canonical: key, Filterable: false, Multi: render=="chips"}` whose **`ParsedSources` = one
  `provider:<ns>` per namespace present** for `(entity_type, entity_id, field_key)` in that entity's shadow rows
  (union across providers); the **baseline** (`file`/intrinsic) is always a candidate (empty for a provider-only
  person/studio key is fine); **`manual` is always available**. Candidates are entity-specific even though the
  promotion row is global — computed at resolve assembly from the already-loaded `Enrichment` map, so purity
  holds (no new per-field query). *(QA 2.5)*
- **Decision/curation attach + auto-registration yields (F36/F30/FR3)**: after materialization, `ResolveFields`
  attaches `Decision`/`Candidates`/`InSync` to a **scalar** promoted field and per-value union + F30 curation
  items to a **`chips`** promoted field — via the existing code paths, with **no merge-core change**;
  `ResolvedField.AutoRegistered == false`; and `AutoRegisterFields` **excludes** the now-mapped key with no new
  predicate, so it renders once, not doubled. Extend the ADR-052 non-video-baseline unit to prove it for video,
  person, and studio. *(QA 2.6, 2.7)*
- **Presentation-global / curation-per-entity (cardinal)**: a `field_source_decisions` / `metadata_curation` row
  on a promoted key for entity **A** does **not** affect entity **B**, while the label/render/order from the
  global promotion is shared. *(QA 2.8)*
- **De-/re-promote survives curation (cardinal)**: `ClearPromotion` reverts the key to an auto-registered row
  (`AutoRegistered == true`, no `Decision`); prior decision/curation rows persist (keyed by `field_key`,
  independent of the promotion row) and **re-apply** on a subsequent `SetPromotion`; the shadow value is never
  touched. *(QA 2.9)*
- **API (`PUT`/`DELETE`/`GET /admin/field-promotions/{entity_type}[/{field_key}]`)**: owner-gated (**401** unauth,
  before the handler); `PUT` → 204 upsert (body `label?/render?/group?/order?`), `DELETE` → 204 **idempotent**,
  `GET` lists a type's rows; `entity_type ∉ {video,person,studio}` → 4xx; canonical/`_` key → 422;
  `render`/`group` coerced to the F39 vocabulary; `label` control-char-stripped + capped 64 (reuse the F39 ingest
  sanitizer). A **type-global** route (not `/{entity}/{id}/fields/...`) — the URL must not imply per-entity scope.
  `httptest` over a real repo, mirroring `person_decisions.go`. *(QA 2.10, 2.12)*
- **Security (owner-authored config the resolver trusts)**: owner-supplied `label`/`render`/`group` are still
  **sanitized/coerced on ingest** (defense in depth — over-long label capped, control-chars stripped, unknown
  `render`→`text`), labels render as **escaped text** (Svelte, never HTML); a promoted **`image_url`** value on a
  **non-allowlisted** host resolves as `text` not `<img>` (the ADR-039 asset perimeter is **not** widened by
  promotion); `Filterable` is unconditionally **false**, so a promoted, provider-valued field never enters the
  browse facet / query-param path (no unvalidated-value-into-browse surface). `/security-review` before merge.
  *(QA 2.10, 2.11)*
- **Backward-compat golden (cardinal)**: with **no** promotions, resolved output + rendering is **byte-identical**
  to pre-F44 (the F39 baseline snapshot), on all three entities. *(QA 2.13)*
- **Frontend** (Vitest + a11y): `AutoFieldRows.svelte` gains `isOwner` — the **Promote** control renders only for
  the owner (**absent from the DOM** for a visitor); the inline editor emits the correct `PUT` body
  (`label?/render?/group?/order?`) and `DELETE` on **Remove promotion**; the render `<select>` offers exactly
  `text|long_text|chips|url|image_url`; after promotion the field leaves `extraFields` and renders through
  `SourceSelect`/`CurationFieldRow` **for free** via the existing `!f.auto_registered` partition; the editor is an
  **inline expander** (no popover/focus-trap machinery — DD1) that traps nothing but returns focus to the opening
  button on Esc/close; tokens-only (the `rg 'zinc-|sky-|…'` guard stays empty). **Live 3-skin visual QA**
  (Cinémathèque/Broadcast/Brutalist) per the QA checklist §4 `[human]` items — the Promote pill vs. the provider
  badge, the accent-outlined editor box, the `--warn` Remove vs. accent Save separation, and the auto→mapped
  partition move — remains the manual gate (needs a running backend + provider + owner session). *(QA 2.14, §3–§4)*

**Derived / computed person fields (F45, HOLODEX-73, ADR-063)** — a new **field-genre** (computed, source-less,
read-only) appended by a **pure `Derive(resolved, now)` post-pass** over `[]ResolvedField`, driven by a **closed Go
formula registry**, stamped with a **non-adoptable `computed:` provenance token**, with the **clock injected at the API
handler boundary** so the resolver stays pure. Seeded with `age` (`now − birthdate`) and `age_at_death`
(`deathdate − birthdate`) on Person. Rides F39's auto-registration append shape (ADR-056) and the ADR-052 entity-generic
core, so the diff is *a pass + two registry fields + a provenance token*, not a new editing model. **No migration, no
stored column, no auth/access/infra change** (`/security-review` N/A — recorded on the gate). Fully CI-testable, **no
network** (fixed `now` injected; birthdate/deathdate come from the enrichment shadow store or an in-memory
`[]ResolvedField`). Cardinal invariants (§4): **resolver stays clock-free**, **computed is never adoptable**,
**age / age-at-death are mutually exclusive**, **missing/unparseable input → no row for everyone**, and
**compute-on-read, never stored**. Maps to the [F45 QA checklist](design/derived-person-fields-qa-checklist.md) §2 smoke
items; the tests land with the implementation.
- **Registry genre marker (`registry.FieldDef.Computed`/`DependsOn`, `internal/registry`)**: `age`
  (`DependsOn:["birthdate"]`) and `age_at_death` (`DependsOn:["birthdate","deathdate"]`) join `KnownFields`, both
  `Computed:true` with labels "Age" / "Age at death"; `Lookup` returns them so label/display resolution and the SPA
  field switch already know the canonicals; a `Computed` predicate is available without a second registry. Every existing
  non-computed field keeps `Computed:false` (regression). *(QA 2.5)*
- **Formula registry — `deriveAge` / `deriveAgeAtDeath` (`internal/resolver`, pure, table-driven)**: `deriveAge` =
  `floor(now − birthdate)` whole years for a present, parseable ISO `birthdate`; `computable=false` (⇒ no row) when
  `birthdate` is **absent or unparseable**; **`deathdate` present ⇒ `deriveAge` yields no row** (age-at-death takes over).
  `deriveAgeAtDeath` = `floor(deathdate − birthdate)` requiring **both** inputs, `computable=false` if either is
  missing/unparseable, with `floor` correctness pinned at exact-anniversary boundaries. A **fixed `now`** makes both
  deterministic (no wall-clock flake). *(QA 2.1, 2.2)*
- **Leap-day convention (AC-9)**: `birthdate = 2000-02-29`, computed on `2026-02-28` vs `2026-03-01`, crosses the
  birthday **exactly once** — the documented convention asserted with a fixed `now`, so a future formula tweak that
  regresses the boundary is caught. *(QA 2.3)*
- **`Derive(resolved, now)` pure pass**: fixed `now` in ⇒ deterministic rows out, **no I/O, no package-level clock**;
  each emitted row is stamped `Computed:true`, `WinningSource == fieldsource.ForComputed(canonical)`
  (`"computed:<canonical>"`), registry `Label`/`Display`, and **nil** `Decision`/`Candidates`/`InSync` (structurally
  non-adoptable, like an auto-registered row); **mutual exclusion** — a person with both inputs yields **exactly one**
  row (`age_at_death`, never both), a living person exactly `age`; **ordering** — the computed row is positioned
  **immediately after `birthdate`** in `resolved[]` (adjacency is a payload guarantee, FR5). Extends the ADR-052
  non-video-baseline unit so the pass is proven entity-generic even though only Person seeds a formula today.
  *(QA 2.4, 2.5, 2.6, 2.7)*
- **Resolver purity guard (AC-8, cardinal)**: a **CI grep guard** (sibling of the token / single-writer guards) asserts
  `internal/resolver/` contains **no** `time.Now` and no package-level clock — `Derive` takes `now` only as a parameter.
  Break this and the resolver's determinism (load-bearing for ADR-051) silently rots. *(QA 2.4)*
- **`computed:` provenance token (`internal/fieldsource`, ADR-063 §D3)**: `Computed` const + `ForComputed(canonical)`
  formatter + `IsComputed` recognizer; the token is **deliberately kept out of `Valid()`/`ForNamespace()`** —
  `Valid("computed:age") == false` and `computed` is **absent** from `ForNamespace` — because it is not an adoptable
  decision source. A single definition backs both the SPA badge detection and the API guard (no drifting string
  literals). *(QA 2.8)*
- **Non-adoptable API guard (cardinal)**: a decision `PUT`/`POST` naming a `Computed` canonical (`age`/`age_at_death`)
  **or** any `computed:` source is **rejected 400** and **never** written to `field_source_decisions` — non-adoptability
  is enforced structurally (nil `Decision`) **and** at the endpoint, on person (and any future entity that seeds a
  computed field). *(QA 2.8, §3.8)*
- **API integration (`personResolved`, `internal/api`)**: with the `Handlers.now` seam set to a fixed clock,
  `personResolved` chains `Derive(resolved, h.now())` after `appendAutoRegistered` and emits the derived row(s) in
  `resolved[]` for a **birthdate-bearing** person, **omits** them otherwise; **owner and visitor payloads are identical**
  (D3 — no owner-only branch); the row carries `computed: true`, `winning_source: "computed:age"`,
  `derived_from` = dependency **labels** (e.g. `["Birthdate"]`), and **no** `decision`/`candidates`/`in_sync`.
  **Time-varying, never stored (AC-2)**: advancing the injected `now` past the next birthday **increments** the rendered
  Age with **no** DB write and **no** migration/column touched. *(QA 2.9, 2.10)*
- **Backward-compat golden (cardinal)**: a person with **no** birthdate (or with the computed fields un-registered)
  resolves + renders **byte-identical** to pre-F45 — the engine is zero-impact until an input is present. *(QA 2.12)*
- **Frontend** (Vitest + a11y): `providerFromWinningSource("computed:age") === ""` (the handoff §3 gotcha guard — no
  phantom "C" provider bubble); `calculatedFrom(["Born"]) === "calculated from Born"` / `calculatedFrom(["Born","Died"])
  === "calculated from Born and Died"`; the person page's compact field loop renders a computed row **read-only** with
  the phrase on the value's `title` + `aria-label` and **no** icon/badge — **no** `SourceSelect`, **no** promote pill,
  **no** Custom entry — identically for owner and visitor; the row sits directly under **Born** in the primary `<dl>`.
  **Live visual QA** (Cinémathèque/Broadcast/Brutalist) per the QA checklist §4 `[human]` items — the bare-number Age
  line under Born, the value hover-tooltip, and the tab-skip / screen-reader behavior — remains the manual gate
  (tokens-only; the `rg 'zinc-|sky-|…'` guard stays empty; the row adds no skin-dependent styling). *(QA 2.11, §3–§4)*

**Enrichment review workflow (F47, ADR-066, HOLODEX-186)** — a review queue + confidence-routed
auto-apply + durable "not matched" verdict + refresh bypass over the **already-shipped** F22/F36/F37/F38
enrichment machinery. **This PR (spec + ADR-066 + design handoff) is docs-only — S1–S4 are unbuilt**, so
the rows above (§4 backend, §5 frontend) and the invariants above are the *target*, not yet exercised.
Fully CI-testable, no network (fake provider, per the F22 precedent). Cardinal invariants: **a dismissal
is as durable as an acceptance**, **auto-apply never invents a new confidence model**, **Refresh never
re-searches** (all three above, §"Critical invariants"). Maps to the [design handoff's QA
checklist](design/enrichment-review-workflow-handoff.md#qa-checklist-3-skin) §2 smoke items; the F43
Duplicates-tab suite (`internal/api/duplicates_test.go`, `owner/duplicates/+page.svelte` tests) is the
direct structural precedent for both the queue endpoint and the tab's Vitest/Playwright coverage.
- **S1 — data model + endpoints** (`enrichment_dismissals` migration, dismiss/undismiss/refresh/
  refresh-all routes per entity type): migration up/down round-trip + cascade-on-entity-delete; the four
  new mutations mirror the existing `enrich/resolve`·`enrich`·`enrich/{provider}` route shape
  (`/people|studios|media/{id}/enrich/{provider}/dismiss` etc., `internal/api/enrich.go`'s `mountEnrich`
  pattern) and are `requireOwner`-gated identically (401 without token) — parameterized across all three
  entity types off one shared table, the same pattern F43's endpoints used for person/studio/tag.
- **S2 — `GET /owner/enrich-queue` + Enrichment tab**: the zero-cost membership query (excludes dismissed
  pairs, no `/resolve` calls); grouping/ordering per the design handoff's resolved Q3 (People → Studios →
  Media, actionable-first within a group); resolve-in-place on row click (no full refetch), matching
  `owner/duplicates/+page.svelte`'s `$state`/`$derived`/`$effect` shape.
- **S3 — `EnrichPicker` additions**: auto-apply threshold boundary (exactly one `>=0.85` vs. two-or-more —
  table-driven, pin the `0/1/2` candidate-count cases); dismissal write-then-close-then-`ondismissed`;
  `profile_url` scheme validation (server-side allowlist of `http`/`https`, adversarial `javascript:`/
  `data:`/malformed inputs — same posture as the rest of the provider contract, ADR-033 §6).
- **S4 — `EnrichProviderChips`**: linked-vs-unlinked primary-action branch (needs the `external_id` on
  hand — via whichever prop shape the implementer picks, per the handoff's open question 1); Refresh-all's
  partial-failure fan-out (one provider erroring/ambiguous must not abort or hide the others' results).
- **Security** (`/security-review`, spec Timeline step 4): the new dismiss/undismiss/refresh/refresh-all
  endpoints get the same `requireOwner` check as every existing enrich mutation — no new ungated surface;
  `profile_url` is the one new externally-influenced input (provider-supplied), covered by the
  scheme-validation test above — it is a plain link, never fetched server-side, so it is not a new
  asset-download/SSRF surface.
- **Known test-scope gap (spec Open Question Q1)**: Person's `external_id` isn't yet load-bearing for
  identity (ADR-055 gap, HOLODEX-125) — Refresh/Refresh-all tests for **Person** should either be scoped
  out until HOLODEX-125 lands, or explicitly assert the graceful-failure path if shipped ahead of it.
  Confirm scope before writing the S4 Person test cases (mirrors the design handoff's own open question 3).

**On-demand metadata extraction (F48, ADR-067, HOLODEX-191/192)** — filename parsing as a new
resolver source, extraction confidence-gated auto-apply/review-queue routing, merge→writeback
propagation, and snapshot-based rollback — generalizing F47/ADR-066's auto-apply pattern to a
**second, local-only** candidate source with a different (weighted, multi-component) scoring model
and a hard exact-match gate provider matching never needed. **This PR (spec + ADR-067 + design
handoff + this testing-strategy update) is docs-only — F48.1–F48.11 are unbuilt**, so the rows
above (§4/§5) and the Critical invariants above are the *target*, not yet exercised. Fully
CI-testable, no network — extraction is pure local parsing, and the F30 write queue / F43 loose-key
detector are already-shipped dependencies this spec reuses rather than reimplements. Cardinal
invariants: **the exact-match gate is never bypassed**, **a manual edit is a one-time-import
boundary**, **a revert is byte-for-byte and revertible itself**, **merge propagation never touches
the filename** (all four above, §"Critical invariants"). Maps to the [design handoff's QA
checklist](design/metadata-extraction-qa-checklist.md) §2 smoke items; F47's enrich-queue endpoint
+ `owner/enrichment/+page.svelte` suite is the direct structural precedent for both the queue
endpoint and this tab's Vitest/Playwright coverage.
- **Phase 1 — filename parsing + `filename:` shadow source** (F48.1/F48.2): pure token-grammar
  compilation and matching, no I/O, no auto-apply yet — table-driven over the fixture filenames ×
  patterns matrix in the spec's [Concepts & Model](specs/metadata-extraction.md#concepts--model).
- **Phase 2 — confidence scoring + routing** (F48.3/F48.4): the architectural risk lives here per
  the spec's own phasing note — ships behind a flag with auto-apply disabled (log-only) until
  ADR-067 lands (Action Item 2); tests should cover both the flag-off (log-only, zero writes) and
  flag-on paths, not just the scoring math in isolation.
- **Phase 3 — rollback foundation** (F48.9): snapshot-on-write lands *before* auto-apply is
  enabled (ADR-067 Action Item 3) — migrations 0025 (`metadata_extraction_review`)/0026
  (`file_writeback_snapshots`) up/down round-trip, `batch_id` grouping, and the byte-for-byte
  revert invariant above.
- **Phase 4 — triggers** (F48.5): on-demand first, then batch, then import-time, in the spec's own
  increasing-blast-radius order. The one-code-path regression guard (F48.5d) is the highest-value
  test in this phase — assert the shared extraction function's behavior once, then assert each of
  the three entry points calls it, rather than triplicating the routing assertions per trigger.
- **Phase 5 — Extraction review queue UI + preview** (F48.6/F48.7): the `ExtractionQueueRow` +
  preview-dialog rows in §5 above. Grouping **by video** rather than by entity type (a deliberate
  divergence from Enrichment — see the design handoff's "Resolved: grouping" section) is the one
  layout choice worth a dedicated component test: assert group membership/ordering (most-fields-
  pending-first), not just per-row rendering.
- **Phase 6 — merge → writeback propagation** (F48.8): depends on Phase 3 (snapshotting) being
  live, per the spec's own dependency note. **N** affected videos → **N** writeback jobs is the
  core assertion — extend the existing F23.9 person-merge test suite (and the future studio-merge
  one) rather than introduce a parallel merge test path.
- **Security** (`/security-review`, spec Routing): `requireOwner` gating on every new endpoint
  (extraction trigger/resolve/dismiss/revert — merge-triggered writeback is internal-only, so it
  adds no new endpoint), untrusted filename-token sanitization (F48.10b, same posture as F30.6b's
  manual-value handling), and the "no new outbound network surface" invariant (F48.10c) — extraction
  is local parsing, so a provider-SSRF-style test doesn't apply here, but a **zero-HTTP-calls**
  assertion over the extraction path takes its place.
- **Known test-scope gap (spec Non-Goals)**: filename *rewriting* (the deferred rename-schema
  follow-up, [HOLODEX-192](https://whoiskevinrich.atlassian.net/browse/HOLODEX-192)) is explicitly
  out of scope for this spec — no revert or merge test should assert a changed on-disk *filename*;
  flag it in review if a future PR conflates the two features.

**Claimed provider keys (F49, ADR-074, HOLODEX-218)** — a canonical field may **claim** a
differently-named provider key, so the key contributes its value as a candidate of that field and stops
auto-registering as a separate display-only row (the GH #178 fix: one paragraph rendering as *Overview*,
*Synopsis* **and** *Comments*). The root cause is a namespace mismatch — `appendAutoRegistered` built its
suppression set from **canonical names** while `AutoRegisterFields` tested it against the **raw provider
key**, so a key was suppressed only on spelling coincidence. Pure resolver change plus (slice B) a
`field_claims` store; **no network**, fully CI-testable. The cardinal invariants are **claims are
provider-scoped**, **suppression is unconditional**, **a claim can never suppress into a black hole**, and
**zero-impact when unused**.

- **Derivation (`resolver.ClaimedKeys`, pure)**: over the **effective** `[]mapping.Field` (post-
  `mergePromotions`), not the claims store — ADR-074 §D2. `sources: [tmdb:overview, provA:synopsis]` claims
  both; a **bare** source (`Comment`) and the **`file:`** namespace claim **nothing** (else one mapping's
  `Comment` would swallow every provider's `comment` key); an empty key claims nothing; providers and keys
  compare lowercased + trimmed, because synthesized person/studio fields and F44 promotions are built in
  code and never pass through the YAML parser.
- **Suppression (`AutoRegisterFields`, cardinal)**: a field whose `provider:key` is claimed does not
  auto-register. The pre-existing `rendered` canonical check is **retained** — the two catch different
  cases (`rendered` catches `tmdb:overview` when `overview` renders from the `file:` baseline alone with no
  provider source at all; `claimed` catches `provA:synopsis` feeding `overview`). A test must fail if
  either check is deleted.
- **Provider-scoped (cardinal)**: with `provA:rating` claimed and `provB:rating` not, the `rating` row
  **still auto-registers**, carrying provB's value, provB's provenance, and `WinningSource == provB:rating`
  only. Falls out of the existing per-`(provider, key)` accumulator with no special case — pin it so a
  future refactor to per-key suppression is caught.
- **Unconditional (cardinal)**: suppression does not depend on whether the claimed source **won** resolution
  for the entity being viewed — a claim states identity, not a per-entity outcome. Structurally guaranteed
  (`AutoRegisterFields` receives no resolution outcome); the observable half is pinned by asserting identical
  suppression with the claimed source listed **first** (wins) and **last** (loses) in the claiming field.
- **No black hole (cardinal, slice B)**: a claim whose target canonical is absent from the effective field
  set is **inert** — it neither suppresses nor appends, and the key auto-registers again exactly as pre-F49
  (ADR-074 §D4). This is a property of §D2, not a guard: suppression reads the *materialized* field set, so
  a claim that fails to materialize has nothing to suppress from. Assert the dangling case end-to-end
  rather than asserting a log line.
- **Store + API (slice B)**: migration `0029_field_claims` up/down applies cleanly and **preserves claims**
  across the round-trip; the PK is `(entity_type, provider, field_key)` — **wider than `field_promotions`
  on purpose**, so `provA:synopsis` claimed while `provB:synopsis` is not is representable. `PUT`/`DELETE`/
  `GET /admin/field-claims/{entity_type}[/{provider}/{field_key}]` owner-gated (**401** unauth before the
  handler), `DELETE` **idempotent**, unknown `entity_type` → 400, reserved (`_`) or canonical `field_key`
  → 422, target canonical not a field of that entity type → 422. A claim materializes as an **appended**
  `mapping.Source` (lowest precedence — adding a claim must never move the current winner) and merges
  **after** promotions. `PUT` on a promoted key clears the promotion **in the same transaction** (RD3).
- **Attached keys list (slice B, FR8)**: the owner-hub list is the only durable surface a claim has, so test it
  as data-integrity, not decoration. It renders every DB claim across all three entity types (a claim missing
  from the list is a claim the owner cannot remove); a **dangling** claim — target absent from
  `GET /admin/field-targets/{entity_type}` — renders as **Inactive** rather than being hidden or pruned
  (ADR-074 §D4, handoff DD9); Remove issues the idempotent `DELETE` and the key auto-registers again on the
  next resolve; Remove does **not** resurrect a promotion that claiming cleared. Partial load failure fails the
  page rather than rendering a list that silently omits an entity type. YAML `sources:` claims are **not**
  listed — assert that too, so the omission stays deliberate rather than becoming a bug report.
- **Frontend surfaces (slice B)**: no automated coverage — the Attach affordance, the DD5 confirmation strip
  and the FR8 list are verified live against the checklist,
  [claimed-provider-keys-qa-checklist.md](design/claimed-provider-keys-qa-checklist.md). Two items there earn
  their place: the fixture needs a **two-provider** auto-registered row (the DD3 checklist and the
  partial-attach outcome are both unreachable with one provider), and DD7 moved a **shipped** F44 control, so
  the promote pill is re-verified on `long_text` rows rather than only the new Attach pill.
- **Backward-compat golden (cardinal)**: with no claims and no provider source listed in any mapping,
  resolved output is **byte-identical** to pre-F49, on all three entities. Note this is *not* the same as
  "purely additive" — see below.
- **Known behaviour change (spec §8)**: a key an operator **already** lists under `sources:` stops
  auto-registering the day slice A ships. It was rendering twice, so a duplicate goes away rather than data
   — but a *losing* claimed source moves from the page to one click behind the F36 source chip. A test
  asserting the old doubled rendering is asserting the bug; update it rather than preserving it. Release
  note required.

**Tag governance & video enrichment (F50, ADR-075, HOLODEX-224)** — extends F43's tag identity spine
with a global deny-list, a strict one-parent hierarchy, automatic materialization of video-enrichment
`genres` into real Tag rows, owner-editable tag chips on the media page, and full-ancestor-chain genre
writeback. Also fixes a latent correctness gap found while spec'ing this feature: `video_tags` carries no
provenance today, so `replaceAssociations()`'s unconditional delete-and-reinsert on every rescan would
silently wipe any manually-added or enrichment-derived tag the moment either of those write paths
existed. **This PR (spec + ADR-075 + design handoff + this testing-strategy update) is docs-only — none
of S1–S9 is built yet**, so the §4/§5 rows and Critical invariants above are the *target*. Fully
CI-testable, no new network surface — deny-list/hierarchy/materialization are pure DB + resolver-layer
changes reusing F43's already-shipped `resolveOrCreateByName`/`entity_aliases`/near-miss machinery rather
than reimplementing it. Cardinal invariants: **rescan never destroys a non-file tag association**, **the
deny-list is unbypassable because it lives inside the resolver, not at each caller**, **a hierarchy edit
can never create a cycle**, **materialization is idempotent and alias-canonicalizing** (all four above,
§"Critical invariants"). Maps to the [design handoff's QA
checklist](design/tag-governance-and-video-enrichment-qa-checklist.md); `/tags`' existing pill ⋯ menu
(rename/alias/merge) and its near-miss card are the direct structural precedent for both the hierarchy
menu action and the media-page add-tag flow's collision handling.
- **S1 — `video_tags` provenance + `replaceAssociations` fix** (P0-1, ADR-075 D3): ships first,
  independently of any new feature — it is a latent bug fix, not new surface area. The single
  highest-value test in this spec (§4 row above); every later slice's tests depend on this one being
  correct, since S4/S5 are the write paths whose data S1 stops the scanner from silently destroying.
- **S2 — deny-list** (P0-2/P0-3, ADR-075 D2): table-driven across all three enforcement call sites
  (scanner/manual-attach/materialization) sharing one assertion helper, since they route through the same
  `resolveOrCreateByName` guard — a passing S2 suite that only exercises one call site would miss a
  regression at the other two.
- **S3 — hierarchy** (P0-4/P0-5, ADR-075 D1): cycle-guard boundary cases (self, direct parent, deep
  ancestor, unrelated sibling) plus the recursive descendant-expansion query, which is the one piece of
  this slice with no F43 precedent to reuse — write it as its own fixture (a 4-level tag tree) rather than
  bolting it onto an existing one.
- **S4 — video tag attach/detach + media-page UI** (P0-7/P0-8, handoff §1): backend endpoint tests plus
  the Vitest/Playwright rows in §5 above; the near-miss/collision-handling assertions can be lifted nearly
  verbatim from `/tags`' existing `actionNearMiss` test fixture rather than re-authored.
- **S5 — enrichment materialization** (P0-6, ADR-075 D4): idempotency (re-enrich N times, assert row
  count) and alias-canonicalization are the two assertions that matter; the deny-list-skip case is a
  regression guard proving S5 didn't grow a fourth, un-tested enforcement path outside S2's table.
- **S6 — genre writeback** (P0-9/P0-10): ancestor-chain flattening plus the dual-filter regression case
  (§4 row above) — write the "denied only in TMDB-raw" case explicitly, since it's the exact gap the owner
  caught during spec review and the easiest one for an implementer to reintroduce.
- **S7 — merge reparenting** (P1, ADR-075 D1 merge extension): extends F43/ADR-061's existing person/
  studio/tag merge test suite with the children-reparent-onto-survivor assertion, rather than a parallel
  merge test path — mirrors how F48.8 (merge→writeback) extended F23.9's person-merge suite instead of
  duplicating it.
- **S8 — P1 management UI** (deny-list tab, hierarchy pill-menu action, handoff §2-3): the §5 rows above;
  cross-reference rather than duplicate the [QA
  checklist](design/tag-governance-and-video-enrichment-qa-checklist.md)'s manual/agent layer — that
  checklist owns 3-skin visual verification, this suite owns interaction/state-transition correctness.
- **Security** (`/security-review`, spec Timeline step 4): the new mutations (`videos/{id}/tags`,
  `tags/{id}/parent`, `owner/tags/denylist`) are all `requireOwner`-gated — no new ungated surface; no new
  externally-influenced input beyond what F43 already validates (tag names go through the same sanitize
  perimeter materialization and manual-attach both already share).

**Tag writeback exclusion (ADR-077, HOLODEX-239)** — lets an owner exclude a tag's name from a video
file's Genre writeback while it stays fully searchable/browsable in Holodex, plus a manual sync to push
the current decision out to already-written files. Unlike the F50 entry above, **this feature is already
fully built and tested** (backend: commit `cb88390`; `ListTags` writeback_enabled gap fix: commit
`3465782`; frontend: commit `ef435fd`) — this pass documents/cross-references the coverage that shipped
alongside the code rather than writing new tests ahead of implementation. §4/§9 rows above and the E2E
flow in §6 describe real, currently-passing suites.
- **D1 — flat filter** (migration 0033, `TagNamesForVideo`): `TestTagNamesForVideo_WritebackFlagFlat`
  (`internal/repo/tag_hierarchy_test.go`) is the ADR's own action-item-7 scenario verbatim — a disabled
  mid-chain tag drops only itself from the Genre-bound name set, never a further ancestor or a descendant
  reached through it.
- **D2 — manual sync** (`VideoIDsForTag(s)`, `syncTagWriteback`): `TestSyncTagWriteback_RecomputesFullUnion`
  and `TestSyncTagWritebackBulk_DedupsSharedVideo` (`internal/api/tag_writeback_sync_test.go`) cover the
  per-video full-union recompute and the bulk video-dedup; `TestSetTagWriteback` /
  `TestSetTagsWritebackBulk_AppliesRegardlessOfPriorState` prove the flag-flip endpoints never enqueue
  (nil write-queue would panic on any accidental `Enqueue`), keeping "toggle" and "sync" as two genuinely
  independent actions per the spec.
- **D3 — batch status** (`GetWritebackBatchStatus`): `TestWritebackBatchStatusEndpoint`
  (`internal/api/tag_writeback_sync_test.go`) plus `internal/repo/writequeue_test.go`'s aggregation test
  cover the `pending`/`running`/`done`/`failed` rollup end to end, including the unknown-batch-id →
  200-zero-counts case (a batch is a derived aggregate over `writequeue`/`job_runs`, not a stored row, so
  "not found" isn't a meaningful response here).
- **`ListTags` regression** (commit `3465782`, found during S4 frontend work): `Repo.ListTags` didn't
  select `writeback_enabled` (it isn't part of `namedCountQuery`, shared with `ListStudios`), so the Go
  zero-value serialized `false` on every `/tags` list row regardless of true state. Fixed with the same
  batch-attach pattern `tagParents` already uses; `TestListTagsWritebackEnabled` guards the regression by
  asserting the list and detail reads agree on a disabled tag's flag.
- **Frontend** (S4, commit `ef435fd`): `writebackJob.test.ts`'s `waitForWritebackBatch` suite (6 cases) is
  real automated coverage of the shared polling logic; the component layer (Details card,
  `WritebackBatchDialog`, bulk bar) is manual-QA-verified only (§5 row above), matching the standing §0
  gap rather than a feature-specific shortfall.
- **Security** (`/security-review`, pending): the write-back path reuses the existing `writequeue`/
  `writeback` file-I/O machinery (F30/ADR-048) unchanged in shape — the new surface is 4 owner-gated
  mutation endpoints plus one owner-gated read (`GET /writeback/batches/{batchID}/status`); review still
  needs to confirm the batch-status read leaks nothing across batches (a guessed/enumerated `batchID`
  should reveal only aggregate counts, never per-video paths or filenames) before this gate closes.

**Tag categories (HOLODEX-240, ADR-078)** — a deliberately reduced entity for hand-curated tag grouping
(create/rename/delete only, no provenance, no alias/merge machinery — Category never joins the F43/ADR-061
identity spine, ADR-078 D1/D4) plus a many-to-many `category_tags` junction mirroring `video_tags`. The one
genuinely new pattern is cross-table name uniqueness (a category can't share a name with a tag or vice
versa) — enforced at two layers (app pre-check + DB triggers, ADR-078 D3), the first cross-table uniqueness
requirement this codebase has needed. **Unlike F50 above, this entry documents work already built**: spec
(S1) → ADR-078 (S3) → backend (S4) → frontend (S5) all shipped per `docs/plans/HOLODEX-240.md` before this
testing-strategy update landed — S4's implementation already carried its own backend test suite
(`internal/repo/categories_test.go`, `internal/api/categories_test.go`), covering CRUD, both collision
directions, cascade-delete, and the facet query. This update's own contribution: two S5 gaps the plan
explicitly flagged as untested (`ListCategories`' `tag_count`/`tag_ids` fields, `ResolveOrCreateTag`/`POST
/tags`) now have dedicated repo + API test coverage (§4 rows above), and in the process surfaced a real,
previously-undiscovered edge case — a zero-video tag created via `POST /tags` is invisible to
`ListTags`/`GET /tags` (§4 row, §11) — plus a pre-existing (not newly-introduced) focus-restore gap in the
`EntityPicker`/`CategoryPicker` dialog shell, found while manually QA'ing this session's `PickerShell`
extraction (§5 row, §11).
- **S1–S3 — spec/ADR/backend-adjacent decisions**: no test surface of their own; validated via S4's tests.
- **S4 — backend** (`internal/repo/categories.go`, `internal/api/categories.go`, migration `0035`): CRUD +
  cross-table collision + cascade-delete + facet query, all covered at S4-build-time (§4 rows above);
  `resolveOrCreateByName`'s tag path gaining the symmetric category-collision pre-flight check is covered
  via `TestCategoryCrossTableCollision`'s `AttachTagToVideo` case, including the scanner's silent-skip path.
- **S5 — frontend** (`tags/+page.svelte` unified filter + category pills, `entity/CategoryPicker.svelte`,
  `routes/categories/[id]/+page.svelte`, browse Categories facet): zero Vitest component-test coverage,
  consistent with **every** other Svelte component in this codebase (§0's own standing gap, not new to this
  feature) — verification is driven-browser QA. This session's QA covered `PickerShell`/`EntityPicker`/
  `CategoryPicker` `mode="add"` across all 3 skins (§5 rows above); the type filter, category pill
  Manage-mode asymmetry, `/categories/{id}`, the browse facet, and `mode="remove"` are **not yet QA'd** —
  `docs/plans/HOLODEX-240.md`'s own "Up next" #1, tracked in §11, not silently skipped.
- **Security** (`/security-review`, HOLODEX-240.md gate checklist): **not started** — the new owner-gated
  mutations (`POST/DELETE /categories*`, `POST /tags`) need the same scrutiny as F50's tag mutation surface;
  ADR-078 Action Item 8 names the specific check (category names inherit the existing `model.MaxNameLen`
  cap the tag path already enforces — confirmed by this session's `ResolveOrCreateTag` tests).

**Tag & category create affordance (HOLODEX-243, epic HOLODEX-240)** — closes the gap the Tag Categories
epic left open: nothing on `/tags` starts a *brand-new* tag or category (both were previously creatable
only as a side effect — tagging a video, or `CategoryPicker`'s inline "+ Create" fallback during
category-assign). Spec (`docs/specs/tag-category-create-affordance.md`) and design handoff
(`docs/design/tag-category-create-affordance-handoff.md`) landed 2026-07-31; implementation landed
2026-08-01. **Not UI-only after all** — the spec's own Non-Goal assumed both backend endpoints needed no
change, but implementation surfaced a real bug the design work didn't anticipate: `ListTags` inner-joined
`video_tags`, so a bare-created tag (zero videos, by construction — this is the *first* caller that ever
creates one) never appeared in `GET /tags`, silently breaking the feature's own P0 goal #1. Fixed with a
narrowly-scoped change (`namedCountQuery`'s new `includeZero` param, §4 row above, and the matching
Critical invariant above) rather than expanding the spec — see the spec's own "Implementation note"
addendum. The frontend layer (§5 row above) is now built and manually driven-browser QA'd across all 3
skins this session (create/collision/near-miss/empty-state/owner-gating/focus-restore all verified live);
**no automated Vitest/Playwright coverage exists yet** for it, tracked in §11 alongside the equivalent gap
every other recent frontend feature in this file carries at this stage.

**Video credits → People (F32, HOLODEX-102/HOLODEX-22)** — a video's TMDB enrich response carries structured
`people[]` cast/crew credits (name, role, external_id, order, headshot) instead of flat `actors`/`director`
text; core enrich resolve-or-creates each Person by namespaced external_id (ADR-055's identity spine,
mirroring F38's studio dedup) and downloads a real headshot through the existing SSRF-guarded asset pipeline
**before** `RelinkVideoPeople` (F40/ADR-072, sole writer of `video_people`) derives links from the
already-resolved `actors`/`director` text. F32 itself never writes `video_people` — its job is Person
population plus a `_person_external_ids` sidecar (mirrors `_studio_external_ids`), consumed via an
`extIDByName` map. Fully CI-testable, no network (in-process `Fake` TMDB).
- **Resolve-or-create + dedup + merge-repoint (`internal/repo/aliases_test.go`)**: `TestReconcileVideoPeople_ExternalIDDedup`
  — two credits sharing an external id converge to one Person; `TestReconcileVideoPeople_ExternalIDCascade`
  — a name-only Person later gains an id and back-fills, converging a further spelling (the studio
  back-fill pattern, §F38 above); `TestMergePersons_RepointsExternalID` — `person_external_ids` survives
  `MergePersons` (F23).
- **Provider (`providers/tmdb/tmdb_test.go`)**: `TestBuildPeopleCreditsCapsAt20` / `TestBuildPeopleCreditsCapsCrew`
  — `people[]` caps at the top 20 billed cast plus up to 10 director/crew, each requiring a positive
  provider id (`id`≤0 dropped); `TestDescribe` — `/describe` advertises `credits: true`.
- **Core enrich consumption (`internal/enrich/enrich_test.go`)**: `TestEnrichVideoConsumesPeopleCredits` —
  `resolvePeopleCredits` runs resolve-or-create + headshot download per credit; `TestEnrichVideoCreditHeadshotIgnoresProviderKind`
  — the headshot download **forces `Kind: "photo"`** regardless of what the provider sends (closes a
  role-diversion bug); `TestSanitizePeopleRejectsWhitespaceInExternalID` — provider `name`/`role`/`external_id`
  are treated as **untrusted** input; whitespace inside `external_id` is rejected because the sidecar's
  `"<external_id> <name>"` format is parsed by splitting on the first space, so an unsanitized value could
  let a malicious provider hijack a video credit onto an unrelated existing Person — found and fixed
  (`SanitizeValue` + `strings.Cut` + explicit space-rejection) during this feature's own `/security-review`.
- **Side-map parse (`internal/api/person_links_internal_test.go`)**: `TestPersonExternalIDsFromRows` —
  white-box parse of `_person_external_ids` rows into `extIDByName`, mirroring `studioExternalIDsFromRows`
  (§Studio external-id de-dup above).
- **Frontend**: no code changes — F30's actor/director chips (`CurationFieldRow.svelte`/`CurationChip.svelte`)
  and the People poster grid (`media/[id]/+page.svelte`, `PersonImageFrame`/`PersonPoster`/`PersonAvatar`)
  already name-match against `video.people` and render headshots with a themed placeholder fallback when
  absent. **Verified live** (real TMDB "Dune (2021)" enrich, not a fixture): all 10 actors plus a
  brand-new director Person linked in `video.people`; all 11 People got a real downloaded JPEG (headshot
  and poster, confirmed via direct HTTP content-type checks, not just the UI); both the People-grid poster
  links and the Actors/Director text chips resolve to `/people/{id}` with distinct, readable link-vs-label
  contrast across Cinémathèque/Broadcast/Brutalist (computed-style verification, per this repo's
  screenshot-times-out convention).

---

## 10. Example Test Cases (concrete)

**MKV precedence (ADR-010) — adversarial**
```
Given multilevel.mkv with TITLE="Episode 1" at level 50,
      TITLE="The Collection" at level 70,
      and TITLE="Director Commentary" on an audio track (level 30)
When the file is indexed
Then video.title == "Episode 1"
 And "The Collection" and "Director Commentary" are NOT the title
 And "Director Commentary" does not appear in people or tags
```

**Resolution tolerance boundary (ADR-012)**
```
Given files of width 3455 and 3456
When classified
Then 3455 → "FHD" and 3456 → "4K+"
```

**Cache invalidation (ADR-008) — the cardinal sin**
```
Given a media list query result is cached
When a file in that result is re-indexed with a new title
Then the next identical query returns the new title (not the cached one)
```

**Incremental scan idempotency (ADR-018)**
```
Given a library scanned once
When scanned again with no filesystem changes
Then zero extraction subprocess calls occur
 And record count and ids are identical (no duplicates)
```

**Range request (ADR-015)**
```
Given an indexed video
When GET /api/v1/media/:id/stream with header "Range: bytes=100-199"
Then status 206, Content-Range "bytes 100-199/<size>", body length 100
```

**REST/MCP parity (ADR-005)**
```
Given the same filter (tags=[X], resolution=4K)
When applied via GET /api/v1/media and via MCP search_videos
Then both return the same set of video ids
```

**Mapping precedence (ADR-013)**
```
Given mapping studio.sources = [Publisher, Studio]
  And a file with Publisher="A" and Studio="B"
When the Studio field resolves
Then value == "A" (first source in precedence order)
```

**Unified resolution — provider/file interleave (ADR-033, F22.3)**
```
Given field birthdate.sources = [tmdb, file:Birthdate]
  And a person with a file-sourced Birthdate="1970-01-01"
  And a TMDB-enriched birthdate="1941-01-05" in the shadow store
When the birthdate field resolves
Then value == "1941-01-05" (provider first in precedence)
  And with sources = [file:Birthdate, tmdb] it resolves "1970-01-01"
```

**Enrichment is non-destructive on re-scan (ADR-033, F22.3c) — cardinal invariant**
```
Given a video whose file title is "AMV Original"
  And a provider enriched the canonical title to "Official Title" (file-first config)
When the file is re-scanned
Then video.title remains "AMV Original"
  And the shadow enrichment row is untouched
```

**Matching — ID-first, search fallback (ADR-033, F22.5b)**
```
Given a person with no embedded external id
When the owner searches "Miyazaki" via the picker
Then /resolve returns ranked candidates and NO field is applied yet
  And only after the owner confirms a candidate does /enrich run
When instead an embedded tmdb id is present
Then resolve auto-selects it without showing the picker
```

**Enrichment SSRF allowlist + no leaked keys (ADR-033, F22.9)**
```
Given a provider configured with base_url http://holodex-fake:9100
When /resolve returns a candidate whose asset url points to http://169.254.169.254/
Then core does NOT fetch that host (only allowlisted base_url + same-origin assets)
  And no response, log line, or /admin/activity field contains the provider's API key
```

**Untrusted provider response (ADR-033, F22.9b)**
```
Given the fake provider returns a 100 KB "bio" and a non-image "photo" asset
When the person is enriched
Then the stored/displayed bio is length-capped
  And the photo asset is rejected on content-type, field display still succeeds
  And a malformed JSON response fails just that fetch (server stays up)
```

**Promotion tier-0 outranks YAML + renders once (F44/ADR-062, D3/FR3) — cardinal**
```
Given a video key "director_notes" with BOTH a metadata-mappings.yaml mapping
      (label "Notes", render text) AND a field_promotions row
      (label "Director's Notes", render long_text)
When the media detail resolves
Then the field's label == "Director's Notes" and render == "long_text" (promotion wins)
  And the field is curatable (carries Decision/Candidates), AutoRegistered == false
  And it appears exactly once (the promotion mapping.Field replaced the YAML one;
      AutoRegisterFields excluded the now-mapped key)
```

**Promotion is presentation-global, curation is per-entity (F44/ADR-062, D1) — cardinal**
```
Given a person field_promotions row for "measurements" (label "Measurements")
  And person A with a metadata_curation suppress on "measurements"
When persons A and B (both carry the shadow key) resolve
Then both show label "Measurements" (shared, from the global promotion row)
  And A's value is suppressed while B's value is intact (per-entity curation)
When the promotion row is cleared then re-created
Then both revert to the auto-registered title-cased label, then relabel again
  And A's suppress row (keyed by field_key) re-applies on re-promotion — never lost
```

**Promotion cannot shadow the schema contract (F44/ADR-062) — guard**
```
Given the code registry knows canonical key "bio"
When PUT /admin/field-promotions/person/bio is called by the owner
Then status 422 and no field_promotions row is created
  And the same for a "_"-prefixed key (e.g. _studio_external_ids) → 422
```

**Derived Age — compute-on-read + mutual exclusion (F45/ADR-063, D1/D4) — cardinal**
```
Given a person with birthdate=1990-03-14 and no deathdate
When personResolved runs with a fixed now=2026-07-08
Then an "age" ResolvedField follows birthdate in resolved[] with value "36"
  And it carries computed:true, winning_source "computed:age", derived_from ["Birthdate"],
      and nil decision/candidates/in_sync
When now is advanced to 2027-03-15 (past the next birthday) — no DB write, no migration
Then the same read yields value "37"        (time-varying, never stored)
Given instead the person also has deathdate=2020-01-01
Then resolved[] contains exactly ONE computed row — "age_at_death", value "29" —
     and NO running "age" row
```

**Derived field — missing/unparseable input is absent for everyone (F45/spec D3)**
```
Given a person with NO birthdate
When personResolved runs (as owner AND as visitor)
Then neither an "age" nor an "age_at_death" row appears — no placeholder, no "—", no nudge
  And the two payloads are byte-identical (no owner-only branch)
Given instead birthdate="unknown" (non-ISO)
Then deriveAge returns computable=false and no "age" row renders (no error, no partial value)
```

**Computed source is non-adoptable + resolver stays pure (F45/ADR-063, D3/AC-8) — guard**
```
Given a rendered "age" row with winning_source "computed:age"
When PUT /media|people/{id}/fields/age/decision names "age" or source "computed:age"
Then status 400 and nothing is written to field_source_decisions
  And fieldsource.Valid("computed:age") == false; "computed" is absent from ForNamespace
When grep scans internal/resolver/ for time.Now
Then there are zero matches (Derive takes `now` as a parameter, injected at Handlers.now)
```

**Enrich endpoint owner-gated (ADR-033, F22.9a)**
```
Given ADMIN_TOKEN is set
When POST /api/v1/people/:id/enrich is requested WITHOUT the token
Then status 401 and no provider call is made
  And the SPA renders no Enrich control for that (non-owner) client
```

**Person alias — search match + dedup (ADR-036, F23.5)**
```
Given a person "David Bowie" with aliases "Ziggy" and "Bowie"
When GET /api/v1/search?q=zig is requested
Then "David Bowie" appears in the people results exactly once
When GET /api/v1/search?q=bowie is requested (matches both name and an alias)
Then "David Bowie" still appears exactly once (id-deduped)
And the diacritic-folded alias "Beyoncé" is matched by q=beyonce
```

**Person alias — owner-gated CRUD (ADR-036, F23.2/F23.3)**
```
Given ADMIN_TOKEN is set
When POST /api/v1/people/:id/aliases is requested WITHOUT the token
Then status 401 and no alias is created
When POST … WITH the token and body {"alias":"  Rob  "}
Then status 200, the alias stored trimmed as "Rob", returned in the list
And re-POSTing "rob" is idempotent (list unchanged, no error)
And POST with an empty/200+char alias → 400
And DELETE …/aliases/:unknownId → 404; DELETE of another person's alias id → 404
```

**Aliases survive a re-scan (ADR-036)**
```
Given a person with alias "Ziggy"
When a full re-scan runs (rebuilding people/video_people)
Then the alias "Ziggy" is still present and still search-matchable
```

**Search returns a matched person's media (ADR-036, F23.5)**
```
Given a person "Zeta Person" with a video titled "Untitled Clip" (title shares no terms with the name)
When GET /api/v1/search?q=zeta is requested
Then "Untitled Clip" is in the video results (matched via person, not title)
When the alias "Zed" is added and GET /api/v1/search?q=zed is requested
Then "Untitled Clip" is still in the video results
And a title-only query (q=untitled) still returns it, with no person attached, video ids de-duped
```

**Person merge + scan-time routing (ADR-036, F23.8/F23.9) — cardinal invariant**
```
Given separate people "Jennifer Lawrence" and "J Law", and a film credited under both
When the owner merges "J Law" into "Jennifer Lawrence"
Then "Jennifer Lawrence" owns the de-duped union of their videos (the dual-credited film counts once)
  And "J Law" is one of her aliases and is search-matchable
  And the "J Law" person no longer exists
When a file tagged "J Law" is (re-)scanned
Then it links to "Jennifer Lawrence" — no "J Law" person is recreated (the merge holds)
```

**Same-name collision is surfaced, not auto-merged (ADR-036, F23.10)**
```
Given a person "Chris Evans" (actor) and another person being edited
When the owner adds the alias "Chris Evans" to the other person
Then the API responds 409 with the actor as `conflict` (id, name, video_count)
  And no merge happens and no alias is created until the owner confirms
```

**Owner gate — open vs. gated (ADR-030, F21.7)**
```
Given ADMIN_TOKEN is set
When GET /api/v1/admin/activity is requested WITHOUT the token
Then status 401
 And the same request WITH the correct token → 200
When ADMIN_TOKEN is unset (single-user pass-through)
Then the request → 200
```

**Fail-loud default-open (F21.7 condition 1)**
```
Given ADMIN_TOKEN is empty
When the server binds a non-loopback HOST
Then a warn/error is logged at startup
 And GET /admin/activity → system.controls_unauthenticated == true
When instead bound to loopback, or ADMIN_TOKEN is set
Then no such warning is logged
 And system.controls_unauthenticated == false
```

**Constant-time token comparison + cookie attributes (F21.7 condition 2)**
```
Given the gate compares a presented token to ADMIN_TOKEN
When the comparison runs
Then it uses crypto/subtle.ConstantTimeCompare (never ==)
 And IF owner identity is carried by a cookie
Then that cookie is HttpOnly + Secure + SameSite=Lax|Strict
 And the raw token is never written to localStorage
```

**CSRF on state-changing controls (F21.7 condition 3)**
```
Given the owner is authenticated
When a cross-site form issues POST /api/v1/admin/rescan
     without the required request header / CSRF token
Then the request is rejected (no scan is triggered)
 And the same applies to POST /admin/reload-config and the regenerate-thumbnail route
```

**Activity read-model — no secrets (ADR-028, F21.1/F21.3)**
```
Given a library with an extraction failure whose error text contains an absolute path
When GET /api/v1/admin/activity, GET /api/v1/admin/activity/history, and GET /api/v1/admin/activity/digest are read
Then no response field contains a filesystem path, env value, or token
     (including job_runs.error_message, in both the history rows and the digest's failures list)
 And system.media_path_present is a boolean, not the path
```

**Related-media selection + self-exclusion (ADR-031, QW2)**
```
Given a library of N=10 active videos
  And item V with people [A (global count 5), B (global count 2)]
  And tags [T_universal (global count 9), T_theme (global count 4)]
  And other active items share A, B, and both tags in various combinations
When GET /api/v1/media/{V}/related
Then person.id == A   (highest global count; tie-break lowest id)
 And tag.id == T_theme — the DISTINCTIVE tag wins over the near-universal one:
     score(T_universal) = 9·(1−9/10) = 0.9  <  score(T_theme) = 4·(1−4/10) = 2.4
 And every id in person.items shares A and != V
 And every id in tag.items shares T_theme and != V
 And len(person.items) <= 5 and len(tag.items) <= 5
 And all returned items are active, each with its people/tags attached
   (selection is deterministic; item order is not — assert the set, never the sequence)
```

**Related-media empty + null blocks (ADR-031, QW2)**
```
Given item V whose only person A appears on no other active item
  And V has no tags
When GET /api/v1/media/{V}/related
Then person.id == A and person.items == []   (block present, no siblings)
 And tag == null                              (item has no tags)
When the same is requested for a non-existent or inactive id
Then status 404
```

**Fluid Back — scroll + loaded pages restored (ADR-032, QW4)**
```
Given the browse grid scrolled down past "Load more" ×2 (150 items loaded)
  And the window scrolled to Y
When an item is opened and the browser Back button is pressed
Then the grid renders all 150 items from cache (no GET /api/v1/media fires)
 And window.scrollY == Y (within a few px), with no jump-to-top or Loading… flash
When instead a filter is changed (signature differs)
Then the cache is discarded, the grid refetches from offset 0, scroll resets to top
When instead the page is hard-reloaded
Then the grid rebuilds from the URL filters at page 0 (no stale restore)
```

**Search history — dedupe, cap, defensive parse (QW1)**
```
Given an empty search history
When queries are submitted: "amv", "AMV editor:foo", "amv"
Then history == ["amv", "AMV editor:foo"]   (case-insensitive dedupe; "amv" moved to top)
When 10 further distinct queries are submitted
Then len(history) == 10 and the oldest entry was evicted
Given localStorage holds malformed JSON under the history key
When the store reads history
Then it returns [] and search still works (never throws into the UI)
```

**Person link resolved-derivation — cutover + round-trip (F39/ADR-056) — adversarial**
```
Given a scanned library where person "Denis Villeneuve" exists ONLY via the file Director tag
      and person "Roger Deakins" exists ONLY via the file Cast tag
When migration 00NN runs and the one-time backfill re-derives video_people via RelinkVideoEntity
Then both people still have their links (active link count >= pre-migration)
 And their roles are director / actor respectively (derived from the source field)
 And the backfill job row records pre and post counts and did NOT fail
When instead `director` is (mistakenly) unmarked from the person-typed set
Then the loss-guard trips (post < pre) and the backfill fails loudly — the dropped source is caught

Given video V whose resolved actors = [canonical "Chloé Zhao", "Frances McDormand"]
When the owner writes back actors -> Artist
Then Artist == "Chloé Zhao, Frances McDormand" (canonical names, comma-delimited)
When the file is re-scanned
Then splitMulti yields exactly 2 people and RelinkVideoEntity re-links the SAME two entities
 And there is no "Chloé Zhao, Frances McDormand" single person, no duplicate Chloé
When instead the file had originally spelled her as an alias "Chloe Zhao"
Then writeback still emits the canonical "Chloé Zhao" and the re-scan resolves to the same entity

Given a curated actor "Carol Kane" added to V's actors field (NOT in the file)
When the curation add commits
Then Carol appears in video_people (role=actor) with NO rescan, and on her person page
When the owner clears that curation and Carol has no other links
Then Carol is stamped orphaned_at (NOT deleted); a re-add before the sweep clears the stamp
When the 30-day sweep runs and Carol has a curated headshot
Then Carol is KEPT (authored-identity guard); a plain orphan past 30 days is deleted
```

**Tag writeback exclusion — flat filter + recompute + aggregation (ADR-077) — adversarial**
```
Given tag tree Animal > Mammal > Dog > GermanShepherd, video V tagged only GermanShepherd
When Dog's writeback_enabled is set to false
Then TagNamesForVideo(V) == [Animal, Mammal, GermanShepherd]  (Dog excluded; neither further
     ancestor nor the descendant leaf is suppressed)
 And V still shows the "Dog" tag/chip and V is still findable by searching/filtering on "Dog"
     (writeback_enabled only ever affects the Genre field, nothing else)

Given video V's file was last written with Genre="Animal; Mammal; Dog; GermanShepherd"
When Dog's writeback_enabled is set to false and the owner clicks "Sync writeback now" on Dog
Then the enqueued write's target Genre value is "Animal; Mammal; GermanShepherd" — the CURRENT
     full union of V's still-enabled tags, not the old value with "Dog" merely omitted from an append

Given tags Comedy and Action, video "Shared" carrying both, videos "ComedyOnly"/"ActionOnly" each
      carrying one
When the owner bulk-syncs {Comedy, Action}
Then exactly 3 jobs are enqueued sharing one batch_id (Shared counted once, not twice)
 And GET /writeback/batches/{batchID}/status initially reports pending=3
When all 3 jobs settle (2 done, 1 failed)
Then the status reports pending=0 running=0 done=2 failed=1
 And GET .../status for a batch_id that was never issued returns 200 {0,0,0,0}, not 404
```

**Studio image roles — migration carry-forward, entity-generic assets, provenance lock (F51/ADR-079) — adversarial**
```
Given a studio with an existing studio_logos row (provider TMDB, width/height/byte_size set)
When migration 0036 runs
Then studio_images has exactly one row for that studio: role='logo', source='enrichment',
     same provider/width/height/byte_size, and studio_logos no longer exists
 And any field_source_decisions row for (entity_type='studio', field_key='logo') is gone
 And GET /api/v1/studios/{id}/images/logo serves the same bytes GET /studios/{id}/logo used to
     (post-migration file move landed, or the slot re-fetches on next enrich if it didn't)

Given a studio with no prior logo
When a TMDB enrich apply runs
Then the company response's logo arrives as an Asset (kind="logo"), not a Fields["logo"] entry
 And GET /studios/{id}/fields no longer lists "logo" among resolved fields (field retired)
 And studio_images gains one role='logo' row, source='enrichment'

Given the owner uploads a custom logo for a studio (POST .../images/logo)
Then studio_images(role='logo') becomes source='upload', and GET .../images/logo serves it
When the same studio is re-enriched (or a scheduled TMDB apply runs again)
Then the logo slot is UNCHANGED (LockedCoreRoles reports 'logo' for this studio; downloadAssets
     skips it before any fetch — no network call, not just a discarded result)
When the owner then DELETEs .../images/logo
Then the slot is empty (404 on GET) and the NEXT enrich fills it from the provider again

Given a person enrich run with headshot+banner+poster assets (the existing F25 flow)
When downloadAssets(ctx, "person", personID, ...) runs after the entityType widening
Then behavior is byte-for-byte identical to pre-F51: same roles fill, same lock/suppress/dedup/
     poster-seed-from-headshot semantics — the full existing Person image test suite passes
     unchanged (the acceptance bar for the ImageSink generalization)

Given a studio enrich run supplies an asset kind "icon" or "poster" (no real provider does yet)
When downloadAssets(ctx, "studio", studioID, ...) processes it
Then assetRoleFor("studio", kind) maps it to the correct role and it stores like logo does
 (SuppressedAssetURLs/ExistingAssetURLs return empty for studio — nothing is ever skipped as
 already-suppressed/already-held, since studio has no gallery to suppress from or dedup against)

Given an unauthenticated request
When POST or DELETE .../studios/{id}/images/{role} is attempted
Then 401/403 (requireOwner), matching every other studio mutation endpoint
Given role="banner" (not one of icon|logo|poster)
When POST .../images/banner is attempted
Then 400, and nothing is written to studio_images or disk

Given an uploaded file that is actually an SVG/polyglot/decompression bomb
When POST .../images/{role} processes it
Then personimage.Normalize rejects it (same guard every other image upload passes) and the
     request fails without writing to disk — no new decode path was introduced by this feature
```

**Two-tier video poster resolution — dual-derivative extraction, fallback serving (F53, HOLODEX-253) — adversarial**
```
Given a video whose embedded cover art decodes to width w
When Tier 1 extraction (extractCoverArt) runs
Then for w <= min(ThumbnailWidth, PosterWidth): {id}.jpg and {id}-poster.jpg are byte-identical
 And for ThumbnailWidth < w <= PosterWidth: {id}-poster.jpg is the untouched source bytes,
     {id}.jpg is scaled down to ThumbnailWidth
 And for w > PosterWidth: both files are independently scaled from the same in-memory buffer
 And exactly ONE exiftool -b invocation occurs regardless of which band w falls in (no second
     extraction read to produce the second file)

Given a video with no embedded cover art (Tier 2 / generateFrame path)
When background frame-grab generation runs
Then the video is seeked and decoded exactly ONCE (assert via a call-count/spy on the
     seek/decode seam, not just that both files exist) — {id}-poster.jpg is written from that
     single captured frame at PosterWidth, and {id}.jpg is derived from the SAME decoded frame
     at ThumbnailWidth, never a second ffmpeg seek against the source video

Given a video that has never been extracted since this feature shipped (pre-existing library
     item, RD6 lazy backfill)
When GET /api/v1/media/{id}/poster is requested
Then it serves the existing {id}.jpg thumbnail bytes (200, not 404) — Video.PosterURL always
     resolves to a valid image from the moment this ships, even before that video's next
     natural extraction trigger fires
When the video is later re-scanned, re-enriched, has its poster re-uploaded, or the owner
     clicks Regenerate
Then {id}-poster.jpg now exists and GET .../poster switches to serving it — no explicit
     migration or bulk backfill job runs at any point

Given the list view (VideoCard / GET /media list payload)
When this feature ships
Then VideoCard.svelte still binds to thumbnail_url unchanged, the list JSON payload gains no
     new required field per item beyond poster_url appearing (additive, non-breaking), and list
     page load time/bandwidth is unchanged from pre-F53 (the negative test this feature's Goal
     #2 depends on)

Given the media/{id} detail page
When a video's PosterURL points to a poster-tier file wider than its old thumbnail-tier file
Then the rendered <video poster> element visibly reads as sharp rather than upscaled-blurry —
     manual QA, not automatable, but the underlying byte-size/dimension difference between
     .../thumbnail and .../poster for the same id IS automatable and should be asserted
```

**Configurable provider search patterns — precedence, token grammar, unconditional sanitizer (F54, ADR-080, HOLODEX-254) — adversarial**
```
Given a provider with an operator `search_pattern` set AND a `/describe`-advertised
     `preferred_search_pattern` set AND a global `default_search_pattern` set, all three different
When BuildQuery renders the query for a video with every field resolved
Then the OPERATOR pattern wins — the provider's advertised pattern and the global default are
     never consulted (D2's precedence order asserted with all three present simultaneously, not
     each tested in isolation, which would miss an inversion bug)

Given a pattern `{studio} {title?} {performers?} {year?}` (studio REQUIRED, no `?`) on a video
     with no resolved studio
When BuildQuery renders this tier
Then the ENTIRE tier falls through to the next-lower precedence tier — it does NOT render with a
     gap where {studio} would have been (e.g. never " My Title Some Actor 2023" with a leading
     space standing in for the missing required field)

Given a pattern `{studio?} {title?} {performers?} {year?}` (all optional) on the same video
When BuildQuery renders this tier
Then {studio} is silently omitted and the remaining tokens render normally — proving optional-
     token omission and required-token fallthrough are genuinely different code paths, not the
     same behavior described two ways

Given a raw title "Agent 007" and a raw title "Suite 1080p Deluxe"
When sanitizeTitle runs (unconditional, no config gate)
Then "Agent 007" is returned UNCHANGED (007 is not word-bounded to a resolution token) and
     "Suite 1080p Deluxe" has ONLY "1080p" stripped ("Suite  Deluxe" collapsed to "Suite Deluxe")
     — the resolution regex must be word-bounded, or every three-digit number in a real title
     becomes a false-positive strip

Given a raw title "[720p]" (bracket + resolution noise only, nothing else)
When sanitizeTitle strips brackets and the resolution token
Then the intermediate result is empty/whitespace-only, so the function returns the ORIGINAL
     unsanitized "[720p]" — never an empty string. A test asserting blank output here is
     asserting the exact bug this rule exists to prevent (spec AC-8a)

Given an operator-configured search_pattern containing an unknown token name, e.g.
     "{studio?} {director?}" where {director} is not a recognized token
When the config is loaded (config-load time, not render time)
Then a warning is logged AND the provider is NOT disabled — Enrich still works for it, and at
     render time the malformed tier is skipped (falls to the next-lower tier), same as any other
     required-token-missing fallthrough

Given a video with zero resolved studio/performers/year and no search_pattern configured
     anywhere for its provider (the universal floor case — most newly scanned, un-enriched files)
When BuildQuery renders
Then the result is the SANITIZED raw title, not the literal raw title — this is the one
     unconditional behavior change (D4) that fires even when an operator has configured nothing

Given the wire request to a provider's POST /resolve endpoint
When a pattern renders a query (or the sanitizer changes the floor-tier value) vs. pre-F54
     behavior (raw title, unsanitized)
Then the REQUEST BODY SHAPE is byte-identical to pre-F54 — only hint.query's string CONTENT
     differs, never a new field, a renamed field, or a structural change (D1's "wire contract
     unchanged" claim, verified as a golden test, not asserted by reading the ADR)
```

**Poster View for the People list page — conditional border, density lockstep, keyboard-focus (F55, HOLODEX-255) — adversarial**
```
Given a person who has an existing headshot (`headshot_version > 0`) but has never had a
     poster image uploaded (`poster_version` absent/0)
When PersonPosterCard renders that person
Then the card SHOWS the placeholder border (`border-color: var(--rule)`) — reading
     headshot_version instead of poster_version here is the exact bug P0-6 exists to prevent;
     a test asserting borderless in this fixture is asserting the bug, not the feature

Given the inverse fixture: a poster uploaded, no headshot
When the same card renders
Then the card is BORDERLESS (poster_version > 0), independent of headshot_version — proving the
     two fields are read independently, not that one happens to work by coincidence of a shared
     test fixture that always sets both together

Given density.svelte.ts's real TIERS table changes in some future, unrelated change (e.g. a
     fifth breakpoint is added, or an existing cap is retuned)
When PersonPosterGrid computes its own column cap
Then the poster grid's cap changes in lockstep (derived as `viewportTierCap.value * 2`, per the
     handoff's Q2 resolution) — a hand-maintained second table that a developer forgot to update
     alongside TIERS would pass every test written against today's breakpoints and still be
     wrong; the test must assert the DERIVATION, not just today's four numeric outputs

Given a populated PersonPosterGrid (12+ people, so at least one full row at the widest tier)
When a keyboard user presses Tab repeatedly from before the grid
Then focus visits every PersonPosterCard's `<a>` in DOM order — none skipped, none duplicated —
     and each shows a computed `outline-color` resolving to that skin's `--accent` value; run
     this same pass in Cinémathèque, Broadcast, AND Brutalist, since RD4 is a brand-new
     affordance with zero prior portrait-frame coverage to inherit a regression guard from —
     a pass in only the default skin would not catch a per-skin outline-color token typo

Given the same Tab pass, but triggered by a mouse CLICK on a card instead of keyboard Tab
When the card receives regular `:focus` (not `:focus-visible`)
Then NO outline renders — the two focus states must be visibly distinguished; a test asserting
     "a ring appears on focus" without distinguishing click-focus from keyboard-focus would pass
     even if `:focus-visible` were mistakenly written as `:focus` everywhere

Given the People list page's density slider (rendered only in Poster view) and the Videos grid's
     own density slider (rendered on `/`)
When the owner drags either slider
Then BOTH grids' effective column counts change on next render — they read the same
     `mediaDensity.value`, per RD8. A test that seeds only `/people`'s slider and asserts only
     `/people`'s grid reacts would miss a regression where a future change accidentally forks
     People onto its own persisted value instead of sharing Videos'
```

**Films entity — asserted links, subtractive flag, suspended resolver source (F56, HOLODEX-279, ADR-085) — adversarial**
```
Given a film with three attached scenes and a full-film file, each carrying its own
     resolved cast/tags from file+provider+curation
When the film's baseline is computed and a full library rescan (RelinkVideoEntity's normal
     derived-link prune-on-empty cycle) subsequently runs
Then the film's cast/tags are the SET UNION of its scenes' resolved values (no double-count
     for a person credited on two scenes) — and every film_videos row for this film is BYTE-
     IDENTICAL before and after the rescan. A test that only checks the baseline union without
     also re-running a relink cycle across it would miss the actual P0-2 regression: a resolver
     change that starts treating film_videos as derivable and silently detaches real assertions

Given a video attached to a film with films_enabled=true, and that video's Album/Title fields
     have a standing decision pointing at the film's provider:film:<id> source
When the operator flips films_enabled to false and the video's fields are next resolved
Then the resolved Album/Title FALL BACK to the file chip via the resolver's pre-existing
     "decided source currently unmatched" path — NOT a new "source unavailable" badge, NOT an
     error, and the field_source_decisions row itself is left completely untouched (assert the
     row's contents are unchanged, not just that the UI recovered)
When the operator flips films_enabled back to true, with no further owner action
Then the film source resolves again and Album/Title return to the film-written values
     AUTOMATICALLY — a test asserting the owner must re-pick the source after re-enabling is
     asserting the wrong (data-loss) behavior; RD7's whole point is that suspension is lossless

Given films_enabled=true and a video flagged as representing an entire film (RD6)
When that video is fetched via browse, RelatedShelf, the landing page's recently-added rail,
     global search, AND EntityVideos.svelte (the shared component person/studio/tag pages use)
Then it is ABSENT from all five surfaces — but its own /media/{id} detail page remains reachable
     by direct URL, and the film's own page still shows it in the full-film region. A test that
     only checks ONE of the five hidden surfaces would miss a regression where a new video-list
     call site (e.g. a future "similar videos" widget) forgets to apply the same films-hide filter

Given the same full-film-flagged video, and a person credited only on that video (no other
     scene, no other file)
When that person's detail page renders
Then the person's Films row shows the film (RD6's compensating surface) — the person is NOT
     left with an empty cast/tag section and no explanation for why their only credit vanished
     from browse. Asserting only "the video is hidden" without also asserting "the films row
     appeared" tests the hiding half of RD6 while missing the half that keeps it from being a
     dead end (this project's standing no-empty-heading convention)

Given a film that already has a scene at scene_number=6
When a second video is attached to the same film also requesting scene_number=6
Then the attach is REJECTED with an error naming the CURRENT occupant by name/id — never a
     silent overwrite, never an auto-bump to 6b/7. This must hold identically through both
     attach entry points: the video→film single-select dialog AND the film→video bulk picker's
     sequential-numbering commit (a batch collision there is all-or-nothing, not partial)
```

**Film-studio cascade — best-effort failure boundary, autostart hand-off, owner gating (F57, HOLODEX-285, ADR-087) — adversarial**
```
Given a film with three attached videos, and a manual studio pick that would set video 2 into
     a composite-key collision with an unrelated existing video
When the owner commits the cascade (POST /films/{id}/studio/cascade)
Then videos 1 and 3 both land in "enqueued" and their writeback jobs are enqueued in the shared
     batch, while video 2 alone reports "collision" and contributes NO job — the cascade does
     NOT abort videos 1/3 because video 2 failed. A test written by analogy to ADR-077's
     syncTagWriteback (which DOES abort the whole batch on a read failure) would assert the
     wrong thing here: syncTagWriteback aborts because nothing is committed yet at read time;
     this cascade's per-video decision-set IS the commit, so by the time video 2 fails, video 1
     already durably changed — aborting 3 afterward would only leave MORE videos out of sync

Given a film where every attached video already has this exact studio manually decided
When the owner runs the cascade again with the same studio value
Then every video lands in "enqueued" with no error and no collision — re-deciding an
     already-decided value to the SAME value is not a collision (the HOLODEX-270 composite-key
     gate compares {title, people, date, studio} against OTHER videos, not against the video's
     own prior decision). A test that only exercises a changing value would miss a regression
     where a same-value re-cascade spuriously reports every video as colliding with itself

Given a film where the chosen studio causes every attached video to collide or error
When the cascade completes
Then the response's batch_id is the empty string and results contains zero "enqueued" entries
     — the frontend's Results step renders NO "View writeback progress →" button at all (absent
     from the DOM, not disabled) since there is nothing to poll. A test asserting only "the
     button is disabled" would pass a regression that leaves a dead, permanently-disabled
     control in the DOM instead of omitting it, violating this project's standing no-dead-
     affordance convention

Given the cascade's POST response already carries a non-empty batch_id (jobs were enqueued
     synchronously as part of that same request, per ADR-087 D2)
When the owner clicks "View writeback progress →" and WritebackBatchDialog mounts with
     autostart
Then it renders straight into the 'progress' phase on mount — NO 'confirm' step, NO 'starting'
     step ever appear, and trigger() fires exactly once without a click. A test that asserts
     only "the progress bar eventually shows" without asserting the confirm/starting phases were
     SKIPPED (not just fast) would miss a regression that re-introduces a confirm click the
     owner would read as "start this batch," misrepresenting a batch that already started
When the same component mounts WITHOUT autostart (every existing Tag-sync caller)
Then the existing 'confirm' → click → 'starting' → 'progress' sequence is byte-identical to
     today — autostart is additive, not a replacement, and every current caller must be proven
     unaffected by its addition, not just "not broken in manual testing"

Given a film's Studios row rendered for a non-owner, and the same row rendered for an owner
     outside owner-view mode
When the DOM is inspected
Then the edit pencil is ABSENT (no element, not a hidden/disabled one) in both cases, and the
     POST /films/{id}/studio/cascade endpoint itself returns 401 for a request carrying no
     owner token — matching RD1's "gated purely on owner-view state" and this table's existing
     owner-gating convention for every other mutation (line 191/206 above). A test that only
     checks the frontend pencil's visibility without also hitting the endpoint directly would
     miss a backend-only regression where the route is mounted outside requireOwner
```

**Film poster/thumb self-hosted images — source-scoped image roles (F56, HOLODEX-280, ADR-085/ADR-079) — adversarial**
```
Given a film with no images
When the owner POSTs a valid JPEG to /films/{id}/images/poster
Then film_images gains exactly one row: role='poster', source='upload', and GET
     /films/{id}/images/poster serves it as 200 image/jpeg + immutable cache; the film's
     /api/v1/films and /api/v1/films/{id} payloads both carry the versioned poster_url

Given a film with an existing uploaded poster (version N)
When the owner POSTs a replacement image to the same role
Then film_images still has exactly one role='poster'/source='upload' row (delete+insert scoped
     by film_id+role+source, not an accumulating gallery), the served URL's ?v= advances past N,
     and the prior on-disk file is removed — never left orphaned

Given a film's poster and thumb roles are independent
When the owner uploads only a poster
Then GET /films/{id}/images/thumb still 404s (no cross-role fallback, no placeholder route) and
     the film list/detail payload's thumb_url is omitted, not pointing at the poster

Given the schema's UNIQUE(film_id, role, source) (the one genuine divergence from Studio's
     UNIQUE(studio_id, role))
When an owner-uploaded poster exists for a film
Then every film-image function (GetFilmImage/ReplaceFilmImage/DeleteFilmImage/
     filmImageVersions) is scoped by source, not just role — proven by filmImageVersions
     filtering to source='upload' only, so a future provider-sourced row (HOLODEX-284, not yet
     built) cannot silently collide with or shadow the uploaded one once that writer exists

Given an unauthenticated request
When POST or DELETE /films/{id}/images/{role} is attempted
Then 401/403 (requireOwner), matching every other film mutation endpoint
Given role="banner" (not one of poster|thumb)
When POST /films/{id}/images/banner is attempted
Then 400, and nothing is written to film_images or disk

Given an uploaded file that is actually an SVG/polyglot/decompression bomb
When POST /films/{id}/images/{role} processes it
Then personimage.Normalize rejects it (the same guard every other image upload in the codebase
     passes through) and the request fails without writing to disk — no new decode path

Given no imagesink.Sink dispatch entry exists yet for entityType "film" (deliberately deferred
     to HOLODEX-284, the future enrichment ticket — no provider writer exists in this ticket)
When a film is enriched via any existing provider flow
Then nothing writes to film_images — ReplaceFilmImageFile is reachable only from the owner
     upload handler in this ticket, never from an enrichment code path
```

**StudioLinkCard — shared-query extension and icon/count fallback (HOLODEX-290) — adversarial**
```
Given a studio with no icon_url set (NULL column)
When StudiosForVideos/FilmStudios scan the widened row into model.Studio
Then IconURL scans to the Go zero value "" without error — a naive Scan straight into a
     non-pointer string on a NULL column panics/errors; the fix must coerce NULL to "" (e.g.
     sql.NullString), not merely happen to work against today's seeded, icon-less test data

Given mergeEntity's writeback propagation (internal/api/entity_identity.go:185), which calls
     StudiosForVideos and reads ONLY .Name off each result
When StudiosForVideos' SELECT is widened to also fetch icon_url + a video-count aggregate
Then the existing merge-writeback test suite passes UNMODIFIED — the widened query is additive
     to the result's fields, not a change to which rows come back or in what order, so a test
     that only added new icon/count assertions and skipped re-running that suite could miss a
     regression where the added join/subquery silently changes row cardinality (e.g. a naive
     join against video_studios for the count duplicates a studio row per video it appears in)

Given a video/film linked to two studios, one with icon_url set and one without
When StudioLinkCard renders both in the same row
Then each renders its OWN correct frame — solid border + image for the first, dashed border +
     monogram for the second — proving the icon/no-icon branch is per-card state, not a
     page-level flag that could leak one studio's icon presence into its sibling's fallback
```

---

**Fire-and-forget writeback — absence-means-silence, supersede rule, no auto-retry (ADR-091, HOLODEX-323) — adversarial**
```
Given a video whose writeback job has just completed successfully
 And FinishWriteback has deleted its writeback_queue row
When GET /media/{id} is read
Then the payload reports no pending and no failed writeback
 And the Metadata header renders zero writeback badges
 And the job_runs row for that write still exists (the audit trail is elsewhere)

Given a video with a failed writeback row that the owner has not dismissed
When the owner submits a NEW writeback for that same video
Then the failed row is cleared on enqueue (RD5)
 And a second video's failed row is untouched
 And the header shows pending + out of sync, never failed + pending together

Given a writeback job that has been marked failed
When the queue worker polls for work
Then it never picks the failed row up (ClaimNextWriteback selects only 'pending')
 And attempts stays at 1 — there is no auto-retry to assert
When the owner clicks Retry
Then the row returns to 'pending', the queue is kicked, and the worker claims it

Given a queued writeback sitting behind a large EnqueueMany batch
When the media page renders while that job is still pending
Then BOTH 'writing to file' and 'out of sync' are present (RD6)
 And a test asserting out-of-sync is hidden would be pinning the bug
```

## 11. Known Gaps & Open Questions

- **Browser codec coverage**: real playback depends on browser/codec; E2E asserts the *delivery* (206/range), not decode of every codec — codec matrix is manual/non-goal (transcoding out of scope).
- **Filesystem watcher** (inotify/FSEvents/Windows) is OS-specific; CI validates the Linux container path only — note this explicitly (no silent gap).
- **50k perf dataset**: confirm target hardware profile for the CI perf runner so thresholds are meaningful.
- **Visual regression baseline churn**: decide tolerance/diff threshold to avoid flaky screenshot tests.
- **mkvpropedit/mkv tag XML** authoring in fixtures needs mkvtoolnix in the CI image (in addition to ffmpeg/exiftool) — add to the test image.
- **`ORDER BY RANDOM()` non-determinism** (ADR-031): related-media tests must assert set membership / exclusion / count, never a fixed order or a seeded sequence — over-specifying order would make them flaky. If a future change needs reproducible draws, that's a seed decision to revisit in ADR-031, not a test workaround.
- **F47 enrichment review workflow (ADR-066, HOLODEX-186)**: spec + ADR + design handoff landed 2026-07-12; the test plan above (§4/§5/Phase 3/Critical invariants) is written ahead of S1–S4 implementation — none of it is automated yet. Not a silent gap: tracked against the spec's Timeline step 3.
- **F48 on-demand metadata extraction (ADR-067, HOLODEX-191/192)**: spec + ADR-067 + design handoff + QA checklist landed 2026-07-14; the test plan above (§4/§5/Phase 3/Critical invariants) is written ahead of F48.1–F48.11 implementation — none of it is automated yet. Not a silent gap: tracked against the spec's own Phasing (§Phasing) and ADR-067's Action Items 2–3 (auto-apply stays flagged log-only, and rollback must land, before any write goes live).
- **Scroll-restoration E2E reliability** (ADR-032): the QW4 Back-restoration assertion depends on layout settling before the scroll check; allow a small Y tolerance and wait for the cached grid to paint, or it will flake. First scroll-restoration test in the suite — treat as the reference pattern.
- **F50 tag governance & video enrichment (ADR-075, HOLODEX-224)**: spec + ADR-075 + design handoff + QA checklist landed 2026-07-29; the test plan above (§4/§5/§9/Critical invariants) is written ahead of S1–S9 implementation — none of it is automated yet. Not a silent gap: tracked against the epic's own gate checklist and the spec's suggested slice order (S1 first, as the standalone correctness fix).
- **Tag categories (HOLODEX-240, ADR-078)**: unlike F50 above, backend + frontend are already built (S1–S5, `docs/plans/HOLODEX-240.md`); this update closed the two backend test gaps the plan flagged (`ListCategories` `tag_count`/`tag_ids`, `ResolveOrCreateTag`/`POST /tags`) and extracted+manually-QA'd a shared `PickerShell` out of `EntityPicker`/`CategoryPicker`'s duplicated dialog chrome. Two things remain open, tracked here rather than silently closed: (1) **live 3-skin driven-browser QA of the newer S5 surfaces** — the unified type filter/search, category-pill Manage-mode asymmetry, `/categories/{id}`, the browse Categories facet, and `CategoryPicker`'s `mode="remove"` — blocked last session by a sandboxed dev-server restart denial, `docs/plans/HOLODEX-240.md`'s own "Up next" #1; (2) **`/security-review`** — not started, the new owner-gated mutation surface (`POST/DELETE /categories*`, `POST /tags`) needs the same scrutiny F50's tag mutations got.
- **Alias collapse (ADR-088, HOLODEX-306)**: ADR + design handoff landed 2026-09-02; the test plan above (§4/§5/Critical invariants) is written ahead of the migration, the enrich write path, and the frontend — none of it is automated yet. Not a silent gap: tracked against `docs/plans/HOLODEX-306.md`'s own gate checklist, which per ADR-069 holds the PR in Draft until the spec gate joins this one. Four specifics worth naming rather than discovering later: (1) **nothing today constrains the behaviour being added** — `TestEnrichmentShadowStore` already stores provider `aliases` values in `entity_enrichment` and asserts the multi-value split, so the collapse has no existing test pulling against it and needs its new tests written as the *specification*, not as regression cover; (2) `internal/extract/*_test.go` contains **zero** occurrences of "alias" — the extraction→materialization path stubs its resolver, so alias-driven scan routing is proven at the repo seam (`TestAliasRoutesOnScan`, `TestScanResolvesAliasToCanonical`) but never end-to-end through extract, which is a pre-existing gap this change *widens the blast radius of* without causing; (3) `SetCuration`/`ClearCuration` have no repo-level test for the `aliases` field specifically (only the API-level collision gate), so the D6 migration's source data is itself uncovered — the migration test has to seed those rows by raw SQL rather than leaning on a tested writer; (4) the D3 risk that a junk provider AKA claims files **cannot be closed by any test** — no fixture proves a real library's provider AKAs are sane. That one is a post-deploy observation item on the first enrichment sweep, and ADR-088 records it as an accepted risk bounded by D4/D5, not as something the suite will catch.
- ~~**`ResolveOrCreateTag`'s zero-video-tag visibility gap**~~ — **Resolved (HOLODEX-243, 2026-08-01)**: the product decision this bullet asked for landed as part of implementing the `/tags` "+ New" pill, which needed exactly this fix to satisfy its own P0 goal. `namedCountQuery` gained an `includeZero` param; `ListTags` now left-joins (`ListStudios` still inner-joins — no such gap exists there). See §4/§9/Critical invariants above. Original gap text, for history: a tag created via `/categories/{id}`'s "+ Add tag" control with a brand-new name got zero video associations, and `ListTags`' inner join with `video_tags` made it invisible on `/tags`, in the merge picker, and in search until some video was later tagged with it.
- **`EntityPicker`/`CategoryPicker` focus-restore-to-trigger gap** (found while manually QA'ing this session's `PickerShell` extraction): on Escape/close, focus lands on `<body>` rather than back on the button that opened the dialog. Confirmed **pre-existing** — byte-identical behavior on the code as it stood before the `PickerShell` extraction, not a regression it introduced. Root cause not yet diagnosed (likely the trigger element already being removed from the DOM — e.g. a closed ⋯-menu button — by the time `onMount`'s cleanup reads `document.activeElement`). A real, if minor, a11y regression worth its own fix, separate from this testing-strategy pass.
- **Tag & category create affordance (HOLODEX-243)**: spec + design handoff + implementation landed 2026-07-31/2026-08-01, including the `ListTags` left-join backend fix (§4/§9 above, Go-test-covered). What's left: **no automated Vitest/Playwright coverage** for the new `/tags` create pill/form/near-miss/collision flow — only manual driven-browser QA across all 3 skins this session. Not a silent gap: tracked against the ticket's own gate checklist (spec/design/testing/implementation all done; automated frontend tests are the one open item).
- **F54 configurable provider search patterns (ADR-080, HOLODEX-254)**: spec + ADR-080 + design handoff + QA checklist landed 2026-08-05; the test plan above (§4 backend row, §5 frontend row, §9 adversarial block, Critical invariants) is written ahead of implementation — none of it is automated yet. Not a silent gap: tracked against ADR-080's own Action Items (5 = implementation, 6 = provider-contract doc update, 7 = security-review, the last gated on an actual diff existing). **Numbering note**: this feature was originally mislabeled F53 in its first drafting pass, colliding with the already-shipped Two-tier video poster resolution feature (also F53, HOLODEX-253); corrected to **F54** across ADR-080, the spec, the design docs, and the Jira issue on 2026-08-05 — if an older cached copy of any of those artifacts still says F53, it predates the correction.
- **Fire-and-forget writeback (ADR-091, HOLODEX-323)**: spec + ADR-091 + design handoff landed 2026-09-06; the plan above (§4 row, §5 row, §10 adversarial block, Critical invariants) is written ahead of implementation — none of it is automated yet. Not a silent gap: tracked against `docs/plans/HOLODEX-323.md`'s gate checklist, which per ADR-069 holds PR #303 in Draft. Four specifics worth naming now rather than discovering later: (1) **the `pollUntilSettled` tests are already green and stay green**, which is exactly why they prove nothing about this change — the new assertion is that the *dialog* stopped calling `waitForWritebackJob`, and a suite that only re-runs the module tests will pass against an implementation that never removed the poll; (2) **nothing today constrains the delete-on-success behaviour the whole “silence means success” model rests on** — the existing ADR-073 row asserts `done` for an *absent row* from the job-status endpoint's side, but no test asserts `FinishWriteback` deletes, so the invariant is currently held up by an implementation detail no test pins; (3) **the frontend half inherits §0's standing automation gap** — like every recent UI feature, the badges and disclosures will likely be verified by manual 3-skin QA first, and the hover/focus/tap disclosure requirement is precisely the kind of thing manual QA on a mouse-driven browser passes while the keyboard and touch paths are broken; (4) **security review is not a formality here** — Retry and Dismiss are two genuinely new owner-gated mutation routes, and Dismiss deletes a row, so it deserves the same scrutiny the tag mutations got rather than inheriting “the writeback route was already gated.”
- **F32 video credits → People (HOLODEX-102/HOLODEX-22)**: automated coverage (§9 above) plus live TMDB
  end-to-end QA closed this feature's gate. One item deliberately left out of scope: **HOLODEX-258** tracked
  a sibling pre-existing vulnerability in the analogous `_studio_external_ids` mechanism (F38) — the same
  split-on-first-space parsing existed there and was not hardened as part of this pass, since F32's
  `/security-review` scope was the newly-written person path, not a retrofit of the already-shipped studio
  one. **Fixed** by `sanitizeStudioExternalIDs` — see the Studio external-id de-dup section above
  (HOLODEX-122, ADR-054) and HOLODEX-258.
- **F55 Poster View for the People list page (HOLODEX-255)**: spec + design handoff landed 2026-08-05; the test plan above (§4 backend row, §5 frontend row, §10 adversarial block, Critical invariants) is written ahead of implementation — none of it is automated yet. Not a silent gap: tracked against the PR #215 gate checklist (`/design-handoff` done, `/testing-strategy` closed by this update, `/security-review`/`/architecture` explicitly not required). **Corrections found during design-handoff, folded into this update**: the spec's RD5 (Cinémathèque top bar) and RD6 (Brutalist counter) both assumed `.portrait-frame` had per-skin flourishes mirroring `.video-frame`'s — grounding against the real `app.css` found neither exists on `.portrait-frame` today, so `PersonPosterCard` needs zero new Cinémathèque/Brutalist CSS (Broadcast's real `.portrait-frame::after` scanline applies for free). The spec's two non-blocking open questions (Q1: Merge-button behavior in Poster view; Q2: tier-cap derivation) were locked in by the handoff for buildability — the auto-apply/lockstep test cases above assert those chosen resolutions, not the alternatives.
- **Tag detail hierarchy & categories (HOLODEX-259, epic HOLODEX-240)**: spec + design handoff + backend + frontend all landed 2026-08-07 (`docs/plans/HOLODEX-259.md`). The two new backend queries (`ChildrenForTag`/`CategoriesForTag`) are Go-test-covered (§4 row above); the new `tags/[id]` Hierarchy & categories card is manually driven-browser QA'd across all 3 skins (§5 row above) but has **no automated Vitest/Playwright coverage** — the same standing gap every other recent frontend feature in this file carries at this stage. What remains open on the epic's own gate checklist: `/security-review` of the two new mutation-adjacent read paths — both ride the existing `requireOwner`-gated `SetTagParent`/`ResolveOrCreateTag`/`AssignTagsToCategory`/`UnassignTagsFromCategory` endpoints, no new mutation surface, so the review is expected to be light.
- **Entity Completeness Score (F55/HOLODEX-260, ADR-081/082)**: spec + ADR-081 + ADR-082 (D5 supersession) + design handoff landed across this epic; backend (score computation, browse sort/filter, remediation queue, detail-page field, not-applicable mutation) is Go-test-covered end to end (§4 rows above, Critical invariants above) and the frontend (browse controls, remediation queue page, per-entity breakdown panel) is manually driven-browser QA'd across all 3 skins (§5 row above) but has **no automated Vitest/Playwright coverage** — the same standing frontend-automation gap as every other recent feature in this file. What remains open on the epic's own gate checklist (`docs/plans/HOLODEX-260.md`): `/security-review` of the not-applicable mutation (the one new owner-gated write surface this epic adds). **Numbering collision, found while writing this update, not fixed by it**: this epic's spec/ADR-081/ADR-082/design-handoff/Jira epic all self-label as **F55**, but that number was already claimed by the already-shipped Poster View for the People list page (HOLODEX-255, §4/§5 rows above and the adversarial block above it) as of 2026-08-05 — five days before this epic's spec was written. Unlike the F54 collision (line above, caught and renumbered before any code shipped), this one is not corrected here: HOLODEX-260's ADRs are immutable once accepted (supersede-only, per `docs/architecture/README.md`) and its spec/design-handoff/Jira epic description all cross-reference "F55" by now, so a rename would touch more surface than this testing-strategy pass owns. Every new row/bullet added for this epic in this update disambiguates by Jira key (`F55/HOLODEX-260` vs. bare `F55, HOLODEX-255`) to keep the two apart in this file; a future pass with bandwidth to touch the ADRs/spec should consider a formal supersession to assign HOLODEX-260 its own number.
- **Provider link badge — person/studio (HOLODEX-266, ADR-083)**: architecture + design handoff + backend + frontend landed on branch `HOLODEX-266-provider-link-badge-person-studio` (draft [PR #226](https://github.com/whoiskevinrich/holodex/pull/226)), extending the F55/ADR-082 video badge concept to person/studio detail pages. This update closes the testing gate: `externalLinksForEntity`'s HTTP-layer projection and `Service.BuildProviderLink`'s template resolution — previously untested at both layers, only their lower-level building blocks (`ExternalIDsForEntity`, `ValidateLinkTemplate`, `SanitizeLinkTemplates`) had coverage — are now Go-test-covered end to end against a fake HTTP provider (§4 row above, Critical invariants above), and the frontend's pure helpers (`sortExternalLinks`, `isHttpUrl`) gained unit coverage (§5 row above); the component layer (`ProviderLinkBadge`, `EntityVideoMeta`) stays manual-QA-only, the same standing frontend-automation gap every other recent feature in this file carries. What remains open on the epic's own gate checklist (`docs/plans/HOLODEX-266.md`): `/security-review` of `LinkTemplates` — a malicious provider's `/describe` response must not be able to inject via the URL it constructs — the one gate left before the draft PR can leave draft.
- **Two-tier field editing model (F56/HOLODEX-268, epic HOLODEX-267)**: spec + design handoff + QA checklist landed 2026-08-09/2026-08-10 (`docs/plans/HOLODEX-268.md`); the test plan above (§5 row) is written ahead of implementation — none of it is automated yet, and there is no manual driven-browser QA either, since `SourceBadge.svelte` doesn't exist. Not a silent gap: tracked against the epic's own gate checklist, which now closes this update's gate (`/testing-strategy`). Architecture and security-review are explicitly marked not-applicable in the spec — this is a presentation-layer restructuring of the already-shipped F36/ADR-051 decision model (same API, same RD1–RD5 rules), introducing no new mutation surface. The one open question carried into implementation (from both the spec and the design handoff): whether `SourceSelect.svelte` is deleted now or kept alive for Person's `onadopt`-intercepted name field (F37 RD1) until HOLODEX-269 lands.
- **Unified name-edit mechanism (HOLODEX-269, epic HOLODEX-267)**: spec + design handoff + frontend landed on branch `HOLODEX-269-name-edit-mechanism` (draft [PR #229](https://github.com/whoiskevinrich/holodex/pull/229)). One shared `NameEditControl` (docked-pencil, at-rest visitor-identical, inline edit, async `onCommit`) + `MergeOfferCard` (extracted verbatim from `AliasPanel.svelte`'s existing collision-card markup, contract unchanged — `conflict`/`onmerge`/`onkeepseparate`) replace the three previously-divergent rename UIs (Person's `SourceSelect`/`onadopt` intercept, Studio's `AliasPanel` Rename trigger, Tag's none-existed-before) and add a rename affordance where none existed (Video Title). No backend changes — the audit (`docs/plans/HOLODEX-269.md` session log, 2026-08-10) confirmed `POST /{people|studios|tags}/{id}/rename` already returned parity 204/409 shapes; the one bug found was frontend-only: `renamePerson` (`web/src/lib/api.ts`) didn't unwrap `body.conflict` on 409, masked by a unit test mocking the wrong response shape. Retiring it in favor of the shared `renameEntity('person', ...)` fixes the bug as a parity side effect and is now unit-tested (`api.test.ts`: 204 resolves empty, 409 surfaces `conflict` instead of throwing, other failures still throw `ApiError`) — the prior `renamePerson`-specific tests are gone with the function. `NameEditControl`/`MergeOfferCard` themselves have **no automated Vitest/Playwright coverage** — the same standing frontend-automation gap every other recent feature in this file carries — but got thorough manual driven-browser QA across all 4 entity pages: rename-success on Person/Studio/Tag/Video Title; 409 conflict → `MergeOfferCard` renders as a full-width sibling block (not squeezed into the name row) for Person/Studio/Tag; "keep separate" dismisses cleanly with no state corruption; a full Person merge-into-alias round-trip confirmed ADR-061 D6 (loser's name becomes an alias of the survivor); Video Title confirmed it never shows conflict UI (Video isn't on the identity spine — collision seam stays a no-op checker per the spec, HOLODEX-270 fills it in later) and that the old Title/Name row correctly disappeared from the generic Metadata `dl` (`canonical !== 'title'` filter). **3-skin token check** (Cinémathèque/Broadcast/Brutalist, computed-style reads rather than screenshots per this environment's known screenshot-timeout gap): pencil border/text, edit-form input, and `MergeOfferCard`'s primary/neutral buttons all resolved to distinct per-skin token values (no hardcoded colors bleeding across skins) in all three. `/security-review` closed the epic's last gate: one identification pass over the full diff found zero findings (frontend-only diff, no new backend mutation surface — it consumes the pre-existing owner-gated rename/merge endpoints audited above). All gates on `docs/plans/HOLODEX-269.md` are now green.
- **People attach/detach + relationship picker (F56.5/HOLODEX-272, epic HOLODEX-267)**: spec + design handoff + backend (`FindPeopleCollision`, `Person.Role`, the `TestCurationAPI_PeopleCollision`/`_Suppress`/`TestFindPeopleCollision` coverage) + frontend (`PersonPicker.svelte`) all landed this update; see the §5 row above for the full manual driven-browser QA account (multi-role attach/detach, already-attached-role exclusion, create-fallback, 3-skin token check) — **no automated Vitest/Playwright coverage yet**, the same standing gap every other recent feature in this file carries. **Found and fixed incidentally, out of this story's scope**: `MappedFacets.svelte` crashed app-wide (breaking SvelteKit hydration on every route, not just the video page) on a nil-slice-to-`null` marshaling bug in `/api/v1/facets` — same bug class as the HOLODEX-270 `VideoCollision.People` fix — patched with a minimal `facet.values?.length` frontend guard to unblock this session's QA; the backend-side root cause is a separate, still-open follow-up. `/security-review` of the attach/detach curation-collision gate closed clean — zero findings: the mutation stays inside the existing owner-gated route group, the new collision query and repo changes are fully parameterized, and `PersonPicker.svelte` introduces no unsafe Svelte sink. All gates on `docs/plans/HOLODEX-272.md` are now green.
- **Nil-slice-to-`null` JSON marshaling audit (HOLODEX-275)**: `GET /api/v1/facets` returned `"values": null` for any zero-value facet (Go nil-slice marshaling), crashing `MappedFacets.svelte`'s unguarded `facet.values.length` read and tearing down SvelteKit hydration app-wide — found live while browser-QA'ing HOLODEX-272 (a one-line frontend guard shipped there as an incidental unblock). This update is the backend root-cause fix, following the HOLODEX-270 precedent (commit 90a691f, explicit `[]T{}` init instead of a bare `var`): `Repo.FacetValues`, `Repo.ListVideos`, `Repo.ListPeople`, and `Repo.Search` (all four `SearchResult` fields, including the empty-query early return) now always return a non-nil slice. Fixed at the shared repo layer rather than per-handler, so `GET /api/v1/media`, `/api/v1/people`, `/api/v1/search`, and the person/tag detail video lists — every caller of the four functions — are covered by the one change. New `TestNilSliceRegressions` (`internal/repo/repo_test.go`) asserts all four zero-result paths marshal as `[]`, not `null`. Audited the rest of `internal/api`/`internal/repo` for the same pattern (per HOLODEX-275's own ask) and confirmed the remaining JSON-response slice fields are already guarded (`enrichQueue`/`extractionQueue`/`adminActivityHistory`'s explicit `if rows == nil` checks, `RelatedShelf.Items`'s `make([]T, 0, limit)`, `VideoCollision.People`/`.Studios`'s HOLODEX-270/271 precedent) or not reachable with an empty result (config-sourced fields, request-decode targets, an internal-only helper never serialized directly) — no further instances found. `/security-review` not required: no auth/access/mutation surface touched, read-only query-shape fix only. **Correction (code-review re-pass, same day):** that audit was incomplete — `Repo.ListTags`, `Repo.ListStudios`, `Repo.videoMetadata` (backing `GET /media/{id}`'s `metadata` field), and `GET /media/{id}`'s `studios` field (a `StudiosForVideos` map-lookup miss, nil for any video with no studio link) all had the same bare-`var out []T`/nil-map-lookup pattern, unfixed and unguarded. All four are now fixed the same way as the original four (non-nil init at declaration, or a nil-check guard on the map lookup). `Repo.ListAllVideos` has the same pattern too but is latent, not live — every current caller re-wraps its result before any response — fixed anyway for consistency. `TestNilSliceRegressions` extended to cover all of these at the Go-value level; still does not assert at the JSON-wire level (`json.Marshal` output), which is a standing gap the in-memory-only assertion style can't close — a future regression that reintroduces literal `null` specifically at encoding time (e.g. a stale cache entry, see below) would still pass this test.
- **Writeback hides the target file tag and reports success for fields it silently skipped (HOLODEX-216, parent epic HOLODEX-167)**: the sync writeback path (`POST /media/{id}/writeback` with no durable-queue job, `internal/api/writeback.go`) returned a bare `204 No Content` for a batch mixing mapped and unmapped fields — a field with no `writeback.TagForField`/`ImageTagForField` mapping for the video's actual container was silently dropped from the exiftool/mkvpropedit call, but the dialog still marked its row `done`. Fixed at two layers, matching the design handoff's "show it, never offer it, never lie about it" framing: (1) **backend** — the sync response now carries `{written: string[], skipped: string[]}` instead of 204 (`markWriteTargets` stamps a new `resolver.ResolvedField.WriteTarget` onto each field at the API layer post-resolve, the same "markPromoted" pattern `field_promotions.go` already established for API-layer-only data the entity-generic resolver can't itself know); `TestWritebackEndpoint_MixedBatchReportsWrittenAndSkipped` (`internal/api/writeback_test.go`) posts `title`+`director` together against an MP4 fixture with only `title` mapped and asserts `written=["title"], skipped=["director"]` plus that exactly one exiftool call landed; `TestGetMedia_WriteTarget` asserts `GET /media/{id}` surfaces `write_target="QuickTime:Title"` for the mapped field and `""` for the unmapped one; `TestGenreWritebackEndpoint` (`internal/api/genre_writeback_test.go`) updated for the 204→200 response-shape change. The nil-`unmapped`-slice-to-`[]` fix follows the HOLODEX-275 precedent directly above. (2) **frontend** — `write_target` flows through `ResolvedField` (`types.ts`) into a new pure predicate `isWritable` (`f36.ts`, unit-tested in `f36.test.ts`) that `needsWriteback` now also requires, so an unwritable field can never auto-check into a write that would only silently drop it; `WritebackFormDialog.svelte`'s row rendering shows the destination tag (`→ QuickTime:Title`) next to the provenance tag for a writable row and swaps the checkbox for a muted non-interactive icon + "No file tag for this container — can't be written to file" copy for an unwritable one (no `disabled:opacity-*` on `text-muted`, per the standing theming rule); `submit()`'s success handling now branches on the sync response's actual `written` set rather than assuming every checked row succeeded, and `onclose()` only auto-fires once every submitted row reached `done`. All 143 Vitest tests and the full `go test ./...` suite pass; `npm run check` is clean (0 errors). **Scoped down from the ticket's full ask**: the durable-queue (async, 202) path still reports only `job_id`/`queued` — no per-field written/skipped outcome after the job completes — since production always wires the queue (`cmd/holodex/main.go`), making the sync path's fix the one that closes the "reports success for fields it silently skipped" defect for the UI (which now never offers an unwritable field regardless of which path executes). Exposing structured per-field outcomes on the async job-status endpoint would need new `job_runs` columns and is deliberately deferred to a follow-up issue rather than expanding this fix's blast radius. `/security-review` not applicable (no auth/access/infrastructure touched). Design handoff: [writeback-target-visibility-handoff.md](design/writeback-target-visibility-handoff.md).
- **Films entity (F56, HOLODEX-279, epic)**: spec + ADR-085 + design handoff landed 2026-08-09/2026-08-23 (`docs/plans/HOLODEX-279.md`); this update closes the epic's testing-strategy gate. Unlike a from-scratch "planned" feature, the **backend is already built and Go-test-covered** — film CRUD/attach/detach/scene-numbering (`internal/repo/films_test.go`, `internal/api/films_test.go`), the union-of-scenes baseline (`internal/resolver/film_baseline_test.go`), and the RD7 resolver-source injection/suspend/disambiguation plus the P0-2 zero-relink-participation guard (`internal/resolver/film_source_test.go`, `internal/api/film_injection_test.go`, `internal/api/film_links_test.go`) — see the three new §4 rows and the new Critical-invariants bullets above. **What's not yet built, tracked here rather than silently assumed done**: (1) the entire frontend (`/films` list + detail, `FilmAttachDialog`, the film-side bulk picker, films rows on person/studio/tag pages) — §5's new row records target coverage against the design handoff, no code exists yet; (2) ~~RD6's subtractive video-list hiding across browse/RelatedShelf/landing/search/`EntityVideos.svelte` and its films-row compensating surface — no test file for this half of RD6 was found during this update, so it is either not yet implemented or implemented without coverage~~ — **Resolved (HOLODEX-282, 2026-08-25)**, see the dedicated bullet below; (3) `/security-review` of the new owner-gated `POST/DELETE /films*` attach/detach mutation surface — not started, `needs-security-review` label still on the Jira epic; (4) ~~the two filed backend follow-ups, HOLODEX-280 (poster/thumbnail asset pipeline) and HOLODEX-281 (`film_people_roles` film-level cast/role CRUD), deliberately deferred out of this vertical slice per the epic's own worklog~~ — HOLODEX-281 **resolved (2026-08-27)**, see the dedicated bullet below; HOLODEX-280 remains open.
- **RD6 subtractive video-hiding for full-film files (HOLODEX-282, epic HOLODEX-279)**: closes item (2) of the epic bullet above — the "Known Gap" flagged during the HOLODEX-279 testing-strategy pass (no `VideoFilter`/repo code implementing the hide existed anywhere). Implemented as a single `VideoFilter.HideFullFilmVideos` flag consumed by `internal/repo/repo.go`'s `build()` (the same `NOT EXISTS (SELECT 1 FROM film_videos ...)` exclusion-join pattern already used for `ExcludeAttachedToFilmID`), plus threaded through `Search`, `Related`/`relatedShelf`/`randomSiblings`. Every list-surface caller sets it from `h.filmsEnabled`: `listMedia` (browse), `search`, `getRelated` (RelatedShelf), `getPerson`/`getTag`/`getStudio` (the reads backing `EntityVideos.svelte` on person/studio/tag pages), and `completenessFacets`. The landing page's recently-added rail and `EntityVideos.svelte` ride these same endpoints, so no separate surface-specific code was needed for either. New `TestHideFullFilmVideos` (`internal/repo/films_test.go`) covers `ListVideos`/`GetVideo`/`Search`/`Related` with the flag on and off; new HTTP-level `TestFullFilmVideoHiddenFromListSurfaces` (`internal/api/film_candidates_test.go`) asserts all six surfaces (`/media`, `/search`, `/media/{id}/related`, `/people/{id}`, `/tags/{id}`, `/studios/{id}`) exclude a full-film video while its own `/media/{id}` detail page still returns 200 — directly exercising the adversarial scenario written ahead of implementation earlier in this doc (§9/§10, "Films entity — asserted links, subtractive flag..."). **Deliberately scoped out, and regression-tested against**: the owner-only film→video attach picker (`filmVideoCandidates`, `internal/api/film_videos.go`) reuses the same shared `videoFilterFromQuery` builder as `listMedia`, but must keep surfacing full-film videos (to reveal "already attached to another film" conflicts) — the flag is therefore set per-caller rather than inside the shared builder, and `TestFilmVideoCandidates` was strengthened (its `elsewhere` fixture now attaches with `isFullFilm=true` instead of `false`) to prove the hide doesn't leak into the picker. Frontend: `FilmAttachDialog.svelte`'s "entire film" checkbox helper copy — written in future tense pending this feature during the HOLODEX-279 pass — updated to present tense now that the behavior is live. `/security-review` not applicable: a read-path visibility filter, no new mutation surface.
- **Film-studio cascade edit affordance (F57, HOLODEX-285, ADR-087)**: spec + ADR-087 + design handoff landed 2026-08-25 (`docs/plans/HOLODEX-285.md`); this update closes the epic's `/testing-strategy` gate (3/7 → 4/7 on the worklog's own checklist). The test plan above (§4's two new rows, §5's new row, §10's adversarial block) is written entirely ahead of implementation — no backend or frontend code exists yet, so none of it is automated. Not a silent gap: tracked against the worklog's own "Up next" queue, which explicitly orders `/security-review` next, then the four backend/frontend implementation steps, in that sequence — per ADR-069, no implementation code is written until both remaining gates (testing, closed by this update, and security) are green. ADR-087 itself stays **Proposed** (not Accepted) until `/security-review` closes its Action Item 7 (new owner-gated bulk-mutation endpoint touching N videos' decisions and file writes in one action).
- **film_people_roles CRUD (HOLODEX-281, epic HOLODEX-279)**: resolves the HOLODEX-281 half of the epic bullet's deferred item (4) above. Owner-gated `POST/PUT/DELETE /films/{filmId}/roles...` (`internal/api/film_people_roles.go`, mirroring `film_videos.go`'s attach/detach shape) over the already-migrated `film_people_roles` table (migration 0043, ADR-085) — a person's film-level role/billing_order, additive and separate from the per-video `video_people` link (migration 0037), constrained at the app layer to one credited row per person per film (`repo.ErrFilmPersonAlreadyCredited`) so the role text stays addressable by `(filmId, personId)` rather than needing a role-string-in-URL scheme. Go-test-covered end to end: `internal/repo/film_people_roles_test.go` (add/edit/remove, re-add conflict, edit/remove-uncredited `ErrNotFound`, and that `FilmCast`'s inherited union stays unaffected by crediting) and `internal/api/film_people_roles_test.go` (the same at the HTTP layer, plus unauthenticated-401/403 and unknown-person-404, and confirming `getFilm`'s new `credited_roles` field is independent of the pre-existing read-only `cast` field). No new migration — API-surface-only over ADR-085's existing schema, no ADR/spec/design update needed. `/security-review` closed clean (owner-gated via the existing `requireOwner` group, parameterized SQL, film/person existence checked before mutation, no new data exposure). **Deferred, tracked rather than silently assumed done**: the frontend surface (film detail page add/edit/remove UI for credited roles) — `docs/plans/HOLODEX-281.md`'s "Up next" queue.
- **TagLinkChip — shared tag display (HOLODEX-292)**: design handoff landed 2026-08-29 (`docs/design/tag-link-chip-handoff.md`); this update closes the story's `/testing-strategy` gate. Pure frontend markup consolidation — no backend/API/schema change, so no new Go test coverage is needed (mirrors `StudioLinkCard`'s HOLODEX-290 precedent directly above in scope, not just in shape). No new automated Vitest/Playwright coverage either — the same standing frontend-automation gap every other recent presentational component in this file carries — but this one got thorough manual driven-browser QA: owner view (`/media/{id}` as owner) confirmed the `·file`/`·provider` suffix and hover-reveal remove button render identically to the pre-extraction markup; visitor view (`/media/{id}` unauthenticated) confirmed **no** provenance suffix leaks through (see next sentence); Film detail (`/films/{id}`) confirmed the always-read-only variant with no eyebrow label. **Regression caught by this QA pass, not by review**: the first draft of `TagLinkChip` showed the provenance suffix unconditionally, which would have been a new capability exposed to non-owner visitors that neither Media's prior visitor branch nor Film's prior markup ever had — fixed by gating the suffix on `onremove` (owner-only) before this gate closed, and confirmed fixed by re-running the visitor-view check. 3-skin token check (Cinémathèque/Broadcast/Brutalist) passed — no hardcoded colors/radii in the new component. `/security-review` and `/architecture` not applicable per the handoff's own "why no spec/ADR" section (no new capability, no schema/cross-cutting decision). People details is explicitly out of scope for this story (tracked separately, HOLODEX-39) — see the handoff's callout.
- **`categories/[id]` migrated to shared TagLinkChip (HOLODEX-293, follow-up to HOLODEX-292)**: the category detail page was the one caller HOLODEX-292 left unmigrated (out of that story's Media/Film-only scope) — still had a byte-near-identical pre-extraction inline owner/visitor chip pair. Swapped to `<TagLinkChip tag={t} busy={tagBusy} onremove={isOwner ? removeTag : undefined} />`, deleting the duplicated markup and a stale comment pointing at code no longer present on the Media page. Pure like-for-like markup swap onto an already-designed, already-reviewed component — no spec/ADR/design-handoff gate, same precedent as HOLODEX-292 itself. `npm run check`: 0 errors. Manual driven-browser QA: owner view (add/remove a tag, confirm the remove button + full padded hit-target render); visitor view (confirm the read-only pill is the full 72×29.6px `<a>`, not just the text); both checked across Cinémathèque, Broadcast, and Brutalist. Category tags carry no `source` field, so the provenance suffix simply never renders here — matches Film's existing behavior, not a new code path. `/security-review` not applicable (no auth/access/infrastructure touched).
