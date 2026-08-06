# Graph Report - laughing-wu-649060  (2026-08-05)

## Corpus Check
- 719 files · ~1,016,986 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 5488 nodes · 10898 edges · 566 communities (335 shown, 231 thin omitted)
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 1735 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `1b2679da`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- types.ts
- people/[id]/+page.svelte
- session-start.mjs
- Backfill
- testPatterns
- NewService
- Repo
- Context
- Holodex Project Working Agreements (CLAUDE.md)
- Handlers
- devDependencies
- New
- imagetools.mjs
- Server
- EnrichmentRow
- Service
- resolver.go
- format.ts
- postTok
- newRepo
- Scanner
- pathID
- Flightplan — portable session-state plugin
- run
- architecture/README.md
- enrich/enrich_test.go
- repo/delete_test.go
- tmdb.go
- Auth
- Spec: Metadata Source Plugins (F22/F27/F28)
- tmdb_test.go
- Configuration Reference (holodex.yaml layers)
- Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)
- mcp.go
- Spec: Tag governance & video enrichment (F50)
- extractor.go
- .Resolve
- ResolveReviewAction
- activity.svelte.ts
- navSearch.svelte.ts
- jira-sync.mjs
- Field
- tmdbClient
- person_fields.go
- Open
- sendDecision
- Queue
- Registry
- newHandler
- generate.mjs
- Manual QA Checklist: Per-field source-of-truth decisions (F36)
- sendTok
- PromoteFieldEditor.svelte
- sampleVideo
- AutoRegisterFields
- ResolveFields
- writeback/writeback.go
- Handlers
- Context
- getJSON
- openAt
- resolveOrCreateByName
- identity_ops_test.go
- fakeRepo
- scanner_test.go
- f36.ts
- ADR-046 (per qa-metadata-curation.md): Metadata curation and write queue
- process_test.go
- Design Handoff: Unified nav search — live, tabbed, in-place filtering panel
- seedPerson
- routes/tags/+page.svelte
- Provider
- Handlers
- Quick Wins batch (overlay fix, search history, related shelves, fluid Back)
- ADR-058 (Jira transitions via direct REST API)
- Manager
- api/person_images_test.go
- .scaleToWidth
- Write
- enrich/enrich.go
- 0001_init.up.sql
- Process
- authServer
- Context
- New
- SanitizeFieldHints
- thumbServer
- identityServer
- Derive
- queue
- Release promotes by retagging the canaried digest
- QA Checklist: System Activity (F21)
- metadata-mappings.yaml config file: source-key-to-canonical-field mapping with precedence
- Decision
- routes/+layout.svelte
- QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)
- JaroWinkler
- Context
- seedTagTree
- resolver/decisions_test.go
- .setPersonFieldDecision
- .addEntityAlias
- refreshServer
- deleted_at soft-delete column (orthogonal to active)
- QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)
- Orchestrator
- ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam
- ADR-080: Configurable per-provider metadata search query patterns
- Context
- newRepoDB
- repo/related_test.go
- repo/studios_test.go
- TestQueue_SnapshotsAndReverts
- .claimTarget
- .setFieldPromotion
- Decision
- BatchRunner
- ADMIN_TOKEN env var — v1 owner identity, default-open when unset
- compilerOptions
- listScroll.svelte.ts
- .SetDelete
- Lookup
- Fake
- Handlers
- FlagNearMiss
- query_test.go
- Candidate
- Route
- Store
- .add
- Spec: Tag Writeback Exclusion — per-tag Genre writeback control
- TestAttachMaterializedTags
- nationality.ts
- ADR-002: SvelteKit chosen as frontend framework (SPA/static-adapter mode)
- newAssetClient
- .enrichQueueForType
- extractServer
- identity_test.go
- Context
- ResolveForContainer
- demo/package.json
- Spec: Owner-authored person & studio ↔ media links, with file writeback (F40)
- .dismissDuplicate
- writeError
- Health
- MergePersons(canonical, merged) transaction
- Shared ingest normalization pipeline (decode → bound → re-encode → strip)
- Spec: Tag Categories — grouping tags without merging them
- QA Checklist: Derived / calculated person fields — Age & Age at death (F45)
- model.go
- confidence.go
- personDerivedServer
- Context
- seedTwoTags
- Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)
- attachTagTx
- tagWritebackSyncServer
- QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)
- Manual QA Checklist: People Images (F25)
- .CurationForVideos
- Context
- Context
- placeholders
- ImagePath
- Spec: Job History — Digest, Pagination, Entity Search (F21.3b)
- WritebackFormDialog.svelte
- .adminActivity
- Context
- recordingSink
- Context
- Context
- .index
- fakeRepo
- SearchHistory
- ReadCurrentValues
- hooks
- buildVideo
- Decision
- TestStoreRoundTrip
- Test Fixtures
- field_source_decisions table
- Spec: Unified entity name-identity (F43)
- .genreWritebackItems
- Router
- holoShuffle
- Spec: People Images (F25)
- F38: Studio entity pages
- stub.js
- job_runs table (kind, trigger, status, counts, 30-day retention)
- Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)
- jira-sync.mjs REST transition mechanism (idempotent, match-by-name, soft-fail)
- Keyset cursor over (started_at, id) for job history
- activity/CLAUDE.md
- curation/CLAUDE.md
- Context
- ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision
- Context
- gen-country-names.mjs
- field_claims.go
- duplicates/CLAUDE.md
- .ProviderFieldHints
- tag-categories-handoff.md
- .Baseline
- .RoundTrip
- gen.sh
- svelte.config.js
- .propagateMerge
- frontendFS
- frontendFS
- vite.config.ts
- cmd/holodex/holodex.manifest — manifest source XML (requestedExecutionLevel=asInvoker)
- .SeedIdentityReviewQueue
- .RefreshTarget
- enrichment/CLAUDE.md
- Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)
- decisionBody
- entity/CLAUDE.md
- app.d.ts
- +layout.ts
- ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)
- Holodex media detail page — Broadcast skin
- Screenshot: Holodex video detail page in the 'Brutalist' theme skin — shows top nav (Media/People/Tags, skin switcher with Cinémathèque/Broadcast/Brutalist options), a video player for 'Nightshade' (Thriller, 2021), and metadata panel with format badge (4K, 3840x2160, 1:58:12, 2021), People chips (Lana Reyes, Marcus Vane), and Tags chips (Noir, Thriller)
- Screenshot: Video detail page in the Cinémathèque skin — shows player, title card, metadata, people, and tags for a sample video 'Nightshade'
- Grid/Browse View Screenshot (Broadcast Skin)
- Screenshot: Holodex media browse grid view in the 'Brutalist' skin — top nav (HOLODEX logo, global search Ctrl-K, Media/People/Tags links, Cinematheque/Broadcast/Brutalist skin switcher with Brutalist active), filter bar (title search, resolution toggle All/SD/HD/FHD/4K, duration min-max, year range, People and Tags filters), '18 videos' result count, and a 3-column card grid of 18 video entries each showing a resolution badge (4K/HD/FHD/SD), duration, abstract colored thumbnail art, uppercase title, and genre tag chips (e.g. Vantablack: Horror/Thriller, Solar Drift: Adventure/Sci-Fi, Amélie en Hiver: Drama/Romance)
- Screenshot: Holodex browse/grid view in the 'Cinémathèque' skin. Dark theme with warm amber/gold accents. Top bar shows 'Holodex' logo, global 'Search everything... (Ctrl-K)' box, nav links (Media, People, Tags), and a three-way skin switcher (Cinémathèque selected/amber, Broadcast/blue, Brutalist/green). Filter row includes title search, Resolution toggle (All/SD/HD/FHD/4K), Duration (min) range, Year range, People multiselect, and Tags multiselect. Results header reads '18 videos'. Grid of movie-poster style cards (3 rows x 6 columns) each showing a colored gradient thumbnail with a resolution badge (4K/HD/FHD/SD) top-left, duration badge (e.g. 1:36:40) bottom-right, title overlay, and below the card a title repeated plus genre/tag chips (e.g. Vantablack - Horror, Thriller; Tin Soldier - Drama, War; The Long Saturday - Comedy; The Quiet Coast - Drama; The Cartographer - Drama; Static Bloom - Experimental, Short; Solar Drift - Adventure, Sci-Fi; Overgrowth - Documentary, Nature; Paper Moons - Animation, Family; Nightshade - Noir, Thriller; Neon Tide - Crime, Noir; Migration - Documentary, Nature; Harbor Lights - Drama, Romance; Glasshouse - Mystery, Thriller; Dust & Echoes - Drama, Western; Concrete Garden - Documentary; Amélie en Hiver - Drama, Romance; Ferrous - Sci-Fi, Thriller).
- Dependabot Config
- Deploy Landing Page Workflow
- Spec: Configurable per-provider search query patterns (F54)
- Spec: Studio image roles — icon / logo / poster (F51)
- holodex
- TMDB Brand Logo (tmdb-brand.png)
- Holodex media detail page screenshot (Broadcast skin)
- Screenshot: media detail page (brutalist skin)
- Detail page screenshot (Cinémathèque skin) — Holodex showcase site
- Screenshot: Holodex media grid/browse view in the 'Broadcast' theme skin — dark background, teal/cyan accent, top nav (HOLODEX logo, global search 'Search everything... (Ctrl-K)', Media/People/Tags links, theme switcher showing Cinémathèque/Broadcast/Brutalist skins with Broadcast active), filter bar (title search, Resolution chips All/SD/HD/FHD/4K, Duration min/max, Year from/to, People and Tags multi-select inputs), and an 18-video responsive card grid with thumbnails (resolution badge, duration), titles, and genre tag pills (e.g. Vantablack/Horror,Thriller; Tin Soldier/Drama,War; The Long Saturday/Comedy; Solar Drift/Adventure,Sci-Fi; Nightshade/Noir,Thriller; Amélie en Hiver/Drama,Romance)
- Screenshot: Holodex media browse grid, Brutalist skin (18 videos, filter panel, resolution/duration/year/people/tags filters, poster cards with duration+genre tags)
- Screenshot: Holodex browse/grid view in the 'Cinémathèque' skin, showing a 18-video poster grid with search, resolution/duration/year filters, and people/tag filters
- extractReviewServer
- extraction/CLAUDE.md
- ADR-014: Configuration and Data Layout
- Age-in-media derived field
- chips render mode (read-only pill list)
- Spec: System Activity — "Under the Hood" (F21)
- CurationFieldRow.svelte
- F36: Per-field source-of-truth
- ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity
- F39: Provider render hints / auto-registered non-canonical fields
- F44: In-app promote/override affordance
- Spec: People on the unified source-of-truth model (F37)
- Normalize
- asset_hosts allowlist ({base_url host} ∪ operator-listed hosts) in metadata-sources.yaml
- Design Handoff: Owner tooling hub + nav split (F35)
- F47: Enrichment review workflow
- F48: On-demand metadata extraction from filenames & tags
- F49: Claimed provider keys
- Spec: Studio as a first-class entity (F38)
- Flightplan session-state plugin
- Job history digest, pagination, entity search
- Leading logo well (studio list monogram/logo)
- needsWriteback(field) selection predicate
- Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31)
- Three-skin QA contract (Cinémathèque · Broadcast · Brutalist, tokens-only)
- ADR-001: Backend
- Spec: Runtime owner-editable settings (F41)
- ADR-002: Frontend
- ADR-008: Caching Strategy — In-Process Cache with Redis-Ready Interface
- Debian bookworm-slim base image chosen over Alpine (exiftool/ffmpeg compatibility)
- video_metadata table: captures every extracted container tag key-value per video
- ADR-018: Scan change detection
- ADR-019: Observability
- worklog.md
- Jira HOLODEX-166 (System Activity epic)
- Decision
- Copy → exiftool-write → atomic rename file-safety model
- CSS custom-property design tokens (--bg, --surface, --ink, --accent, --font-display, --radius) per [data-theme]
- codeql.yml — CodeQL static analysis for go + javascript-typescript
- Design Handoff: Metadata Enrichment UI for People (F22)
- GET /api/v1/admin/activity aggregated read-model endpoint
- ProviderClient interface (HTTP default; in-process fake for CI)
- Spec: Quick Wins batch — Search history & "More with …" shelves
- Spec: Sticky sort preferences + Random sort
- ADR-001: Backend Language — Go
- Decision
- Unified field resolution — sources: [tmdb, file:Publisher, imdb] precedence list
- ghcr GitHub Deployment environment (Release ↔ Deployments linkage)
- git-cliff changelog generation (cliff.toml, orhun/git-cliff-action)
- Design Handoff: Media page — one sync verb, render-once fields (F36 / F39)
- person_aliases table + person_aliases_fts external-content mirror (migration 0007)
- person_images table (role, source, provider, external_id) + partial unique index (migration 0009)
- Placeholder resolution (active_skin, role, gender_bucket) → programmatic SVG asset
- providers/tmdb/ — standalone stdlib-only Go source, own binary + Dockerfile.provider-tmdb
- Functional Requirements
- Design Handoff — People Images (F25)
- file_writebacks audit table (video_id, field_key, tag_name, value, source, written_at)
- ADR-045 (owner-session, per promote-override-fields.md): owner gate / Admin mode / effectiveOwner
- ADR-046 (Proposed, owner-session-persistence.md): Owner session persistence via HttpOnly token-exchange cookie
- QA: TMDB Provider Sidecar + ADR-039 Core Changes
- holodex_session cookie — HttpOnly, Secure, SameSite=Strict, signed self-contained payload
- refresh service (plan/apply split)
- Media page restructure — one sync verb, render once
- entity_enrichment shadow store (entity_type, entity_id, provider, field_key)
- RefreshReport (sources_disagree flag)
- metadata_curation table (manual source, add/suppress/nowrite)
- writeback_queue table (durable job queue)
- filters.ts
- LockedCoreRoles (implicit provenance lock)
- person_images.content_hash column + backfill
- Functional Requirements
- BaselineSource interface
- ResolveFields entity-agnostic merge core
- ADR-052: BaselineSource contract
- RelinkVideoStudios reconcile (sole writer, prune-on-empty)
- studios / video_studios / studios_fts data model
- studio_external_ids table (external_id PK, global convergence)
- /describe.field_hints manifest extension
- Spec: Owner tooling hub + visitor/owner nav split (F35)
- Spec: Person Aliases (F23)
- studio_logos table / RelinkStudioLogo
- provider_icons table / RelinkProviderIcon
- entity_aliases / nameKey identity spine (polymorphic)
- Context
- entity_keep_separate table (durable negative assertion)
- field_promotions table (tier-0 override)
- per-epic worklog (docs/plans/<KEY>.md)
- .github/workflows/provider-tmdb.yml — dedicated CI for the TMDB provider image
- PersonImageInsert struct with OverCap bool flag
- SessionStart orientation hook (compact digest, In Progress fire)
- ADR-065: Typed field registry and relationship-scoped computed fields (Superseded)
- DeriveRelationship(person, video, now) two-entity pass (unbuilt)
- studio-entity.md
- Spec — Showcase Demo Corpus
- FieldType / Operator taxonomy (text/categorical/numeric/date)
- Decision
- enrichment_dismissals table (durable rejection verdict)
- file_writeback_snapshots table (batch revert, undo-of-undo)
- filename extraction confidence scoring rubric (tiered, exact-match gate)
- .resolveStudio
- ADR-068: Extraction entity-field chip refinement (D2)
- ReExtract post-write re-extract hook (file-only, no re-enrich)
- @theme inline block in app.css (replaces tailwind.config.ts)
- Draft PR as pre-implementation gate carrier
- Canary pins by digest, not by tag
- Polymorphic (entity_type, entity_id) job_runs attribution, no FK
- 30-day person orphan grace + authored-identity guard
- video_people derived via unified RelinkVideoEntity
- GET /writeback/jobs/{id} + SPA poll to terminal state
- Admin mode header toggle (presentation-only gate)
- Age-in-media corner badge on cast poster card
- "Attach to…" pill + picker (F49 claim action)
- "Attached keys" owner-tooling list (/owner/fields)
- ClaimFieldEditor with outcome-before-commit copy
- Tag distinctiveness score c·(1−c/N)
- holo_shuffle(id, seed) deterministic scalar SQLite function (splitmix64-style hash)
- QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)
- QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore)
- Trash view (/trash) with Restore / Delete permanently
- Derived Age / Age-at-death row, tooltip-only provenance
- Manual QA Checklist: Metadata Enrichment for People (F22)
- EnrichProviderChips Refresh/Re-match/Clear split
- /owner/enrichment review queue tab
- ADR-010: MKV (Matroska) Tag-Target Precedence
- /tags pill-native manage mode (rename/alias/merge)
- Manual QA Checklist: Admin Mode (F29)
- CurationChip radio mode (shared shell, dot vs ✕ glyph)
- Manual QA Checklist: Person Aliases (F23)
- SourceSelect radiogroup (replace-field source-of-truth)
- ADR-070 (canary release candidate and promote-by-retag)
- HOLODEX-208 (main-HEAD freshness false-positive)
- ADR-069 (Draft PRs for pre-implementation gates)
- CI transition scripts (jira-branch-sync.mjs, jira-release-sync.mjs)
- HOLODEX-185 epic auto-transition guard
- Jira Free-plan Automation quota (100 runs/month, shared) motivating REST migration
- Jira status ladder: To Do → In Progress → In Review → Done → Released
- INBOX.md idea capture (Stage 0)
- The one rule: never let durable state depend on remembering
- Worklog gates (spec/architecture/backend/frontend/testing/security)
- ADR-021 (frontend theming and skins)
- Design Handoff: Tag & category create affordance (HOLODEX-243)
- ADR-030 (access control gating seam)
- Owner tooling hub (F35) follow-up rename
- Admin mode header toggle (visitor-preview, complete hide set)
- Reference: Canonical fields registry (F27)
- ADR-030 (owner gate)
- ADR-037 (soft-delete and purge)
- Grace-period purge job (background hard-delete sweep)
- ADR-051 (per-field source-of-truth decisions)
- Atomic, one-WriteBatch-per-file writeback (RD5, non-negotiable)
- Per-field source decision (keep file / adopt provider / custom)
- default_source: file — file-first global default (RD4)
- ADR-039 (provider asset URLs) perimeter, unchanged
- WebP decoder registration (golang.org/x/image/webp blank import)
- internal/personimage Normalize gauntlet (decode→bomb-guard→re-encode JPEG)
- ADR-048 (metadata curation and write queue)
- manual: curation source + tombstones (suppress/nowrite)
- Cross-source merge & dedup resolution mode
- Durable bounded-concurrency writeback_queue (F30.4)
- ADR-067 (filename extraction confidence and rollback)
- Exact-match gate — hard rule for auto-apply (never fuzzy)
- filename: shadow-store namespace source
- Merge → writeback propagation (F48.8, no second confirm)
- Filename pattern token-grammar parsing
- file_writeback_snapshots rollback (F48.9, amends ADR-041)
- ADR-047 (per-item metadata refresh)
- POST /media/{id}/refresh owner-gated endpoint
- Forced file re-extract (bypasses size/mtime change-detection)
- RefreshReport structured outcome (plan/apply seam, F31.14/F31.15)
- TMDB-specific field mapping (person/movie/studio)
- Provider HTTP contract: /healthz /describe /resolve /enrich (protocol v1)
- TMDB sidecar security requirements (S1-S7)
- Choice A: provider-side structured people[] credits
- Critical adversarial invariants (precedence, no stale cache, scan idempotency, identity never forks)
- Testing principles (metadata correctness is the product, behavior over implementation, fast feedback, real deps over mocks)
- Test pyramid: Unit / Integration / E2E (Playwright)
- Flightplan hooks: SessionStart / PostToolUse(Skill) / Stop
- Worklog schema: frontmatter, gates, up-next, session log
- internal/extract/ (filename parsing, confidence, routing)
- Jira HOLODEX-10 (S5 People F37)
- Jira HOLODEX-112 (S7 chip redesign)
- Jira HOLODEX-114 (F40/ADR-059 dependency)
- Jira HOLODEX-126 (leading logo well)
- Jira HOLODEX-128
- Jira HOLODEX-171
- Jira HOLODEX-213
- Jira HOLODEX-222 (slice C, proactive duplicate detection)
- rsrc (akavel/rsrc) Windows resource compiler
- web/src/app.css
- AddValueInput.svelte
- Component classification rule: consumer-based then function-based, shared/ as fallback
- ImageUploader.svelte
- LinkPicker.svelte
- PersonPicker.svelte
- PlaceholderImage.svelte
- web/src/lib/f36.ts
- web/src/lib/peopleScroll.svelte.ts
- web/src/lib/searchHistory.ts
- web/src/routes/+layout.svelte
- web/src/routes/media/[id]/+page.svelte
- web/src/routes/studios/[id]/+page.svelte
- Spec: Tag & Category Create Affordance — closing the /tags creation gap
- decisionServer
- Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)
- HOLODEX-240.md
- .runTagWritebackSync
- Context
- repo/categories_test.go
- .setFieldDecision
- itoa
- .denyTag
- PersonImageRef
- Noop
- Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)
- .listProviders
- .getMedia
- HOLODEX-239.md
- QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)
- fakeBaseline
- .setTagParent
- Design handoff: In-app promote / override affordance for auto-registered fields (F44)
- .Baseline
- 0022_entity_name_identity.up.sql
- 0004_job_runs.up.sql
- 0005_entity_enrichment.up.sql
- 0013_metadata_curation.up.sql
- 0016_field_source_decisions.up.sql
- 0019_provider_field_hints.up.sql
- 0021_provider_icons.up.sql
- 0023_field_promotions.up.sql
- 0024_enrichment_dismissals.up.sql
- 0025_metadata_extraction_review.up.sql
- 0026_file_writeback_snapshots.up.sql
- 0029_field_claims.up.sql
- 0031_denied_tags.up.sql
- Sink
- density.svelte.ts
- Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided"
- Sweeper
- .RelinkVideoPeople
- coverArtManager
- .mergePerson
- .setStudioFieldDecision
- ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field
- HOLODEX-114.md
- HOLODEX-253.md
- New
- Context
- .uploadVideoPoster
- .resolvedFieldForVideo
- newTestService
- HOLODEX-244.md
- jobruns_test.go
- .uploadStudioImage
- Design handoff: Studio image roles — icon / logo / poster (F51)
- Placeholder
- NavSearch
- TestDecisions_ForVideosBatch
- TestGetWritebackBatchStatus
- WriteBatchFunc
- TestRefreshTarget

## God Nodes (most connected - your core abstractions)
1. `newRepo()` - 155 edges
2. `writeError()` - 97 edges
3. `sampleVideo()` - 96 edges
4. `itoa()` - 89 edges
5. `pathID()` - 73 edges
6. `writeJSON()` - 71 edges
7. `Router()` - 62 edges
8. `Handlers` - 59 edges
9. `Open()` - 58 edges
10. `Server` - 54 edges

## Surprising Connections (you probably didn't know these)
- `Holodex landing page (site/index.html)` --semantically_similar_to--> `SvelteKit app.html shell (default data-theme=cinematheque)`  [INFERRED] [semantically similar]
  site/index.html → web/src/app.html
- `runMCPStdio()` --calls--> `NewAuth()`  [INFERRED]
  cmd/holodex/main.go → internal/api/auth.go
- `runMCPStdio()` --calls--> `Open()`  [INFERRED]
  cmd/holodex/main.go → internal/db/db.go
- `run()` --calls--> `NewAuth()`  [INFERRED]
  cmd/holodex/main.go → internal/api/auth.go
- `run()` --calls--> `NewHandlers()`  [INFERRED]
  cmd/holodex/main.go → internal/api/handlers.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **ADR-070 canary + retag release pipeline** — github_workflows_image_yml, github_workflows_release_please_yml, github_workflows_release_yml, github_workflows_release_candidate_yml, concept_adr_070_canary_retag_promotion [INFERRED 0.85]
- **Jira status-transition pipeline (ADR-058/069)** — claude_claude_md, claude_flightplan_yaml, github_workflows_jira_sync_yml, github_workflows_release_yml, concept_adr_058_jira_rest_transitions [INFERRED 0.85]
- **Frontend token-discipline design + CI enforcement** — claude_rules_frontend_theming_md, github_workflows_ci_yml, concept_adr_021_frontend_theming_skins [INFERRED 0.85]
- **FTS5 search subsystem: SQLite FTS5 choice, migration-managed virtual tables/triggers, global+filter search architecture** — docs_architecture_adr_003_database_sqlite_decision, docs_architecture_adr_016_database_migrations_decision, docs_architecture_adr_017_search_architecture_decision [INFERRED 0.80]
- **ADR-075's four tag-governance decisions form one feature** — docs_architecture_adr_075_tag_governance_and_video_enrichment, docs_architecture_adr_075_tag_governance_and_video_enrichment_parent_hierarchy, docs_architecture_adr_075_tag_governance_and_video_enrichment_denied_tags, docs_architecture_adr_075_tag_governance_and_video_enrichment_video_tags_source, docs_architecture_adr_075_tag_governance_and_video_enrichment_materialize_tags [INFERRED 0.85]
- **F50 Tag Governance & Video Enrichment design package (plan + handoff + QA checklist)** — docs_plans_holodex_224, docs_design_tag_governance_and_video_enrichment_handoff, docs_design_tag_governance_and_video_enrichment_qa_checklist [EXTRACTED 1.00]
- **Entity-generic F36 decision model proven across person, studio, and derived links** — docs_specs_people_source_of_truth, docs_specs_studio_entity, docs_specs_person_media_linking [INFERRED 0.85]
- **Client-only, zero-backend-state UI persistence/derivation pattern (localStorage/session, no migration)** — docs_specs_quick_wins, docs_specs_sort_persistence, docs_specs_people_nationality_flag [INFERRED 0.65]
- **curation/enrichment/entity component folders jointly implement the per-field source-of-truth + identity spine UI** — web_src_lib_components_curation_claude, web_src_lib_components_enrichment_claude, web_src_lib_components_entity_claude [INFERRED 0.75]
- **Video card/grid primitives and the shelves built on top of them** — web_src_lib_components_video_videocard, web_src_lib_components_video_videogrid, web_src_lib_components_video_recentlyaddedshelf, web_src_lib_components_video_relatedshelf [INFERRED 0.75]
- **Sort controls shared across browse/people/tags index pages** — web_src_lib_components_sort_sortdropdown, web_src_lib_components_sort_sortreroll, web_src_lib_components_sort_sorttoggle [INFERRED 0.75]

## Communities (566 total, 231 thin omitted)

### Community 0 - "types.ts"
Cohesion: 0.02
Nodes (121): ADR-0006, ADR-0028, ADR-0036, ADR-0056, ADR-0072, ADR-0073, ADR-0080, checkRedirect() (+113 more)

### Community 1 - "people/[id]/+page.svelte"
Cohesion: 0.07
Nodes (16): provider(), Shared components, runEnrichRefresh(), runEnrichRefreshAll(), providers, ProvidersStore, ADR-0059, EnrichEntityKind (+8 more)

### Community 2 - "session-start.mjs"
Cohesion: 0.07
Nodes (63): bareSkill(), DEFAULTS, loadConfig(), ADR-0064, resolveKey(), emitJson(), ADR-0064, relPath() (+55 more)

### Community 3 - "Backfill"
Cohesion: 0.18
Nodes (15): Image, Backfill(), Context, Logger, discardLog(), Logger, T, TestBackfillHashesAndRemoves() (+7 more)

### Community 4 - "testPatterns"
Cohesion: 0.06
Nodes (53): compiledPatterns, countingResolver, fakeEnrichmentCall, fakeEnrichmentWriter, fakeVideoLister, fakeVideoLookup, Pattern, patternFile (+45 more)

### Community 5 - "NewService"
Cohesion: 0.07
Nodes (46): Match, Context, Service, EnrichedField, extraPairs(), fileLayerChanged(), Context, ExtraMetadata (+38 more)

### Community 6 - "Repo"
Cohesion: 0.08
Nodes (26): ftsPrefixQuery(), Context, DB, ExtraMetadata, Mutex, Person, Pointer, Repo (+18 more)

### Community 7 - "Context"
Cohesion: 0.20
Nodes (6): CorePersonImageRole(), ValidPersonImageRole(), Context, PersonImage, PersonImageSet, Repo

### Community 8 - "Holodex Project Working Agreements (CLAUDE.md)"
Cohesion: 0.07
Nodes (41): Holodex Project Working Agreements (CLAUDE.md), Branch↔Jira linkage, Core resolver model (baseline / enrichment / curation / decisions), Pre-commit checklist, Secrets & publishing rules, Jira task tracking (HOLODEX project), Flightplan Config (flightplan.yaml), Frontend Theming Rules (+33 more)

### Community 9 - "Handlers"
Cohesion: 0.17
Nodes (8): FacetValue, Handlers, Context, Duration, Request, ResponseWriter, Service, redactFileMetadataForVisitors()

### Community 10 - "devDependencies"
Cohesion: 0.04
Nodes (48): flag-icons, @fontsource/share-tech-mono, @fontsource-variable/archivo, @fontsource-variable/fraunces, @fontsource-variable/spline-sans-mono, @fontsource/vt323, svelte-check, @sveltejs/adapter-static (+40 more)

### Community 11 - "New"
Cohesion: 0.09
Nodes (27): Context, Duration, Logger, Time, New(), Context, JobRun, T (+19 more)

### Community 12 - "imagetools.mjs"
Cohesion: 0.08
Nodes (47): ADR-0035, ADVISORY_TYPES, classify(), COMMENT_MARKER, main(), ADR-0076, NON_DOC_GLOBS, parseCommitType() (+39 more)

### Community 13 - "Server"
Cohesion: 0.13
Nodes (36): iconEnv, Handlers, Repo, Service, T, newProviderIconEnv(), providersDirectory(), TestProviderIcon_DownloadNormalizeServe() (+28 more)

### Community 14 - "EnrichmentRow"
Cohesion: 0.17
Nodes (7): T, TestStudioExternalIDsFromRows(), studioExternalIDsFromRows(), Context, Repo, Time, EnrichmentRow

### Community 15 - "Service"
Cohesion: 0.10
Nodes (24): assetFetcher, EnrichRepo, ImageSink, Manifest, ProviderClient, SourceInfo, Context, Service (+16 more)

### Community 16 - "resolver.go"
Cohesion: 0.15
Nodes (35): FieldCandidate, FieldDecision, applyCasing(), baselineValue(), BrowseTitle(), decidedItem(), firstNonEmpty(), ExtraMetadata (+27 more)

### Community 17 - "format.ts"
Cohesion: 0.05
Nodes (16): batchId(), revert(), progressed, progressPct, calculatedFrom(), filterByTitle(), formatDuration(), formatYear() (+8 more)

### Community 18 - "postTok"
Cohesion: 0.25
Nodes (19): aliasList(), aliasServer(), Repo, T, TestAddAliasConflict409(), TestAliasEndpointsGatedAndValidated(), TestGetPersonIncludesAliases(), TestMergeEndpoint() (+11 more)

### Community 19 - "newRepo"
Cohesion: 0.09
Nodes (40): T, TestClaims_SetClearsPromotionInSameWrite(), TestClaims_SetListClear(), T, TestPromotions_SetListClear(), T, TestProviderFieldHints_EmptyClears(), TestProviderFieldHints_ReplaceAndRead() (+32 more)

### Community 20 - "Scanner"
Cohesion: 0.11
Nodes (16): Context, Duration, Logger, ScanStatus, Time, isMedia(), New(), Config (+8 more)

### Community 21 - "pathID"
Cohesion: 0.22
Nodes (9): decodeJSON(), Handlers, EnrichedField, Request, ResponseWriter, Handlers, Request, ResponseWriter (+1 more)

### Community 22 - "Flightplan — portable session-state plugin"
Cohesion: 0.13
Nodes (15): ADR-058 (Jira transitions via REST API) — cited as evidence, Flightplan — portable session-state plugin, /handoff skill (gate ticking, release_note promotion), HOLODEX-182 tracking issue, Never let durable state depend on agent discipline, SessionStart hook (fires In Progress, prints orientation), Stop hook (mechanical worklog-staleness nag), /triage skill (drains INBOX.md) (+7 more)

### Community 23 - "run"
Cohesion: 0.12
Nodes (32): backfillPersonLinks(), backfillStudioLinks(), Context, Logger, Repo, main(), newLogger(), run() (+24 more)

### Community 24 - "architecture/README.md"
Cohesion: 0.13
Nodes (4): Architecture Decision Records, Cross-cutting, Phase specs, Person components

### Community 25 - "enrich/enrich_test.go"
Cohesion: 0.13
Nodes (38): passthroughFetcher, Repo, Service, T, newSvc(), TestAssetClientAssetHosts(), TestAssetClientNonBaseHostRequiresHTTPS(), TestAssetClientSSRF() (+30 more)

### Community 26 - "repo/delete_test.go"
Cohesion: 0.48
Nodes (6): T, TestExpiredSoftDeletedAndHardDelete(), TestRestore(), TestSoftDeleteHidesFromEverySurface(), TestSoftDeleteIdempotentAndNotFound(), TestStatByPathSurfacesDeleted()

### Community 27 - "tmdb.go"
Cohesion: 0.10
Nodes (35): buildCompanyEnrichResponse(), buildMovieEnrichResponse(), disambiguate(), movieDisambiguate(), movieYear(), parseReleaseFilename(), slugify(), tmdbEntityURL() (+27 more)

### Community 28 - "Auth"
Cohesion: 0.11
Nodes (18): Auth, sessionClaims, POST /api/v1/session (token exchange) + DELETE /api/v1/session (sign-out), deriveSessionSecret(), Handlers, Duration, Handler, Request (+10 more)

### Community 29 - "Spec: Metadata Source Plugins (F22/F27/F28)"
Cohesion: 0.08
Nodes (24): ADR-033 (metadata source plugins), Spec: Metadata Source Plugins (F22/F27/F28), Provider HTTP/JSON protocol contract (F22.1, 4 endpoints), entity_enrichment shadow store (F22.4), Unified field resolver (F27, internal/resolver), Metadata writeback (F28, exiftool copy→write→rename), brand_icon provider-level asset (ADR-059), Spec: Holodex Metadata Provider Contract (hand-off, protocol v1) (+16 more)

### Community 30 - "tmdb_test.go"
Cohesion: 0.17
Nodes (34): buildEnrichResponse(), newTMDBClient(), clientWith(), fakeTMDB(), T, TestBioTrimAtSentence(), TestBuildEnrichResponseCapsAt20(), TestBuildEnrichResponseFallsBackToProfilePath() (+26 more)

### Community 31 - "Configuration Reference (holodex.yaml layers)"
Cohesion: 0.07
Nodes (29): ADR-056 (provider field render hints, F39), ADR-074 (claimed provider keys), Claiming a provider key (F49) cookbook, Derived/computed field genre (F45, ADR-063), Canonical Field Registry (operator reference), admin_token / owner session authentication, ADR-046 (owner session persistence), default_source / provider_trust_order (F36, ADR-051) (+21 more)

### Community 32 - "Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)

### Community 33 - "mcp.go"
Cohesion: 0.09
Nodes (44): filterNamed(), CallToolRequest, CallToolResult, Context, ExtraMetadata, T, Video, isOwner() (+36 more)

### Community 34 - "Spec: Tag governance & video enrichment (F50)"
Cohesion: 0.15
Nodes (19): F50: Tag governance & video enrichment, Suppression derives from merged []mapping.Field, not the claims table, ADR-075: Tag governance & video enrichment, denied_tags global term deny-list table, Write-on-resolve tag materialization via afterEnrichApply, tags.parent_tag_id strict-tree hierarchy, video_tags.source column; partial-replace rescan, Design Handoff: Tag Governance & Video Enrichment (F50) (+11 more)

### Community 35 - "extractor.go"
Cohesion: 0.12
Nodes (26): canonicalKey(), dedupe(), Context, ExtraMetadata, Time, isBinaryValue(), mapExiftool(), mapFfprobe() (+18 more)

### Community 36 - ".Resolve"
Cohesion: 0.21
Nodes (29): Context, ResolvedValue, T, mergeField(), stubField(), TestBrowseTitle_FallbackToFileTitle(), TestBrowseTitle_NoBrowseField(), TestBrowseTitle_ProviderWins() (+21 more)

### Community 37 - "ResolveReviewAction"
Cohesion: 0.28
Nodes (14): ResolvedWrite, ReviewAction, IsMultiValueField(), ResolveReviewAction(), T, TestResolveReviewAction_Filename(), TestResolveReviewAction_FilenameRequiresValue(), TestResolveReviewAction_Manual() (+6 more)

### Community 38 - "activity.svelte.ts"
Cohesion: 0.05
Nodes (22): web/src/lib/browse.svelte.ts — module-scoped browse-state cache, web/src/routes/+page.svelte — the browse grid, web/src/lib/theme.svelte.ts — established module-scoped singleton pattern, Client-side seeded shuffle for unpaged People/Tags lists (mulberry32 PRNG), activity, ActivityState, ADR-0030, ADR-0046 (+14 more)

### Community 39 - "navSearch.svelte.ts"
Cohesion: 0.07
Nodes (16): Sort components, pageScopeFor(), SEARCH_TABS, SearchTab, firstLetter(), letterAnchors(), seededShuffle(), ADR-0045 (+8 more)

### Community 40 - "jira-sync.mjs"
Cohesion: 0.12
Nodes (23): log, main(), missing, ADR-0058, ADR-0069, bailSoft(), log, main() (+15 more)

### Community 41 - "Field"
Cohesion: 0.14
Nodes (20): Dedupe(), Empty(), ExtraMetadata, Pointer, Load(), NewStore(), parse(), parseSources() (+12 more)

### Community 42 - "tmdbClient"
Cohesion: 0.24
Nodes (9): Client, Context, rankConfidence(), splitID(), candidate, enrichResponse, hintBody, tmdbClient (+1 more)

### Community 43 - "person_fields.go"
Cohesion: 0.13
Nodes (21): ResolvedField, Source, personDecisionSource(), personField(), personFieldByCanonical(), personFields(), personizeResolved(), providerSources() (+13 more)

### Community 44 - "Open"
Cohesion: 0.09
Nodes (31): rescanner, searchMetrics, thumbnailer, T, TestGetMedia_EnrichQueries(), TestGetMedia_EnrichQueries_OmittedForVisitor(), genreWritebackServer(), Handlers (+23 more)

### Community 45 - "sendDecision"
Cohesion: 0.25
Nodes (25): sendDecision(), claimServer(), claimURL(), getJSONList(), Repo, T, TestClaim_ClearRestoresRow(), TestClaim_ClearsPromotionAndDoesNotRestoreIt() (+17 more)

### Community 46 - "Queue"
Cohesion: 0.15
Nodes (12): fakeEnqueuer, detailLine(), Context, Duration, Logger, Repo, WritebackJob, BatchJob (+4 more)

### Community 47 - "Registry"
Cohesion: 0.14
Nodes (18): formatFloat(), Builder, Duration, HandlerFunc, Int64, Mutex, New(), newHistogram() (+10 more)

### Community 48 - "newHandler"
Cohesion: 0.15
Nodes (19): decode(), Logger, Request, ResponseWriter, isSupportedEntity(), newHandler(), writeJSON(), main() (+11 more)

### Community 49 - "generate.mjs"
Cohesion: 0.12
Nodes (21): ADR-0004, ADR-0017, buildItem(), ensureFfmpeg(), here, hms(), main(), ADR-0009 (+13 more)

### Community 50 - "Manual QA Checklist: Per-field source-of-truth decisions (F36)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist), Manual QA Checklist: Per-field source-of-truth decisions (F36)

### Community 51 - "sendTok"
Cohesion: 0.27
Nodes (15): sendTok(), deleteServer(), Repo, T, TestDeleteEndpointsGated(), TestDeleteNotFoundPaths(), TestPurgeNowEndpoint(), TestSoftDeleteRestoreFlow() (+7 more)

### Community 52 - "PromoteFieldEditor.svelte"
Cohesion: 0.08
Nodes (18): ADR-0021, busy, error, onKey(), orderDraft, save(), scopeVerb, CORE_ROLE_ASPECT (+10 more)

### Community 53 - "sampleVideo"
Cohesion: 0.12
Nodes (44): countPeople(), Person, T, Video, hasVideoTitle(), personIDByName(), TestAliasesSurviveRescan(), TestMergePersons() (+36 more)

### Community 54 - "AutoRegisterFields"
Cohesion: 0.22
Nodes (22): AutoRegisterFields(), ClaimedKeys(), ResolvedField, newAutoAcc(), claimField(), T, hintLookup(), TestAutoRegisterFields_ClaimedKeysSuppressed() (+14 more)

### Community 55 - "ResolveFields"
Cohesion: 0.26
Nodes (22): Person, NewPersonBaseline(), ResolvedField, T, personTestFields(), resolvedByCanonical(), TestPersonBaseline_ClaimsNamespaceWithEmptyValue(), TestPersonBaseline_ManualPinStaysFrozen() (+14 more)

### Community 56 - "writeback/writeback.go"
Cohesion: 0.21
Nodes (23): buildFFmpegArgs(), copyFile(), downloadImageToTemp(), existingTagsXML(), ffmpegMetadataKey(), Builder, Context, isNotFound() (+15 more)

### Community 57 - "Handlers"
Cohesion: 0.25
Nodes (8): refreshAllResult, Hint, Handlers, Context, EnrichedField, HandlerFunc, Request, ResponseWriter

### Community 58 - "Context"
Cohesion: 0.16
Nodes (12): canonicalTable(), Context, Repo, Tx, mergeEntityLookupErr(), orderPair(), selfRefAncestorIDs(), New() (+4 more)

### Community 59 - "getJSON"
Cohesion: 0.24
Nodes (21): fakeRescanner, getJSON(), Repo, T, linkPeople(), linkPeopleAs(), newServer(), seedVideo() (+13 more)

### Community 60 - "openAt"
Cohesion: 0.17
Nodes (16): T, TestMigration0031DeniedTagsUpAndDown(), T, TestMigration0029FieldClaimsProviderGrain(), count(), DB, T, mustExec() (+8 more)

### Community 61 - "resolveOrCreateByName"
Cohesion: 0.17
Nodes (13): attachExternalID(), Context, Repo, Tx, lookupByNameKey(), nameKeyExpr(), resolveOrCreateByName(), Context (+5 more)

### Community 62 - "identity_ops_test.go"
Cohesion: 0.19
Nodes (21): Repo, T, studioIDByName(), tagIDByName(), TestEntityConflictExcludesSelf(), TestKeepSeparateStore(), TestMergeEntitiesValidation(), TestMergeEntitiesWithAffectedVideos_UnknownEntityType() (+13 more)

### Community 63 - "fakeRepo"
Cohesion: 0.12
Nodes (12): Time, Context, ExtraMetadata, JobRun, Mutex, Video, VideoStat, artExtractor (+4 more)

### Community 64 - "scanner_test.go"
Cohesion: 0.26
Nodes (19): T, TestBuildVideoFromFileForcesExtractWithoutPersisting(), T, TestExtractionHook(), TestExtractionHook_ErrorDoesNotFailScan(), activeCount(), T, newFakeRepo() (+11 more)

### Community 65 - "f36.ts"
Cohesion: 0.18
Nodes (23): baselineCandidateValue(), decidedSource(), fileCandidateValue(), isPendingSelection(), isProviderSource(), isReplaceField(), needsWriteback(), outOfSync() (+15 more)

### Community 67 - "process_test.go"
Cohesion: 0.16
Nodes (15): ExtractionReviewCall, fakeManualSource, fakeResolver, fakeReviewStore, Context, T, TestProcess_EntityField_ExactMatchAutoApplies(), TestProcess_EntityField_FuzzyMatchQueuesWithSuggestion() (+7 more)

### Community 68 - "Design Handoff: Unified nav search — live, tabbed, in-place filtering panel"
Cohesion: 0.06
Nodes (33): Accessibility notes (summary), Design Handoff: Unified nav search — live, tabbed, in-place filtering panel, Design-system fit, Mobile (< 640px, the primary complaint driving this spec), Overview, Part A — The tab row lives with the box, not inside the dropdown, Part B — `SearchResultsPanel.svelte` (NS1), Part C — Per-page removal (NS4) (+25 more)

### Community 69 - "seedPerson"
Cohesion: 0.24
Nodes (20): Person, Repo, T, headshotVersionOf(), seedPerson(), TestCollapseDuplicateGalleryExtras(), TestCoreSlotReplace(), TestDeleteSuppressesEnrichmentURL() (+12 more)

### Community 70 - "routes/tags/+page.svelte"
Cohesion: 0.06
Nodes (32): PopoverMenu, PopoverMenuOptions, i(), createAndAssign(), focusOption(), onKey(), onOptionKey(), optionCount (+24 more)

### Community 71 - "Provider"
Cohesion: 0.27
Nodes (9): ForComputed(), ForNamespace(), ForProvider(), Provider(), T, TestForNamespace(), TestProviderRoundTrip(), TestValid() (+1 more)

### Community 72 - "Handlers"
Cohesion: 0.27
Nodes (7): Handlers, PersonImageSet, Reader, Request, ResponseWriter, parseImageID(), readAllLimited()

### Community 75 - "Manager"
Cohesion: 0.19
Nodes (7): Context, Int64, Logger, Manager, New(), Config, Repository

### Community 76 - "api/person_images_test.go"
Cohesion: 0.36
Nodes (18): deleteReq(), fillGallery(), getStatus(), Repo, Response, T, personImageServer(), personImageServerCfg() (+10 more)

### Community 77 - ".scaleToWidth"
Cohesion: 0.14
Nodes (15): Context, Manager, writeAtomic(), absPath(), Context, Reader, Manager, lastLine() (+7 more)

### Community 78 - "Write"
Cohesion: 0.34
Nodes (14): TestReadCurrentValues_RoundTrips(), T, mustReadDir(), requireExiftool(), requireMkvpropedit(), TestBuildFFmpegArgs_AlwaysMapsAllStreams(), TestMergeTagsXML(), TestMergeTagsXML_NoExisting() (+6 more)

### Community 79 - "enrich/enrich.go"
Cohesion: 0.19
Nodes (15): fileConfig, IconRef, Registry, Store, Empty(), entityTypesSupport(), Source, Logger (+7 more)

### Community 80 - "0001_init.up.sql"
Cohesion: 0.05
Nodes (30): people, people_fts, tags, tags_fts, video_metadata, video_people, video_tags, videos (+22 more)

### Community 81 - "Process"
Cohesion: 0.24
Nodes (15): Decision, Deps, Enqueuer, FieldExtraction, ManualSourceChecker, Outcome, Resolver, ReviewStore (+7 more)

### Community 82 - "authServer"
Cohesion: 0.30
Nodes (20): authServer(), exchange(), findCookie(), getCookie(), getTok(), Cookie, Repo, Response (+12 more)

### Community 83 - "Context"
Cohesion: 0.22
Nodes (10): Context, JobRun, Repo, Rows, Time, jobRunCutoff(), scanJobRuns(), JobKindDigest (+2 more)

### Community 84 - "New"
Cohesion: 0.45
Nodes (16): TestQueue_RevertUnknownBatch(), New(), Logger, Repo, T, newRepo(), seedVideo(), testLogger() (+8 more)

### Community 85 - "SanitizeFieldHints"
Cohesion: 0.39
Nodes (8): FieldHint, SanitizeFieldHints(), T, TestAssetHostAllowed(), TestManifest_DecodeBackwardCompat(), TestSanitizeFieldHints_DropsCanonicalReservedAndUnknownVocab(), TestSanitizeFieldHints_EmptyAndNil(), TestSanitizeFieldHints_LabelSanitized()

### Community 86 - "thumbServer"
Cohesion: 0.17
Nodes (21): stubThumbs, Context, Repo, T, seedThumbVideo(), TestAdminStatus(), TestListEnqueuesVisibleAndExposesURL(), TestRegenerateDisabled() (+13 more)

### Community 87 - "identityServer"
Cohesion: 0.13
Nodes (28): T, reqTokBody(), TestCategoryEndpoints(), TestResolveOrCreateTagEndpoint(), getJSONTok(), T, TestDuplicatesEndpoints(), TestNearMissEndpoint() (+20 more)

### Community 88 - "Derive"
Cohesion: 0.16
Nodes (25): fieldByKey(), ResolvedField, dependencyLabels(), Derive(), deriveAge(), deriveAgeAtDeath(), firstValues(), ResolvedField (+17 more)

### Community 89 - "queue"
Cohesion: 0.19
Nodes (8): Mutex, newQueue(), drain(), T, TestQueueDedupAndDepth(), TestQueueDedupWhileInFlight(), TestQueueHighPriorityFirst(), queue

### Community 91 - "QA Checklist: System Activity (F21)"
Cohesion: 0.17
Nodes (11): F21: System Activity — Under the Hood, Accessibility, Controls (owner), Gating (F21.7) — needs `ADMIN_TOKEN` to exercise, Header activity indicator, Job history, QA Checklist: System Activity (F21), Reachability & shell (+3 more)

### Community 93 - "Decision"
Cohesion: 0.14
Nodes (14): ADR-023: Image Distribution — Published GHCR Image + Pull-Based Compose, Consequences, Context, Decision, Tagging, ci.yml — PR/push gate (go vet, go test, svelte-check, Vitest, vite build, theming grep guard), image.yml — reusable multi-arch build/push + Trivy scan, release.yml — tag v* triggers image.yml then cuts a GitHub Release (+6 more)

### Community 94 - "routes/+layout.svelte"
Cohesion: 0.11
Nodes (18): DismissableOptions, activeRowIndex, activeTabIndex, announcement, flatCount, focusRow(), onRowKey(), onTabKey() (+10 more)

### Community 95 - "QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)"
Cohesion: 0.40
Nodes (4): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human look, QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)

### Community 96 - "JaroWinkler"
Cohesion: 0.26
Nodes (13): BestFuzzyMatch(), classifyAgreement(), classifySpecificity(), commonPrefixLen(), jaro(), JaroWinkler(), approxEqual(), T (+5 more)

### Community 97 - "Context"
Cohesion: 0.28
Nodes (6): attachStudioExternalID(), Context, Repo, Studio, Tx, resolveOrCreateStudio()

### Community 98 - "seedTagTree"
Cohesion: 0.24
Nodes (22): assertTagParent(), Context, Repo, T, ptr(), seedTagTree(), TestAncestorNamesForTag(), TestListTagsWritebackEnabled() (+14 more)

### Community 99 - "resolver/decisions_test.go"
Cohesion: 0.34
Nodes (14): decide(), ResolvedField, T, providerCandidate(), TestResolve_CandidatesListFileAndMatchedProviders(), TestResolve_DecisionAdoptProvider(), TestResolve_DecisionKeepFileOverridesMappingOrder(), TestResolve_DecisionManualLiteral() (+6 more)

### Community 100 - ".setPersonFieldDecision"
Cohesion: 0.21
Nodes (9): curationBody, Handlers, Request, ResponseWriter, validateCurationBody(), validCurationAction(), Handlers, Request (+1 more)

### Community 101 - ".addEntityAlias"
Cohesion: 0.45
Nodes (5): identityRoutes, Handlers, Context, Request, ResponseWriter

### Community 102 - "refreshServer"
Cohesion: 0.31
Nodes (12): stubFileExtractor, Context, ExtraMetadata, Repo, T, Video, refreshPOST(), refreshServer() (+4 more)

### Community 104 - "QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)"
Cohesion: 0.33
Nodes (5): §1 Setup, §2 Smoke, §3 Agent live QA (all 3 skins), §4 Human, QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)

### Community 105 - "Orchestrator"
Cohesion: 0.20
Nodes (10): cachingResolver, FieldOutcome, Orchestrator, Result, VideoLookup, fileTagValues(), Context, Mutex (+2 more)

### Community 106 - "ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam"
Cohesion: 0.13
Nodes (15): Action Items, ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam, Consequences, Context, Current state (survey, 2026-07-31), D1 — `tags.writeback_enabled` column; filtered at `TagNamesForVideo`, uniformly per name regardless of how it was reached, D1 — where the flag is enforced, D2 — Manual sync batch-enqueues per-video via `genreWritebackValuesForVideo`, not a precomputed name list; shared `batchID` across single- and bulk-tag triggers (+7 more)

### Community 107 - "ADR-080: Configurable per-provider metadata search query patterns"
Cohesion: 0.08
Nodes (25): A — core renders a string, `/resolve` contract unchanged (chosen), A — embed in the existing entity payload (chosen), A — operator > provider > global default > raw title (chosen), A — strip bracket punctuation + resolution tokens, collapse whitespace (chosen), Action Items, ADR-080: Configurable per-provider metadata search query patterns, B — leave the floor tier literal; rely on operator-configured patterns to work around messy titles, B — new endpoint, picker fetches on open (+17 more)

### Community 108 - "Context"
Cohesion: 0.13
Nodes (11): Context, Person, Repo, Tx, resolveOrCreatePerson(), Context, Repo, Tx (+3 more)

### Community 109 - "newRepoDB"
Cohesion: 0.36
Nodes (13): DB, Repo, newRepoDB(), DB, reviewPair, T, hasPair(), mustExec() (+5 more)

### Community 110 - "repo/related_test.go"
Cohesion: 0.35
Nodes (13): Repo, T, itemIDs(), personID(), sameSet(), tagID(), TestRelatedActiveOnly(), TestRelatedEmptyAndNullBlocks() (+5 more)

### Community 111 - "repo/studios_test.go"
Cohesion: 0.30
Nodes (13): Repo, Studio, T, studioByName(), studioNames(), TestGetStudio_NotFound(), TestListStudios_AttachesImageVersions(), TestReconcileVideoStudios_CreateReplacePrune() (+5 more)

### Community 112 - "TestQueue_SnapshotsAndReverts"
Cohesion: 0.64
Nodes (8): batchIDFromDetail(), T, newMinimalMKV(), requireExiftool(), requireFFmpeg(), TestQueue_EnqueueBatch_SharedBatchIDGroupsMultipleVideos(), TestQueue_SnapshotsAndReverts(), TestQueue_SnapshotSurvivesCrashRetry()

### Community 113 - ".claimTarget"
Cohesion: 0.38
Nodes (5): Handlers, Context, Request, ResponseWriter, IsKnown()

### Community 114 - ".setFieldPromotion"
Cohesion: 0.33
Nodes (6): Handlers, Context, Request, ResolvedField, ResponseWriter, parseEntityType()

### Community 115 - "Decision"
Cohesion: 0.12
Nodes (16): 1. Type source: PR title, not individual commits, 2. Scope signal: changed-file globs, with a threshold, 3. Advisory, not blocking, 4. New workflow, not a job in `jira-sync.yml`, 5. Script shape, 6. Allowlist, not the `docs/**`-denylist `jira-sync.yml` uses, Action Items, ADR-076: Advisory CI check — `docs`/`chore`-typed PRs that touch non-doc code (+8 more)

### Community 116 - "BatchRunner"
Cohesion: 0.26
Nodes (7): BatchRunner, JobRecorder, VideoLister, Context, Logger, Mutex, Time

### Community 118 - "compilerOptions"
Cohesion: 0.15
Nodes (12): ./.svelte-kit/tsconfig.json, compilerOptions, allowJs, checkJs, esModuleInterop, forceConsistentCasingInFileNames, moduleResolution, resolveJsonModule (+4 more)

### Community 119 - "listScroll.svelte.ts"
Cohesion: 0.22
Nodes (11): browseCache, BrowseSnapshot, ADR-0032, listScroll, ListScrollSnapshot, ADR-0032, createNavSnapshot(), createNavSnapshotRegistry() (+3 more)

### Community 120 - ".SetDelete"
Cohesion: 0.33
Nodes (4): purger, trashItem, Duration, Time

### Community 121 - "Lookup"
Cohesion: 0.15
Nodes (14): promotionBody, promotionView, Handlers, Context, ResolvedField, hasNonEmpty(), lookupHint(), normalizeGroupOrEmpty() (+6 more)

### Community 122 - "Fake"
Cohesion: 0.29
Nodes (7): resolveCounter, Asset, EnrichResult, Fake, FakePerson, Context, idNamespace()

### Community 123 - "Handlers"
Cohesion: 0.31
Nodes (7): categoryTagIDsBody, Handlers, Category, Context, Request, ResponseWriter, parseCategoryName()

### Community 124 - "FlagNearMiss"
Cohesion: 0.23
Nodes (11): looseKeyExpr(), sqlStringLit(), dropReviewPairsFor(), FlagNearMiss(), Context, EntityRef, Repo, ReviewPair (+3 more)

### Community 125 - "query_test.go"
Cohesion: 0.18
Nodes (19): QueryFields, queryToken, Source, parseQueryPattern(), renderPattern(), sanitizeTitle(), T, TestSanitizeTitle() (+11 more)

### Community 126 - "Candidate"
Cohesion: 0.17
Nodes (10): Candidate, httpClient, Client, Context, Source, Source, newHTTPClient(), T (+2 more)

### Community 127 - "Route"
Cohesion: 0.36
Nodes (10): Decision, Route(), T, TestRoute_BelowThreshold_RoutesToReview(), TestRoute_ExactMatchGate_AutoApplies(), TestRoute_FuzzyMatchNeverAutoApplies(), TestRoute_ManualOverrideAlwaysWins(), TestRoute_NonEntityField_AutoAppliesWithoutEntityMatch() (+2 more)

### Community 128 - "Store"
Cohesion: 0.35
Nodes (9): EnrichmentWriter, Context, Store(), Repo, T, newRepo(), TestFilenameSourceResolvesWithNoResolverChange(), TestStore_EmptyFieldsIsNoop() (+1 more)

### Community 129 - ".add"
Cohesion: 0.45
Nodes (9): Manager, T, newFakeRepo(), TestDisabledManagerNoops(), TestExtractEmbedded(), testManager(), TestProcessGeneratesAndMarks(), TestProcessMarksFailed() (+1 more)

### Community 130 - "Spec: Tag Writeback Exclusion — per-tag Genre writeback control"
Cohesion: 0.17
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 131 - "TestAttachMaterializedTags"
Cohesion: 0.53
Nodes (5): T, TestAttachMaterializedTags(), TestAttachTagToVideo(), TestAttachTagToVideo_UpgradesFileSource(), TestDetachTagFromVideo()

### Community 132 - "nationality.ts"
Cohesion: 0.07
Nodes (27): Derivation (see the spec for detail), Design Handoff: Nationality flag beside the person name (HOLODEX-139), Placement & measurements, States, Theming notes (what bites these surfaces), 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — all three skins (+19 more)

### Community 134 - "newAssetClient"
Cohesion: 0.24
Nodes (8): AssetClient, assetHostAllowed(), assetRoleFor(), Client, Context, Source, newAssetClient(), URL

### Community 135 - ".enrichQueueForType"
Cohesion: 0.31
Nodes (6): EnrichQueueProviderState, Context, EntityRef, Repo, EnrichQueueProviderState, EnrichQueueRow

### Community 136 - "extractServer"
Cohesion: 0.53
Nodes (10): extractPOST(), extractServer(), Repo, T, TestAdminExtractAllAccepted(), TestAdminExtractAllUnavailable(), TestExtractMediaMatch(), TestExtractMediaNotFound() (+2 more)

### Community 137 - "identity_test.go"
Cohesion: 0.38
Nodes (10): contains(), Repo, T, peopleCount(), tagCount(), TestAliasRoutesOnScan(), TestEntityNames(), TestExactEntityMatch() (+2 more)

### Community 138 - "Context"
Cohesion: 0.24
Nodes (5): Context, DB, Repo, statusCounts(), WritebackJobInsert

### Community 139 - "ResolveForContainer"
Cohesion: 0.31
Nodes (9): ImageTagForField(), ResolveForContainer(), TagForField(), T, TestImageTagForField(), TestTagForField(), buildBatch(), FieldValues (+1 more)

### Community 140 - "demo/package.json"
Cohesion: 0.18
Nodes (10): sharp, dependencies, sharp, description, name, private, scripts, generate (+2 more)

### Community 141 - "Spec: Owner-authored person & studio ↔ media links, with file writeback (F40)"
Cohesion: 0.10
Nodes (20): API, Before implementation, Behavior detail, Future considerations (P2), Goals, Link derivation (RD2/RD3), Must-have (P0), Non-Goals (+12 more)

### Community 142 - ".dismissDuplicate"
Cohesion: 0.27
Nodes (5): Handlers, HandlerFunc, Request, ResponseWriter, validEntityType()

### Community 143 - "writeError"
Cohesion: 0.12
Nodes (18): Handlers, Request, ResponseWriter, Handlers, Request, ResponseWriter, ResponseWriter, writeError() (+10 more)

### Community 144 - "Health"
Cohesion: 0.18
Nodes (7): Health, scanStatusSource, Bool, Time, Request, ResponseWriter, writeStatus()

### Community 145 - "MergePersons(canonical, merged) transaction"
Cohesion: 0.67
Nodes (3): MergePersons(canonical, merged) transaction, videos.deleted_at TEXT NULL column (migration 0010), person_image_suppressions table (person_id, source_url) + person_images.source_url (migration 0012)

### Community 147 - "Spec: Tag Categories — grouping tags without merging them"
Cohesion: 0.17
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 148 - "QA Checklist: Derived / calculated person fields — Age & Age at death (F45)"
Cohesion: 0.33
Nodes (5): §1 Setup, §2 Smoke (run in `make test` / `npm run test`), §3 Agent live QA (preview tools against §1 stack), §4 Human eyes — 3-skin QA (Cinémathèque · Broadcast · Brutalist), QA Checklist: Derived / calculated person fields — Age & Age at death (F45)

### Community 149 - "model.go"
Cohesion: 0.17
Nodes (17): Person, Tag, Time, Category, EnrichedField, EntityRef, ExtraMetadata, JobRun (+9 more)

### Community 150 - "confidence.go"
Cohesion: 0.23
Nodes (17): Agreement, EntityMatch, Specificity, Tier, AutoApplyThreshold(), IsEntityField(), scoreAgreement(), ScoreEntity() (+9 more)

### Community 151 - "personDerivedServer"
Cohesion: 0.56
Nodes (9): findField(), getResolved(), T, indexOf(), personDerivedServer(), TestPersonDerived_AgeAtDeathReplacesAge(), TestPersonDerived_AgeUnderBirthdate(), TestPersonDerived_ComputedDecisionRejected() (+1 more)

### Community 152 - "Context"
Cohesion: 0.31
Nodes (5): Context, Repo, Time, ProviderIcon, ProviderIconInsert

### Community 153 - "seedTwoTags"
Cohesion: 0.44
Nodes (9): Repo, T, seedTwoTags(), TestNearMiss(), TestReviewQueue_DismissRecordsKeepSeparate(), TestReviewQueue_InternalWhitespaceVariation(), TestReviewQueue_MergeDropsPair(), TestReviewQueue_ScanFlagsAndList() (+1 more)

### Community 154 - "Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)"
Cohesion: 0.10
Nodes (21): API, Behavior detail, Future considerations (P2), Gate status, Goals, Must-have (P0), Non-Goals, Open Questions (+13 more)

### Community 155 - "attachTagTx"
Cohesion: 0.33
Nodes (7): attachTagTx(), Context, Repo, Tag, Tx, resolveOrCreateTagName(), MaterializedTag

### Community 156 - "tagWritebackSyncServer"
Cohesion: 0.38
Nodes (15): Handlers, Mutex, Repo, T, noQueueTagServer(), patchTok(), seedGenreVideo(), tagWritebackSyncServer() (+7 more)

### Community 157 - "QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)"
Cohesion: 0.40
Nodes (4): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — all three skins, QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)

### Community 158 - "Manual QA Checklist: People Images (F25)"
Cohesion: 0.40
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: People Images (F25)

### Community 159 - ".CurationForVideos"
Cohesion: 0.39
Nodes (4): curationNorm(), Context, Repo, CurationRow

### Community 160 - "Context"
Cohesion: 0.36
Nodes (3): Context, Repo, DecisionRow

### Community 161 - "Context"
Cohesion: 0.44
Nodes (3): Context, Repo, PromotionRow

### Community 162 - "placeholders"
Cohesion: 0.24
Nodes (7): placeholders(), Context, DB, Repo, Tag, isTagDescendant(), videoIDsForTagsQuery()

### Community 163 - "ImagePath"
Cohesion: 0.44
Nodes (7): ImagePath(), Remove(), Store(), studioDir(), T, TestImagePath_ServerAssignedIDsOnly(), TestStoreRemove_RoundTrip()

### Community 164 - "Spec: Job History — Digest, Pagination, Entity Search (F21.3b)"
Cohesion: 0.40
Nodes (5): ADR-071 (job-run attribution and paginated history), entity_type/entity_id/batch_id attribution columns (P0-1/P0-2), Job-run digest (per-kind aggregate, P0-3), Spec: Job History — Digest, Pagination, Entity Search (F21.3b), P0-4/P0-6 (paginated log, adjacency rollup) dropped after Q1

### Community 165 - "WritebackFormDialog.svelte"
Cohesion: 0.12
Nodes (15): Writeback components, onKeydown(), submit(), trapTab(), BatchStatus, JOB_POLL_TIMEOUT_MS, pollUntilSettled(), fast (+7 more)

### Community 166 - ".adminActivity"
Cohesion: 0.20
Nodes (9): activityResponse, activitySystem, Handlers, Request, ResponseWriter, ScanStatus, atoiDefault(), LibraryCounts (+1 more)

### Community 167 - "Context"
Cohesion: 0.33
Nodes (5): Category, Context, Repo, Tag, nameCollidesInTable()

### Community 168 - "recordingSink"
Cohesion: 0.43
Nodes (3): recordingSink, storedAsset, Context

### Community 169 - "Context"
Cohesion: 0.43
Nodes (3): Context, Repo, ClaimRow

### Community 170 - "Context"
Cohesion: 0.32
Nodes (4): Context, Repo, Time, WritebackSnapshot

### Community 172 - "fakeRepo"
Cohesion: 0.43
Nodes (4): Context, Mutex, ThumbnailCandidate, fakeRepo

### Community 174 - "ReadCurrentValues"
Cohesion: 0.33
Nodes (7): currentTagValue(), Context, ReadCurrentValues(), snapshotValueToString(), T, TestReadCurrentValues_AbsentTagIsEmpty(), TestReadCurrentValues_SkipsImageFields()

### Community 175 - "hooks"
Cohesion: 0.29
Nodes (6): hooks, PostToolUse, PreToolUse, SessionStart, Stop, $schema

### Community 176 - "buildVideo"
Cohesion: 0.33
Nodes (5): DirEntry, FileInfo, buildVideo(), ExtraMetadata, Video

### Community 177 - "Decision"
Cohesion: 0.09
Nodes (21): ADR-018: Scanner Change Detection — Incremental Scan by (path, size, mtime), Consequences, Context, Decision, Mid-copy protection, Rationale, Scan algorithm, Stored fields (+13 more)

### Community 178 - "TestStoreRoundTrip"
Cohesion: 0.52
Nodes (5): ImagePath(), Remove(), Store(), T, TestStoreRoundTrip()

### Community 179 - "Test Fixtures"
Cohesion: 0.33
Nodes (5): Deterministic fixture corpus + golden-file pattern (testdata/gen.sh), Golden files, Regenerate, Test Fixtures, What's in the corpus

### Community 180 - "field_source_decisions table"
Cohesion: 0.67
Nodes (3): field_source_decisions table, four-tier label/render/group/order resolution ladder, settings KV table + typed Registry (validation/UI schema)

### Community 181 - "Spec: Unified entity name-identity (F43)"
Cohesion: 0.11
Nodes (17): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Owner tooling hub + nav split (F35), ADR-066 (enrichment auto-apply and dismissal), Auto-apply threshold (>=0.85 strong match, RD1), enrichment_dismissals table (durable not-matched verdict) (+9 more)

### Community 182 - ".genreWritebackItems"
Cohesion: 0.18
Nodes (14): firstResolvedValue(), ResolvedField, Video, resolvedValues(), videoHint(), applyGenreWriteback(), genreWritebackFieldValues(), Handlers (+6 more)

### Community 183 - "Router"
Cohesion: 0.14
Nodes (4): Handler, Handlers, Logger, Router()

### Community 184 - "holoShuffle"
Cohesion: 0.40
Nodes (4): holoShuffle(), registerShuffle(), T, TestHoloShuffle()

### Community 185 - "Spec: People Images (F25)"
Cohesion: 0.08
Nodes (26): Access control & security, Addendum — configurable cap, owner override & enrichment suppression ([ADR-043](../architecture/ADR-043-gallery-cap-and-enrichment-suppression.md), 2026-06-25), Addendum — enrichment photos are deduplicated in the gallery ([ADR-050](../architecture/ADR-050-image-content-dedup.md), F34, 2026-06-29), Addendum — owner/admin cap bypass, gallery grid modal, image viewer (HOLODEX-174, 2026-07-08), Addendum — owner-set core images take precedence over enrichment ([ADR-049](../architecture/ADR-049-manual-image-precedence.md), F33, 2026-06-28), Artifacts to produce (project working agreements), Data, storage & serving (direction — finalized in the ADR), F25.26–30 — Person-page polish (follow-ups) (+18 more)

### Community 187 - "stub.js"
Cohesion: 0.33
Nodes (3): fields, http, ADR-0033

### Community 189 - "Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)"
Cohesion: 0.13
Nodes (14): Accessibility Notes, Content specification (the string the owner actually sees), Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254), Design-system fit (the `/design-system` check), Edge case: empty sanitization result, Measured contrast, Non-goals (explicitly out of this change), Optional P1: seeded-value transparency caption (+6 more)

### Community 195 - "ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision"
Cohesion: 0.13
Nodes (15): Action Items, ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision, Consequences, Context, Current state (survey, 2026-07-31), D1 — `categories` table: minimal, no identity-spine membership, tag-style fold for its own uniqueness, D1 — where Category's CRUD lives, D2 — `category_tags` junction: mirrors `video_tags` exactly, no provenance column (+7 more)

### Community 197 - "gen-country-names.mjs"
Cohesion: 0.40
Nodes (4): countries, entries, OVERRIDE, require

### Community 198 - "field_claims.go"
Cohesion: 0.50
Nodes (3): claimBody, claimView, targetView

### Community 201 - "tag-categories-handoff.md"
Cohesion: 0.21
Nodes (4): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human look, QA Checklist: Tag writeback exclusion frontend (HOLODEX-239)

### Community 203 - ".RoundTrip"
Cohesion: 0.50
Nodes (3): Request, Response, rebaseTransport

### Community 205 - "svelte.config.js"
Cohesion: 0.50
Nodes (3): config, ADR-0002, ADR-0007

### Community 214 - "Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)"
Cohesion: 0.13
Nodes (15): 1. `/tags` — unified type filter + search, 2. Category pill, 3. `/categories/{id}` detail page (new route), 4. Bulk "Add to category…" / "Remove from category…" (Manage-mode bar), 5. Browse-page "Categories" facet, Accessibility notes, Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240), Design-system-fit audit (+7 more)

### Community 219 - "+layout.ts"
Cohesion: 0.50
Nodes (3): prerender, ssr, ADR-0002

### Community 223 - "ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)"
Cohesion: 0.29
Nodes (7): ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary), Client Configuration Examples, Configuration, Consequences, Context, Decision, Rationale

### Community 236 - "Spec: Configurable per-provider search query patterns (F54)"
Cohesion: 0.11
Nodes (18): Acceptance Criteria, FR1 — Operator pattern config (`metadata-sources.yaml`), FR2 — Provider-advertised preference (`/describe.preferred_search_pattern`), FR3 — Token grammar, rendering, and precedence fallthrough, FR4 — Unconditional title sanitizer, FR5 — Wiring: choke point, response payload, zero picker changes, Functional Requirements, Future Considerations (P2) (+10 more)

### Community 247 - "Spec: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.11
Nodes (18): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+10 more)

### Community 258 - "extractReviewServer"
Cohesion: 0.37
Nodes (16): extractReviewGET(), extractReviewPOST(), extractReviewServer(), Repo, T, TestDismissExtractionReview(), TestExtractionQueue_Empty(), TestExtractionQueue_ListsPendingRowsVideoJoined() (+8 more)

### Community 263 - "Spec: System Activity — "Under the Hood" (F21)"
Cohesion: 0.10
Nodes (21): Cross-References, Data Model Extensions, F21.1 — Activity read-model API, F21.2 — Scanner status accessor, F21.3 — Persisted job history (30-day), F21.4 — Dedicated activity page (polled), F21.5 — Header activity indicator, F21.6 — In-UI controls (wires existing admin actions) (+13 more)

### Community 264 - "CurationFieldRow.svelte"
Cohesion: 0.10
Nodes (19): commitEdit(), draft, editing, isProvider, onEditKey(), provenance, add(), adding (+11 more)

### Community 266 - "ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity"
Cohesion: 0.10
Nodes (20): A — Mandatory, no name fallback (chosen), A — Shared namespace, cross-provider convergence (chosen), Action Items, ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity, B — Preferred, name fallback quarantined, B — Provider-scoped keys `(provider, external_id)`, Conformance table (the invariant applied per entity), Consequences (+12 more)

### Community 269 - "Spec: People on the unified source-of-truth model (F37)"
Cohesion: 0.10
Nodes (20): API, Behavior detail, Future considerations (P2), Goals, Merge (RD5), Must-have (P0), Name materialization (RD1), Non-Goals (+12 more)

### Community 270 - "Normalize"
Cohesion: 0.32
Nodes (16): Normalize(), forgePNGDims(), T, jpegBytes(), pngBytes(), TestGenderBucket(), TestNormalizeAcceptsWebP(), TestNormalizeDownscales() (+8 more)

### Community 272 - "Design Handoff: Owner tooling hub + nav split (F35)"
Cohesion: 0.11
Nodes (19): Accessibility, Design Handoff: Owner tooling hub + nav split (F35), Design-system fit (the `/design-system` check), Design tokens used (header), Design tokens used (hub), Edge cases, Implementation pointers (non-binding), Interaction (+11 more)

### Community 276 - "Spec: Studio as a first-class entity (F38)"
Cohesion: 0.11
Nodes (19): API, Behavior detail, Facet (P0-7), Future considerations (P2), Goals, Link derivation (RD1), Must-have (P0), Non-Goals (+11 more)

### Community 281 - "Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31)"
Cohesion: 0.11
Nodes (18): Accessibility notes, Agent / human (per skin: Cinémathèque, Broadcast, Brutalist), Animation / motion, Components, Copy (exact — sentence case, verb-first, no "successfully"), Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31), Design-system fit (the `/design-system` check), Design tokens used (+10 more)

### Community 284 - "Spec: Runtime owner-editable settings (F41)"
Cohesion: 0.11
Nodes (18): Acceptance Criteria, FR1 — Generic settings store (migration `0021`), FR2 — Override precedence + hot-reload, FR3 — Typed registry (allowlist), FR4 — Owner-gated settings API, FR5 — `/owner/settings` tab, Functional Requirements, Future Considerations (P2) (+10 more)

### Community 286 - "ADR-008: Caching Strategy — In-Process Cache with Redis-Ready Interface"
Cohesion: 0.09
Nodes (23): ADR-008: Caching Strategy — In-Process Cache with Redis-Ready Interface, Cache Interface, Configuration, Consequences, Context, Decision, Invalidation Strategy, What Gets Cached (+15 more)

### Community 291 - "worklog.md"
Cohesion: 0.10
Nodes (19): 2026-06-29 · S1 backend — the decision engine, 2026-06-30 · S2 frontend — SourceSelect, 2026-07-01 · S3 gate — integration + live QA, example showing how that hand-rolled plan maps onto the four-section schema. Not a live worklog., Gates — definition of done, HOLODEX-6 · Per-field source-of-truth (F36 / ADR-051), MIGRATION EXAMPLE (ADR-064 Action Item 2) — the F36 rollout expressed in the Flightplan schema., Session log — append-only (cap: last 8 sessions; older → archive/) (+11 more)

### Community 293 - "Decision"
Cohesion: 0.13
Nodes (15): ADR-003: SQLite (modernc.org/sqlite) + FTS5 chosen as database, WAL mode enabling concurrent reads during scanner writes, ADR-016: Database Migrations — golang-migrate with Embedded Versioned SQL, Consequences, Context, Decision, Rationale, 1. Global search (command-palette style) — primary search box (+7 more)

### Community 297 - "Design Handoff: Metadata Enrichment UI for People (F22)"
Cohesion: 0.12
Nodes (16): Accessibility Notes, Animation / Motion, Components, Confidence display, Design Handoff: Metadata Enrichment UI for People (F22), Design Tokens Used, Edge Cases, Layout (+8 more)

### Community 300 - "Spec: Quick Wins batch — Search history & "More with …" shelves"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, QW1 — Search history (client-only) (+8 more)

### Community 301 - "Spec: Sticky sort preferences + Random sort"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, Requirements (+8 more)

### Community 302 - "ADR-001: Backend Language — Go"
Cohesion: 0.15
Nodes (12): .showcase.md Portfolio Self-Report, Incremental indexing (mtime/size change detection), Metadata writeback, Open enrichment protocol (provider sidecars), Unified field resolution, ADR-001: Backend Language — Go, Consequences, Context (+4 more)

### Community 303 - "Decision"
Cohesion: 0.12
Nodes (15): ADR-011: Symlink Handling & Path Resolution, Configuration, Consequences, Context, Decision, Hardlinks, Loop protection, Resolution & dedup (+7 more)

### Community 307 - "Design Handoff: Media page — one sync verb, render-once fields (F36 / F39)"
Cohesion: 0.13
Nodes (15): Behaviour notes, Behaviour notes, Contrast, Design Handoff: Media page — one sync verb, render-once fields (F36 / F39), Design-system fit (the `/design-system` check), Layout, Layout, Not in scope (+7 more)

### Community 312 - "Functional Requirements"
Cohesion: 0.13
Nodes (15): F10: MCP Server, F11: Thumbnail Generation, F12: Browse UI Polish, F13: Observability, F20: Configurable Metadata Field Mapping, Functional Requirements, `get_video` response, In Scope (+7 more)

### Community 313 - "Design Handoff — People Images (F25)"
Cohesion: 0.14
Nodes (14): A. People list (`/people`) — headshot, Accessibility notes, Animation / motion, B. Person page (`/people/[id]`) — banner hero + gallery + owner tools, C. Video page (`/media/[id]`) — poster cards, Components, Design Handoff — People Images (F25), Design tokens used (+6 more)

### Community 317 - "QA: TMDB Provider Sidecar + ADR-039 Core Changes"
Cohesion: 0.13
Nodes (14): 0. Setup, 1. Provider contract — smoke (no real TMDB, no network), 2. ADR-039 core changes — `asset_hosts` allowlist, 3. End-to-end via Holodex + real TMDB provider, 4. Provider image (Docker), 5. Security checks, 6. Non-functional, 7. Film / Video enrichment (F26) (+6 more)

### Community 325 - "filters.ts"
Cohesion: 0.18
Nodes (10): buildQuery(), DEFAULT_SORT, filtersToParams(), MEDIA_SORTS, paramsToFilters(), SORT_ORDERS, ADR-0045, MediaFilters (+2 more)

### Community 328 - "Functional Requirements"
Cohesion: 0.15
Nodes (13): Data Model Extensions (Phase 3), F14: People Enrichment, F15: Tag Enrichment, F16: Metadata Source Plugins, F17: Metadata Writeback, F18: Autogenerated Preview Trailers, Functional Requirements, In Scope (+5 more)

### Community 336 - "Spec: Owner tooling hub + visitor/owner nav split (F35)"
Cohesion: 0.17
Nodes (12): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, Requirements (+4 more)

### Community 337 - "Spec: Person Aliases (F23)"
Cohesion: 0.17
Nodes (12): API, Data model, Functional Requirements, In scope, Non-functional, Objective, Open questions, Out of scope (tracked follow-ups, not gaps) (+4 more)

### Community 341 - "Context"
Cohesion: 0.23
Nodes (7): ExtractionCandidate, SplitJoined(), Context, Repo, ExtractionCandidate, ExtractionQueueRow, ExtractionReviewRow

### Community 350 - "studio-entity.md"
Cohesion: 0.07
Nodes (21): §1 Setup, §2 Smoke (run in `make test` / `npm run test`), §3 Agent live QA (preview tools against §1 stack), §4 Human (3-skin eyeball — Cinémathèque, Broadcast, Brutalist), QA Checklist: People on the unified source-of-truth model (F37), 1. Studio next to the title, 2. Commentary block, 3. Poster upload (+13 more)

### Community 351 - "Spec — Showcase Demo Corpus"
Cohesion: 0.07
Nodes (25): ADR-006: REST + OpenAPI 3.1 API design under /api/v1, ADR-012: Resolution Classification — Width-Based Buckets with 10% Tolerance, Consequences, Context, Decision, Effective cutoffs (nominal − 10%), Nominal tier widths, Rationale (+17 more)

### Community 353 - "Decision"
Cohesion: 0.12
Nodes (13): FileSystem, Request, ResponseWriter, 1. Embed source lives in the `cmd/holodex` package, 2. SPA fallback handler, 3. BuildKit cache mounts, 4. `.dockerignore`, 5. Startup logs the URL (+5 more)

### Community 357 - ".resolveStudio"
Cohesion: 0.27
Nodes (8): Studio, setStudioImageURLs(), Handlers, Context, Request, ResolvedField, ResponseWriter, Studio

### Community 374 - "QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)"
Cohesion: 0.29
Nodes (6): §1 Setup, §2 Smoke (`make test`), §3 Agent (live, one skin), §4 Human (all three skins — Cinémathèque, Broadcast, Brutalist), §5 Known gaps, QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)

### Community 375 - "QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore)"
Cohesion: 0.29
Nodes (6): Agent (verified this session via DOM inspection), F25.29 — Post-enrichment image freshness, Human (needs eyes — not capturable in the headless preview), QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore), Setup, Smoke

### Community 378 - "Manual QA Checklist: Metadata Enrichment for People (F22)"
Cohesion: 0.17
Nodes (10): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs a human's eye, Manual QA Checklist: Metadata Enrichment for People (F22), Behaviour, F22 enrich stub — fake metadata-source provider for manual QA, Slow-network mode (QA 3.18 / 3.21) (+2 more)

### Community 381 - "ADR-010: MKV (Matroska) Tag-Target Precedence"
Cohesion: 0.22
Nodes (8): metadata.Extractor Go interface encapsulating tool implementations, ADR-004: layered ffprobe + exiftool + ffmpeg metadata extraction pipeline, ADR-010: MKV (Matroska) Tag-Target Precedence, Consequences, Context, Decision, Rationale, Tool Mapping

### Community 383 - "Manual QA Checklist: Admin Mode (F29)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Admin Mode (F29)

### Community 385 - "Manual QA Checklist: Person Aliases (F23)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Person Aliases (F23)

### Community 398 - "Design Handoff: Tag & category create affordance (HOLODEX-243)"
Cohesion: 0.15
Nodes (13): 1. The "+ New" pill, 2. Expanded form, 3. Submit behavior — diverges by type (important asymmetry), 4. Empty-state wiring, 5. Interaction states, 6. Edge cases, Accessibility notes, Design Handoff: Tag & category create affordance (HOLODEX-243) (+5 more)

### Community 459 - "Spec: Tag & Category Create Affordance — closing the /tags creation gap"
Cohesion: 0.15
Nodes (13): Goals, Implementation note (2026-08-01), Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement (+5 more)

### Community 460 - "decisionServer"
Cohesion: 0.30
Nodes (13): decisionServer(), Repo, T, resolvedField(), TestDecisionAPI_AdoptProviderThenClear(), TestDecisionAPI_ManualLiteral(), TestDecisionAPI_OwnerGated(), TestDecisionAPI_Validation() (+5 more)

### Community 461 - "Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)"
Cohesion: 0.15
Nodes (13): Behaviour notes, Bulk bar (`tags/+page.svelte`), Component: `WritebackBatchDialog.svelte`, Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239), Design-system fit (the `/design-system` check), Details card (`tags/[id]/+page.svelte`), Layout, Measured contrast (all three skins, dialog + card surfaces) (+5 more)

### Community 462 - "HOLODEX-240.md"
Cohesion: 0.40
Nodes (4): 2026-07-31 · session, Gates — definition of done, Session log   (append-only), Up next   (ordered — position is the priority; top line is the next action)

### Community 463 - ".runTagWritebackSync"
Cohesion: 0.47
Nodes (4): Handlers, Context, Request, ResponseWriter

### Community 464 - "Context"
Cohesion: 0.18
Nodes (8): fakePersonRepo, fakeStudioRepo, Context, PersonImage, Time, PersonImageInsert, StudioImage, StudioImageInsert

### Community 466 - "repo/categories_test.go"
Cohesion: 0.31
Nodes (10): Context, Repo, T, mustVideoID(), TestCategoryCrossTableCollision(), TestCategoryCRUD(), TestCategoryTagAssignment(), TestCategoryVideoFilterFacet() (+2 more)

### Community 467 - ".setFieldDecision"
Cohesion: 0.53
Nodes (3): Handlers, Request, ResponseWriter

### Community 468 - "itoa"
Cohesion: 0.45
Nodes (14): itoa(), getPersonBody(), Repo, T, personDecisionServer(), personResolvedField(), TestPersonCurationAPI_AliasesMergeField(), TestPersonDecisionAPI_OwnerGated() (+6 more)

### Community 469 - ".denyTag"
Cohesion: 0.48
Nodes (3): Handlers, Request, ResponseWriter

### Community 470 - "PersonImageRef"
Cohesion: 0.48
Nodes (3): Context, fakeBackfillRepo, PersonImageRef

### Community 471 - "Noop"
Cohesion: 0.24
Nodes (6): Cache, Noop, Store, Context, Duration, New()

### Community 473 - "Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)"
Cohesion: 0.14
Nodes (14): API, Before implementation, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Problem Statement, Requirements (+6 more)

### Community 474 - ".listProviders"
Cohesion: 0.29
Nodes (6): providerInfo, Handlers, Context, Request, ResponseWriter, providerIconURL()

### Community 475 - ".getMedia"
Cohesion: 0.22
Nodes (9): curationFromRows(), decisionsFromRows(), enrichmentFromRows(), Video, redactFileMetadataForVisitor(), setThumbnailURL(), Handlers, Person (+1 more)

### Community 476 - "HOLODEX-239.md"
Cohesion: 0.40
Nodes (4): 2026-07-31 · session, Gates — definition of done, Session log   (append-only), Up next   (ordered — position is the priority; top line is the next action)

### Community 477 - "QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)"
Cohesion: 0.33
Nodes (6): 1. Overlay on playback (media detail page), 2. Search-history dropdown (header), 3. "More with …" shelves (media detail page), 4. Fluid Back (browse grid), 5. Cross-cutting, QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)

### Community 479 - ".setTagParent"
Cohesion: 0.40
Nodes (3): Handlers, Request, ResponseWriter

### Community 480 - "Design handoff: In-app promote / override affordance for auto-registered fields (F44)"
Cohesion: 0.15
Nodes (13): 10. Three-skin QA (required), 11. What is explicitly not in this handoff, 1. The Promote control (owner-only, on the auto row), 2. The inline editor (shared promote + edit — DD2), 3. After promotion — the partition move (shared by all treatments), 4. Edit / Remove promotion (owner-only, on the promoted row), 5. States, 6. Responsive behavior (+5 more)

### Community 539 - "Sink"
Cohesion: 0.33
Nodes (5): personRepo, Sink, StudioRepo, Context, ReplaceStudioImageFile()

### Community 540 - "density.svelte.ts"
Cohesion: 0.21
Nodes (8): capForWidth(), clamp(), DENSITY_MAX, DENSITY_MIN, load(), MediaDensity, TIERS, ViewportTierCap

### Community 541 - "Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided""
Cohesion: 0.17
Nodes (11): Accessibility, Decided visual spec (Option 1), Design Handoff: Writeback dialog — poster comparison + enrichment/decision legibility gap, Fix options considered, Issue 1 — the dialog never shows the file's current poster next to the enriched candidate, Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided", Layout, Overview (+3 more)

### Community 542 - "Sweeper"
Cohesion: 0.32
Nodes (8): Context, Duration, Logger, Time, New(), Config, Repo, Sweeper

### Community 543 - ".RelinkVideoPeople"
Cohesion: 0.35
Nodes (5): relinkContext, Handlers, Context, ExtraMetadata, Video

### Community 544 - "coverArtManager"
Cohesion: 0.35
Nodes (9): assertDecodedWidth(), coverArtManager(), Manager, T, pngOfWidth(), TestWriteCoverArtTiersScaling(), TestWriteCoverArtTiersWithinBothCaps(), T (+1 more)

### Community 545 - ".mergePerson"
Cohesion: 0.31
Nodes (6): Handlers, Request, ResponseWriter, T, mergeBatchID(), namesByVideo()

### Community 546 - ".setStudioFieldDecision"
Cohesion: 0.47
Nodes (3): Handlers, Request, ResponseWriter

### Community 547 - "ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field"
Cohesion: 0.20
Nodes (10): 1. `studio_images` replaces `studio_logos` — three core roles, no gallery, 2. `enrich.ImageSink` / `downloadAssets` become entity-generic, 3. The studio `logo` field is retired; TMDB emits it as an asset, 4. Serving, upload, delete — mirrors ADR-057 §4 with an owner-write path added, Action items, ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field, Consequences, Context (+2 more)

### Community 548 - "HOLODEX-114.md"
Cohesion: 0.20
Nodes (9): 2026-07-10 · what happened this session, 2026-08-05 · session, Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing)., Flightplan worklog — one epic, one worklog, one definition of done., Gates — definition of done, HOLODEX-114 · <epic title>, Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 549 - "HOLODEX-253.md"
Cohesion: 0.20
Nodes (9): 2026-07-10 · what happened this session, 2026-08-05 · Backend + frontend implementation of F53, all applicable gates green, Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing)., Flightplan worklog — one epic, one worklog, one definition of done., Gates — definition of done, HOLODEX-253 · Two-tier video poster resolution (F53), Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 550 - "New"
Cohesion: 0.58
Nodes (9): New(), T, jpegBytes(), TestSinkRollsBackOnStoreFailure_Person(), TestSinkRollsBackOnStoreFailure_Studio(), TestSinkSkipsDuplicate(), TestSinkStoreAsset_Person_Normalizes(), TestSinkStoreAsset_Studio_Normalizes() (+1 more)

### Community 551 - "Context"
Cohesion: 0.36
Nodes (4): ValidStudioImageRole(), Context, Repo, Studio

### Community 552 - ".uploadVideoPoster"
Cohesion: 0.44
Nodes (5): Handlers, Request, ResponseWriter, PosterPath(), ThumbPath()

### Community 553 - ".resolvedFieldForVideo"
Cohesion: 0.42
Nodes (5): Handlers, Context, ExtraMetadata, ResolvedField, Video

### Community 554 - "newTestService"
Cohesion: 0.44
Nodes (8): Buffer, Service, T, newTestService(), TestPersistPreferredPattern_CachesValidPattern(), TestPersistPreferredPattern_EmptyClearsPriorValue(), TestPersistPreferredPattern_InvalidPatternDroppedAndLogged(), TestPersistPreferredPattern_PerProviderIsolation()

### Community 555 - "HOLODEX-244.md"
Cohesion: 0.22
Nodes (8): 2026-07-10 · what happened this session, Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing)., Flightplan worklog — one epic, one worklog, one definition of done., Gates — definition of done, HOLODEX-244 · <epic title>, Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 556 - "jobruns_test.go"
Cohesion: 0.39
Nodes (8): T, TestHasSuccessfulJobRun(), TestJobRunDigest(), TestJobRunDigestCleanWindow(), TestJobRunsAttributionRoundTrip(), TestJobRunsRecordAndList(), TestJobRunsRetention(), TestLibraryCounts()

### Community 557 - ".uploadStudioImage"
Cohesion: 0.54
Nodes (4): Handlers, Request, ResponseWriter, studioImageRole()

### Community 558 - "Design handoff: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.25
Nodes (8): 1. `/studios` list — logo well data source change only, 2. `/studios/{id}` detail — role-generic image control, 3. Provenance (P1, non-blocking), 4. Accessibility & 3-skin QA checklist, Design handoff: Studio image roles — icon / logo / poster (F51), Empty vs. populated states (per role), Interaction, Layout

### Community 559 - "Placeholder"
Cohesion: 0.46
Nodes (7): GenderBucket(), minF(), paletteFor(), Placeholder(), placeholderDims(), shoulderWidth(), skinPalette

### Community 561 - "TestDecisions_ForVideosBatch"
Cohesion: 0.60
Nodes (4): T, TestDecisions_ForVideosBatch(), TestDecisions_SetGetClear(), TestHasManualSource()

### Community 562 - "TestGetWritebackBatchStatus"
Cohesion: 0.67
Nodes (3): T, TestGetWritebackBatchStatus(), TestGetWritebackJobStatus()

## Knowledge Gaps
- **1384 isolated node(s):** `$schema`, `SessionStart`, `PostToolUse`, `Stop`, `PreToolUse` (+1379 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **231 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Canonical Field Registry (operator reference)` connect `Configuration Reference (holodex.yaml layers)` to `Lookup`?**
  _High betweenness centrality (0.288) - this node is a cross-community bridge._
- **Why does `Spec: Derived/calculated person fields (F45)` connect `Configuration Reference (holodex.yaml layers)` to `studio-entity.md`?**
  _High betweenness centrality (0.277) - this node is a cross-community bridge._
- **Why does `Open()` connect `Open` to `Store`, `extractReviewServer`, `extractServer`, `Handlers`, `Server`, `postTok`, `newRepo`, `run`, `personDerivedServer`, `enrich/enrich_test.go`, `tagWritebackSyncServer`, `coverArtManager`, `mcp.go`, `.uploadStudioImage`, `sendDecision`, `sendTok`, `holoShuffle`, `writeback/writeback.go`, `getJSON`, `openAt`, `Handlers`, `decisionServer`, `api/person_images_test.go`, `authServer`, `itoa`, `New`, `thumbServer`, `identityServer`, `.listProviders`, `refreshServer`, `newRepoDB`?**
  _High betweenness centrality (0.198) - this node is a cross-community bridge._
- **Are the 140 inferred relationships involving `newRepo()` (e.g. with `TestAliasesSurviveRescan()` and `TestMergePersons()`) actually correct?**
  _`newRepo()` has 140 INFERRED edges - model-reasoned connections that need verification._
- **Are the 94 inferred relationships involving `writeError()` (e.g. with `.addAlias()` and `.addEntityAlias()`) actually correct?**
  _`writeError()` has 94 INFERRED edges - model-reasoned connections that need verification._
- **Are the 85 inferred relationships involving `sampleVideo()` (e.g. with `TestAliasesSurviveRescan()` and `TestMergePersons()`) actually correct?**
  _`sampleVideo()` has 85 INFERRED edges - model-reasoned connections that need verification._
- **Are the 85 inferred relationships involving `itoa()` (e.g. with `TestAddAliasConflict409()` and `TestAliasEndpointsGatedAndValidated()`) actually correct?**
  _`itoa()` has 85 INFERRED edges - model-reasoned connections that need verification._