---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-260                 # the tracker key; must match the branch key regex
status: in-progress                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Owner mode can now sort/filter entities by how complete their metadata is, and work a remediation queue to fill in the gaps.
---

# HOLODEX-260 · Entity Completeness Score (F55)

A per-entity completeness score (weighted, tiered by source trust) plus a separate actionability
signal, surfaced as an owner-mode browse sort/filter, a facet-first remediation queue, and a
per-entity breakdown panel — done when the owner can find and fix metadata gaps without scrolling
the library by eye. Ships as **one release**, not phased (explicit owner call during brainstorming).

**Design package:** [entity-completeness-score.md](../specs/entity-completeness-score.md) · [ADR-081](../architecture/ADR-081-entity-completeness-score.md) · [design handoff](../design/entity-completeness-handoff.md) · testing-strategy TBD

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/entity-completeness-score.md`
- [x] architecture `architecture` → `docs/architecture/ADR-081-entity-completeness-score.md`
- [x] design `design-handoff` → remediation queue, breakdown panel, browse filter/sort, all three skins
- [/] backend
- [ ] frontend
- [ ] testing `testing-strategy`
- [ ] security `security-review` — new owner-gated not-applicable mutation

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [architecture] ADR: facet criticality metadata, `facet_not_applicable` table, score/actionability computation seam — `docs/architecture/ADR-081-entity-completeness-score.md`
2. [x] [backend] registry criticality metadata (D1) + `facet_not_applicable` table/mutation (D2) — `internal/registry/registry.go`, `internal/db/migrations/0039_facet_not_applicable.{up,down}.sql`, `internal/repo/facet_not_applicable.go`, `internal/api/facet_not_applicable.go`
2b. [x] [backend] score/actionability computation (D3) — `internal/resolver/complete.go`
2c. [x] [backend] list-wide resolve-all backend predicate (D4) — `internal/api/completeness.go`, generic `*ForEntities` batch loaders in `internal/repo/{enrichment,curation,decisions,facet_not_applicable}.go`
3. [ ] [backend] `imdb_id` → `external_provider_id` rename migration across the 9 `field_key`-keyed tables — `internal/registry/registry.go`, `internal/db/migrations/`
4. [ ] [frontend] browse "Completeness" sort + "Missing facet" filter chip (reuse `FacetFilter.svelte`, `SortDropdown`) — `web/src/routes/+page.svelte`, `web/src/routes/people/+page.svelte`, `web/src/routes/studios/+page.svelte`
5. [ ] [backend+frontend] facet-first remediation queue (candidate-ready vs needs-research, individual apply/search/upload) — new `web/src/routes/owner/completeness/+page.svelte`, backend predicate shared with #4
6. [ ] [frontend] per-entity completeness breakdown panel — video/person/studio detail pages
7. [ ] [testing] F55 block in `docs/testing-strategy.md`
8. [ ] [security] `/security-review` on the not-applicable mutation

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-08 · Backend D4 — list-wide resolve-all predicate for browse sort/filter + remediation queue
- skills: simplify
- handoff: implemented item #2c — the ADR-081 D4 backend predicate all three future consumers
  (browse completeness sort, the missing-facet filter, the remediation queue) will share, per
  the design handoff's §9/§1 "same backend predicate" requirement. New `internal/api/completeness.go`:
  `completenessForVideos`/`completenessForPeople`/`completenessForStudios` on `*Handlers`, each
  fetching the full per-type entity set (`ListAllVideos`/`ListPeople`/`ListStudios`, bypassing SQL
  `LIMIT`/`OFFSET` per D4's explicit call) and replicating its detail handler's exact resolve
  pipeline (`mergePromotions` → `mergeClaims` → resolve → `markPromoted` → `appendAutoRegistered` →
  person-only `Derive`) in a loop over batch-loaded inputs, then scoring via D3's `resolver.Complete`.
  Batch loading needed four entity-type-parameterized siblings of the existing video-only bulk
  loaders — `EnrichmentForEntities`/`CurationForEntities`/`DecisionsForEntities`/
  `FacetsNotApplicableForEntities` — with the three that had video-specific predecessors refactored
  into one-line delegating wrappers (zero behavior change for existing callers).
  Self-found correctness gap: D3's own note flagged that studio `branding_image` has no resolver
  row (asset-only, F51/ADR-079), but I found the identical gap on person `photo` too — both are
  Critical-weighted facets that `personFields()`/`studioFields()` explicitly exclude because
  they're "delivered as an asset, not a field value," so `resolver.Complete` would have silently
  under-scored every person and studio. Fixed with a shared `injectAssetFacet` helper (modeled on
  `Derive`'s `insertComputed`, but appending to both `fields` and `resolved` since — unlike a
  computed/display-only row — these are genuinely scored facets) wired off data both list functions
  already batch-load for free (`ListStudios`→`attachStudioImages`→`ImageVersions`,
  `ListPeople`→`attachPersonImageVersions`→`HeadshotVersion`). Present scores at the curated tier
  (`"manual:"` winning source) — an asset is binary present/absent with no unapplied-candidate
  concept the way text fields have. **Open judgment call for the user to confirm**: `photo`
  presence is keyed on `HeadshotVersion != 0` only, not compositing with `PosterVersion` the way
  `branding_image` composites icon/logo/poster — the spec's person facet table uses singular
  "Portrait image" language (unlike branding_image's explicit "composite" language) and
  `PersonImageHeadshot`'s doc comment calls it "the default avatar," but `PosterVersion`'s own doc
  comment ties it to "F55 P0-6" and neither the spec nor ADR-081 elaborate what P0-6 required —
  flagging rather than treating as settled. New tests: repo-layer entity-type-isolation tests for
  all four `*ForEntities` loaders (a person id and video id sharing a numeric value must not
  cross-contaminate) in `internal/repo/{enrichment,curation,decisions,facet_not_applicable}_test.go`
  + new `internal/repo/curation_test.go`; API-layer `internal/api/completeness_test.go` (internal
  `package api`, since it calls the unexported `completenessFor*` methods directly) covering video
  critical-facet scoring, not-applicable exclusion, and both synthetic-facet injections with a
  before/after-image score assertion each. Ran `/simplify` (4-agent pass): reuse and altitude came
  back clean (the asset-facet placement and `ListAllVideos` were checked against `Derive`/
  `ExtractionQueue` precedent and confirmed as the right depth, not special-casing); efficiency
  flagged a redundant `registry.Lookup` label lookup recomputed every loop iteration for a
  loop-invariant canonical — hoisted `photoLabel`/`brandingLabel` above their loops; simplification
  flagged repeated video/person-seed boilerplate across both new test files — extracted
  `seedVideo` (`completeness_test.go`) and `seedVideoAndPerson` (`repo_test.go`) helpers. `go build`,
  `go vet`, and the full `go test ./...` are all clean — no regressions. Next: item #3 (imdb_id →
  external_provider_id rename), or the D4 frontend consumers (item #4, browse sort/filter) if
  picked up first — either way, surface the photo/PosterVersion compositing question to the user
  before the breakdown panel (item #6) ships, since it renders per-facet status and would need to
  agree with whatever scoring decides.

### 2026-08-07 · Backend D1+D2 — facet criticality metadata + not-applicable mutation
- skills: simplify
- handoff: implemented the narrower half of item #2 — D1 and D2 only, D3 (score/
  actionability computation) deliberately deferred to a follow-up session. D1: added
  `Criticality` (`CriticalityCritical`/`CriticalityNiceToHave`/`""`) to
  `registry.FieldDef` and tagged every P0-scored facet per the spec's per-entity-type
  tables (video/person/studio), including a new synthetic `branding_image` `FieldDef`
  entry so the studio icon/logo/poster composite facet (F51/ADR-079) has one
  code-reviewed home for its weight. D2: new `facet_not_applicable(entity_type,
  entity_id, canonical_field, created_at)` table (migration 0039, modeled on
  `person_image_suppressions`/0012) + `internal/repo/facet_not_applicable.go`
  (Set/Clear/FacetsForEntity, mirroring `decisions.go`'s shape) + owner-gated
  `PUT`/`DELETE /media/{id}/fields/{canonical}/not-applicable` in
  `internal/api/facet_not_applicable.go`, wired into `Mount`. Video-only for v1,
  matching the codebase's existing pattern of separate per-entity-type route files
  (`decisions.go`/`person_decisions.go`/`studio_fields.go`) rather than one generic
  route — person/studio not-applicable mutation is a follow-up, not a gap in this
  session's scope. New tests: `registry_test.go` (criticality tagging + Computed
  auto-exclusion invariant), `facet_not_applicable_test.go` in both `repo` and `api`
  (CRUD/idempotency, 404/409 validation, owner-gating). Ran `/simplify`: reuse and
  efficiency came back clean; one simplification finding (`created_at` looking
  redundant against the table's own "no other columns" comment) and two altitude
  findings (video-only route, cross-entity-type validation looseness) were all
  checked against precedent and skipped as consistent with established patterns
  elsewhere in the codebase (see PR description for the full reasoning). `go build`,
  `go vet`, and the full `go test ./...` are all clean — no regressions. Next: D3
  (score/actionability compute-on-read post-pass in `internal/resolver`, item #2b),
  then item #3 (imdb_id → external_provider_id rename).

### 2026-08-07 · Backend D3 — score/actionability computation in internal/resolver
- skills: simplify
- handoff: implemented item #2b — `internal/resolver/complete.go`'s `Complete(fields
  []mapping.Field, resolved []ResolvedField, notApplicable map[string]bool)
  Completeness`, a pure post-pass mirroring `Derive`'s shape (no clock needed, unlike
  `Derive`'s Age). Per-facet tier is read straight off `ResolvedField.WinningSource`
  (empty → missing/0.0; `file:`/`manual:` namespace via `fieldsource.ForNamespace` →
  curated/1.0; anything else → provider/0.7) — no new per-field resolution logic, per
  D3. Score is `round(100 * Σ(weight*tier) / Σ(weight))` over non-not-applicable
  scored facets (weight 3 critical / 1 nice-to-have, matching the spec's worked
  example exactly — reproduced verbatim as `TestComplete_WorkedExample`).
  Actionability is `nil` (not 0) when there are no missing facets, since the ratio is
  undefined; otherwise the fraction of missing facets with a cached unapplied
  provider candidate, read off `ResolvedField.Candidates` (F36/ADR-051) rather than
  re-deriving availability — merge fields (Candidates always nil per RD1) are
  correctly never actionable when missing. One deliberate divergence from the ADR's
  suggested signature: `Complete` takes the full `fields []mapping.Field` list, not
  just `resolved`, because `ResolveFields` silently drops an empty/undecided field's
  row entirely — without `fields`, a genuinely missing scored facet could have no row
  at all and would silently vanish from the score's denominator. Studio's synthetic
  `branding_image` facet (no resolver row, resolved via `studio_images` directly) is
  deliberately out of scope here — a future D4 caller injects it as a synthetic row
  before calling `Complete`, same pattern `Derive` uses for computed rows. New tests:
  `complete_test.go` (worked example, no-missing-facets, all-excluded/zero-score,
  Computed-never-scored, merge-field-missing-never-actionable). Ran `/simplify`
  (4-agent pass): reuse + altitude both flagged the same duplication —
  `classifyTier`'s inline file/manual-vs-provider check re-derived what
  `fieldsource.ForNamespace` already owns — fixed by delegating to it; simplification
  flagged the tier weight/name pair as two parallel const blocks that could drift —
  collapsed into a single comparable `tier` struct with three package vars; efficiency
  came back clean. `go build`, `go vet`, and the full `go test ./...` are all clean —
  no regressions. Not yet wired into any API endpoint — that's D4 (list-wide
  resolve-all for browse sort/filter + the remediation queue), still open. Next: #3
  (imdb_id → external_provider_id rename), or D4 if picked up first.

### 2026-08-07 · Design gate closed — remediation queue + breakdown panel handoff written
- skills: design-handoff, graphify, simplify
- handoff: wrote `docs/design/entity-completeness-handoff.md`, covering both requested surfaces
  plus §9 (browse sort/filter) to close the whole design gate in one document. Followed the
  repo's real handoff convention (numbered DDn decisions ending in "chosen over," not the
  generic Figma-oriented template) after researching prior art via a subagent. Remediation
  queue (DD1-3): grouped by facet, critical-first then count-descending, candidate-ready rows
  above needs-research within a group, one `.btn-accent` action per row for both apply-mutate
  and search-navigate (explicitly not color-coded, not two co-equal buttons) — visual language
  only from `ExtractionQueueRow.svelte`/HOLODEX-199, not the component itself, since this queue
  groups by facet with a uniform row shape vs. extraction's per-video heterogeneous editors.
  Breakdown panel (DD4-8): new owner-only wholesale-gated card high on the three detail pages;
  score bar using only `bg-surface-2`/`bg-accent`; facets split Critical/Nice-to-have; status
  pills map the new tri-state vocabulary onto existing idioms — curated=accent pill,
  provider=reused `ProvenanceBadge`, missing=dashed pill echoing `CurationChip`'s pending motif,
  not-applicable=plain muted text; not-applicable toggle reuses the tag-detail writeback
  toggle's dt/dd + icon-button shape with no `ConfirmDialog` (direct toggle, not a reparent-style
  confirm). Rendered a Cinémathèque-skin mockup via the visualization tool for both surfaces.
  Next: item #2 in Up next — backend registry criticality metadata + `facet_not_applicable`
  table/mutation + score/actionability computation.

### 2026-08-07 · Architecture gate closed — ADR-081 written
- skills: architecture, design-handoff, graphify
- handoff: wrote `docs/architecture/ADR-081-entity-completeness-score.md`, resolving the four
  things the spec punted to an ADR. D1: facet criticality is a new static `Criticality` field on
  `registry.FieldDef` (reuses the existing `Computed` bool for auto-exclusion, no double
  bookkeeping). D2: not-applicable persists in a new dedicated `facet_not_applicable(entity_type,
  entity_id, canonical_field)` table — modeled on `person_image_suppressions` (migration 0012),
  explicitly rejected as a 4th `field_source_decisions.source` value because relevance and
  source-selection are different questions (same overload mistake ADR-063 already avoided for
  `computed:`). D3: score/actionability are a pure compute-on-read post-pass in `internal/resolver`
  keyed off the already-computed `ResolvedField.WinningSource` (missing=0/provider=0.7/curated=1.0
  tier table), no new per-field resolution logic, no storage — twin of ADR-063's `Derive`. D4:
  browse-sort, the missing-facet filter, and the remediation queue all resolve+score the full
  per-type entity set in Go rather than pushing into SQL `ORDER BY`/`WHERE`, bounded by this app's
  personal-library scale (explicitly flagged as the first thing to revisit if that changes). D5:
  `imdb_id` → `external_provider_id` is a straight rename — registry edit + one data migration
  rewriting stored `field_key` strings across the 9 tables that key on it (confirmed via grep:
  `field_source_decisions`, `entity_enrichment`, `metadata_curation`, `provider_field_hints`,
  `field_promotions`, `metadata_extraction_review`, `file_writeback_snapshots`,
  `file_writebacks`, `field_claims`), no parallel legacy field, since production never populated
  the old name. Added the ADR-081 row to `docs/architecture/README.md`'s index and flipped the F55
  Phase-specs line's "ADR TBD" to the real link. Next: `/design-handoff` for the remediation queue,
  breakdown panel, and browse filter/sort, then backend implementation of D1–D3 (item #2 in Up
  next).

### 2026-08-07 · Spec written, Jira epic + stories created, branch/issue wired up
- skills: product-brainstorming, write-spec, architecture
- handoff: brainstormed the completeness-score design end to end (two-metric split —
  completeness score vs. a non-score-affecting actionability signal; tri-state
  resolved/missing/not-applicable facet status; critical/nice-to-have weighting; generalizing
  `imdb_id` to a provider-agnostic `external_provider_id`; individual-apply-only remediation
  queue per the HOLODEX-199 precedent), then wrote the full spec at
  `docs/specs/entity-completeness-score.md` (facet tables per entity type, scoring formula +
  worked 65% example, F55.1–F55.18 requirements, data/frontend/security sections, ships as one
  release per an explicit owner override of my phased-rollout recommendation). Created Jira epic
  HOLODEX-260 plus five child stories (HOLODEX-261–265: scoring engine, browse sort/filter,
  remediation queue, breakdown panel, not-applicable affordance) with `needs-adr` /
  `needs-design` / `needs-security-review` / `needs-testing-strategy` labels on the epic. Renamed
  the worktree branch to `HOLODEX-260-entity-completeness-score` and fired the Jira In Progress
  transition. Added the F55 line to `docs/architecture/README.md`'s Phase specs index. Next:
  open the Draft PR with the spec (first pre-implementation gate landing, ADR-069), then start
  the ADR — it blocks the `external_provider_id` generalization and the not-applicable
  persistence shape before any backend work.
