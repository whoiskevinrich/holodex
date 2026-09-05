# Graph Report - laughing-wu-649060  (2026-09-04)

## Corpus Check
- 918 files · ~1,292,303 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 6753 nodes · 14400 edges · 617 communities (383 shown, 234 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 1702 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d1c9de16`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- types.ts
- media/[id]/+page.svelte
- session-start.mjs
- Path
- Repo
- NewService
- decisionServer
- model.go
- Holodex Project Working Agreements (CLAUDE.md)
- testMappings
- devDependencies
- JobRun
- imagetools.mjs
- newProviderIconEnv
- Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)
- Service
- resolver.go
- format.ts
- Handlers
- sampleVideo
- Scanner
- newRepo
- Spec: Unified entity name-identity (F43)
- Design handoff: StudioLinkCard (reusable Studio display)
- architecture/README.md
- enrich/enrich_test.go
- Design Handoff: Unified name-edit mechanism (HOLODEX-269)
- tmdb.go
- Auth
- QA: Metadata Writeback (F28)
- net/http.Request
- Configuration Reference (holodex.yaml layers)
- Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)
- mcp.go
- ADR-075: Tag governance & video enrichment
- extractor.go
- Resolve
- ResolveReviewAction
- activity.svelte.ts
- people/+page.svelte
- jira-sync.mjs
- log/slog.Logger
- people
- People attach/detach + relationship picker (F56.5, HOLODEX-272)
- Open
- Spec: Tag & Category Create Affordance — closing the /tags creation gap
- Queue
- time.Duration
- identityServer
- generate.mjs
- Manual QA Checklist: Per-field source-of-truth decisions (F36)
- Video composite-key collision check (F56.3, HOLODEX-270)
- theme.svelte.ts
- CurationFieldRow.svelte
- studios
- ResolveFields
- writeback/writeback.go
- Handlers
- Design Handoff: Extract from filename on the media detail page (F48.5a)
- getJSON
- process.go
- .setFieldDecision
- identity_ops_test.go
- testing.T
- scanner_test.go
- f36.ts
- ADR-046 (per qa-metadata-curation.md): Metadata curation and write queue
- Process
- Design Handoff: Unified nav search — live, tabbed, in-place filtering panel
- seedPerson
- routes/tags/+page.svelte
- videos
- Studio relationship-edit popover (F56.4, HOLODEX-271)
- Quick Wins batch (overlay fix, search history, related shelves, fluid Back)
- ADR-058 (Jira transitions via direct REST API)
- Manager
- ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action
- .scaleToWidth
- sync.Mutex
- enrich/enrich.go
- Video
- writeError
- authServer
- Spec: Tag Detail — Hierarchy & Category Controls
- Orchestrator
- Repo
- Design handoff: PeopleGrid (reusable People/Cast display)
- itoa
- time.Time
- queue
- Release promotes by retagging the canaried digest
- QA Checklist: System Activity (F21)
- metadata-mappings.yaml config file: source-key-to-canonical-field mapping with precedence
- Decision
- .uploadVideoPoster
- QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)
- JaroWinkler
- Design handoff: Films entity (F56)
- seedTagTree
- Repo
- Session log — append-only (cap: last 8 sessions; older → archive/)
- .addEntityAlias
- New
- deleted_at soft-delete column (orthogonal to active)
- QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)
- BatchRunner
- ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam
- ADR-080: Configurable per-provider metadata search query patterns
- FilmStudioCascadeDialog.svelte
- Design handoff: TagLinkChip (reusable Tag display)
- repo/related_test.go
- repo/studios_test.go
- Design handoff: Media detail page reorder
- 0001_init.up.sql
- ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write
- Decision
- HOLODEX-280 · Film poster/thumbnail asset pipeline
- ADMIN_TOKEN env var — v1 owner identity, default-open when unset
- compilerOptions
- Design Handoff: Person Aliases ("Also known as") (F23)
- Design Handoff — People Images (F25)
- 0043_films.up.sql
- service.go
- Backfill
- Requirements
- query.go
- NewService
- Route
- Store
- .add
- fakeStudioRepo
- Spec: Entity Completeness Score (F55)
- nationality.ts
- ADR-002: SvelteKit chosen as frontend framework (SPA/static-adapter mode)
- newAssetClient
- newCompletenessHandlers
- Normalize
- Decision
- context.Context
- Session log — append-only (cap: last 8 sessions; older → archive/)
- demo/package.json
- Spec: Owner-authored person & studio ↔ media links, with file writeback (F40)
- Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)
- database/sql.DB
- filmEntityServer
- MergePersons(canonical, merged) transaction
- Shared ingest normalization pipeline (decode → bound → re-encode → strip)
- httpClient
- QA Checklist: Derived / calculated person fields — Age & Age at death (F45)
- ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio
- confidence.go
- completeness.go
- Repo
- ADR-086: Film provider enrichment — own `entity_type`, poster as an asset
- Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)
- Spec: Films as a first-class entity (F56)
- 0007_person_aliases.up.sql
- QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)
- Spec: Poster View for the People list page (F55)
- api/person_images_test.go
- Design handoff: Film Studio cascade edit affordance (F57)
- Spec: Tag Categories — grouping tags without merging them
- Repo
- Field
- ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename
- writebackJob.ts
- thumbServer
- Tag
- Session log — append-only (cap: last 8 sessions; older → archive/)
- 0022_entity_name_identity.up.sql
- ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation
- New
- fakeRepo
- routes/+layout.svelte
- Repo
- hooks
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Decision
- AutoRegisterFields
- Alias state seed (F58 / ADR-088)
- field_source_decisions table
- Spec: Metadata Source Plugins (F22/F27/F28)
- ResolvedField
- Spec: Unified Studio edit affordance + Film-level cascade writeback (F57)
- Decision
- Spec: People Images (F25)
- F38: Studio entity pages
- stub.js
- job_runs table (kind, trigger, status, counts, 30-day retention)
- Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)
- jira-sync.mjs REST transition mechanism (idempotent, match-by-name, soft-fail)
- Keyset cursor over (started_at, id) for job history
- activity/CLAUDE.md
- curation/CLAUDE.md
- Spec: Two-Tier Field Editing Model (F56)
- ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision
- Spec: Collapse provider aliases into the canonical alias spine (F58)
- gen-country-names.mjs
- field_claims.go
- duplicates/CLAUDE.md
- personDerivedServer
- Spec: Tag Writeback Exclusion — per-tag Genre writeback control
- Sink
- gen.sh
- svelte.config.js
- Design Handoff: Writeback hides the target file tag (HOLODEX-216)
- net/http.ResponseWriter
- Design Handoff: Two-Tier Field Editing Model (F56)
- vite.config.ts
- cmd/holodex/holodex.manifest — manifest source XML (requestedExecutionLevel=asInvoker)
- 0004_job_runs.up.sql
- 0005_entity_enrichment.up.sql
- enrichment/CLAUDE.md
- Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)
- ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field
- entity/CLAUDE.md
- app.d.ts
- +layout.ts
- 0013_metadata_curation.up.sql
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
- 0016_field_source_decisions.up.sql
- F36: Per-field source-of-truth
- ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity
- F39: Provider render hints / auto-registered non-canonical fields
- F44: In-app promote/override affordance
- Spec: People on the unified source-of-truth model (F37)
- 0019_provider_field_hints.up.sql
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
- HOLODEX-6 · Per-field source-of-truth (F36 / ADR-051)
- Jira HOLODEX-166 (System Activity epic)
- Design handoff: Studio image roles — icon / logo / poster (F51)
- Copy → exiftool-write → atomic rename file-safety model
- CSS custom-property design tokens (--bg, --surface, --ink, --accent, --font-display, --radius) per [data-theme]
- codeql.yml — CodeQL static analysis for go + javascript-typescript
- Design Handoff: Metadata Enrichment UI for People (F22)
- GET /api/v1/admin/activity aggregated read-model endpoint
- ProviderClient interface (HTTP default; in-process fake for CI)
- .serveFilmImage
- Spec: Sticky sort preferences + Random sort
- ADR-001: Backend Language — Go
- 0021_provider_icons.up.sql
- Unified field resolution — sources: [tmdb, file:Publisher, imdb] precedence list
- ghcr GitHub Deployment environment (Release ↔ Deployments linkage)
- git-cliff changelog generation (cliff.toml, orhun/git-cliff-action)
- Design Handoff: Media page — one sync verb, render-once fields (F36 / F39)
- person_aliases table + person_aliases_fts external-content mirror (migration 0007)
- person_images table (role, source, provider, external_id) + partial unique index (migration 0009)
- Placeholder resolution (active_skin, role, gender_bucket) → programmatic SVG asset
- providers/tmdb/ — standalone stdlib-only Go source, own binary + Dockerfile.provider-tmdb
- Functional Requirements
- Session log — append-only (cap: last 8 sessions; older → archive/)
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
- QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)
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
- Decision
- Spec: Person Aliases (F23)
- studio_logos table / RelinkStudioLogo
- provider_icons table / RelinkProviderIcon
- entity_aliases / nameKey identity spine (polymorphic)
- 0023_field_promotions.up.sql
- entity_keep_separate table (durable negative assertion)
- field_promotions table (tier-0 override)
- per-epic worklog (docs/plans/<KEY>.md)
- .github/workflows/provider-tmdb.yml — dedicated CI for the TMDB provider image
- PersonImageInsert struct with OverCap bool flag
- SessionStart orientation hook (compact digest, In Progress fire)
- ADR-065: Typed field registry and relationship-scoped computed fields (Superseded)
- DeriveRelationship(person, video, now) two-entity pass (unbuilt)
- 0024_enrichment_dismissals.up.sql
- Spec — Showcase Demo Corpus
- FieldType / Operator taxonomy (text/categorical/numeric/date)
- Decision
- enrichment_dismissals table (durable rejection verdict)
- file_writeback_snapshots table (batch revert, undo-of-undo)
- filename extraction confidence scoring rubric (tiered, exact-match gate)
- 0025_metadata_extraction_review.up.sql
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
- Design Handoff: Provider Link Badge — Multi-Badge States for Person/Studio (HOLODEX-266)
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
- 0026_file_writeback_snapshots.up.sql
- PlaceholderImage.svelte
- web/src/lib/f36.ts
- web/src/lib/peopleScroll.svelte.ts
- web/src/lib/searchHistory.ts
- web/src/routes/+layout.svelte
- web/src/routes/media/[id]/+page.svelte
- web/src/routes/studios/[id]/+page.svelte
- Load
- 0029_field_claims.up.sql
- Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)
- HOLODEX-240.md
- 0031_denied_tags.up.sql
- holoShuffle
- SanitizeLinkTemplates
- Complete
- cascadeServer
- Repo
- Handoff Spec: Person detail — bio in the header row
- .RelinkProviderIcon
- HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film)
- net/http/httptest.Server
- HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields
- Spec: Quick Wins batch — Search history & "More with …" shelves
- Design Handoff: Poster View for the People list page (F55)
- 0039_facet_not_applicable.up.sql
- .externalLinksForEntity
- Design handoff: In-app promote / override affordance for auto-registered fields (F44)
- 0041_provider_link_templates.up.sql
- parseEntityType
- toAnySlice
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Spec: Owner tooling hub + visitor/owner nav split (F35)
- .ReconcileVideoPeopleLocked
- database/sql.Tx
- Session log — append-only (cap: last 8 sessions; older → archive/)
- fakeFilmRepo
- writeJSON
- Health
- Design Handoff: Studio relationship-edit popover (HOLODEX-271)
- HOLODEX-288 · Fix film-studio cascade code-review findings
- filters.ts
- newTestService
- Session log — append-only (cap: last 8 sessions; older → archive/)
- stubThumbs
- Session log — append-only (cap: last 8 sessions; older → archive/)
- extractServer
- EnrichmentRow
- studio-picker-handoff.md
- HOLODEX-292 · Shared TagLinkChip component
- Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA
- routes/+page.svelte
- Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided"
- fakeResolver
- coverArtManager
- HOLODEX-102 · Video Credits → People + Headshots (F32)
- HOLODEX-255 · <epic title>
- films-entity.md
- HOLODEX-114 · <epic title>
- HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values
- HOLODEX-275 · GET /api/v1/facets marshals empty values as null, not []
- HOLODEX-244 · <epic title>
- HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip
- HOLODEX-273 · Writeback dialog "Select all undecided" doesn't create a standing decision
- SearchHistory
- two-tier-field-editing-handoff.md
- Interaction design
- .ReplaceProviderLinkTemplates
- .extractCoverArt
- filmServer
- Requirements
- listScroll.svelte.ts
- HOLODEX-294 · Reusable PeopleGrid component
- Decision
- dismissable
- ExpandedFieldState
- handler
- 0022_entity_name_identity.down.sql
- completeness/CLAUDE.md
- .sessionCookie
- Design handoff: Completeness panel — collapsible facet fold
- Repo
- Session log — append-only (cap: last 8 sessions; older → archive/)
- ADR-012: Resolution Classification — Width-Based Buckets with 10% Tolerance
- Design handoff — alias collapse (HOLODEX-306)
- Placeholder
- Repo
- .runTagWritebackSync
- ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)
- HOLODEX-298 · Film detail page: match media page's Tags section styling
- HOLODEX-300 · Film bulk-attach dialog: default search term + optional starting scene number
- HOLODEX-302 · Person hero image hover-to-front
- HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise
- HOLODEX-307 · Film detail: poster becomes the header image; remove the Images section
- seedTwoTags
- Holodex landing page (site/index.html)
- HOLODEX-283 · Films: real backend search integration
- HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list
- HOLODEX-301 · Completeness panel collapsible facet fold
- Cache
- .mountTagHierarchy
- 0044_alias_source_and_suppressions.up.sql
- identity_review_queue
- identity_review_queue

## God Nodes (most connected - your core abstractions)
1. `newRepo()` - 185 edges
2. `Repo` - 140 edges
3. `itoa()` - 130 edges
4. `writeError()` - 120 edges
5. `sampleVideo()` - 119 edges
6. `writeJSON()` - 85 edges
7. `pathID()` - 84 edges
8. `Open()` - 81 edges
9. `New()` - 80 edges
10. `Handlers` - 74 edges

## Surprising Connections (you probably didn't know these)
- `Holodex landing page (site/index.html)` --semantically_similar_to--> `SvelteKit app.html shell (default data-theme=cinematheque)`  [INFERRED] [semantically similar]
  site/index.html → web/src/app.html
- `TestDownloadImageToTemp_WritesAllowedBytesToTemp()` --calls--> `cleanup()`  [INFERRED]
  internal/writeback/image_fetch_test.go → testdata/aliasseed/main.go
- `writeExiftoolBatch()` --calls--> `cleanup()`  [INFERRED]
  internal/writeback/writeback.go → testdata/aliasseed/main.go
- `writeMKVWithMkvpropedit()` --calls--> `cleanup()`  [INFERRED]
  internal/writeback/writeback.go → testdata/aliasseed/main.go
- `writeMKVWithFFmpeg()` --calls--> `cleanup()`  [INFERRED]
  internal/writeback/writeback.go → testdata/aliasseed/main.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **F50 Tag Governance & Video Enrichment design package (plan + handoff + QA checklist)** — docs_plans_holodex_224, docs_design_tag_governance_and_video_enrichment_handoff, docs_design_tag_governance_and_video_enrichment_qa_checklist [EXTRACTED 1.00]
- **Client-only, zero-backend-state UI persistence/derivation pattern (localStorage/session, no migration)** — docs_specs_quick_wins, docs_specs_sort_persistence, docs_specs_people_nationality_flag [INFERRED 0.65]
- **curation/enrichment/entity component folders jointly implement the per-field source-of-truth + identity spine UI** — web_src_lib_components_curation_claude, web_src_lib_components_enrichment_claude, web_src_lib_components_entity_claude [INFERRED 0.75]
- **Sort controls shared across browse/people/tags index pages** — web_src_lib_components_sort_sortdropdown, web_src_lib_components_sort_sortreroll, web_src_lib_components_sort_sorttoggle [INFERRED 0.75]
- **Video card/grid primitives and the shelves built on top of them** — web_src_lib_components_video_videocard, web_src_lib_components_video_videogrid, web_src_lib_components_video_recentlyaddedshelf, web_src_lib_components_video_relatedshelf [INFERRED 0.75]
- **FTS5 search subsystem: SQLite FTS5 choice, migration-managed virtual tables/triggers, global+filter search architecture** — docs_architecture_adr_003_database_sqlite_decision, docs_architecture_adr_016_database_migrations_decision, docs_architecture_adr_017_search_architecture_decision [INFERRED 0.80]
- **ADR-070 canary + retag release pipeline** — github_workflows_image_yml, github_workflows_release_please_yml, github_workflows_release_yml, github_workflows_release_candidate_yml, concept_adr_070_canary_retag_promotion [INFERRED 0.85]
- **ADR-075's four tag-governance decisions form one feature** — docs_architecture_adr_075_tag_governance_and_video_enrichment, docs_architecture_adr_075_tag_governance_and_video_enrichment_parent_hierarchy, docs_architecture_adr_075_tag_governance_and_video_enrichment_denied_tags, docs_architecture_adr_075_tag_governance_and_video_enrichment_video_tags_source, docs_architecture_adr_075_tag_governance_and_video_enrichment_materialize_tags [INFERRED 0.85]
- **Entity-generic F36 decision model proven across person, studio, and derived links** — docs_specs_people_source_of_truth, docs_specs_studio_entity, docs_specs_person_media_linking [INFERRED 0.85]
- **Frontend token-discipline design + CI enforcement** — claude_rules_frontend_theming_md, github_workflows_ci_yml, concept_adr_021_frontend_theming_skins [INFERRED 0.85]
- **Jira status-transition pipeline (ADR-058/069)** — claude_claude_md, claude_flightplan_yaml, github_workflows_jira_sync_yml, github_workflows_release_yml, concept_adr_058_jira_rest_transitions [INFERRED 0.85]

## Communities (617 total, 234 thin omitted)

### Community 0 - "types.ts"
Cohesion: 0.02
Nodes (146): ADR-0006, ADR-0028, ADR-0036, ADR-0056, ADR-0073, ADR-0080, checkRedirect(), endSession() (+138 more)

### Community 1 - "media/[id]/+page.svelte"
Cohesion: 0.06
Nodes (16): resolve(), DismissableOptions, provider(), busy, error, orderDraft, remove(), save() (+8 more)

### Community 2 - "session-start.mjs"
Cohesion: 0.07
Nodes (63): bareSkill(), DEFAULTS, loadConfig(), ADR-0064, resolveKey(), emitJson(), ADR-0064, relPath() (+55 more)

### Community 3 - "Path"
Cohesion: 0.16
Nodes (17): entityDir(), Path(), Remove(), Store(), TestPath_ServerAssignedIDsOnly(), TestStoreRemove_RoundTrip(), ImagePath(), Remove() (+9 more)

### Community 4 - "Repo"
Cohesion: 0.32
Nodes (7): database/sql.NullString, Repo, VideoCollision, idKeyOf(), nameKeyOf(), normalizedNameKey(), compositeKeyCandidate

### Community 5 - "NewService"
Cohesion: 0.12
Nodes (26): extraPairs(), fileLayerChanged(), Report, Service, SourceResult, NewService(), personNames(), refreshDetail() (+18 more)

### Community 6 - "decisionServer"
Cohesion: 0.13
Nodes (22): actorsAndDirectorServer(), postCurationNoFatal(), TestCurationAPI_PeopleConcurrentDifferentFields_NoLostUpdate(), decisionServer(), doJSONRequest(), resolvedField(), TestDecisionAPI_AdoptProviderThenClear(), TestDecisionAPI_ManualLiteral() (+14 more)

### Community 7 - "model.go"
Cohesion: 0.08
Nodes (11): fakePersonRepo, CorePersonImageRole(), PersonImage, PersonImageSet, HasThumbnailImage(), ValidPersonImageRole(), PersonImageInsert, PersonImageRef (+3 more)

### Community 8 - "Holodex Project Working Agreements (CLAUDE.md)"
Cohesion: 0.07
Nodes (41): Holodex Project Working Agreements (CLAUDE.md), Branch↔Jira linkage, Core resolver model (baseline / enrichment / curation / decisions), Pre-commit checklist, Secrets & publishing rules, Jira task tracking (HOLODEX project), Flightplan Config (flightplan.yaml), Frontend Theming Rules (+33 more)

### Community 9 - "testMappings"
Cohesion: 0.07
Nodes (40): compiledPatterns, fakeEnrichmentCall, fakeEnrichmentWriter, fakeVideoLister, Pattern, patternFile, recordingJobRecorder, recordingReviewStore (+32 more)

### Community 10 - "devDependencies"
Cohesion: 0.04
Nodes (48): flag-icons, @fontsource/share-tech-mono, @fontsource-variable/archivo, @fontsource-variable/fraunces, @fontsource-variable/spline-sans-mono, @fontsource/vt323, svelte-check, @sveltejs/adapter-static (+40 more)

### Community 11 - "JobRun"
Cohesion: 0.10
Nodes (14): activityResponse, activitySystem, database/sql.Rows, JobRun, TrashItem, scanTrash(), LibraryCounts, Repo (+6 more)

### Community 12 - "imagetools.mjs"
Cohesion: 0.08
Nodes (46): ADR-0035, ADVISORY_TYPES, classify(), COMMENT_MARKER, main(), ADR-0076, NON_DOC_GLOBS, parseCommitType() (+38 more)

### Community 13 - "newProviderIconEnv"
Cohesion: 0.21
Nodes (20): iconEnv, filmImageServer(), TestFilmImage_InvalidRole(), TestFilmImage_MutationsRequireOwner(), TestFilmImage_ReplaceAdvancesVersion(), TestFilmImage_UploadServeDelete(), uploadFilmImage(), newProviderIconEnv() (+12 more)

### Community 14 - "Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)"
Cohesion: 0.14
Nodes (14): API, Before implementation, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Problem Statement, Requirements (+6 more)

### Community 15 - "Service"
Cohesion: 0.08
Nodes (13): refreshAllResult, IconRef, ImageSink, Manifest, ProviderClient, SourceInfo, Service, Source (+5 more)

### Community 16 - "resolver.go"
Cohesion: 0.15
Nodes (34): applyCasing(), baselineValue(), BrowseTitle(), decidedItem(), filmNamespaces(), filmSourceValue(), firstNonEmpty(), gateImageDisplay() (+26 more)

### Community 17 - "format.ts"
Cohesion: 0.04
Nodes (21): batchId(), revert(), i(), Shared components, calculatedFrom(), filterByTitle(), formatDuration(), formatYear() (+13 more)

### Community 18 - "Handlers"
Cohesion: 0.07
Nodes (14): purger, rescanner, scanStatusSource, searchMetrics, thumbnailer, Handlers, chi.Router, injectFilmSources() (+6 more)

### Community 19 - "sampleVideo"
Cohesion: 0.08
Nodes (60): countPeople(), hasVideoTitle(), personIDByName(), TestAliasesSurviveRescan(), TestMergePersons(), TestMergePersons_DedupesSameRoleLinkAtMergeTime(), TestMergePersons_RepointsExternalID(), TestMergePersonsValidation() (+52 more)

### Community 20 - "Scanner"
Cohesion: 0.10
Nodes (14): github.com/fsnotify/fsnotify.Watcher, os.DirEntry, os.FileInfo, ScanStatus, ScanSummary, isMedia(), New(), Config (+6 more)

### Community 21 - "newRepo"
Cohesion: 0.05
Nodes (54): TestClaims_SetClearsPromotionInSameWrite(), TestClaims_SetListClear(), TestCurationForEntities_Batch(), TestDecisions_ForEntitiesBatch(), TestDecisions_ForVideosBatch(), TestDecisions_SetGetClear(), TestHasManualSource(), TestEnrichmentForEntities_Batch() (+46 more)

### Community 22 - "Spec: Unified entity name-identity (F43)"
Cohesion: 0.10
Nodes (21): ADR-058 (Jira transitions via REST API) — cited as evidence, Flightplan — portable session-state plugin, /handoff skill (gate ticking, release_note promotion), HOLODEX-182 tracking issue, Never let durable state depend on agent discipline, SessionStart hook (fires In Progress, prints orientation), Stop hook (mechanical worklog-staleness nag), /triage skill (drains INBOX.md) (+13 more)

### Community 23 - "Design handoff: StudioLinkCard (reusable Studio display)"
Cohesion: 0.18
Nodes (11): 1. Resolved decisions (open questions from the rough mockup), 2. New component: `StudioLinkCard.svelte`, 3. Call-site changes, 4. Backend requirement (blocking), 5. Design tokens used, 6. States and interactions, 7. Responsive behavior, 8. Edge cases (+3 more)

### Community 24 - "architecture/README.md"
Cohesion: 0.06
Nodes (19): Architecture Decision Records, Cross-cutting, Phase specs, 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: People Images (F25) (+11 more)

### Community 25 - "enrich/enrich_test.go"
Cohesion: 0.13
Nodes (38): gateTestHandlers(), Handlers, TestGateImageURL_MergedField(), NewStore(), Service, newSvc(), TestDownloadAssetsFirstSuccessPerRole(), TestEnrichAssetFailureIsNonFatal() (+30 more)

### Community 26 - "Design Handoff: Unified name-edit mechanism (HOLODEX-269)"
Cohesion: 0.05
Nodes (35): Accessibility Notes, Animation / Motion, Component contract (resolves the spec's open question), Components, Cross-context notes, Design Handoff: Unified name-edit mechanism (HOLODEX-269), Design Tokens Used, Edge Cases (+27 more)

### Community 27 - "tmdb.go"
Cohesion: 0.08
Nodes (48): net/url.Values, buildCompanyEnrichResponse(), buildEnrichResponse(), buildMovieEnrichResponse(), buildPeopleCredits(), disambiguate(), headshotFor(), movieDisambiguate() (+40 more)

### Community 28 - "Auth"
Cohesion: 0.18
Nodes (6): sessionClaims, POST /api/v1/session (token exchange) + DELETE /api/v1/session (sign-out), deriveSessionSecret(), Auth, Handlers, parseSessionClaims()

### Community 29 - "QA: Metadata Writeback (F28)"
Cohesion: 0.06
Nodes (30): §1 Setup, §2 Smoke (run in `make test` / `npm run test`), §3 Agent live QA (preview tools against §1 stack), §4 Human (3-skin eyeball — Cinémathèque, Broadcast, Brutalist), QA Checklist: People on the unified source-of-truth model (F37), 1. Studio next to the title, 2. Commentary block, 3. Poster upload (+22 more)

### Community 30 - "net/http.Request"
Cohesion: 0.08
Nodes (13): categoryTagIDsBody, net/http.Request, Handlers, chi.Router, parseCategoryName(), Handlers, Handlers, chi.Router (+5 more)

### Community 31 - "Configuration Reference (holodex.yaml layers)"
Cohesion: 0.07
Nodes (29): ADR-056 (provider field render hints, F39), ADR-074 (claimed provider keys), Claiming a provider key (F49) cookbook, Derived/computed field genre (F45, ADR-063), Canonical Field Registry (operator reference), admin_token / owner session authentication, ADR-046 (owner session persistence), default_source / provider_trust_order (F36, ADR-051) (+21 more)

### Community 32 - "Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)

### Community 33 - "mcp.go"
Cohesion: 0.10
Nodes (35): github.com/mark3labs/mcp-go/mcp.CallToolRequest, github.com/mark3labs/mcp-go/mcp.CallToolResult, github.com/mark3labs/mcp-go/server.MCPServer, filterNamed(), T, isOwner(), jsonResult(), mapSlice() (+27 more)

### Community 34 - "ADR-075: Tag governance & video enrichment"
Cohesion: 0.15
Nodes (19): F50: Tag governance & video enrichment, Suppression derives from merged []mapping.Field, not the claims table, ADR-075: Tag governance & video enrichment, denied_tags global term deny-list table, Write-on-resolve tag materialization via afterEnrichApply, tags.parent_tag_id strict-tree hierarchy, video_tags.source column; partial-replace rescan, Design Handoff: Tag Governance & Video Enrichment (F50) (+11 more)

### Community 35 - "extractor.go"
Cohesion: 0.11
Nodes (24): canonicalKey(), dedupe(), Extracted, isBinaryValue(), mapExiftool(), mapFfprobe(), NewExtractor(), newKeySet() (+16 more)

### Community 36 - "Resolve"
Cohesion: 0.12
Nodes (42): decide(), providerCandidate(), TestResolve_CandidatesListFileAndMatchedProviders(), TestResolve_DecisionAdoptProvider(), TestResolve_DecisionKeepFileOverridesMappingOrder(), TestResolve_DecisionManualLiteral(), TestResolve_EmptyProviderYieldsNoCandidate(), TestResolve_FileFirstDefault_ProviderNoLongerMasksFile() (+34 more)

### Community 37 - "ResolveReviewAction"
Cohesion: 0.26
Nodes (12): ResolvedWrite, ReviewAction, ResolveReviewAction(), TestResolveReviewAction_Filename(), TestResolveReviewAction_FilenameRequiresValue(), TestResolveReviewAction_Manual(), TestResolveReviewAction_ManualMultiValue(), TestResolveReviewAction_ManualRequiresValue() (+4 more)

### Community 38 - "activity.svelte.ts"
Cohesion: 0.05
Nodes (23): web/src/lib/browse.svelte.ts — module-scoped browse-state cache, web/src/routes/+page.svelte — the browse grid, web/src/lib/theme.svelte.ts — established module-scoped singleton pattern, Client-side seeded shuffle for unpaged People/Tags lists (mulberry32 PRNG), activity, ActivityState, ADR-0030, ADR-0046 (+15 more)

### Community 39 - "people/+page.svelte"
Cohesion: 0.04
Nodes (33): onKey(), Sort components, segmentedToggleWrapperClass, firstLetter(), letterAnchors(), seededShuffle(), ADR-0045, keyFor() (+25 more)

### Community 40 - "jira-sync.mjs"
Cohesion: 0.12
Nodes (23): log, main(), missing, ADR-0058, ADR-0069, bailSoft(), log, main() (+15 more)

### Community 41 - "log/slog.Logger"
Cohesion: 0.20
Nodes (17): backfillPersonLinks(), backfillStudioLinks(), main(), newLogger(), promoteEnrichmentAliases(), run(), runHealthcheck(), runMCPStdio() (+9 more)

### Community 42 - "people"
Cohesion: 0.17
Nodes (7): people, video_people, person_images, person_image_suppressions, video_people_old, video_people_new, person_external_ids

### Community 43 - "People attach/detach + relationship picker (F56.5, HOLODEX-272)"
Cohesion: 0.05
Nodes (35): Accessibility Notes, Animation / Motion, Components, Design Handoff: People attach/detach + relationship picker (HOLODEX-272), Design Tokens Used, Edge Cases, Layout, Overview (+27 more)

### Community 44 - "Open"
Cohesion: 0.13
Nodes (48): net/http.Handler, TestMergeEndpoint_PropagatesWritebackToAffectedVideos(), NewAuth(), facetMap(), TestGetMedia_Completeness(), TestGetPerson_Completeness(), TestGetStudio_Completeness(), peopleDecisionServerWithFields() (+40 more)

### Community 45 - "Spec: Tag & Category Create Affordance — closing the /tags creation gap"
Cohesion: 0.15
Nodes (13): Goals, Implementation note (2026-08-01), Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement (+5 more)

### Community 46 - "Queue"
Cohesion: 0.16
Nodes (8): fakeEnqueuer, WritebackJob, detailLine(), JobField, Queue, BatchJob, PostWriteFunc, WriteFunc

### Community 47 - "time.Duration"
Cohesion: 0.16
Nodes (15): strings.Builder, sync/atomic.Int64, time.Duration, formatFloat(), New(), newHistogram(), findLine(), scrape() (+7 more)

### Community 48 - "identityServer"
Cohesion: 0.13
Nodes (26): aliasList(), reqTokBody(), TestCategoryEndpoints(), TestResolveOrCreateTagEndpoint(), completenessBrowseServer(), mediaTitles(), TestCompletenessFacets(), TestListMedia_CompletenessSort_Orders() (+18 more)

### Community 49 - "generate.mjs"
Cohesion: 0.12
Nodes (21): ADR-0004, ADR-0017, buildItem(), ensureFfmpeg(), here, hms(), main(), ADR-0009 (+13 more)

### Community 50 - "Manual QA Checklist: Per-field source-of-truth decisions (F36)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist), Manual QA Checklist: Per-field source-of-truth decisions (F36)

### Community 51 - "Video composite-key collision check (F56.3, HOLODEX-270)"
Cohesion: 0.06
Nodes (32): A required precondition: generalize `NameEditControl`'s conflict type, Accessibility, `CollisionOfferCard.svelte`, Design Handoff: Video composite-key collision verdict (HOLODEX-270), Edge Cases, Layout, New type: `VideoCollisionRef`, Overview (+24 more)

### Community 52 - "theme.svelte.ts"
Cohesion: 0.13
Nodes (12): ADR-0021, CORE_ROLE_ASPECT, cropAffine, CropInput, cropTargetSize(), isTheme(), Theme, THEME_LABELS (+4 more)

### Community 53 - "CurationFieldRow.svelte"
Cohesion: 0.12
Nodes (14): commitEdit(), draft, editing, isProvider, onEditKey(), provenance, adding, busy (+6 more)

### Community 54 - "studios"
Cohesion: 0.14
Nodes (9): studios, studios_fts, video_studios, studio_external_ids, studio_logos, studio_logos, studio_images, person_aliases (+1 more)

### Community 55 - "ResolveFields"
Cohesion: 0.16
Nodes (32): NewFilmBaseline(), filmTestFields(), TestFilmBaseline_NameResolvesFromRecord(), TestFilmBaseline_NilFilmIsEmptyBaseline(), TestFilmBaseline_RD6Additivity(), TestFilmBaseline_RecordBlankPinSuppressesProvider(), collectionField(), TestReplaceMarkers_FilmSourceOffersCandidateNamedAfterFilm() (+24 more)

### Community 56 - "writeback/writeback.go"
Cohesion: 0.06
Nodes (74): encoding/xml.Name, TestDownloadImageToTemp_PropagatesFetcherRefusal(), TestDownloadImageToTemp_RefusesNonHTTPS(), TestDownloadImageToTemp_RefusesWithNoFetcherConfigured(), TestDownloadImageToTemp_WritesAllowedBytesToTemp(), withImageFetcher(), currentTagValue(), ReadCurrentValues() (+66 more)

### Community 57 - "Handlers"
Cohesion: 0.27
Nodes (4): net/http.HandlerFunc, Handlers, videoHint(), Hint

### Community 58 - "Design Handoff: Extract from filename on the media detail page (F48.5a)"
Cohesion: 0.09
Nodes (21): Accessibility, Backend change, Design Handoff: Extract from filename on the media detail page (F48.5a), Goals, Interaction detail, Non-goals, Problem, QA (+13 more)

### Community 59 - "getJSON"
Cohesion: 0.18
Nodes (22): externalLinksEnv, fakeRescanner, linksByProvider(), newExternalLinksEnv(), TestExternalLinks_MalformedIDSkipped(), TestExternalLinks_MultiBadge(), TestExternalLinks_Studio(), TestExternalLinks_TemplateMismatch() (+14 more)

### Community 60 - "process.go"
Cohesion: 0.29
Nodes (9): Decision, Deps, Enqueuer, FieldExtraction, ManualSourceChecker, Resolver, ReviewStore, joinSorted() (+1 more)

### Community 61 - ".setFieldDecision"
Cohesion: 0.08
Nodes (16): decisionBody, filmStudioCascadeResult, decodeDecisionBody(), Handlers, chi.Router, writeCollisionConflict(), Handlers, chi.Router (+8 more)

### Community 62 - "identity_ops_test.go"
Cohesion: 0.14
Nodes (19): tagIDByName(), TestEntityConflictExcludesSelf(), TestKeepSeparateStore(), TestMergeEntitiesValidation(), TestMergeEntitiesWithAffectedVideos_UnknownEntityType(), TestRenameStudioKeepsOldNameAsAlias(), TestRenameTagInternalWhitespaceConflict(), TestStudioAliasCRUD() (+11 more)

### Community 63 - "testing.T"
Cohesion: 0.08
Nodes (56): testing.T, TestRelinkVideoPeople_UnmappedFieldLeavesExistingLinksUntouched(), TestRelinkVideoStudios_UnmappedFieldLeavesExistingLinksUntouched(), TestSourceBuildQuery_EmptyTitleNeverBlank(), TestSourceBuildQuery_OptionalTokenOmittedNoArtifact(), TestSourceBuildQuery_PerformersCapAndOrder(), TestSourceBuildQuery_Precedence(), TestSourceBuildQuery_RequiredTokenFallsThroughTier() (+48 more)

### Community 64 - "scanner_test.go"
Cohesion: 0.18
Nodes (18): VideoStat, TestBuildVideoFromFileForcesExtractWithoutPersisting(), TestExtractionHook(), TestExtractionHook_ErrorDoesNotFailScan(), activeCount(), newFakeRepo(), newTestScanner(), TestChangedFileIsReindexed() (+10 more)

### Community 65 - "f36.ts"
Cohesion: 0.09
Nodes (31): Writeback components, autoResize(), ensureDecision(), onKeydown(), submit(), trapTab(), baselineCandidateValue(), decidedSource() (+23 more)

### Community 67 - "Process"
Cohesion: 0.18
Nodes (14): ExtractionReviewCall, fakeManualSource, fakeReviewStore, Process(), TestProcess_EntityField_ExactMatchAutoApplies(), TestProcess_EntityField_FuzzyMatchQueuesWithSuggestion(), TestProcess_EntityField_NoMatchQueuesWithoutSuggestion(), TestProcess_LogOnly_WhenFlagDisabled() (+6 more)

### Community 68 - "Design Handoff: Unified nav search — live, tabbed, in-place filtering panel"
Cohesion: 0.06
Nodes (33): Accessibility notes (summary), Design Handoff: Unified nav search — live, tabbed, in-place filtering panel, Design-system fit, Mobile (< 640px, the primary complaint driving this spec), Overview, Part A — The tab row lives with the box, not inside the dropdown, Part B — `SearchResultsPanel.svelte` (NS1), Part C — Per-page removal (NS4) (+25 more)

### Community 69 - "seedPerson"
Cohesion: 0.10
Nodes (44): newRepoDB(), reviewPair, hasPair(), mustExec(), readReviewQueue(), rowCount(), TestIdentityBackfill(), TestIdentityBackfillIdempotent() (+36 more)

### Community 70 - "routes/tags/+page.svelte"
Cohesion: 0.05
Nodes (38): PopoverMenu, PopoverMenuOptions, createAndAssign(), focusOption(), onKey(), onOptionKey(), optionCount, pickAt() (+30 more)

### Community 71 - "videos"
Cohesion: 0.22
Nodes (7): videos, file_writebacks, writeback_queue, file_writebacks_old, writeback_queue_old, file_writebacks_new, writeback_queue_new

### Community 72 - "Studio relationship-edit popover (F56.4, HOLODEX-271)"
Cohesion: 0.25
Nodes (8): Existing State (grounded in code, this session), Goals, Non-Goals, Open Questions, Problem Statement, Studio relationship-edit popover (F56.4, HOLODEX-271), Success Metrics, User Stories

### Community 75 - "Manager"
Cohesion: 0.22
Nodes (4): Manager, New(), Config, Repository

### Community 76 - "ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action"
Cohesion: 0.12
Nodes (16): Action Items, ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action, Consequences, Context, Current state (survey, 2026-08-25), D1 — Extract the single-video Studio-decide logic into a shared helper; the cascade calls it once per attached video, D1 — where the per-video Studio-decide logic lives, D2 — `CascadeFilmStudio`: per-video decide (best-effort), then one shared-batch enqueue for every video that succeeded (+8 more)

### Community 77 - ".scaleToWidth"
Cohesion: 0.22
Nodes (10): io.Reader, absPath(), Manager, lastLine(), scaleArgs(), seekSeconds(), TestScaleArgs(), TestSeekSeconds() (+2 more)

### Community 78 - "sync.Mutex"
Cohesion: 0.24
Nodes (3): sync.Mutex, fakeThumbnailer, stats

### Community 79 - "enrich/enrich.go"
Cohesion: 0.14
Nodes (18): FieldHint, fileConfig, Registry, Store, Empty(), entityTypesSupport(), Source, Load() (+10 more)

### Community 80 - "Video"
Cohesion: 0.09
Nodes (20): fakeVideoLookup, Handlers, ExtraMetadata, Video, ftsPrefixQuery(), FacetValue, RelatedShelf, VideoFilter (+12 more)

### Community 81 - "writeError"
Cohesion: 0.07
Nodes (22): curationBody, Handlers, chi.Router, Handlers, chi.Router, validateCurationBody(), validCurationAction(), Handlers (+14 more)

### Community 82 - "authServer"
Cohesion: 0.25
Nodes (18): net/http.Response, authServer(), exchange(), findCookie(), getCookie(), getTok(), TestCapabilities(), TestControlsUnauthenticatedFlag() (+10 more)

### Community 83 - "Spec: Tag Detail — Hierarchy & Category Controls"
Cohesion: 0.06
Nodes (29): 1. Decision logic (when the dialog appears), 2. The confirm dialog, 3. States and interactions, 4. Edge cases, 5. Accessibility, 6. Visual reference, Design Handoff: Reparent-confirm flow for the Children control (HOLODEX-259), Design-system-fit audit (+21 more)

### Community 84 - "Orchestrator"
Cohesion: 0.21
Nodes (9): cachingResolver, FieldOutcome, Outcome, Result, VideoLookup, fileTagValues(), Orchestrator, newCachingResolver() (+1 more)

### Community 85 - "Repo"
Cohesion: 0.21
Nodes (20): stubFileExtractor, tagID(), refreshPOST(), refreshServer(), seedRefreshVideo(), TestRefreshEndpointDisabled(), TestRefreshEndpointRequiresOwner(), TestRefreshEndpointStatuses() (+12 more)

### Community 86 - "Design handoff: PeopleGrid (reusable People/Cast display)"
Cohesion: 0.17
Nodes (12): 10. Verification (as-built), 1. Resolved decisions, 2. New component: `PeopleGrid.svelte`, 3. Call-site changes, 4. Backend requirement, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior (+4 more)

### Community 87 - "itoa"
Cohesion: 0.12
Nodes (42): aliasServer(), sendTok(), TestAddAliasConflict409(), TestAliasEndpointsGatedAndValidated(), TestGetPersonIncludesAliases(), TestMergeEndpoint(), TestPersonDetail_AliasSourceAndSkipped(), deleteServer() (+34 more)

### Community 88 - "time.Time"
Cohesion: 0.12
Nodes (25): trashItem, time.Time, ForComputed(), fieldByKey(), ResolvedField, dependencyLabels(), Derive(), deriveAge() (+17 more)

### Community 89 - "queue"
Cohesion: 0.21
Nodes (6): newQueue(), drain(), TestQueueDedupAndDepth(), TestQueueDedupWhileInFlight(), TestQueueHighPriorityFirst(), queue

### Community 91 - "QA Checklist: System Activity (F21)"
Cohesion: 0.17
Nodes (11): F21: System Activity — Under the Hood, Accessibility, Controls (owner), Gating (F21.7) — needs `ADMIN_TOKEN` to exercise, Header activity indicator, Job history, QA Checklist: System Activity (F21), Reachability & shell (+3 more)

### Community 93 - "Decision"
Cohesion: 0.14
Nodes (14): ADR-023: Image Distribution — Published GHCR Image + Pull-Based Compose, Consequences, Context, Decision, Tagging, ci.yml — PR/push gate (go vet, go test, svelte-check, Vitest, vite build, theming grep guard), image.yml — reusable multi-arch build/push + Trivy scan, release.yml — tag v* triggers image.yml then cuts a GitHub Release (+6 more)

### Community 94 - ".uploadVideoPoster"
Cohesion: 0.39
Nodes (4): Handlers, chi.Router, PosterPath(), ThumbPath()

### Community 95 - "QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)"
Cohesion: 0.40
Nodes (4): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human look, QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)

### Community 96 - "JaroWinkler"
Cohesion: 0.22
Nodes (13): BestFuzzyMatch(), classifyAgreement(), classifySpecificity(), commonPrefixLen(), jaro(), JaroWinkler(), approxEqual(), TestBestFuzzyMatch() (+5 more)

### Community 97 - "Design handoff: Films entity (F56)"
Cohesion: 0.06
Nodes (31): 1. `/films` — list, §1 Setup, 2. `/films/{id}` — detail, §2 Smoke, 2a. Header, 2b. Full-film file section (RD4, P0-10), 2c. Scenes list (RD4), 2d. Film → video attach entry point (+23 more)

### Community 98 - "seedTagTree"
Cohesion: 0.20
Nodes (20): assertTagParent(), ptr(), seedTagTree(), TestAncestorNamesForTag(), TestChildrenForTag(), TestListTagsWritebackEnabled(), TestListVideos_TagFilterIsDescendantInclusive(), TestMergeReparentsChildren() (+12 more)

### Community 99 - "Repo"
Cohesion: 0.13
Nodes (13): filmVideoCandidate, Film, filmSceneOccupant(), FilmAttachment, FilmSceneCollision, Repo, T, insertFilmVideo() (+5 more)

### Community 100 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.17
Nodes (12): 2026-07-10 · what happened this session, 2026-08-09 · Architecture gate closed — ADR-083 written, 2026-08-09 · Backend gate closed — LinkTemplates + external_links projection, 2026-08-09 · Design gate closed — multi-badge handoff written, 2026-08-09 · Frontend gate closed — ProviderLinkBadge.svelte + person/studio wiring, 2026-08-09 · Post-review hardening — high-effort /code-review pass, 6 fixes applied, 2026-08-09 · Security gate closed — LinkTemplates injection review, no findings, 2026-08-09 · Testing gate closed — external_links projection + BuildProviderLink coverage (+4 more)

### Community 101 - ".addEntityAlias"
Cohesion: 0.26
Nodes (6): identityRoutes, Handlers, chi.Router, T, mergeBatchID(), namesByVideo()

### Community 102 - "New"
Cohesion: 0.22
Nodes (12): New(), tempFile(), TestPurgeNow(), TestPurgeNowNotFound(), TestSweepGraceZeroIsNoOp(), TestSweepMissingFileCountsAsSuccess(), TestSweepPurgesExpiredAndRemovesFile(), TestSweepRemovalFailureLeavesRow() (+4 more)

### Community 104 - "QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)"
Cohesion: 0.33
Nodes (5): §1 Setup, §2 Smoke, §3 Agent live QA (all 3 skins), §4 Human, QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)

### Community 105 - "BatchRunner"
Cohesion: 0.33
Nodes (3): JobRecorder, VideoLister, BatchRunner

### Community 106 - "ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam"
Cohesion: 0.13
Nodes (15): Action Items, ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam, Consequences, Context, Current state (survey, 2026-07-31), D1 — `tags.writeback_enabled` column; filtered at `TagNamesForVideo`, uniformly per name regardless of how it was reached, D1 — where the flag is enforced, D2 — Manual sync batch-enqueues per-video via `genreWritebackValuesForVideo`, not a precomputed name list; shared `batchID` across single- and bulk-tag triggers (+7 more)

### Community 107 - "ADR-080: Configurable per-provider metadata search query patterns"
Cohesion: 0.08
Nodes (25): A — core renders a string, `/resolve` contract unchanged (chosen), A — embed in the existing entity payload (chosen), A — operator > provider > global default > raw title (chosen), A — strip bracket punctuation + resolution tokens, collapse whitespace (chosen), Action Items, ADR-080: Configurable per-provider metadata search query patterns, B — leave the floor tier literal; rely on operator-configured patterns to work around messy titles, B — new endpoint, picker fetches on open (+17 more)

### Community 108 - "FilmStudioCascadeDialog.svelte"
Cohesion: 0.13
Nodes (18): active, batchId, collisions, commit(), enqueued, errors, focusOption(), onInput() (+10 more)

### Community 109 - "Design handoff: TagLinkChip (reusable Tag display)"
Cohesion: 0.18
Nodes (11): 1. Resolved decisions, 2. New component: `TagLinkChip.svelte`, 3. Call-site changes, 4. Backend requirement, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior, 8. Edge cases (+3 more)

### Community 110 - "repo/related_test.go"
Cohesion: 0.16
Nodes (18): mustVideoID(), TestCategoriesForTag(), TestCategoryCrossTableCollision(), TestCategoryCRUD(), TestCategoryTagAssignment(), TestCategoryVideoFilterFacet(), TestListCategoriesTagFields(), TestResolveOrCreateTag() (+10 more)

### Community 111 - "repo/studios_test.go"
Cohesion: 0.19
Nodes (14): TestFilmStudios_IncludesIconAndCount(), studioIDByName(), TestStudioMergeSurvivesRederivation(), studioByName(), studioNames(), TestGetStudio_NotFound(), TestListStudios_AttachesImageVersions(), TestReconcileVideoStudios_CreateReplacePrune() (+6 more)

### Community 112 - "Design handoff: Media detail page reorder"
Cohesion: 0.20
Nodes (9): 1. Films + People row, 2. Rejected during iteration, Accessibility / interaction, Design handoff: Media detail page reorder, Edge cases, Final order (top to bottom), Overview, Theming (+1 more)

### Community 113 - "0001_init.up.sql"
Cohesion: 0.24
Nodes (8): people_fts, tags, tags_fts, video_metadata, video_tags, videos_fts, categories, category_tags

### Community 114 - "ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write"
Cohesion: 0.08
Nodes (22): Action Items, ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write, Consequences, Context, Decision, Non-goals, Option A: Re-resolve from source inside `check()` — rejected, Option B: Extend the lock to cover the relink write — chosen (+14 more)

### Community 115 - "Decision"
Cohesion: 0.12
Nodes (16): 1. Type source: PR title, not individual commits, 2. Scope signal: changed-file globs, with a threshold, 3. Advisory, not blocking, 4. New workflow, not a job in `jira-sync.yml`, 5. Script shape, 6. Allowlist, not the `docs/**`-denylist `jira-sync.yml` uses, Action Items, ADR-076: Advisory CI check — `docs`/`chore`-typed PRs that touch non-doc code (+8 more)

### Community 116 - "HOLODEX-280 · Film poster/thumbnail asset pipeline"
Cohesion: 0.33
Nodes (6): 2026-07-10 · what happened this session, 2026-08-25 · full implementation + live verification, Gates — definition of done, HOLODEX-280 · Film poster/thumbnail asset pipeline, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 118 - "compilerOptions"
Cohesion: 0.15
Nodes (12): ./.svelte-kit/tsconfig.json, compilerOptions, allowJs, checkJs, esModuleInterop, forceConsistentCasingInFileNames, moduleResolution, resolveJsonModule (+4 more)

### Community 119 - "Design Handoff: Person Aliases ("Also known as") (F23)"
Cohesion: 0.10
Nodes (20): Accessibility notes, Animation / motion, Chip treatment (decisive), Collision prompt (person page, inline), Components, Design Handoff: Person Aliases ("Also known as") (F23), Design-system fit (the `/design-system` check), Design tokens used (+12 more)

### Community 120 - "Design Handoff — People Images (F25)"
Cohesion: 0.14
Nodes (14): A. People list (`/people`) — headshot, Accessibility notes, Animation / motion, B. Person page (`/people/[id]`) — banner hero + gallery + owner tools, C. Video page (`/media/[id]`) — poster cards, Components, Design Handoff — People Images (F25), Design tokens used (+6 more)

### Community 121 - "0043_films.up.sql"
Cohesion: 0.53
Nodes (5): film_images, film_people_roles, film_videos, films, films_fts

### Community 122 - "service.go"
Cohesion: 0.09
Nodes (26): resolveCounter, Asset, assetFetcher, EnrichRepo, EnrichResult, FakePerson, ProviderPerson, Candidate (+18 more)

### Community 123 - "Backfill"
Cohesion: 0.22
Nodes (8): image.Image, Backfill(), discardLog(), TestBackfillHashesAndRemoves(), downscale(), Hash(), Store(), BackfillRepo

### Community 124 - "Requirements"
Cohesion: 0.50
Nodes (4): Future Considerations (P2), Must-Have (P0), Nice-to-Have (P1), Requirements

### Community 125 - "query.go"
Cohesion: 0.28
Nodes (11): QueryFields, queryToken, Source, parseQueryPattern(), renderPattern(), SanitizeTitle(), sanitizeTitle(), TestSanitizeTitle() (+3 more)

### Community 126 - "NewService"
Cohesion: 0.47
Nodes (8): Store, newTestStore(), TestFetchAllowedImage_AllowedViaBaseHost(), TestFetchAllowedImage_AssetHostsExtendTheAllowlist(), TestFetchAllowedImage_IgnoresDisabledProvider(), TestFetchAllowedImage_RefusesUnlistedHost(), TestFetchAllowedImage_UnionAcrossMultipleProviders(), NewService()

### Community 127 - "Route"
Cohesion: 0.31
Nodes (9): Decision, Route(), TestRoute_BelowThreshold_RoutesToReview(), TestRoute_ExactMatchGate_AutoApplies(), TestRoute_FuzzyMatchNeverAutoApplies(), TestRoute_ManualOverrideAlwaysWins(), TestRoute_NonEntityField_AutoAppliesWithoutEntityMatch(), TestRoute_TierThresholds() (+1 more)

### Community 128 - "Store"
Cohesion: 0.46
Nodes (6): EnrichmentWriter, Store(), newRepo(), TestFilenameSourceResolvesWithNoResolverChange(), TestStore_EmptyFieldsIsNoop(), TestStore_RoundTripsThroughEntityEnrichment()

### Community 129 - ".add"
Cohesion: 0.44
Nodes (8): Manager, newFakeRepo(), TestDisabledManagerNoops(), TestExtractEmbedded(), testManager(), TestProcessGeneratesAndMarks(), TestProcessMarksFailed(), TestRunDrainsBackfill()

### Community 130 - "fakeStudioRepo"
Cohesion: 0.17
Nodes (5): fakeStudioRepo, ValidStudioImageRole(), Repo, StudioImage, StudioImageInsert

### Community 131 - "Spec: Entity Completeness Score (F55)"
Cohesion: 0.08
Nodes (24): Access control & security, Artifacts to produce (project working agreements), Data, storage & serving (direction — finalized in the ADR), Excluded fields, Facet tables per entity type, Facet weight and source tier, Frontend / theming requirements, Functional requirements (+16 more)

### Community 132 - "nationality.ts"
Cohesion: 0.07
Nodes (27): Derivation (see the spec for detail), Design Handoff: Nationality flag beside the person name (HOLODEX-139), Placement & measurements, States, Theming notes (what bites these surfaces), 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — all three skins (+19 more)

### Community 134 - "newAssetClient"
Cohesion: 0.16
Nodes (12): AssetClient, passthroughFetcher, net/http.Client, net/url.URL, assetHostAllowed(), assetRoleFor(), Source, newAssetClient() (+4 more)

### Community 135 - "newCompletenessHandlers"
Cohesion: 0.18
Nodes (20): FacetGroup, QueueRow, Handlers, sortFacetGroups(), sortRowsByName(), facetGroupByCanonical(), TestRemediationQueue_ActionableSplit(), TestRemediationQueue_GroupsByFacet() (+12 more)

### Community 136 - "Normalize"
Cohesion: 0.26
Nodes (15): Normalize(), forgePNGDims(), jpegBytes(), pngBytes(), TestGenderBucket(), TestNormalizeAcceptsWebP(), TestNormalizeDownscales(), TestNormalizeReencodesToJPEG() (+7 more)

### Community 137 - "Decision"
Cohesion: 0.09
Nodes (23): 1 — Data model (migration 0043), 2 — Asserted-link invariant: zero relink participation (realizes spec RD1/P0-2), 3 — `filmBaseline`: the entity whose baseline is other entities (realizes spec RD2), 4 — Film resolver source: a dynamically-namespaced `provider:<name>` (resolves Q1), 5 — `films_enabled` suspend mechanism: reuse the existing "decided source currently unmatched" path (resolves Q2), 6 — `films_enabled` gate wiring, 7 — API surface (owner-gated mutations only; reads public when enabled), A — Caller-injected synthetic namespace into the existing `Enrichment` map, one new `resolveDecided` branch (chosen) (+15 more)

### Community 138 - "context.Context"
Cohesion: 0.03
Nodes (27): Noop, recordingSink, storedAsset, countingResolver, context.Context, holodex/internal/model.PersonAlias, Handlers, Match (+19 more)

### Community 139 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.22
Nodes (9): 2026-07-10 · what happened this session, 2026-08-29 · session (1), 2026-08-29 · session (2), 2026-08-29 · session (3), 2026-08-29 · session (4), Gates — definition of done, HOLODEX-290 · StudioLinkCard (reusable Studio display), Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 140 - "demo/package.json"
Cohesion: 0.18
Nodes (10): sharp, dependencies, sharp, description, name, private, scripts, generate (+2 more)

### Community 141 - "Spec: Owner-authored person & studio ↔ media links, with file writeback (F40)"
Cohesion: 0.10
Nodes (20): API, Before implementation, Behavior detail, Future considerations (P2), Goals, Link derivation (RD2/RD3), Must-have (P0), Non-Goals (+12 more)

### Community 142 - "Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)"
Cohesion: 0.09
Nodes (22): 10. QA gate, 1. The remediation queue, 2. The completeness breakdown panel, 3. Components, 4. Tokens, 5. States, 6. Accessibility, 7. Edge cases (+14 more)

### Community 143 - "database/sql.DB"
Cohesion: 0.09
Nodes (36): filmVideoRow, database/sql.DB, database/sql.NullInt64, attachFilmVideo(), resolvedValue(), seedFilm(), TestFilmSourceInjection_SceneVsFullFilm(), readFilmVideos() (+28 more)

### Community 144 - "filmEntityServer"
Cohesion: 0.44
Nodes (9): decodeFilmItems(), filmEntityServer(), seedPlainVideo(), seedStudio(), seedTag(), TestFilmVideoCandidates(), TestFullFilmVideoHiddenFromListSurfaces(), TestListFilmsForEntity() (+1 more)

### Community 145 - "MergePersons(canonical, merged) transaction"
Cohesion: 0.67
Nodes (3): MergePersons(canonical, merged) transaction, videos.deleted_at TEXT NULL column (migration 0010), person_image_suppressions table (person_id, source_url) + person_images.source_url (migration 0012)

### Community 147 - "httpClient"
Cohesion: 0.23
Nodes (6): httpClient, Source, Source, newHTTPClient(), TestHTTPClientContract(), TestHTTPClientNoCrossHostRedirect()

### Community 148 - "QA Checklist: Derived / calculated person fields — Age & Age at death (F45)"
Cohesion: 0.33
Nodes (5): §1 Setup, §2 Smoke (run in `make test` / `npm run test`), §3 Agent live QA (preview tools against §1 stack), §4 Human eyes — 3-skin QA (Cinémathèque · Broadcast · Brutalist), QA Checklist: Derived / calculated person fields — Age & Age at death (F45)

### Community 149 - "ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio"
Cohesion: 0.10
Nodes (20): A — One badge per stored external-id row (chosen), A — Provider-declared `link_templates`, resolved server-side (chosen), A — Read-only projection of the existing identity tables (chosen), Action Items, ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio, B — Frontend-hardcoded per-namespace URL map, B — Pick a single "primary" badge (first-inserted, or a namespace priority order), B — Promote to a resolver-backed registry facet (widen F55's Person/Studio tables) (+12 more)

### Community 150 - "confidence.go"
Cohesion: 0.20
Nodes (17): Agreement, EntityMatch, Specificity, Tier, AutoApplyThreshold(), IsEntityField(), IsMultiValueField(), scoreAgreement() (+9 more)

### Community 151 - "completeness.go"
Cohesion: 0.22
Nodes (13): FacetSummary, PersonCompleteness, StudioCompleteness, VideoCompleteness, E, T, isMissingAll(), sortByScore() (+5 more)

### Community 152 - "Repo"
Cohesion: 0.32
Nodes (3): Repo, ProviderIcon, ProviderIconInsert

### Community 153 - "ADR-086: Film provider enrichment — own `entity_type`, poster as an asset"
Cohesion: 0.18
Nodes (11): 1 — Film enrichment gets its own `entity_type: "film"`, 2 — Film poster is an asset (`film_images.role = 'poster'`), never a canonical field, 3 — TMDB is the first provider; it needs an entity-type-aware remap, not new endpoints, 4 — Lock the `"film:"` namespace-collision boundary ADR-085 flagged, Action Items, ADR-086: Film provider enrichment — own `entity_type`, poster as an asset, Consequences, Context (+3 more)

### Community 154 - "Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)"
Cohesion: 0.07
Nodes (27): 2026-07-10 · what happened this session, 2026-08-05 · Backend + frontend implementation of F53, all applicable gates green, Gates — definition of done, HOLODEX-253 · Two-tier video poster resolution (F53), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), API, Behavior detail (+19 more)

### Community 155 - "Spec: Films as a first-class entity (F56)"
Cohesion: 0.11
Nodes (19): API, Asserted-link invariant (RD1/P0-2), Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions (+11 more)

### Community 157 - "QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)"
Cohesion: 0.40
Nodes (4): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — all three skins, QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173)

### Community 158 - "Spec: Poster View for the People list page (F55)"
Cohesion: 0.11
Nodes (19): API, Behavior detail, Conditional border — exact rule, Density formula, Future Considerations (P2), Gate status, Goals, Must-Have (P0) (+11 more)

### Community 159 - "api/person_images_test.go"
Cohesion: 0.35
Nodes (15): deleteReq(), fillGallery(), getStatus(), personImageServer(), personImageServerCfg(), pngUpload(), TestGetPersonImages(), TestPersonDetailImageSet() (+7 more)

### Community 160 - "Design handoff: Film Studio cascade edit affordance (F57)"
Cohesion: 0.11
Nodes (18): 1. Media detail page — no visual change, §1 Setup, 2. Film detail page — trigger affordance, §2 Smoke, §3 Agent live QA (preview tools against §1 stack), 3. New component: `FilmStudioCascadeDialog.svelte`, 3a. Step 1 — Picker (open state), 3b. Step 2 — Results (post-commit, same dialog, same `PickerShell`) (+10 more)

### Community 161 - "Spec: Tag Categories — grouping tags without merging them"
Cohesion: 0.17
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 162 - "Repo"
Cohesion: 0.10
Nodes (13): entityCompletenessBatch, EntityAlias, curationNorm(), CurationRow, Repo, Repo, mergeEntityLookupErr(), orderPair() (+5 more)

### Community 163 - "Field"
Cohesion: 0.07
Nodes (30): filmField(), filmFieldByCanonical(), filmFields(), personField(), personFieldByCanonical(), personFields(), providerSources(), rawSources() (+22 more)

### Community 164 - "ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename"
Cohesion: 0.12
Nodes (17): A — Namespace-qualified scalar value (chosen), Action Items, ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename, B — `(provider, external_id)`-keyed schema change across the nine tables, C — Leave `external_provider_id` a bare scalar, disambiguate providers elsewhere, Consequences, Context, Decision (+9 more)

### Community 165 - "writebackJob.ts"
Cohesion: 0.21
Nodes (11): BatchStatus, JOB_POLL_TIMEOUT_MS, pollUntilSettled(), fast, ADR-0041, ADR-0048, ADR-0077, waitForWritebackBatch() (+3 more)

### Community 166 - "thumbServer"
Cohesion: 0.26
Nodes (15): seedThumbVideo(), TestAdminStatus(), TestListEnqueuesVisibleAndExposesURL(), TestRegenerateDisabled(), TestRegenerateThumbnail(), TestServeThumbnailNotReadyThenReady(), thumbServer(), postersPNG() (+7 more)

### Community 167 - "Tag"
Cohesion: 0.18
Nodes (6): Category, EntityRef, Tag, Repo, nameCollidesInTable(), ReviewPair

### Community 168 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.10
Nodes (21): 2026-08-17 · Brainstormed the Films entity end-to-end, opened epic, wrote spec, 2026-08-18 · Wrote ADR-085, resolving spec Q1/Q2, 2026-08-21 · session, 2026-08-21 · session (cont.), 2026-08-21 · session (cont. 2), 2026-08-21 · session (cont. 3), 2026-08-21 · session (cont. 4), 2026-08-23 · session (+13 more)

### Community 170 - "ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation"
Cohesion: 0.12
Nodes (16): Action Items, ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation, Consequences, Context, D1: Facet criticality is static metadata on `registry.FieldDef`, D2: not-applicable persistence, D2: Not-applicable persists in a new, dedicated table — not a 4th decision `source`, D3/D4: score computation and list consumption (+8 more)

### Community 171 - "New"
Cohesion: 0.53
Nodes (8): New(), jpegBytes(), TestSinkRollsBackOnStoreFailure_Person(), TestSinkRollsBackOnStoreFailure_Studio(), TestSinkSkipsDuplicate(), TestSinkStoreAsset_Person_Normalizes(), TestSinkStoreAsset_Studio_Normalizes(), TestSinkUnsupportedEntityType()

### Community 173 - "routes/+layout.svelte"
Cohesion: 0.08
Nodes (22): activeRowIndex, activeTabIndex, announcement, flatCount, focusRow(), onRowKey(), onTabKey(), rowAt() (+14 more)

### Community 174 - "Repo"
Cohesion: 0.19
Nodes (6): ExtractionCandidate, SplitJoined(), Repo, ExtractionCandidate, ExtractionQueueRow, ExtractionReviewRow

### Community 175 - "hooks"
Cohesion: 0.29
Nodes (6): hooks, PostToolUse, PreToolUse, SessionStart, Stop, $schema

### Community 176 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.12
Nodes (16): 2026-07-10 · what happened this session, 2026-08-07 · Architecture gate closed — ADR-081 written, 2026-08-07 · Backend D1+D2 — facet criticality metadata + not-applicable mutation, 2026-08-07 · Backend D3 — score/actionability computation in internal/resolver, 2026-08-07 · Design gate closed — remediation queue + breakdown panel handoff written, 2026-08-07 · Spec written, Jira epic + stories created, branch/issue wired up, 2026-08-08 · Backend D4 — list-wide resolve-all predicate for browse sort/filter + remediation queue, 2026-08-08 · Item #4 — browse Completeness sort + Missing facet filter chip (F55.5/F55.6) (+8 more)

### Community 177 - "Decision"
Cohesion: 0.09
Nodes (21): ADR-018: Scanner Change Detection — Incremental Scan by (path, size, mtime), Consequences, Context, Decision, Mid-copy protection, Rationale, Scan algorithm, Stored fields (+13 more)

### Community 178 - "AutoRegisterFields"
Cohesion: 0.11
Nodes (30): promotionBody, promotionView, Handlers, hasNonEmpty(), lookupHint(), normalizeGroupOrEmpty(), GroupRank(), NormalizeDisplay() (+22 more)

### Community 179 - "Alias state seed (F58 / ADR-088)"
Cohesion: 0.18
Nodes (9): Deterministic fixture corpus + golden-file pattern (testdata/gen.sh), Alias state seed (F58 / ADR-088), Notes, What it stages, Why it goes through the repo, not SQL, Golden files, Regenerate, Test Fixtures (+1 more)

### Community 180 - "field_source_decisions table"
Cohesion: 0.67
Nodes (3): field_source_decisions table, four-tier label/render/group/order resolution ladder, settings KV table + typed Registry (validation/UI schema)

### Community 181 - "Spec: Metadata Source Plugins (F22/F27/F28)"
Cohesion: 0.11
Nodes (20): ADR-066 (enrichment auto-apply and dismissal), Auto-apply threshold (>=0.85 strong match, RD1), enrichment_dismissals table (durable not-matched verdict), Spec: Enrichment review workflow (F47), GET /owner/enrich-queue review queue (zero-cost DB signal), Refresh/Refresh-all bypass using stored external_id (RD7/RD8), ADR-033 (metadata source plugins), Spec: Metadata Source Plugins (F22/F27/F28) (+12 more)

### Community 182 - "ResolvedField"
Cohesion: 0.10
Nodes (23): enrichRoute, FieldCandidate, FieldDecision, Handlers, injectSyntheticFacet(), firstResolvedValue(), resolvedValues(), applyGenreWriteback() (+15 more)

### Community 183 - "Spec: Unified Studio edit affordance + Film-level cascade writeback (F57)"
Cohesion: 0.12
Nodes (16): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+8 more)

### Community 184 - "Decision"
Cohesion: 0.13
Nodes (15): ADR-003: SQLite (modernc.org/sqlite) + FTS5 chosen as database, WAL mode enabling concurrent reads during scanner writes, ADR-016: Database Migrations — golang-migrate with Embedded Versioned SQL, Consequences, Context, Decision, Rationale, 1. Global search (command-palette style) — primary search box (+7 more)

### Community 185 - "Spec: People Images (F25)"
Cohesion: 0.08
Nodes (26): Access control & security, Addendum — configurable cap, owner override & enrichment suppression ([ADR-043](../architecture/ADR-043-gallery-cap-and-enrichment-suppression.md), 2026-06-25), Addendum — enrichment photos are deduplicated in the gallery ([ADR-050](../architecture/ADR-050-image-content-dedup.md), F34, 2026-06-29), Addendum — owner/admin cap bypass, gallery grid modal, image viewer (HOLODEX-174, 2026-07-08), Addendum — owner-set core images take precedence over enrichment ([ADR-049](../architecture/ADR-049-manual-image-precedence.md), F33, 2026-06-28), Artifacts to produce (project working agreements), Data, storage & serving (direction — finalized in the ADR), F25.26–30 — Person-page polish (follow-ups) (+18 more)

### Community 187 - "stub.js"
Cohesion: 0.33
Nodes (3): fields, http, ADR-0033

### Community 189 - "Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)"
Cohesion: 0.13
Nodes (14): Accessibility Notes, Content specification (the string the owner actually sees), Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254), Design-system fit (the `/design-system` check), Edge case: empty sanitization result, Measured contrast, Non-goals (explicitly out of this change), Optional P1: seeded-value transparency caption (+6 more)

### Community 194 - "Spec: Two-Tier Field Editing Model (F56)"
Cohesion: 0.13
Nodes (15): Access control & security, Artifacts to produce (project working agreements), Data, storage & serving, Frontend / theming requirements, Goals, Non-Goals, Open Questions, Origin (+7 more)

### Community 195 - "ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision"
Cohesion: 0.13
Nodes (15): Action Items, ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision, Consequences, Context, Current state (survey, 2026-07-31), D1 — `categories` table: minimal, no identity-spine membership, tag-style fold for its own uniqueness, D1 — where Category's CRUD lives, D2 — `category_tags` junction: mirrors `video_tags` exactly, no provenance column (+7 more)

### Community 196 - "Spec: Collapse provider aliases into the canonical alias spine (F58)"
Cohesion: 0.12
Nodes (16): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+8 more)

### Community 197 - "gen-country-names.mjs"
Cohesion: 0.40
Nodes (4): countries, entries, OVERRIDE, require

### Community 198 - "field_claims.go"
Cohesion: 0.50
Nodes (3): claimBody, claimView, targetView

### Community 200 - "personDerivedServer"
Cohesion: 0.53
Nodes (8): findField(), getResolved(), indexOf(), personDerivedServer(), TestPersonDerived_AgeAtDeathReplacesAge(), TestPersonDerived_AgeUnderBirthdate(), TestPersonDerived_ComputedDecisionRejected(), TestPersonDerived_MissingBirthdateNoRow()

### Community 201 - "Spec: Tag Writeback Exclusion — per-tag Genre writeback control"
Cohesion: 0.17
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 202 - "Sink"
Cohesion: 0.25
Nodes (7): FilmRepo, personRepo, Sink, StudioRepo, filmImageSourceProvider(), ReplaceFilmImageFile(), ReplaceStudioImageFile()

### Community 205 - "svelte.config.js"
Cohesion: 0.40
Nodes (3): config, ADR-0002, ADR-0007

### Community 206 - "Design Handoff: Writeback hides the target file tag (HOLODEX-216)"
Cohesion: 0.14
Nodes (12): Design Handoff: Writeback hides the target file tag (HOLODEX-216), Design-system fit (the `/design-system` check), Non-goals (explicitly out of this change), Problem, QA checklist, Row states (unchanged rows omitted — only the new branch), The "no dimming" rule, applied, 2026-08-13 · session (+4 more)

### Community 207 - "net/http.ResponseWriter"
Cohesion: 0.12
Nodes (11): net/http.ResponseWriter, Handlers, chi.Router, Handlers, chi.Router, Handlers, chi.Router, Handlers (+3 more)

### Community 208 - "Design Handoff: Two-Tier Field Editing Model (F56)"
Cohesion: 0.15
Nodes (13): Accessibility Notes, Animation / Motion, Design Handoff: Two-Tier Field Editing Model (F56), Design-system fit (the `/design-system` check), Design Tokens Used, Edge Cases, Layout, Open Question carried to implementation (+5 more)

### Community 214 - "Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)"
Cohesion: 0.13
Nodes (15): 1. `/tags` — unified type filter + search, 2. Category pill, 3. `/categories/{id}` detail page (new route), 4. Bulk "Add to category…" / "Remove from category…" (Manage-mode bar), 5. Browse-page "Categories" facet, Accessibility notes, Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240), Design-system-fit audit (+7 more)

### Community 215 - "ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field"
Cohesion: 0.20
Nodes (10): 1. `studio_images` replaces `studio_logos` — three core roles, no gallery, 2. `enrich.ImageSink` / `downloadAssets` become entity-generic, 3. The studio `logo` field is retired; TMDB emits it as an asset, 4. Serving, upload, delete — mirrors ADR-057 §4 with an owner-write path added, Action items, ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field, Consequences, Context (+2 more)

### Community 219 - "+layout.ts"
Cohesion: 0.50
Nodes (3): prerender, ssr, ADR-0002

### Community 236 - "Spec: Configurable per-provider search query patterns (F54)"
Cohesion: 0.11
Nodes (18): Acceptance Criteria, FR1 — Operator pattern config (`metadata-sources.yaml`), FR2 — Provider-advertised preference (`/describe.preferred_search_pattern`), FR3 — Token grammar, rendering, and precedence fallthrough, FR4 — Unconditional title sanitizer, FR5 — Wiring: choke point, response payload, zero picker changes, Functional Requirements, Future Considerations (P2) (+10 more)

### Community 247 - "Spec: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.11
Nodes (18): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+10 more)

### Community 258 - "extractReviewServer"
Cohesion: 0.36
Nodes (13): extractReviewGET(), extractReviewPOST(), extractReviewServer(), TestDismissExtractionReview(), TestExtractionQueue_Empty(), TestExtractionQueue_ListsPendingRowsVideoJoined(), TestResolveExtractionReview_AcceptFilenameEnqueuesWrite(), TestResolveExtractionReview_AcceptTagWritesNothing() (+5 more)

### Community 263 - "Spec: System Activity — "Under the Hood" (F21)"
Cohesion: 0.10
Nodes (21): Cross-References, Data Model Extensions, F21.1 — Activity read-model API, F21.2 — Scanner status accessor, F21.3 — Persisted job history (30-day), F21.4 — Dedicated activity page (polled), F21.5 — Header activity indicator, F21.6 — In-UI controls (wires existing admin actions) (+13 more)

### Community 266 - "ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity"
Cohesion: 0.10
Nodes (20): A — Mandatory, no name fallback (chosen), A — Shared namespace, cross-provider convergence (chosen), Action Items, ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity, B — Preferred, name fallback quarantined, B — Provider-scoped keys `(provider, external_id)`, Conformance table (the invariant applied per entity), Consequences (+12 more)

### Community 269 - "Spec: People on the unified source-of-truth model (F37)"
Cohesion: 0.10
Nodes (20): API, Behavior detail, Future considerations (P2), Goals, Merge (RD5), Must-have (P0), Name materialization (RD1), Non-Goals (+12 more)

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

### Community 291 - "HOLODEX-6 · Per-field source-of-truth (F36 / ADR-051)"
Cohesion: 0.13
Nodes (13): 2026-06-29 · S1 backend — the decision engine, 2026-06-30 · S2 frontend — SourceSelect, 2026-07-01 · S3 gate — integration + live QA, Gates — definition of done, HOLODEX-6 · Per-field source-of-truth (F36 / ADR-051), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), flightplan.example.yaml — portability seam config (+5 more)

### Community 293 - "Design handoff: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.25
Nodes (8): 1. `/studios` list — logo well data source change only, 2. `/studios/{id}` detail — role-generic image control, 3. Provenance (P1, non-blocking), 4. Accessibility & 3-skin QA checklist, Design handoff: Studio image roles — icon / logo / poster (F51), Empty vs. populated states (per role), Interaction, Layout

### Community 297 - "Design Handoff: Metadata Enrichment UI for People (F22)"
Cohesion: 0.12
Nodes (16): Accessibility Notes, Animation / Motion, Components, Confidence display, Design Handoff: Metadata Enrichment UI for People (F22), Design Tokens Used, Edge Cases, Layout (+8 more)

### Community 300 - ".serveFilmImage"
Cohesion: 0.28
Nodes (4): serveEntityImageFile(), filmImageRole(), Handlers, chi.Router

### Community 301 - "Spec: Sticky sort preferences + Random sort"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, Requirements (+8 more)

### Community 302 - "ADR-001: Backend Language — Go"
Cohesion: 0.15
Nodes (12): .showcase.md Portfolio Self-Report, Incremental indexing (mtime/size change detection), Metadata writeback, Open enrichment protocol (provider sidecars), Unified field resolution, ADR-001: Backend Language — Go, Consequences, Context (+4 more)

### Community 307 - "Design Handoff: Media page — one sync verb, render-once fields (F36 / F39)"
Cohesion: 0.13
Nodes (15): Behaviour notes, Behaviour notes, Contrast, Design Handoff: Media page — one sync verb, render-once fields (F36 / F39), Design-system fit (the `/design-system` check), Layout, Layout, Not in scope (+7 more)

### Community 312 - "Functional Requirements"
Cohesion: 0.13
Nodes (15): F10: MCP Server, F11: Thumbnail Generation, F12: Browse UI Polish, F13: Observability, F20: Configurable Metadata Field Mapping, Functional Requirements, `get_video` response, In Scope (+7 more)

### Community 313 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.18
Nodes (11): 2026-07-10 · what happened this session, 2026-08-09 · Spec + design handoff for HOLODEX-268, 2026-08-10 · Frontend implementation — `SourceBadge.svelte` + HOLODEX-245 fix, 2026-08-10 · Testing strategy for HOLODEX-268, 2026-08-10 · Video/Studio `SourceBadge` rollout — frontend gate complete, 2026-08-11 · Code-review fixes applied, implementation PR opened, 2026-08-11b · discovered branch was stale, resynced with main, reopened PR, Gates — definition of done (+3 more)

### Community 317 - "QA: TMDB Provider Sidecar + ADR-039 Core Changes"
Cohesion: 0.13
Nodes (14): 0. Setup, 1. Provider contract — smoke (no real TMDB, no network), 2. ADR-039 core changes — `asset_hosts` allowlist, 3. End-to-end via Holodex + real TMDB provider, 4. Provider image (Docker), 5. Security checks, 6. Non-functional, 7. Film / Video enrichment (F26) (+6 more)

### Community 325 - "QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)"
Cohesion: 0.33
Nodes (6): 1. Overlay on playback (media detail page), 2. Search-history dropdown (header), 3. "More with …" shelves (media detail page), 4. Fluid Back (browse grid), 5. Cross-cutting, QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)

### Community 328 - "Functional Requirements"
Cohesion: 0.15
Nodes (13): Data Model Extensions (Phase 3), F14: People Enrichment, F15: Tag Enrichment, F16: Metadata Source Plugins, F17: Metadata Writeback, F18: Autogenerated Preview Trailers, Functional Requirements, In Scope (+5 more)

### Community 336 - "Decision"
Cohesion: 0.12
Nodes (15): ADR-011: Symlink Handling & Path Resolution, Configuration, Consequences, Context, Decision, Hardlinks, Loop protection, Resolution & dedup (+7 more)

### Community 337 - "Spec: Person Aliases (F23)"
Cohesion: 0.17
Nodes (12): API, Data model, Functional Requirements, In scope, Non-functional, Objective, Open questions, Out of scope (tracked follow-ups, not gaps) (+4 more)

### Community 351 - "Spec — Showcase Demo Corpus"
Cohesion: 0.20
Nodes (10): Acceptance criteria, Approach, Content requirements, Follow-ups (tracked separately), Goal, Non-goals, Problem, Showcase surfaces built on this corpus (+2 more)

### Community 353 - "Decision"
Cohesion: 0.12
Nodes (13): frontendFS(), frontendFS(), 1. Embed source lives in the `cmd/holodex` package, 2. SPA fallback handler, 3. BuildKit cache mounts, 4. `.dockerignore`, 5. Startup logs the URL, ADR-020: Frontend Embed Location, SPA Fallback Serving & BuildKit Caching (+5 more)

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

### Community 385 - "Design Handoff: Provider Link Badge — Multi-Badge States for Person/Studio (HOLODEX-266)"
Cohesion: 0.20
Nodes (10): 1. Placement on person and studio pages, 2. Cardinality states (0 / 1 / N), 3. Degraded state: id present, no link template, 4. Interaction and accessibility, Badge anatomy (recap, unchanged), DD1 — Badges join the existing muted metadata line, not a new row, DD2 — Wrap, don't scroll or collapse, DD3 — Badge order: alphabetical by provider label (+2 more)

### Community 398 - "Design Handoff: Tag & category create affordance (HOLODEX-243)"
Cohesion: 0.15
Nodes (13): 1. The "+ New" pill, 2. Expanded form, 3. Submit behavior — diverges by type (important asymmetry), 4. Empty-state wiring, 5. Interaction states, 6. Edge cases, Accessibility notes, Design Handoff: Tag & category create affordance (HOLODEX-243) (+5 more)

### Community 459 - "Load"
Cohesion: 0.16
Nodes (19): Config, applyEnv(), Defaults(), envBool(), envInt(), envInt64(), envStr(), Load() (+11 more)

### Community 461 - "Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)"
Cohesion: 0.07
Nodes (21): Behaviour notes, Bulk bar (`tags/+page.svelte`), Component: `WritebackBatchDialog.svelte`, Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239), Design-system fit (the `/design-system` check), Details card (`tags/[id]/+page.svelte`), Layout, Measured contrast (all three skins, dialog + card surfaces) (+13 more)

### Community 462 - "HOLODEX-240.md"
Cohesion: 0.40
Nodes (4): 2026-07-31 · session, Gates — definition of done, Session log   (append-only), Up next   (ordered — position is the priority; top line is the next action)

### Community 465 - "holoShuffle"
Cohesion: 0.50
Nodes (3): holoShuffle(), registerShuffle(), TestHoloShuffle()

### Community 466 - "SanitizeLinkTemplates"
Cohesion: 0.29
Nodes (8): BuildLink(), SanitizeLinkTemplates(), TestBuildLink(), TestManifest_LinkTemplatesDecodeBackwardCompat(), TestSanitizeLinkTemplates_DropsInvalidNormalizesKeys(), TestSanitizeLinkTemplates_EmptyAndNil(), TestValidateLinkTemplate(), ValidateLinkTemplate()

### Community 467 - "Complete"
Cohesion: 0.20
Nodes (15): ForNamespace(), ForProvider(), actionableCandidate(), classifyTier(), Complete(), criticalityWeight(), ResolvedField, fld() (+7 more)

### Community 468 - "cascadeServer"
Cohesion: 0.49
Nodes (10): cascadePost(), cascadeServer(), seedCascadeVideo(), TestCascadeFilmStudio_AllCollide_EmptyBatch(), TestCascadeFilmStudio_PartialCollision_BestEffort(), TestCascadeFilmStudio_SameValueRedecide_NotACollision(), TestCascadeFilmStudio_UnmatchedProvider_PerVideoError(), TestCascadeFilmStudio_ZeroVideos_EmptyBatch() (+2 more)

### Community 470 - "Handoff Spec: Person detail — bio in the header row"
Cohesion: 0.14
Nodes (14): Accessibility Notes, Animation / Motion, Design-system fit, Design Tokens Used, Edge Cases, Handoff Spec: Person detail — bio in the header row, Layout, Open Questions carried to implementation (+6 more)

### Community 471 - ".RelinkProviderIcon"
Cohesion: 0.25
Nodes (7): providerInfo, Handlers, providerIconURL(), ImagePath(), Remove(), Store(), TestStoreRoundTrip()

### Community 473 - "HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film)"
Cohesion: 0.33
Nodes (5): 2026-08-25 · full implementation + simplify + verification, Gates — definition of done, HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 474 - "net/http/httptest.Server"
Cohesion: 0.13
Nodes (40): net/http/httptest.Server, peopleDecisionServer(), TestCurationAPI_NonPersonFieldSkipsCollisionGate(), TestCurationAPI_PeopleCollision(), TestCurationAPI_PeopleCollision_Suppress(), TestCurationAPI_PersonFieldNotMapped(), putDecisionRaw(), rawRequest() (+32 more)

### Community 475 - "HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-08-13 · Implemented the SSRF perimeter fix end to end, 2026-08-13 · PR #238 opened, then a `/code-review --fix` pass found and closed a follow-on gap, Gates — definition of done, HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 476 - "Spec: Quick Wins batch — Search history & "More with …" shelves"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, QW1 — Search history (client-only) (+8 more)

### Community 477 - "Design Handoff: Poster View for the People list page (F55)"
Cohesion: 0.11
Nodes (19): Accessibility, Component, CSS (new, additive — `app.css`, filed next to the `.portrait-frame` block), Design Handoff: Poster View for the People list page (F55), Design tokens used (all surfaces), Edge cases, Gate status (mirrors the spec), Load-in animation (+11 more)

### Community 479 - ".externalLinksForEntity"
Cohesion: 0.33
Nodes (4): ExternalLink, Handlers, TestNamespaceLabel(), namespaceLabel()

### Community 480 - "Design handoff: In-app promote / override affordance for auto-registered fields (F44)"
Cohesion: 0.15
Nodes (13): 10. Three-skin QA (required), 11. What is explicitly not in this handoff, 1. The Promote control (owner-only, on the auto row), 2. The inline editor (shared promote + edit — DD2), 3. After promotion — the partition move (shared by all treatments), 4. Edit / Remove promotion (owner-only, on the promoted row), 5. States, 6. Responsive behavior (+5 more)

### Community 483 - "parseEntityType"
Cohesion: 0.13
Nodes (8): Handlers, chi.Router, Handlers, chi.Router, Handlers, chi.Router, parseEntityType(), IsKnown()

### Community 484 - "toAnySlice"
Cohesion: 0.08
Nodes (13): EnrichQueueProviderState, Studio, Repo, T, namedCountQuery(), placeholders(), toAnySlice(), Repo (+5 more)

### Community 485 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.14
Nodes (14): 2026-09-02 · ADR-088 + design handoff landed; direction set to a full collapse, 2026-09-02 · P0-1 migration 0044 landed, 2026-09-02 · spec landed — all four pre-implementation gates green, 2026-09-02 · testing gate closed, 2026-09-03 · alias-state seed generator + a bug it immediately found, 2026-09-03 · P0-2/P0-4/P0-5 enrich write path landed, 2026-09-03 · P0-6/P0-6b/P0-7 registry removal + completeness facet landed, 2026-09-03 · P0-8 model + API surface landed — backend complete (+6 more)

### Community 486 - "Spec: Owner tooling hub + visitor/owner nav split (F35)"
Cohesion: 0.11
Nodes (17): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Owner tooling hub + nav split (F35), Future Considerations (P2), Goals, Must-Have (P0) (+9 more)

### Community 487 - ".ReconcileVideoPeopleLocked"
Cohesion: 0.22
Nodes (7): extIDFor(), foldedExtIDIndex(), foldNameKey(), PersonRoleName, Repo, personHasAuthoredIdentity(), personLinkKey

### Community 489 - "database/sql.Tx"
Cohesion: 0.06
Nodes (34): database/sql.Tx, resolveOrCreatePerson(), attachExternalID(), canonicalTable(), externalIDTable(), Repo, lookupByNameKey(), nameKeyExpr() (+26 more)

### Community 490 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.22
Nodes (9): 2026-08-25 · implementation landed — all seven gates green, 2026-08-25 · resolved three rounds of merge conflicts against a fast-moving main, 2026-08-25 · security-review landed — all five pre-implementation gates green, 2026-08-25 · spec, ADR, and design handoff landed, 2026-08-25 · testing-strategy landed; mockup persistence established as a standing rule, Gates — definition of done, HOLODEX-285 · Unified Studio edit affordance + Film-level cascade writeback, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 491 - "fakeFilmRepo"
Cohesion: 0.18
Nodes (5): fakeFilmRepo, ValidFilmImageRole(), FilmImage, FilmImageInsert, Repo

### Community 504 - "writeJSON"
Cohesion: 0.06
Nodes (14): WriteBatchFunc, Handlers, Handlers, chi.Router, validEntityType(), Handlers, setFilmImageURLs(), Handlers (+6 more)

### Community 505 - "Health"
Cohesion: 0.32
Nodes (3): Health, sync/atomic.Bool, writeStatus()

### Community 506 - "Design Handoff: Studio relationship-edit popover (HOLODEX-271)"
Cohesion: 0.20
Nodes (10): Accessibility Notes, Components, Design Handoff: Studio relationship-edit popover (HOLODEX-271), Design Tokens Used, Edge Cases, Layout, Overview, QA (+2 more)

### Community 507 - "HOLODEX-288 · Fix film-studio cascade code-review findings"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-25 · session, Gates — definition of done, HOLODEX-288 · Fix film-studio cascade code-review findings, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 510 - "filters.ts"
Cohesion: 0.18
Nodes (10): buildQuery(), DEFAULT_SORT, filtersToParams(), MEDIA_SORTS, paramsToFilters(), SORT_ORDERS, ADR-0045, MediaFilters (+2 more)

### Community 511 - "newTestService"
Cohesion: 0.39
Nodes (7): bytes.Buffer, Service, newTestService(), TestPersistPreferredPattern_CachesValidPattern(), TestPersistPreferredPattern_EmptyClearsPriorValue(), TestPersistPreferredPattern_InvalidPatternDroppedAndLogged(), TestPersistPreferredPattern_PerProviderIsolation()

### Community 513 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.22
Nodes (9): 2026-08-11 · simplify + security-review, all gates green, ready to commit, 2026-08-11a · spec + design handoff written, 2026-08-11b · backend + frontend implementation, 3-skin live QA, 2026-08-11c · PR #231 code-review fixes + a second /simplify pass, 2026-08-11d · merged origin/main into PR #231, resolved 11-file conflict, Gates — definition of done, HOLODEX-271 · Studio relationship-edit popover (F56.4), Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 518 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.25
Nodes (8): 2026-07-10 · what happened this session, 2026-08-26 · session (1), 2026-08-26 · session (2), 2026-08-26 · session (3), 2026-08-26 · session (4), 2026-08-26 · session (5), 2026-08-26 · session (6), Session log — append-only (cap: last 8 sessions; older → archive/)

### Community 520 - "extractServer"
Cohesion: 0.56
Nodes (8): extractPOST(), extractServer(), TestAdminExtractAllAccepted(), TestAdminExtractAllUnavailable(), TestExtractMediaMatch(), TestExtractMediaNotFound(), TestExtractMediaRequiresOwner(), TestExtractMediaUnavailable()

### Community 523 - "EnrichmentRow"
Cohesion: 0.11
Nodes (12): relinkContext, Handlers, TestPersonExternalIDsFromRows(), personExternalIDsFromRows(), externalIDsFromRows(), Handlers, TestStudioExternalIDsFromRows(), studioExternalIDsFromRows() (+4 more)

### Community 527 - "studio-picker-handoff.md"
Cohesion: 0.33
Nodes (3): Gates — definition of done, HOLODEX-289 · Studio add-affordance on the media detail page, Up next — ordered (position = priority)

### Community 531 - "HOLODEX-292 · Shared TagLinkChip component"
Cohesion: 0.33
Nodes (6): 2026-08-29 · session, 2026-08-29 · session, Gates — definition of done, HOLODEX-292 · Shared TagLinkChip component, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 538 - "Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA"
Cohesion: 0.33
Nodes (6): Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA, Decision: empty-state CTA — "+ Add studio" text button, not a bare pencil, Decision: pencil position — trailing, not leading, Decision: visibility — always-visible, not hover-revealed, Do / Don't, States (trigger, superseding "States and Interactions" above)

### Community 540 - "routes/+page.svelte"
Cohesion: 0.13
Nodes (9): Video components, capForWidth(), clamp(), DENSITY_MAX, DENSITY_MIN, load(), MediaDensity, TIERS (+1 more)

### Community 541 - "Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided""
Cohesion: 0.17
Nodes (11): Accessibility, Decided visual spec (Option 1), Design Handoff: Writeback dialog — poster comparison + enrichment/decision legibility gap, Fix options considered, Issue 1 — the dialog never shows the file's current poster next to the enriched candidate, Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided", Layout, Overview (+3 more)

### Community 544 - "coverArtManager"
Cohesion: 0.36
Nodes (7): assertDecodedWidth(), coverArtManager(), Manager, pngOfWidth(), TestWriteCoverArtTiersScaling(), TestWriteCoverArtTiersWithinBothCaps(), TestGenerateFrameRealFfmpeg()

### Community 545 - "HOLODEX-102 · Video Credits → People + Headshots (F32)"
Cohesion: 0.29
Nodes (6): 2026-06-30 – 2026-08-06 · F32 implementation (4 slices), 2026-08-06 · code-review + simplify + doc/Jira sync, Gates — definition of done, HOLODEX-102 · Video Credits → People + Headshots (F32), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 546 - "HOLODEX-255 · <epic title>"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-05 · session, Gates — definition of done, HOLODEX-255 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 547 - "films-entity.md"
Cohesion: 0.09
Nodes (17): 2026-07-10 · what happened this session, 2026-08-04 · Full epic delivered end-to-end and merged, Gates — definition of done, HOLODEX-247 · Studio image roles: icon, logo, poster (F51), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), 2026-08-27 · Implemented film_people_roles CRUD, Gates — definition of done (+9 more)

### Community 548 - "HOLODEX-114 · <epic title>"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-05 · session, Gates — definition of done, HOLODEX-114 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 553 - "HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values"
Cohesion: 0.29
Nodes (6): 2026-08-13 · code-review xhigh --fix, 2026-08-13 · implementation + tests + docs sync + PR opened, Gates — definition of done, HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 554 - "HOLODEX-275 · GET /api/v1/facets marshals empty values as null, not []"
Cohesion: 0.29
Nodes (6): 2026-08-12 · `/code-review xhigh` follow-up pass, applied, 2026-08-12 · Root-caused and fixed the nil-slice marshaling bug, audited for siblings, Gates — definition of done, HOLODEX-275 · GET /api/v1/facets marshals empty values as null, not [], Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 555 - "HOLODEX-244 · <epic title>"
Cohesion: 0.33
Nodes (5): 2026-07-10 · what happened this session, Gates — definition of done, HOLODEX-244 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 556 - "HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip"
Cohesion: 0.33
Nodes (5): 2026-08-29 · session, Gates — definition of done, HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 557 - "HOLODEX-273 · Writeback dialog "Select all undecided" doesn't create a standing decision"
Cohesion: 0.33
Nodes (5): 2026-08-10 · Implemented, self-corrected, and live-verified the decision-on-checkbox fix, Gates — definition of done, HOLODEX-273 · Writeback dialog "Select all undecided" doesn't create a standing decision, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 563 - "two-tier-field-editing-handoff.md"
Cohesion: 0.31
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist), Manual QA Checklist: Two-Tier Field Editing Model (F56)

### Community 566 - "Interaction design"
Cohesion: 0.40
Nodes (5): At rest, Expand, Interaction design, Sync indicator, The pending (RD6) case, explicitly

### Community 572 - "filmServer"
Cohesion: 0.29
Nodes (11): TestFilmFieldDecision(), filmPut(), TestFilmPeopleRolesCRUD(), filmDelete(), filmPost(), filmServer(), TestBulkAttachFilmVideosUnnumbered(), TestCreateFilm_GetOrCreate() (+3 more)

### Community 573 - "Requirements"
Cohesion: 0.50
Nodes (4): Future considerations (P2), Must-have (P0), Nice-to-have (P1), Requirements

### Community 576 - "listScroll.svelte.ts"
Cohesion: 0.22
Nodes (11): browseCache, BrowseSnapshot, ADR-0032, listScroll, ListScrollSnapshot, ADR-0032, createNavSnapshot(), createNavSnapshotRegistry() (+3 more)

### Community 577 - "HOLODEX-294 · Reusable PeopleGrid component"
Cohesion: 0.40
Nodes (5): 2026-08-29 · session, Gates — definition of done, HOLODEX-294 · Reusable PeopleGrid component, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 578 - "Decision"
Cohesion: 0.15
Nodes (13): ADR-088: Provider aliases collapse into the canonical alias spine, Alternatives considered, Consequences, Context, D1 — `aliases` leaves the field registry, D2 — `entity_aliases` gains a `source` column, D3 — A provider alias is fully live on arrival, D4 — Deleting a provider alias records a suppression (+5 more)

### Community 579 - "dismissable"
Cohesion: 0.80
Nodes (5): dismissable(), activate(), deactivate(), onClick(), onKey()

### Community 581 - "handler"
Cohesion: 0.29
Nodes (6): io.ReadCloser, decode(), isSupportedEntity(), writeJSON(), enrichRequest, handler

### Community 588 - ".sessionCookie"
Cohesion: 0.33
Nodes (5): net/http.Cookie, ttlForClass(), baseSessionCookie(), Handlers, secureCookie()

### Community 589 - "Design handoff: Completeness panel — collapsible facet fold"
Cohesion: 0.18
Nodes (11): Accessibility notes, Animation / motion, Components, Design handoff: Completeness panel — collapsible facet fold, Design tokens used, Edge cases, Layout, Overview (+3 more)

### Community 591 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (8): 2026-08-31 · design-handoff artifacts committed, 2026-08-31 · session, 2026-09-01 · code-review high --fix, 2026-09-01 · implementation landed, Gates — definition of done, HOLODEX-303 · Person detail: move bio into the name/photo header row, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 592 - "ADR-012: Resolution Classification — Width-Based Buckets with 10% Tolerance"
Cohesion: 0.22
Nodes (9): ADR-006: REST + OpenAPI 3.1 API design under /api/v1, ADR-012: Resolution Classification — Width-Based Buckets with 10% Tolerance, Consequences, Context, Decision, Effective cutoffs (nominal − 10%), Nominal tier widths, Rationale (+1 more)

### Community 594 - "Design handoff — alias collapse (HOLODEX-306)"
Cohesion: 0.25
Nodes (8): Accessibility, Chip anatomy, Collision review line, Design handoff — alias collapse (HOLODEX-306), Out of scope, States, Theming, What changes on screen

### Community 595 - "Placeholder"
Cohesion: 0.46
Nodes (7): GenderBucket(), minF(), paletteFor(), Placeholder(), placeholderDims(), shoulderWidth(), skinPalette

### Community 598 - "ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)"
Cohesion: 0.29
Nodes (7): ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary), Client Configuration Examples, Configuration, Consequences, Context, Decision, Rationale

### Community 599 - "HOLODEX-298 · Film detail page: match media page's Tags section styling"
Cohesion: 0.29
Nodes (6): 2026-08-30 · Matched film detail Tags section to the media page's styling, 2026-08-30 · Repositioned Tags back into the header column, underneath Studio, Gates — definition of done, HOLODEX-298 · Film detail page: match media page's Tags section styling, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 600 - "HOLODEX-300 · Film bulk-attach dialog: default search term + optional starting scene number"
Cohesion: 0.29
Nodes (6): 2026-08-30 · `/code-review high --fix` caught and fixed a numbered-attach crash, 2026-08-30 · Implemented, tested, and live-verified both fixes, Gates — definition of done, HOLODEX-300 · Film bulk-attach dialog: default search term + optional starting scene number, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 601 - "HOLODEX-302 · Person hero image hover-to-front"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-31 · implementation + verification + ticket/worklog, Gates — definition of done, HOLODEX-302 · Person hero image hover-to-front, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 602 - "HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise"
Cohesion: 0.29
Nodes (6): 2026-09-01 · session, 2026-09-01 · session (code-review), Gates — definition of done, HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 603 - "HOLODEX-307 · Film detail: poster becomes the header image; remove the Images section"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-09-04 · full implementation + live verification, Gates — definition of done, HOLODEX-307 · Film detail: poster becomes the header image; remove the Images section, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 604 - "seedTwoTags"
Cohesion: 0.52
Nodes (6): seedTwoTags(), TestNearMiss(), TestReviewQueue_DismissRecordsKeepSeparate(), TestReviewQueue_MergeDropsPair(), TestReviewQueue_ScanFlagsAndList(), TestReviewQueue_SeedBatch()

### Community 605 - "Holodex landing page (site/index.html)"
Cohesion: 0.29
Nodes (6): Holodex landing page (site/index.html), Landing page 'One library, three skins' section (Cinémathèque/Broadcast/Brutalist), Deploy, Holodex landing page, Preview locally, SvelteKit app.html shell (default data-theme=cinematheque)

### Community 606 - "HOLODEX-283 · Films: real backend search integration"
Cohesion: 0.33
Nodes (5): 2026-08-27 · session, Gates — definition of done, HOLODEX-283 · Films: real backend search integration, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 607 - "HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list"
Cohesion: 0.33
Nodes (5): 2026-08-30 · Fixed empty candidate list in the film→video bulk attach dialog, Gates — definition of done, HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 608 - "HOLODEX-301 · Completeness panel collapsible facet fold"
Cohesion: 0.33
Nodes (6): 2026-07-10 · what happened this session, 2026-08-31 · design handoff + implementation + retroactive ticket/worklog, Gates — definition of done, HOLODEX-301 · Completeness panel collapsible facet fold, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

## Knowledge Gaps
- **2149 isolated node(s):** `$schema`, `SessionStart`, `PostToolUse`, `Stop`, `PreToolUse` (+2144 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **234 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Canonical Field Registry (operator reference)` connect `Configuration Reference (holodex.yaml layers)` to `AutoRegisterFields`?**
  _High betweenness centrality (0.309) - this node is a cross-community bridge._
- **Why does `Spec: Derived/calculated person fields (F45)` connect `Configuration Reference (holodex.yaml layers)` to `QA: Metadata Writeback (F28)`?**
  _High betweenness centrality (0.302) - this node is a cross-community bridge._
- **Why does `Lookup()` connect `Field` to `parseEntityType`, `EnrichmentRow`, `Service`, `writeError`, `Handlers`, `AutoRegisterFields`, `Complete`, `ResolvedField`, `ResolveFields`, `time.Time`, `.setFieldDecision`?**
  _High betweenness centrality (0.208) - this node is a cross-community bridge._
- **Are the 167 inferred relationships involving `newRepo()` (e.g. with `TestAliasesSurviveRescan()` and `TestMergePersons()`) actually correct?**
  _`newRepo()` has 167 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `SessionStart`, `PostToolUse` to the rest of the system?**
  _2149 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `types.ts` be split into smaller, more focused modules?**
  _Cohesion score 0.01836644591611479 - nodes in this community are weakly interconnected._
- **Should `media/[id]/+page.svelte` be split into smaller, more focused modules?**
  _Cohesion score 0.05945945945945946 - nodes in this community are weakly interconnected._