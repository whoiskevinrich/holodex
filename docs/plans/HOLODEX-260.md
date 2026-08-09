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

**Design package:** [entity-completeness-score.md](../specs/entity-completeness-score.md) · [ADR-081](../architecture/ADR-081-entity-completeness-score.md) (+[ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md), supersedes D5) · [design handoff](../design/entity-completeness-handoff.md) · testing-strategy TBD

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] spec `write-spec` → `docs/specs/entity-completeness-score.md`
- [x] architecture `architecture` → `docs/architecture/ADR-081-entity-completeness-score.md` (D5 superseded by `docs/architecture/ADR-082-external-provider-id-namespace-qualified-value.md`)
- [x] design `design-handoff` → remediation queue, breakdown panel, browse filter/sort, all three skins
- [x] backend
- [x] frontend
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
3. [x] [backend] `imdb_id` → `external_provider_id` rename, value namespace-qualified per ADR-082 — `internal/registry/registry.go`, `internal/db/migrations/0040_rename_imdb_id_field_key.{up,down}.sql`, `providers/tmdb/tmdb.go`, `providers/tmdb/handler.go`
4. [x] [frontend] browse "Completeness" sort + "Missing facet" filter chip (reuse `FacetFilter.svelte`, `SortDropdown`) — `web/src/routes/+page.svelte`, `web/src/routes/people/+page.svelte`, `web/src/routes/studios/+page.svelte`
5. [x] [backend+frontend] facet-first remediation queue (candidate-ready vs needs-research, individual apply/search/upload) — new `web/src/routes/owner/completeness/+page.svelte`, backend predicate shared with #4
6. [x] [frontend] per-entity completeness breakdown panel — video/person/studio detail pages
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

### 2026-08-09 · Item #6 — per-entity completeness breakdown panel (F55.13-15)
- skills: simplify
- handoff: implemented item #6, closing the frontend gate — the last item in the design
  handoff's DD4-8. Backend: `getMedia`/`getPerson`/`getStudio` (`internal/api/handlers.go`,
  `person_fields.go`, `studios.go`) now expose the `fields []mapping.Field` slice each
  already computes during its existing resolve pass (not a second re-resolve), so a new
  owner-gated `completeness` field on all three detail responses can call `resolver.Complete`
  against the same pass — mirrors the pre-existing `enrich_queries` owner-gating pattern.
  `personResolved`→`personResolve` and `studioResolved`/`resolveStudio` widened to return
  `(resolved, fields)` instead of just `resolved`; all call sites were single-chain, updated
  safely. Found and fixed a real gap while wiring this up: `FacetScore.Provider` was only set
  for missing+actionable facets, but DD7 needs it for already-curated provider-tier facets
  too (so the panel's Provider pill can render `ProvenanceBadge` with the actual winning
  namespace) — extended `resolver.Complete`'s per-facet loop with a `case providerTier` branch
  and a new `winningNamespace` helper (refactored out of `classifyTier`'s existing
  `strings.Cut`). New `internal/api/completeness_detail_test.go`: three tests (one per entity
  type) asserting owner-present/visitor-omitted `completeness` shape. Frontend: new
  `CompletenessPanel.svelte` (`components/completeness/`) implements DD4-8 — score bar +
  actionability line ("Fully complete" fallback), facets grouped Critical/Nice-to-have, a
  four-state status pill (Curated=accent outline, Provider=`ProvenanceBadge`,
  Missing=dashed-muted, Not applicable=plain text), and the video-only not-applicable toggle
  on `external_provider_id` reusing the tag-detail page's exact icon-button idiom. Wired into
  all three detail pages (`media/[id]`, `people/[id]`, `studios/[id]`) right after each page's
  primary Metadata/Details card, per DD4's placement rule. QA'd live in the browser end to end
  on both testbeds (backend-amv for media+studio, backend-films for the DD8 toggle since amv's
  local mapping has no `external_provider_id`-mapped field to exercise it) — score bar/pill
  colors verified via computed styles across all three skins (screenshots time out on this
  app), and the not-applicable toggle round-tripped correctly (score recalculates, pill flips
  to "Not applicable", `onchanged` refetches). Also fixed a stale local dev config found during
  QA: `metadata-mappings.local.films.yaml` (gitignored, shared across worktrees, lives at the
  main repo root) still mapped the pre-ADR-082 `imdb_id` canonical instead of the renamed
  `external_provider_id` — updated so the DD8 toggle has something to exercise on that testbed;
  not part of this commit since the file is gitignored. Ran `/simplify` (4-agent pass): reuse
  flagged the not-applicable toggle button hand-forking `border-accent text-accent` /
  `border-rule text-muted hover:text-ink` instead of reusing the shared `.btn-accent`/
  `.btn-ghost` treatments from `app.css` (fixed — also picks up their disabled-state styling,
  which the hand-rolled version was missing entirely); simplification flagged `busyFacet`
  tracking facet identity as a string when only one facet can ever be busy (the toggle only
  ever renders for `external_provider_id`) — simplified to a plain `busy` boolean. Altitude and
  efficiency findings were reviewed and skipped: `getMedia`'s hoisted `var mfields` (altitude)
  would need a larger `resolveMedia`-style extraction to match the person/studio pattern —
  correct but out of scope for this diff, worth a follow-up if `getMedia` grows again; the
  toggle's hardcoded `external_provider_id` canonical (altitude) is intentional v1 scope per
  the design handoff's own text ("v1's only UI target is external_provider_id"), not a bug;
  sequential (not parallelized) `FacetsNotApplicableForEntity` reads (efficiency) are
  negligible for a single owner-only detail-page GET at this app's personal-library scale.
  `go build`/`go vet`/`go test ./internal/resolver/... ./internal/api/...` and `npm run check`
  (0 errors) both clean before and after the simplify fixes; re-verified the toggle button's
  `.btn-accent`/`.btn-ghost` classes live in the browser post-fix. Next: item #7 (F55 block in
  `docs/testing-strategy.md`) and item #8 (`/security-review` on the not-applicable mutation) —
  the two remaining gates before this epic's Draft PR can come out of draft.

### 2026-08-08 · Item #5 — facet-first remediation queue (F55.7/F55.8)
- skills: simplify
- handoff: implemented item #5, closing the frontend half of the remediation queue and the last
  backend endpoint before the breakdown panel (item #6). Backend: `internal/api/completeness_queue.go`
  (`GET /owner/completeness-queue`) reshapes the D4 `completenessFor*` predicate by facet instead of
  by entity — every video/person/studio's missing scored facets, grouped, pre-sorted critical-first-
  then-count-desc, and pre-split candidate-ready/needs-research (DD1) so the frontend does zero
  client-side grouping. `resolver.FacetScore` gained a `Provider` field so candidate-ready rows carry
  which provider's cached candidate they'd apply. Frontend: `CompletenessQueueRow.svelte`
  (`components/completeness/`, new folder) renders one (entity, facet) row per DD2/DD3 — Apply for
  candidate-ready (mutates via the existing per-field decision endpoints, row disappears on success,
  no toast), Search+optional Upload links for needs-research (deep-link to the entity page anchored
  at that facet's control). New `owner/completeness/+page.svelte` route + nav tab. Found and fixed two
  real bugs during live browser QA (neither caught by type-check or unit tests): (1) `FacetGroup`'s
  `CandidateReady`/`NeedsResearch` slices left at their Go zero value serialized as JSON `null`
  instead of `[]` whenever a group was one-sided, crashing the frontend's `.length` access — fixed by
  initializing both as `[]QueueRow{}` at construction; (2) the media detail page's dedicated studio-
  display block (excluded from the generic resolved-fields `id`-tagged loop, unlike every other field)
  was missing its `#field-studio` anchor entirely — every Search link for the queue's single largest
  group (200 of ~400 rows in the test dataset) would have silently done nothing; fixed by adding
  `id="field-studio"` to that block. QA'd all three skins (contrast-checked via computed styles, not
  screenshots — this app's screenshots time out) and the live queue end to end. Ran `/simplify`
  (4-agent pass) and fixed: reuse's flag that a local `ENTITY_PATH` map duplicated `api.ts`'s existing
  `ENRICH_ENTITY_BASE` (exported it, reused); reuse's flag that the row's Apply/Search/Upload buttons
  used ad-hoc padding/text-size utilities instead of the `.btn-row`/`.btn-pill` sizing convention the
  other three owner queue rows (Extraction/Enrich/DuplicatePair) already share (switched); efficiency's
  flag that the queue handler ran three independent full-library scans (`completenessForVideos/People/
  Studios`) sequentially despite them being disjoint lock-free WAL reads (parallelized with goroutines
  + `sync.WaitGroup`, no new dependency); simplification's flags that the per-entity-type filter check
  was copy-pasted three times (hoisted into `addRow`) and that `SEARCH_ANCHOR_OVERRIDE`/`UPLOAD_ANCHOR`
  were two parallel maps expressing one concept (merged into one `FACET_ANCHOR` map keyed by facet).
  Skipped two altitude findings as out-of-scope refactors, noted for a future ticket rather than this
  pass: the `id="field-<canonical>"` anchor convention is hand-repeated across all three detail pages
  with nothing enforcing the queue's anchor references stay in sync with what each page actually
  renders (would want a shared helper/registry-driven mechanism, not a per-page string literal); and
  the studio icon/monogram fallback markup in the new row is a third hand-copy of markup already
  duplicated in `studios/+page.svelte` (wants a shared `StudioIcon` component, pre-existing debt this
  diff added to rather than caused). `go build`/`go vet`/`go test ./internal/api/... ./internal/resolver/...`
  and `npm run check` (0 errors) both clean after fixes; re-verified live in the browser (200 OK on the
  queue endpoint, correct `.btn-row .btn-pill` computed styles) post-refactor. Next: item #6, the
  per-entity completeness breakdown panel on video/person/studio detail pages.

### 2026-08-08 · Item #4 follow-up — shared segmented-toggle class helper
- skills: simplify
- handoff: closed the one `/simplify` finding left unfixed from the item #4 pass — the reuse
  agent's minor flag that `CompletenessSortToggle.svelte` and `SortToggle.svelte` each carried an
  identical `cls(active)` helper plus identical wrapper markup. Extracted both into
  `web/src/lib/components/sort/segmentedToggle.ts` (`segmentedToggleClass`,
  `segmentedToggleWrapperClass`, plain functions/const — no reactivity needed) and pointed both
  components at it; added the file to `sort/CLAUDE.md`. Everything else from that `/simplify` pass
  (reuse+simplification's missing-facet-fetch triplication, reuse's `equalStrings`→`slices.Equal`,
  simplification's owner-gate triplication, efficiency's clean-infinite-loop-fix confirmation) had
  already been fixed in the item #4 commit (`c75a49d`) — this was the sole remainder. The three
  efficiency findings on backend perf (double compute of facets+list, full re-score on every
  "Load more" page, sequential `*ForEntities` batch loaders) stay deferred, per the same
  reasoning as before: pre-existing ADR-081 D4 architecture, out of scope for a browse-UI item,
  candidates for their own HOLODEX ticket once real usage shows it matters at this app's
  personal-library scale. `npm run check` (0 errors) and `go build`/`go vet` both clean; verified
  both toggles live in the browser (correct active-state class, no console/network regressions).
  Next: item #5, the facet-first remediation queue.

### 2026-08-08 · Item #4 — browse Completeness sort + Missing facet filter chip (F55.5/F55.6)
- skills: simplify
- handoff: wired D4's backend predicate into all three public list endpoints and built the
  matching owner-only browse UI, closing item #4. Backend: `listMedia`/`listPeople`/`listStudios`
  now route `sort=completeness_asc|desc` or any `missing_facet` param to the D4 completeness path
  (`listMediaByCompleteness` etc.) instead of the normal SQL-paginated one; new owner-gated `GET
  /completeness/facets?entity_type=` returns `FacetSummary[]` (canonical/label/criticality/
  missing_count) for the filter chip's options, scored against the exact same filtered subset a
  sort/filter request would return so the chip's counts can never disagree with the filter itself.
  Frontend: `SortDropdown` gains owner-filtered completeness options, a new
  `CompletenessSortToggle.svelte` (`entity/`, F55.5) for People/Studios — kept separate from
  `SortToggle` since its `PeopleTagSort` type is shared with the out-of-scope Tags page — and
  `FacetFilter.svelte` widened to generic `Id extends string | number` so it can host both the
  existing numeric tag/person facets and the new string canonical-field ones. All three browse
  pages gate the *outgoing request* on `isOwner` (`isOwner ? realValue : fallback`), never the
  persisted UI state itself — a transient pre-capabilities-load `isOwner=false` self-heals into
  the real request once `activity.effectiveOwner` resolves, instead of clobbering a
  URL/localStorage-restored preference and risking a doomed 401. Found and fixed a real bug during
  QA: the missing-facet options fetch was gated on `options.length === 0`, which loops forever
  when the API legitimately returns `[]` (a fresh empty array is itself a `$state` write) — it
  crashed the dev server mid-session; fixed with a plain non-reactive `fetched` boolean instead,
  since extracted into a shared `web/src/lib/missingFacetOptions.svelte.ts` composable so the
  fetch-once effect isn't triplicated across the three pages. QA'd owner/non-owner request
  composition and all three skins (Cinémathèque/Broadcast/Brutalist) live in the browser. Ran
  `/simplify` (4-agent pass): altitude's highest-priority finding was that the three inline
  `if !h.auth.authorized(r) { writeError(...); return }` gate blocks bypassed `requireOwner`'s
  middleware, silently dropping `maybeRenewSession` (ADR-046 session-lifetime slide) and dead-cookie
  clearing for these three endpoints — a real bug, not just duplication; fixed by extracting
  `requireOwnerInline(w, r) bool` in `internal/api/auth.go` (both `requireOwner` and the three call
  sites now delegate to it) plus a `wantsCompleteness(sort, missingFacets) bool` helper collapsing
  the repeated condition; reuse flagged a hand-rolled `equalStrings` test helper duplicating stdlib
  `slices.Equal` (fixed). Deferred (noted, not fixed — architectural, wants its own review): three
  efficiency findings — `listMediaByCompleteness` computing facets and the list separately on the
  same request, a full re-score of the whole matching set on every browse "Load more" page under a
  completeness sort, and the `*ForEntities` batch loaders running sequentially rather than
  concurrently — candidates for a follow-up HOLODEX ticket once real usage shows it matters at this
  app's personal-library scale (ADR-081 D4's own explicit bound). `go build`/`go vet`/`go test
  ./internal/api/...` and `npm run check` all clean. Next: item #5, the facet-first remediation
  queue (backend+frontend, shares this session's backend predicate) — or file the deferred
  efficiency findings as their own HOLODEX ticket first if picked up before #5.

### 2026-08-08 · Backend D4 — list-wide resolve-all predicate for browse sort/filter + remediation queue
- skills: simplify, architecture
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
