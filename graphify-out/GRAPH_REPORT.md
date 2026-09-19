# Graph Report - laughing-wu-649060  (2026-09-18)

## Corpus Check
- 1085 files · ~1,676,419 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 8604 nodes · 18163 edges · 711 communities (400 shown, 231 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 2099 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7d25ebb3`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- types.ts
- media/[id]/+page.svelte
- whats-left.mjs
- Decision
- Repo
- newRepo
- api/writeback_test.go
- ladder_test.go
- Holodex Project Working Agreements (CLAUDE.md)
- testMappings
- devDependencies
- JobRun
- imagetools.mjs
- newProviderIconEnv
- NameEditControl.svelte
- Service
- resolver.go
- writeback/writeback.go
- Handlers
- ResolveReviewAction
- time.Time
- NewService
- toAnySlice
- Design handoff: StudioLinkCard (reusable Studio display)
- Spec: Page-scoped hotkeys (F62)
- enrich/enrich_test.go
- Design Handoff: Unified name-edit mechanism (HOLODEX-269)
- tmdb.go
- Auth
- QA: Metadata Writeback (F28)
- handler
- Spec: Derived/calculated person fields (F45)
- ADR-090-two-layer-entity-metadata-management.md
- mcp.go
- Spec: Tag governance & video enrichment (F50)
- extractor.go
- Resolve
- service.go
- activity.svelte.ts
- people/+page.svelte
- jira-sync.mjs
- claim_test.go
- people
- People attach/detach + relationship picker (F56.5, HOLODEX-272)
- New
- density.svelte.ts
- Queue
- ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam
- images_test.go
- generate.mjs
- Flightplan — portable session-state plugin
- Video composite-key collision check (F56.3, HOLODEX-270)
- theme.svelte.ts
- run.mjs
- studios
- ResolveFields
- count
- Handlers
- tmdb_test.go
- getJSON
- Manager
- SanitizeValue
- review_queue_test.go
- Mappings
- extraction.ts
- f36.ts
- ADR-046 (per qa-metadata-curation.md): Metadata curation and write queue
- Process
- Design Handoff: Unified nav search — live, tabbed, in-place filtering panel
- seedPerson
- Derive
- videos
- Studio relationship-edit popover (F56.4, HOLODEX-271)
- Quick Wins batch (overlay fix, search history, related shelves, fluid Back)
- ADR-058 (Jira transitions via direct REST API)
- fakeStudioRepo
- ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action
- Spec: Sticky sort preferences + Random sort
- ResolveForContainer
- enrich/enrich.go
- report.mjs
- .mountDelete
- .add
- Spec: Tag Detail — Hierarchy & Category Controls
- process.go
- database/sql.DB
- Design handoff: PeopleGrid (reusable People/Cast display)
- getJSONTok
- routes/tags/+page.svelte
- Normalize
- Release promotes by retagging the canaried digest
- QA Checklist: System Activity (F21)
- metadata-mappings.yaml config file: source-key-to-canonical-field mapping with precedence
- Decision
- entityKind
- QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)
- jarowinkler.go
- Design handoff: Films entity (F56)
- Fake
- Repo
- decide
- .addEntityAlias
- Design handoff: Media detail — Films + People sections
- deleted_at soft-delete column (orthogonal to active)
- Spec: Tag & Category Create Affordance — closing the /tags creation gap
- BatchRunner
- Spec: Tag Categories — grouping tags without merging them
- ADR-080: Configurable per-provider metadata search query patterns
- FilmStudioCascadeDialog.svelte
- Design handoff: TagLinkChip (reusable Tag display)
- itoa
- repo/related_test.go
- tmdbClient
- category_tags
- ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write
- Decision
- seedCountInto
- ADMIN_TOKEN env var — v1 owner identity, default-open when unset
- compilerOptions
- Design Handoff: Person Aliases ("Also known as") (F23)
- Complete
- 0043_films.up.sql
- seedTagTree
- Design handoff: Film provider enrichment (F59)
- Requirements
- query.go
- Design handoff: Media detail stage layout
- Route
- Store
- model.go
- Design handoff: Media detail Metadata — move, trim, fold
- Spec: Entity Completeness Score (F55)
- nationality.ts
- routes/+layout.svelte
- assetHostAllowed
- newCompletenessHandlers
- Spec: Holodex Metadata Provider Contract (hand-off, protocol v1)
- Decision
- context.Context
- Session log — append-only (cap: last 8 sessions; older → archive/)
- web/package.json
- Spec: Owner-authored person & studio ↔ media links, with file writeback (F40)
- Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)
- manifest.go
- generate
- MergePersons(canonical, merged) transaction
- Shared ingest normalization pipeline (decode → bound → re-encode → strip)
- queue
- wireSvc
- ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio
- confidence.go
- repo/studios_test.go
- Design handoff — Entity identity card (F60)
- ADR-086: Film provider enrichment — own `entity_type`, poster as an asset
- Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)
- Spec: Films as a first-class entity (F56)
- Design Handoff: Tag & category create affordance (HOLODEX-243)
- Spec: Revealable per-candidate record summary in the resolve picker (F61)
- Spec: Poster View for the People list page (F55)
- api/person_images_test.go
- Design handoff: Film Studio cascade edit affordance (F57)
- QA checklist: Responsive page width (HOLODEX-331)
- nameKeyExpr
- Geometry assertion harness (HOLODEX-349)
- ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename
- writebackJob.ts
- Write
- Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)
- Session log — append-only (cap: last 8 sessions; older → archive/)
- keyed
- ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation
- Find
- adr-claims.mjs
- Video
- scanner_test.go
- settings.json
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Decision
- .mountCategories
- Stress fixture seeder (HOLODEX-342)
- field_source_decisions table
- architecture/README.md
- ResolvedField
- Spec: Unified Studio edit affordance + Film-level cascade writeback (F57)
- AutoRegisterFields
- Spec: People Images (F25)
- F38: Studio entity pages
- stub.js
- job_runs table (kind, trigger, status, counts, 30-day retention)
- Design handoff — Media parts (`part` badge and pill)
- jira-sync.mjs REST transition mechanism (idempotent, match-by-name, soft-fail)
- Keyset cursor over (started_at, id) for job history
- activity/CLAUDE.md
- person-detail-bio-header-handoff.md
- Spec: Two-Tier Field Editing Model (F56)
- ADR-078: Tag Categories — a deliberately reduced entity, its junction shape, and cross-table name collision
- Spec: Collapse provider aliases into the canonical alias spine (F58)
- gen-country-names.mjs
- field_claims.go
- duplicates/CLAUDE.md
- personDerivedServer
- Design Handoff — People Images (F25)
- Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating
- studio-link-card-handoff.md
- gen.sh
- Design handoff: Entity Films row — card-height match + hover lift
- Design Handoff: Writeback hides the target file tag (HOLODEX-216)
- net/http/httptest.Server
- Design Handoff: Two-Tier Field Editing Model (F56)
- .RelinkProviderIcon
- cmd/holodex/holodex.manifest — manifest source XML (requestedExecutionLevel=asInvoker)
- 0004_job_runs.up.sql
- 0005_entity_enrichment.up.sql
- Rules
- Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)
- ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field
- New
- app.d.ts
- @sveltejs/kit
- +layout.ts
- CurationFieldRow.svelte
- dependencies
- scripts
- 0013_metadata_curation.up.sql
- Holodex media detail page — Broadcast skin
- Screenshot: Holodex video detail page in the 'Brutalist' theme skin — shows top nav (Media/People/Tags, skin switcher with Cinémathèque/Broadcast/Brutalist options), a video player for 'Nightshade' (Thriller, 2021), and metadata panel with format badge (4K, 3840x2160, 1:58:12, 2021), People chips (Lana Reyes, Marcus Vane), and Tags chips (Noir, Thriller)
- Screenshot: Video detail page in the Cinémathèque skin — shows player, title card, metadata, people, and tags for a sample video 'Nightshade'
- Grid/Browse View Screenshot (Broadcast Skin)
- Screenshot: Holodex media browse grid view in the 'Brutalist' skin — top nav (HOLODEX logo, global search Ctrl-K, Media/People/Tags links, Cinematheque/Broadcast/Brutalist skin switcher with Brutalist active), filter bar (title search, resolution toggle All/SD/HD/FHD/4K, duration min-max, year range, People and Tags filters), '18 videos' result count, and a 3-column card grid of 18 video entries each showing a resolution badge (4K/HD/FHD/SD), duration, abstract colored thumbnail art, uppercase title, and genre tag chips (e.g. Vantablack: Horror/Thriller, Solar Drift: Adventure/Sci-Fi, Amélie en Hiver: Drama/Romance)
- Screenshot: Holodex browse/grid view in the 'Cinémathèque' skin. Dark theme with warm amber/gold accents. Top bar shows 'Holodex' logo, global 'Search everything... (Ctrl-K)' box, nav links (Media, People, Tags), and a three-way skin switcher (Cinémathèque selected/amber, Broadcast/blue, Brutalist/green). Filter row includes title search, Resolution toggle (All/SD/HD/FHD/4K), Duration (min) range, Year range, People multiselect, and Tags multiselect. Results header reads '18 videos'. Grid of movie-poster style cards (3 rows x 6 columns) each showing a colored gradient thumbnail with a resolution badge (4K/HD/FHD/SD) top-left, duration badge (e.g. 1:36:40) bottom-right, title overlay, and below the card a title repeated plus genre/tag chips (e.g. Vantablack - Horror, Thriller; Tin Soldier - Drama, War; The Long Saturday - Comedy; The Quiet Coast - Drama; The Cartographer - Drama; Static Bloom - Experimental, Short; Solar Drift - Adventure, Sci-Fi; Overgrowth - Documentary, Nature; Paper Moons - Animation, Family; Nightshade - Noir, Thriller; Neon Tide - Crime, Noir; Migration - Documentary, Nature; Harbor Lights - Drama, Romance; Glasshouse - Mystery, Thriller; Dust & Echoes - Drama, Western; Concrete Garden - Documentary; Amélie en Hiver - Drama, Romance; Ferrous - Sci-Fi, Thriller).
- Dependabot Config
- Deploy Landing Page Workflow
- Must-Have (P0)
- Spec: Studio image roles — icon / logo / poster (F51)
- holodex
- TMDB Brand Logo (tmdb-brand.png)
- Holodex media detail page screenshot (Broadcast skin)
- Screenshot: media detail page (brutalist skin)
- Detail page screenshot (Cinémathèque skin) — Holodex showcase site
- Screenshot: Holodex media grid/browse view in the 'Broadcast' theme skin — dark background, teal/cyan accent, top nav (HOLODEX logo, global search 'Search everything... (Ctrl-K)', Media/People/Tags links, theme switcher showing Cinémathèque/Broadcast/Brutalist skins with Broadcast active), filter bar (title search, Resolution chips All/SD/HD/FHD/4K, Duration min/max, Year from/to, People and Tags multi-select inputs), and an 18-video responsive card grid with thumbnails (resolution badge, duration), titles, and genre tag pills (e.g. Vantablack/Horror,Thriller; Tin Soldier/Drama,War; The Long Saturday/Comedy; Solar Drift/Adventure,Sci-Fi; Nightshade/Noir,Thriller; Amélie en Hiver/Drama,Romance)
- Screenshot: Holodex media browse grid, Brutalist skin (18 videos, filter panel, resolution/duration/year/people/tags filters, poster cards with duration+genre tags)
- Screenshot: Holodex browse/grid view in the 'Cinémathèque' skin, showing a 18-video poster grid with search, resolution/duration/year filters, and people/tag filters
- testing.T
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
- newHandler
- Jira HOLODEX-166 (System Activity epic)
- Design handoff: Studio image roles — icon / logo / poster (F51)
- Copy → exiftool-write → atomic rename file-safety model
- CSS custom-property design tokens (--bg, --surface, --ink, --accent, --font-display, --radius) per [data-theme]
- codeql.yml — CodeQL static analysis for go + javascript-typescript
- Design Handoff: Metadata Enrichment UI for People (F22)
- GET /api/v1/admin/activity aggregated read-model endpoint
- ProviderClient interface (HTTP default; in-process fake for CI)
- stubThumbs
- ADR-089: Film enrichment field vocabulary — where each provider value lands on a film
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
- HOLODEX-342 · Dev-time stress fixture for UX and layout QA
- Design handoff: In-app promote / override affordance for auto-registered fields (F44)
- Spec: Media parts — denote the files of a multi-file media (`part`)
- holodex_session cookie — HttpOnly, Secure, SameSite=Strict, signed self-contained payload
- refresh service (plan/apply split)
- Media page restructure — one sync verb, render once
- entity_enrichment shadow store (entity_type, entity_id, provider, field_key)
- RefreshReport (sources_disagree flag)
- metadata_curation table (manual source, add/suppress/nowrite)
- writeback_queue table (durable job queue)
- Spec: Film provider enrichment on the film detail page (F59)
- LockedCoreRoles (implicit provenance lock)
- person_images.content_hash column + backfill
- Functional Requirements
- BaselineSource interface
- ResolveFields entity-agnostic merge core
- Spec: Entity identity card — reference handle, unified external ids, films in the spine, edition, display name (F60)
- RelinkVideoStudios reconcile (sole writer, prune-on-empty)
- studios / video_studios / studios_fts data model
- studio_external_ids table (external_id PK, global convergence)
- /describe.field_hints manifest extension
- manifest.mjs
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
- Backfill
- DeriveRelationship(person, video, now) two-entity pass (unbuilt)
- 0024_enrichment_dismissals.up.sql
- Spec — Showcase Demo Corpus
- FieldType / Operator taxonomy (text/categorical/numeric/date)
- Decision
- enrichment_dismissals table (durable rejection verdict)
- file_writeback_snapshots table (batch revert, undo-of-undo)
- filename extraction confidence scoring rubric (tiered, exact-match gate)
- 0025_metadata_extraction_review.up.sql
- Spec: Candidate thumbnail in the resolve picker (F64)
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
- Design handoff: Media detail page reorder
- filmYearServer
- Trash view (/trash) with Restore / Delete permanently
- Derived Age / Age-at-death row, tooltip-only provenance
- Manual QA Checklist: Metadata Enrichment for People (F22)
- EnrichProviderChips Refresh/Re-match/Clear split
- /owner/enrichment review queue tab
- Sink
- /tags pill-native manage mode (rename/alias/merge)
- Manual QA Checklist: Admin Mode (F29)
- CurationChip radio mode (shared shell, dot vs ✕ glyph)
- .uploadVideoPoster
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
- Repo
- ADR-030 (access control gating seam)
- Owner tooling hub (F35) follow-up rename
- Admin mode header toggle (visitor-preview, complete hide set)
- Repo
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
- Design handoff: Completeness panel — collapsible facet fold
- Design handoff: Fire-and-forget writeback
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
- ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field
- web/src/lib/peopleScroll.svelte.ts
- web/src/lib/searchHistory.ts
- evaluate.mjs
- .getFilm
- SourceBadge.svelte
- Load
- 0029_field_claims.up.sql
- ReadbackGaps
- HOLODEX-240.md
- ADR-096: Entity identity card — one reference, one external-id store, films in the spine, edition as a file field, display name as a decision
- 0031_denied_tags.up.sql
- holoShuffle
- SanitizeLinkTemplates
- Decisions
- cascadeServer
- extractReviewServer
- Handoff Spec: Person detail — bio in the header row
- authServer
- HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film)
- Design Handoff: People on the unified source-of-truth model (F37)
- HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields
- Spec: Quick Wins batch — Search history & "More with …" shelves
- Design Handoff: Poster View for the People list page (F55)
- 0039_facet_not_applicable.up.sql
- Configuration Reference (holodex.yaml layers)
- Design Handoff: Provider Link Badge — Multi-Badge States for Person/Studio (HOLODEX-266)
- 0041_provider_link_templates.up.sql
- Session log — append-only (cap: last 8 sessions; older → archive/)
- thumbServer
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Session log — append-only (cap: last 8 sessions; older → archive/)
- reviewServer
- .ReconcileVideoPeopleLocked
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Repo
- net/http.ResponseWriter
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Design Handoff: Studio relationship-edit popover (HOLODEX-271)
- HOLODEX-288 · Fix film-studio cascade code-review findings
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Design handoff: Responsive page width — player column, metadata rail, intrinsic grid density
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Design Handoff: Revealable candidate detail in the Enrich picker (HOLODEX-380 / F61)
- Session log — append-only (cap: last 8 sessions; older → archive/)
- extractServer
- Spec: Owner tooling hub + visitor/owner nav split (F35)
- studio-picker-handoff.md
- tag-link-chip-handoff.md
- Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA
- New
- Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided"
- Spec: Tag Writeback Exclusion — per-tag Genre writeback control
- Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369)
- HOLODEX-102 · Video Credits → People + Headshots (F32)
- HOLODEX-255 · <epic title>
- ADR-085-films-entity.md
- HOLODEX-114 · <epic title>
- HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values
- HOLODEX-275 · GET /api/v1/facets marshals empty values as null, not []
- HOLODEX-244 · <epic title>
- HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip
- HOLODEX-273 · Writeback dialog "Select all undecided" doesn't create a standing decision
- SearchHistory
- Session log — append-only (cap: last 8 sessions; older → archive/)
- filelayer.go
- Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)
- Design handoff: Manage block — destructive split button
- ADR-098: Provider source URL — `_source_url` as the per-pill fallback behind link templates
- New
- httpClient
- Session log — append-only (cap: last 8 sessions; older → archive/)
- people-grid-handoff.md
- Decision
- QA: TMDB Provider Sidecar + ADR-039 Core Changes
- api.test.ts
- postTok
- ADR-092: Flightplan repo extraction — relocate the plugin to its own repository
- Completeness components
- probe-edition-filenames.mjs
- Match
- CurationRow
- coverArtManager
- Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)
- .resolveMovie
- filmEntityServer
- Session log — append-only (cap: last 8 sessions; older → archive/)
- run
- Design Handoff: Extract from filename on the media detail page (F48.5a)
- HOLODEX-298 · Film detail page: match media page's Tags section styling
- HOLODEX-300 · Film bulk-attach dialog: default search term + optional starting scene number
- HOLODEX-302 · Person hero image hover-to-front
- HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise
- HOLODEX-307 · Film detail: poster becomes the header image; remove the Images section
- header-narrow-width-handoff.md
- Session log — append-only (cap: last 8 sessions; older → archive/)
- .scaleToWidth
- HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list
- .partsFor
- NewService
- Session log — append-only (cap: last 8 sessions; older → archive/)
- 0044_alias_source_and_suppressions.up.sql
- identity_review_queue
- identity_review_queue
- fakeRepo
- Spec: Fire-and-forget writeback with page-level status
- media-detail-metadata-fold-handoff.md
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Field
- 3. QA
- loadEnrichPlan
- QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)
- Repo
- Design handoff — `ExpandableText` shared component
- Repo
- HOLODEX-362.md
- HOLODEX-321 · Broken in-page anchors in the provider hand-off specs
- HOLODEX-326 · Film/Media detail pages: no way to edit a scene number after attach
- HOLODEX-320 · Media detail: move, trim, and fold the Metadata section
- TestClassifyCredential
- .runTagWritebackSync
- Spec: Provider link badge coverage — person, studio, film, and media (F63)
- refreshServer
- searchedCaption.ts
- Alias state seed (F58 / ADR-088)
- HOLODEX-355 · `stage-grid`'s single-column branch omits the `minmax(0, …)` guard
- Session log — append-only (cap: last 8 sessions; older → archive/)
- HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`)
- .extractCoverArt
- HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster
- format.ts
- HOLODEX-280 · Film poster/thumbnail asset pipeline
- QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)
- buildEnrichResponse
- HOLODEX-284 · Film provider enrichment (ADR-086)
- HOLODEX-341 · Local dev credentials come from the environment; CI enforces it
- Person
- HOLODEX-358 · a long field value still widens the media page below ~440px
- HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram
- HOLODEX-386 · Refuse a portrait image for the film banner role (landscape guard)
- .ReplaceProviderLinkTemplates
- Spec: Dev-time stress fixture for UX and layout QA
- HOLODEX-370 · Clear / Dismiss never revert a written-back provider — say so, point at Revert
- HOLODEX-380 · F61 — `candidates[].detail`: revealable per-candidate record summary
- Design handoff: StudioLinkCard draws the studio logo
- navSearch.svelte.ts
- urlParamID
- Store
- QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore)
- newSourceURLService
- downloadImageToTemp
- Enrich stub — fake metadata-source providers for manual QA
- Session log — append-only (cap: last 8 sessions; older → archive/)
- 0046_entity_external_ids.up.sql
- Session log — append-only (cap: last 8 sessions; older → archive/)
- resolvedByCanonical
- NewStudioBaseline
- HOLODEX-247 · Studio image roles: icon, logo, poster (F51)
- HOLODEX-413 · Writeback (ffmpeg path) attaches PNG covers as image/jpeg and never replaces cover.jpg
- Decision
- HOLODEX-406 · F64 — `candidates[].image_url`: candidate thumbnail in the resolve picker
- HOLODEX-415 · Replaced person poster stays stale on the Cast grids
- Repo
- ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)
- HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`
- HOLODEX-397 · Studio logo on the Film and Media detail pages
- UI Vocabulary
- Repo
- .providerInfos
- Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)
- QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)
- HOLODEX-283 · Films: real backend search integration
- HOLODEX-396 · Transparent entity images lose their alpha channel on upload
- HOLODEX-407 · F-number claims — `scripts/feature-claims.mjs`
- HOLODEX-411 · Studio logo sits bare on the page background
- HOLODEX-414 · Candidate slot shape per entity kind (media backdrop, studio logo)
- fakeResolver
- Manual QA Checklist: Owner tooling hub + nav split (F35)
- Manual QA Checklist: People Images (F25)
- gateTestHandlers
- applyGenreWriteback
- Handlers
- .mountTagHierarchy
- api/categories.go
- .mountAliases
- .mountEnrich
- .mountExtractionReview
- .mountFilmImages
- .mountPersonImages
- .mountStudioImages
- .mountTagDenylist
- .mountVideoTags
- .mountWriteback
- validEntityType

## God Nodes (most connected - your core abstractions)
1. `newRepo()` - 198 edges
2. `Repo` - 171 edges
3. `itoa()` - 161 edges
4. `sampleVideo()` - 135 edges
5. `writeError()` - 124 edges
6. `New()` - 98 edges
7. `Open()` - 97 edges
8. `writeJSON()` - 91 edges
9. `pathID()` - 87 edges
10. `Handlers` - 76 edges

## Surprising Connections (you probably didn't know these)
- `Holodex landing page (site/index.html)` --semantically_similar_to--> `SvelteKit app.html shell (default data-theme=cinematheque)`  [INFERRED] [semantically similar]
  site/index.html → web/src/app.html
- `TestDownloadImageToTemp_WritesAllowedBytesToTemp()` --calls--> `cleanup()`  [INFERRED]
  internal/writeback/image_fetch_test.go → testdata/aliasseed/main.go
- `runMCPStdio()` --calls--> `NewAuth()`  [EXTRACTED]
  cmd/holodex/main.go → internal/api/auth.go
- `runMCPStdio()` --calls--> `Load()`  [EXTRACTED]
  cmd/holodex/main.go → internal/config/config.go
- `runMCPStdio()` --calls--> `Open()`  [EXTRACTED]
  cmd/holodex/main.go → internal/db/db.go

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

## Communities (711 total, 231 thin omitted)

### Community 0 - "types.ts"
Cohesion: 0.02
Nodes (163): ADR-0006, ADR-0028, ADR-0036, ADR-0056, ADR-0073, ADR-0080, ApiError, buildQuery() (+155 more)

### Community 1 - "media/[id]/+page.svelte"
Cohesion: 0.03
Nodes (36): resolve(), dismissable(), activate(), deactivate(), onClick(), onKey(), DismissableOptions, collisionOpen() (+28 more)

### Community 2 - "whats-left.mjs"
Cohesion: 0.18
Nodes (24): emptyWorklog(), flipGate(), frontmatter(), logSkillRun(), maskComments(), ADR-0064, ADR-0092, parseGates() (+16 more)

### Community 3 - "Decision"
Cohesion: 0.06
Nodes (35): A — basename verbatim (chosen), A — manifest opt-in list `resolve_hints` (chosen), A — residue rule at §4.9 render (chosen), A — resolved values, as-is, ∩ provider vocabulary (chosen), A — `searched[]` on the response, caption + activity detail (chosen), Action Items, ADR-095: Structured resolve hints — `hint.fields`, `hint.filename`, `hint.query_source`, and `searched[]`, B — a stem-vs-tag provenance marker on `title` (+27 more)

### Community 4 - "Repo"
Cohesion: 0.27
Nodes (8): filmStudioCascadeResult, database/sql.NullString, Repo, VideoCollision, idKeyOf(), nameKeyOf(), normalizedNameKey(), compositeKeyCandidate

### Community 5 - "newRepo"
Cohesion: 0.05
Nodes (107): countPeople(), hasVideoTitle(), personIDByName(), TestAliasesSurviveRescan(), TestMergePersons(), TestMergePersons_DedupesSameRoleLinkAtMergeTime(), TestMergePersons_RepointsExternalID(), TestMergePersonsValidation() (+99 more)

### Community 6 - "api/writeback_test.go"
Cohesion: 0.29
Nodes (16): getMediaWritebackStatus(), jobStatusURL(), mediaWritebackURL(), seedFailedWriteback(), syncWritebackServer(), TestDismissWriteback_DeletesRow(), TestEnqueueWriteback_ClearsPriorFailedForVideo(), TestGetMedia_WritebackStatusRedactedForVisitor() (+8 more)

### Community 7 - "ladder_test.go"
Cohesion: 0.12
Nodes (31): TestEnrichRungsSeedTheirNamespaces(), TestEnrichZeroRungStoresNothing(), TestNoOtherDimensionIsEnriched(), addressable(), countRows(), overviewRows(), paletteValue(), seed() (+23 more)

### Community 8 - "Holodex Project Working Agreements (CLAUDE.md)"
Cohesion: 0.06
Nodes (46): Holodex Project Working Agreements (CLAUDE.md), Branch↔Jira linkage, Core resolver model (baseline / enrichment / curation / decisions), Pre-commit checklist, Secrets & publishing rules, Jira task tracking (HOLODEX project), Flightplan Config (flightplan.yaml), Frontend Theming Rules (+38 more)

### Community 9 - "testMappings"
Cohesion: 0.07
Nodes (43): compiledPatterns, fakeEnrichmentCall, fakeEnrichmentWriter, fakeVideoLister, Pattern, patternFile, recordingJobRecorder, recordingReviewStore (+35 more)

### Community 10 - "devDependencies"
Cohesion: 0.15
Nodes (13): devDependencies, playwright, svelte, svelte-check, @sveltejs/adapter-static, @sveltejs/kit, @sveltejs/vite-plugin-svelte, tailwindcss (+5 more)

### Community 11 - "JobRun"
Cohesion: 0.08
Nodes (15): activityResponse, activitySystem, database/sql.Rows, JobRun, Repo, TrashItem, scanTrash(), LibraryCounts (+7 more)

### Community 12 - "imagetools.mjs"
Cohesion: 0.08
Nodes (46): ADR-0035, ADVISORY_TYPES, classify(), COMMENT_MARKER, main(), ADR-0076, NON_DOC_GLOBS, parseCommitType() (+38 more)

### Community 13 - "newProviderIconEnv"
Cohesion: 0.17
Nodes (26): iconEnv, filmImageServer(), filmImageServerDir(), solidJPEG(), TestFilmImage_BannerRequiresLandscape(), TestFilmImage_DeleteClearsProviderRow(), TestFilmImage_InvalidRole(), TestFilmImage_MutationsRequireOwner() (+18 more)

### Community 14 - "NameEditControl.svelte"
Cohesion: 0.23
Nodes (12): busy, cancelEdit(), closeEdit(), commit(), editing, error, focusPencil(), open() (+4 more)

### Community 15 - "Service"
Cohesion: 0.09
Nodes (13): IconRef, ImageSink, Manifest, ProviderClient, SourceInfo, TestValidatePattern(), ValidatePattern(), Service (+5 more)

### Community 16 - "resolver.go"
Cohesion: 0.14
Nodes (34): Source, applyCasing(), baselineValue(), BrowseTitle(), decidedItem(), filmNamespaces(), filmSourceValue(), firstNonEmpty() (+26 more)

### Community 17 - "writeback/writeback.go"
Cohesion: 0.16
Nodes (26): encoding/xml.Name, buildFFmpegArgs(), copyFile(), coverMIME(), existingTagsXML(), ffmpegMetadataKey(), FieldWrite, isNotFound() (+18 more)

### Community 18 - "Handlers"
Cohesion: 0.06
Nodes (18): Health, purger, rescanner, scanStatusSource, searchMetrics, thumbnailer, WriteBatchFunc, sync/atomic.Bool (+10 more)

### Community 19 - "ResolveReviewAction"
Cohesion: 0.26
Nodes (12): ResolvedWrite, ReviewAction, ResolveReviewAction(), TestResolveReviewAction_Filename(), TestResolveReviewAction_FilenameRequiresValue(), TestResolveReviewAction_Manual(), TestResolveReviewAction_ManualMultiValue(), TestResolveReviewAction_ManualRequiresValue() (+4 more)

### Community 20 - "time.Time"
Cohesion: 0.08
Nodes (17): trashItem, github.com/fsnotify/fsnotify.Watcher, os.DirEntry, os.FileInfo, time.Time, ScanStatus, ScanSummary, buildVideo() (+9 more)

### Community 21 - "NewService"
Cohesion: 0.12
Nodes (26): extraPairs(), fileLayerChanged(), Report, Service, SourceResult, NewService(), personNames(), refreshDetail() (+18 more)

### Community 22 - "toAnySlice"
Cohesion: 0.10
Nodes (9): Studio, matchesDisplayQuery(), T, namedCountQuery(), placeholders(), toAnySlice(), Repo, Repo (+1 more)

### Community 23 - "Design handoff: StudioLinkCard (reusable Studio display)"
Cohesion: 0.18
Nodes (11): 1. Resolved decisions (open questions from the rough mockup), 2. New component: `StudioLinkCard.svelte`, 3. Call-site changes, 4. Backend requirement (blocking), 5. Design tokens used, 6. States and interactions, 7. Responsive behavior, 8. Edge cases (+3 more)

### Community 24 - "Spec: Page-scoped hotkeys (F62)"
Cohesion: 0.06
Nodes (29): Accessibility, Content, Decision, Design handoff: `?` keyboard-shortcuts sheet + hotkey firing feedback (F62), Edge cases, Layout, Motion, States and interactions (+21 more)

### Community 25 - "enrich/enrich_test.go"
Cohesion: 0.16
Nodes (28): TestServiceRecordSearched_AppliedDetail(), TestServiceResolveGatesImageURL(), Service, newSvc(), TestDownloadAssetsFirstSuccessPerRole(), TestEnrichAssetFailureIsNonFatal(), TestEnrichDownloadsAssets(), TestEnrichDownloadsFilmAssets() (+20 more)

### Community 26 - "Design Handoff: Unified name-edit mechanism (HOLODEX-269)"
Cohesion: 0.05
Nodes (35): Accessibility Notes, Animation / Motion, Component contract (resolves the spec's open question), Components, Cross-context notes, Design Handoff: Unified name-edit mechanism (HOLODEX-269), Design Tokens Used, Edge Cases (+27 more)

### Community 27 - "tmdb.go"
Cohesion: 0.09
Nodes (38): buildMovieEnrichResponse(), buildPeopleCredits(), fieldHint, headshotFor(), movieAliases(), movieDisambiguate(), movieYear(), parseReleaseFilename() (+30 more)

### Community 28 - "Auth"
Cohesion: 0.11
Nodes (11): sessionClaims, POST /api/v1/session (token exchange) + DELETE /api/v1/session (sign-out), net/http.Cookie, deriveSessionSecret(), Auth, Handlers, parseSessionClaims(), ttlForClass() (+3 more)

### Community 29 - "QA: Metadata Writeback (F28)"
Cohesion: 0.08
Nodes (20): §1 Setup, §2 Smoke (run in `make test` / `npm run test`), §3 Agent live QA (preview tools against §1 stack), §4 Human (3-skin eyeball — Cinémathèque, Broadcast, Brutalist), QA Checklist: People on the unified source-of-truth model (F37), 2026-09-17 · debug → repro → fix, Gates — definition of done, HOLODEX-408 · Embedded person tags link on first import (+ HOLODEX-409) (+12 more)

### Community 30 - "handler"
Cohesion: 0.29
Nodes (6): io.ReadCloser, decode(), isSupportedEntity(), writeJSON(), enrichRequest, handler

### Community 31 - "Spec: Derived/calculated person fields (F45)"
Cohesion: 0.10
Nodes (20): ADR-065: Typed field registry and relationship-scoped computed fields (Superseded), 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — all three skins, QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173), §1 Setup, §2 Smoke (run in `make test` / `npm run test`), §3 Agent live QA (preview tools against §1 stack) (+12 more)

### Community 32 - "ADR-090-two-layer-entity-metadata-management.md"
Cohesion: 0.08
Nodes (20): UI vocabulary: translate, link, record, ADR-093: A field's sync state is unknown, not false, when nothing reads the written tag back, Consequences, Context, Decision, 1. Setup, 2. Smoke, 3. Agent (+12 more)

### Community 33 - "mcp.go"
Cohesion: 0.06
Nodes (47): RefKindError, github.com/mark3labs/mcp-go/mcp.CallToolRequest, github.com/mark3labs/mcp-go/mcp.CallToolResult, github.com/mark3labs/mcp-go/server.MCPServer, ParseRef(), TestParseRef(), filterNamed(), T (+39 more)

### Community 34 - "Spec: Tag governance & video enrichment (F50)"
Cohesion: 0.07
Nodes (27): F50: Tag governance & video enrichment, Suppression derives from merged []mapping.Field, not the claims table, ADR-075: Tag governance & video enrichment, denied_tags global term deny-list table, Write-on-resolve tag materialization via afterEnrichApply, tags.parent_tag_id strict-tree hierarchy, video_tags.source column; partial-replace rescan, Design Handoff: Tag Governance & Video Enrichment (F50) (+19 more)

### Community 35 - "extractor.go"
Cohesion: 0.09
Nodes (26): canonicalKey(), dedupe(), Extracted, isBinaryValue(), leadingOrdinal(), mapExiftool(), mapFfprobe(), newKeySet() (+18 more)

### Community 36 - "Resolve"
Cohesion: 0.14
Nodes (32): TestResolve_NoBaselineSource_UndecidedStaysInSync(), imageField(), mergeImageField(), TestResolveFields_ImageGate(), Resolve(), mergeField(), stubField(), TestBrowseTitle_FallbackToFileTitle() (+24 more)

### Community 37 - "service.go"
Cohesion: 0.09
Nodes (24): refreshAllResult, assetFetcher, EnrichRepo, TestSanitizeImageURL(), TestSanitizeCandidatesAutoApply(), TestSanitizeFieldsCaps(), TestSanitizePeopleRejectsWhitespaceInExternalID(), TestSanitizeProfileURL() (+16 more)

### Community 38 - "activity.svelte.ts"
Cohesion: 0.06
Nodes (24): web/src/lib/browse.svelte.ts — module-scoped browse-state cache, web/src/routes/+page.svelte — the browse grid, web/src/lib/theme.svelte.ts — established module-scoped singleton pattern, Client-side seeded shuffle for unpaged People/Tags lists (mulberry32 PRNG), vitest, activity, ActivityState, ADR-0030 (+16 more)

### Community 39 - "people/+page.svelte"
Cohesion: 0.04
Nodes (45): browseCache, BrowseSnapshot, ADR-0032, onKey(), Sort components, segmentedToggleWrapperClass, listScroll, ListScrollSnapshot (+37 more)

### Community 40 - "jira-sync.mjs"
Cohesion: 0.12
Nodes (23): log, main(), missing, ADR-0058, ADR-0069, bailSoft(), log, main() (+15 more)

### Community 41 - "claim_test.go"
Cohesion: 0.36
Nodes (11): claim, inspect(), markOwned(), tableNames(), newDatabase(), TestInspect_ContentRowsAreForeign(), TestInspect_EmptyDatabaseIsClaimable(), TestInspect_MarkerClaimsTheDatabase() (+3 more)

### Community 42 - "people"
Cohesion: 0.12
Nodes (10): people, person_aliases, person_aliases_fts, person_images, person_image_suppressions, person_aliases, person_aliases_fts, video_people_old (+2 more)

### Community 43 - "People attach/detach + relationship picker (F56.5, HOLODEX-272)"
Cohesion: 0.05
Nodes (35): Accessibility Notes, Animation / Motion, Components, Design Handoff: People attach/detach + relationship picker (HOLODEX-272), Design Tokens Used, Edge Cases, Layout, Overview (+27 more)

### Community 44 - "New"
Cohesion: 0.08
Nodes (67): net/http.Handler, NewAuth(), facetMap(), TestGetMedia_Completeness(), TestGetPerson_Completeness(), TestGetStudio_Completeness(), putDecisionRaw(), rawRequest() (+59 more)

### Community 45 - "density.svelte.ts"
Cohesion: 0.11
Nodes (15): Stage cap vs. window width — decide it once, Video components, capForWidth(), clamp(), DENSITY_MAX, DENSITY_MIN, effectiveDensity(), invertDensity() (+7 more)

### Community 46 - "Queue"
Cohesion: 0.14
Nodes (8): fakeEnqueuer, WritebackJob, detailLine(), JobField, Queue, BatchJob, PostWriteFunc, WriteFunc

### Community 47 - "ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam"
Cohesion: 0.13
Nodes (15): Action Items, ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam, Consequences, Context, Current state (survey, 2026-07-31), D1 — `tags.writeback_enabled` column; filtered at `TagNamesForVideo`, uniformly per name regardless of how it was reached, D1 — where the flag is enforced, D2 — Manual sync batch-enqueues per-video via `genreWritebackValuesForVideo`, not a precomputed name list; shared `batchID` across single- and bulk-tag triggers (+7 more)

### Community 48 - "images_test.go"
Cohesion: 0.11
Nodes (29): image/color.NRGBA, image.NRGBA, HasThumbnailImage(), aspect, imageSlot, imageTargets, imageVariant, imageWriter (+21 more)

### Community 49 - "generate.mjs"
Cohesion: 0.07
Nodes (31): ADR-0017, sharp, buildItem(), ensureFfmpeg(), here, hms(), main(), ADR-0004 (+23 more)

### Community 50 - "Flightplan — portable session-state plugin"
Cohesion: 0.10
Nodes (21): ADR-052: BaselineSource contract, 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist), Manual QA Checklist: Per-field source-of-truth decisions (F36), ADR-058 (Jira transitions via REST API) — cited as evidence, Flightplan — portable session-state plugin (+13 more)

### Community 51 - "Video composite-key collision check (F56.3, HOLODEX-270)"
Cohesion: 0.06
Nodes (32): A required precondition: generalize `NameEditControl`'s conflict type, Accessibility, `CollisionOfferCard.svelte`, Design Handoff: Video composite-key collision verdict (HOLODEX-270), Edge Cases, Layout, New type: `VideoCollisionRef`, Overview (+24 more)

### Community 52 - "theme.svelte.ts"
Cohesion: 0.11
Nodes (14): ADR-0021, Writeback components, CORE_ROLE_ASPECT, cropAffine, CropInput, cropTargetSize(), index(), isTheme() (+6 more)

### Community 53 - "run.mjs"
Cohesion: 0.11
Nodes (23): playwright, goto(), launch(), matrix(), ADR-0030, ADR-0095, open(), PREPARATIONS (+15 more)

### Community 54 - "studios"
Cohesion: 0.14
Nodes (9): studios, studios_fts, video_studios, studio_external_ids, studio_logos, studio_logos, studio_images, person_external_ids (+1 more)

### Community 55 - "ResolveFields"
Cohesion: 0.38
Nodes (11): NewPersonBaseline(), personTestFields(), TestPersonBaseline_ClaimsNamespaceWithEmptyValue(), TestPersonBaseline_ManualPinStaysFrozen(), TestPersonBaseline_NameResolvesFromRecord(), TestPersonBaseline_NilPersonIsEmptyBaseline(), TestPersonBaseline_ProviderPinFollowsReEnrich(), TestPersonBaseline_RD6Additivity() (+3 more)

### Community 56 - "count"
Cohesion: 0.17
Nodes (17): migrate.Migrate, seedAliasCollapse(), TestMigration0044Down(), TestMigration0044PromotesAliasCuration(), TestMigration0044SuppressionsDieWithTheirEntity(), TestMigration0045ReviewQueueDetail(), TestMigration0031DeniedTagsUpAndDown(), TestMigration0046FoldsExternalIDsUpAndDown() (+9 more)

### Community 57 - "Handlers"
Cohesion: 0.26
Nodes (4): net/http.HandlerFunc, Handlers, chi.Router, Handlers

### Community 58 - "tmdb_test.go"
Cohesion: 0.25
Nodes (19): clientWith(), fakeTMDB(), TestTMDBEnrich(), TestTMDBEnrichMovie(), TestTMDBEnrichStudio(), TestTMDBEnrichStudioNoWebsite(), TestTMDBEnrichUnknownID(), TestTMDBResolveByIMDBID() (+11 more)

### Community 59 - "getJSON"
Cohesion: 0.17
Nodes (27): externalLinksEnv, fakeRescanner, linksByProvider(), newExternalLinksEnv(), seedSourceURLPerson(), TestExternalLinks_EnrichmentDisabled(), TestExternalLinks_Film(), TestExternalLinks_MalformedIDSkipped() (+19 more)

### Community 60 - "Manager"
Cohesion: 0.20
Nodes (5): sync/atomic.Int64, Manager, New(), Config, Repository

### Community 61 - "SanitizeValue"
Cohesion: 0.06
Nodes (21): curationBody, decisionBody, Handlers, chi.Router, validateCurationBody(), validCurationAction(), decodeDecisionBody(), Handlers (+13 more)

### Community 62 - "review_queue_test.go"
Cohesion: 0.12
Nodes (26): tagIDByName(), TestEntityConflictExcludesSelf(), TestKeepSeparateStore(), TestMergeEntitiesValidation(), TestMergeEntitiesWithAffectedVideos_UnknownEntityType(), TestRenameStudioKeepsOldNameAsAlias(), TestRenameTagInternalWhitespaceConflict(), TestStudioAliasCRUD() (+18 more)

### Community 63 - "Mappings"
Cohesion: 0.10
Nodes (20): Cache, New(), Dedupe(), Empty(), Mappings, Store, Load(), parse() (+12 more)

### Community 64 - "extraction.ts"
Cohesion: 0.08
Nodes (28): partBadgeLabel(), buildPreviewItems(), FIELD_LABEL_ALIASES, FIELD_ORDER, fieldRank(), groupByVideo(), isEntityField(), makeFieldLabel() (+20 more)

### Community 65 - "f36.ts"
Cohesion: 0.09
Nodes (33): autoResize(), busy, enqueueError, ensureDecision(), onKeydown(), submit(), trapTab(), baselineCandidateValue() (+25 more)

### Community 67 - "Process"
Cohesion: 0.18
Nodes (15): ExtractionReviewCall, fakeManualSource, fakeReviewStore, Process(), TestProcess_EntityField_ExactMatchAutoApplies(), TestProcess_EntityField_FuzzyMatchQueuesWithSuggestion(), TestProcess_EntityField_NoMatchQueuesWithoutSuggestion(), TestProcess_LogOnly_WhenFlagDisabled() (+7 more)

### Community 68 - "Design Handoff: Unified nav search — live, tabbed, in-place filtering panel"
Cohesion: 0.06
Nodes (33): Accessibility notes (summary), Design Handoff: Unified nav search — live, tabbed, in-place filtering panel, Design-system fit, Mobile (< 640px, the primary complaint driving this spec), Overview, Part A — The tab row lives with the box, not inside the dropdown, Part B — `SearchResultsPanel.svelte` (NS1), Part C — Per-page removal (NS4) (+25 more)

### Community 69 - "seedPerson"
Cohesion: 0.08
Nodes (56): newRepoDB(), TestDisplayNames(), filmAliases(), filmPairs(), mustCreateFilm(), TestApplyProviderAliases_Film(), TestFilmAliasRoutingByYear(), TestFilmAmbiguousNoYearQueues() (+48 more)

### Community 70 - "Derive"
Cohesion: 0.14
Nodes (17): ForComputed(), ForNamespace(), ForProvider(), TestForNamespace(), TestProviderRoundTrip(), TestValid(), Repo, dependencyLabels() (+9 more)

### Community 71 - "videos"
Cohesion: 0.14
Nodes (14): people_fts, tags, tags_fts, video_metadata, video_people, video_tags, videos, videos_fts (+6 more)

### Community 72 - "Studio relationship-edit popover (F56.4, HOLODEX-271)"
Cohesion: 0.25
Nodes (8): Existing State (grounded in code, this session), Goals, Non-Goals, Open Questions, Problem Statement, Studio relationship-edit popover (F56.4, HOLODEX-271), Success Metrics, User Stories

### Community 75 - "fakeStudioRepo"
Cohesion: 0.25
Nodes (4): fakeStudioRepo, ValidStudioImageRole(), StudioImage, StudioImageInsert

### Community 76 - "ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action"
Cohesion: 0.12
Nodes (16): Action Items, ADR-087: Film-studio cascade — decide-then-enqueue across N videos in one owner action, Consequences, Context, Current state (survey, 2026-08-25), D1 — Extract the single-video Studio-decide logic into a shared helper; the cascade calls it once per attached video, D1 — where the per-video Studio-decide logic lives, D2 — `CascadeFilmStudio`: per-video decide (best-effort), then one shared-batch enqueue for every video that succeeded (+8 more)

### Community 77 - "Spec: Sticky sort preferences + Random sort"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, Requirements (+8 more)

### Community 78 - "ResolveForContainer"
Cohesion: 0.18
Nodes (11): NewExtractor(), TestEditionRoundTrip_BothContainers(), TestPartRoundTrip_BothContainers(), Mapped, ImageTagForField(), ResolveForContainer(), TagForField(), TestImageTagForField() (+3 more)

### Community 79 - "enrich/enrich.go"
Cohesion: 0.09
Nodes (26): Candidate, FieldHint, fileConfig, Registry, Store, Empty(), entityTypesSupport(), ResolveResult (+18 more)

### Community 80 - "report.mjs"
Cohesion: 0.21
Nodes (13): ASSERTIONS, ADR-0095, stressedPicker, validate(), failed(), METRICS, exitCode(), ICON (+5 more)

### Community 82 - ".add"
Cohesion: 0.44
Nodes (8): Manager, newFakeRepo(), TestDisabledManagerNoops(), TestExtractEmbedded(), testManager(), TestProcessGeneratesAndMarks(), TestProcessMarksFailed(), TestRunDrainsBackfill()

### Community 83 - "Spec: Tag Detail — Hierarchy & Category Controls"
Cohesion: 0.06
Nodes (29): 1. Decision logic (when the dialog appears), 2. The confirm dialog, 3. States and interactions, 4. Edge cases, 5. Accessibility, 6. Visual reference, Design Handoff: Reparent-confirm flow for the Children control (HOLODEX-259), Design-system-fit audit (+21 more)

### Community 84 - "process.go"
Cohesion: 0.13
Nodes (17): Decision, cachingResolver, Deps, Enqueuer, FieldExtraction, FieldOutcome, ManualSourceChecker, Outcome (+9 more)

### Community 85 - "database/sql.DB"
Cohesion: 0.26
Nodes (15): filmVideoRow, database/sql.DB, database/sql.NullInt64, readFilmVideos(), seedFilmVideo(), TestFilmVideosSurviveFullRelinkCycle(), deleteAliasByName(), ensurePerson() (+7 more)

### Community 86 - "Design handoff: PeopleGrid (reusable People/Cast display)"
Cohesion: 0.17
Nodes (12): 10. Verification (as-built), 1. Resolved decisions, 2. New component: `PeopleGrid.svelte`, 3. Call-site changes, 4. Backend requirement, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior (+4 more)

### Community 87 - "getJSONTok"
Cohesion: 0.10
Nodes (38): partsFixture, reqTokBody(), TestCategoryEndpoints(), TestResolveOrCreateTagEndpoint(), completenessBrowseServer(), mediaTitles(), TestCompletenessFacets(), TestListMedia_CompletenessSort_Orders() (+30 more)

### Community 88 - "routes/tags/+page.svelte"
Cohesion: 0.06
Nodes (34): PopoverMenu, PopoverMenuOptions, createAndAssign(), focusOption(), onKey(), onOptionKey(), optionCount, pickAt() (+26 more)

### Community 89 - "Normalize"
Cohesion: 0.15
Nodes (26): Normalize(), forgePNGDims(), jpegBytes(), pngBytes(), TestGenderBucket(), TestNormalizeAcceptsWebP(), TestNormalizeDownscales(), TestNormalizeJPEGFlattensTransparency() (+18 more)

### Community 91 - "QA Checklist: System Activity (F21)"
Cohesion: 0.17
Nodes (11): F21: System Activity — Under the Hood, Accessibility, Controls (owner), Gating (F21.7) — needs `ADMIN_TOKEN` to exercise, Header activity indicator, Job history, QA Checklist: System Activity (F21), Reachability & shell (+3 more)

### Community 93 - "Decision"
Cohesion: 0.14
Nodes (14): ADR-023: Image Distribution — Published GHCR Image + Pull-Based Compose, Consequences, Context, Decision, Tagging, ci.yml — PR/push gate (go vet, go test, svelte-check, Vitest, vite build, theming grep guard), image.yml — reusable multi-arch build/push + Trivy scan, release.yml — tag v* triggers image.yml then cuts a GitHub Release (+6 more)

### Community 94 - "entityKind"
Cohesion: 0.13
Nodes (36): dimension, entityKind, fixtureFields, links, rung, spec, textVariant, TestEncodedNamesDistinguishRungsWithinADimension() (+28 more)

### Community 95 - "QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)"
Cohesion: 0.40
Nodes (4): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human look, QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)

### Community 96 - "jarowinkler.go"
Cohesion: 0.21
Nodes (14): BestFuzzyMatch(), classifyAgreement(), classifySpecificity(), commonPrefixLen(), isDigits(), jaro(), JaroWinkler(), approxEqual() (+6 more)

### Community 97 - "Design handoff: Films entity (F56)"
Cohesion: 0.06
Nodes (32): 1. `/films` — list, §1 Setup, 2. `/films/{id}` — detail, §2 Smoke, 2a. Header, 2b. Full-film file section (RD4, P0-10), 2c. Scenes list (RD4), 2d. Film → video attach entry point (+24 more)

### Community 98 - "Fake"
Cohesion: 0.22
Nodes (9): resolveCounter, Asset, EnrichResult, FakePerson, ProviderPerson, Hint, Fake, idNamespace() (+1 more)

### Community 99 - "Repo"
Cohesion: 0.10
Nodes (16): filmVideoCandidate, Handlers, Film, filmSceneOccupant(), FilmAttachment, FilmSceneCollision, FilmYearCollision, Repo (+8 more)

### Community 100 - "decide"
Cohesion: 0.27
Nodes (15): decide(), providerCandidate(), TestResolve_CandidatesListFileAndMatchedProviders(), TestResolve_DecisionAdoptProvider(), TestResolve_DecisionKeepFileOverridesMappingOrder(), TestResolve_DecisionManualLiteral(), TestResolve_DeclaredBaseline_StaysKnowable(), TestResolve_Edition_TagBeatsFilename() (+7 more)

### Community 101 - ".addEntityAlias"
Cohesion: 0.26
Nodes (6): identityRoutes, Handlers, chi.Router, T, mergeBatchID(), namesByVideo()

### Community 102 - "Design handoff: Media detail — Films + People sections"
Cohesion: 0.13
Nodes (15): 10. Scope, 1. Overview, 1a. What's wrong, 1b. The rule, 2. State matrix, 3. Design tokens, 4. Layout, 4a. Tile sizing decision (+7 more)

### Community 104 - "Spec: Tag & Category Create Affordance — closing the /tags creation gap"
Cohesion: 0.15
Nodes (13): Goals, Implementation note (2026-08-01), Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement (+5 more)

### Community 105 - "BatchRunner"
Cohesion: 0.39
Nodes (3): JobRecorder, VideoLister, BatchRunner

### Community 106 - "Spec: Tag Categories — grouping tags without merging them"
Cohesion: 0.17
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 107 - "ADR-080: Configurable per-provider metadata search query patterns"
Cohesion: 0.08
Nodes (25): A — core renders a string, `/resolve` contract unchanged (chosen), A — embed in the existing entity payload (chosen), A — operator > provider > global default > raw title (chosen), A — strip bracket punctuation + resolution tokens, collapse whitespace (chosen), Action Items, ADR-080: Configurable per-provider metadata search query patterns, B — leave the floor tier literal; rely on operator-configured patterns to work around messy titles, B — new endpoint, picker fetches on open (+17 more)

### Community 108 - "FilmStudioCascadeDialog.svelte"
Cohesion: 0.13
Nodes (18): active, batchId, collisions, commit(), enqueued, errors, focusOption(), onInput() (+10 more)

### Community 109 - "Design handoff: TagLinkChip (reusable Tag display)"
Cohesion: 0.18
Nodes (11): 1. Resolved decisions, 2. New component: `TagLinkChip.svelte`, 3. Call-site changes, 4. Backend requirement, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior, 8. Edge cases (+3 more)

### Community 110 - "itoa"
Cohesion: 0.09
Nodes (64): peopleDecisionServer(), peopleDecisionServerWithFields(), TestCurationAPI_NonPersonFieldSkipsCollisionGate(), TestCurationAPI_PeopleCollision(), TestCurationAPI_PeopleCollision_Suppress(), TestCurationAPI_PersonFieldNotMapped(), actorsAndDirectorServer(), postCurationNoFatal() (+56 more)

### Community 111 - "repo/related_test.go"
Cohesion: 0.16
Nodes (18): mustVideoID(), TestCategoriesForTag(), TestCategoryCrossTableCollision(), TestCategoryCRUD(), TestCategoryTagAssignment(), TestCategoryVideoFilterFacet(), TestListCategoriesTagFields(), TestResolveOrCreateTag() (+10 more)

### Community 112 - "tmdbClient"
Cohesion: 0.32
Nodes (4): net/url.Values, splitID(), enrichResponse, tmdbClient

### Community 114 - "ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write"
Cohesion: 0.08
Nodes (22): Action Items, ADR-084: Locked curation-relink commit — extending `SetCurationChecked`'s `writeMu` to cover the People relink write, Consequences, Context, Decision, Non-goals, Option A: Re-resolve from source inside `check()` — rejected, Option B: Extend the lock to cover the relink write — chosen (+14 more)

### Community 115 - "Decision"
Cohesion: 0.12
Nodes (16): 1. Type source: PR title, not individual commits, 2. Scope signal: changed-file globs, with a threshold, 3. Advisory, not blocking, 4. New workflow, not a job in `jira-sync.yml`, 5. Script shape, 6. Allowlist, not the `docs/**`-denylist `jira-sync.yml` uses, Action Items, ADR-076: Advisory CI check — `docs`/`chore`-typed PRs that touch non-doc code (+8 more)

### Community 116 - "seedCountInto"
Cohesion: 0.28
Nodes (12): countBulkRows(), TestAssertPooledBoundsBothEnds(), TestBreadthDoesNotMoveAddressedEntities(), TestBreadthIsReproducible(), TestBreadthPopulatesEveryKind(), TestBreadthStaysOutOfAddressedBlocks(), TestBulkCategoriesCarryATag(), TestBulkFilmsCarryAScene() (+4 more)

### Community 118 - "compilerOptions"
Cohesion: 0.15
Nodes (12): ./.svelte-kit/tsconfig.json, compilerOptions, allowJs, checkJs, esModuleInterop, forceConsistentCasingInFileNames, moduleResolution, resolveJsonModule (+4 more)

### Community 119 - "Design Handoff: Person Aliases ("Also known as") (F23)"
Cohesion: 0.10
Nodes (20): Accessibility notes, Animation / motion, Chip treatment (decisive), Collision prompt (person page, inline), Components, Design Handoff: Person Aliases ("Also known as") (F23), Design-system fit (the `/design-system` check), Design tokens used (+12 more)

### Community 120 - "Complete"
Cohesion: 0.12
Nodes (28): FacetSummary, PersonCompleteness, StudioCompleteness, VideoCompleteness, E, T, isMissingAll(), sortByScore() (+20 more)

### Community 121 - "0043_films.up.sql"
Cohesion: 0.53
Nodes (5): film_images, film_people_roles, film_videos, films, films_fts

### Community 122 - "seedTagTree"
Cohesion: 0.20
Nodes (20): assertTagParent(), ptr(), seedTagTree(), TestAncestorNamesForTag(), TestChildrenForTag(), TestListTagsWritebackEnabled(), TestListVideos_TagFilterIsDescendantInclusive(), TestMergeReparentsChildren() (+12 more)

### Community 123 - "Design handoff: Film provider enrichment (F59)"
Cohesion: 0.07
Nodes (29): 1. Details section — provider chips, §1 Setup, 2. Header — banner behind the poster row, §2 Smoke — automated (green in CI), 2a. Resolved: the banner sits behind an otherwise-unchanged header (spec Q1), 2b. Band geometry, 2c. Empty states (spec Q2) — as built, 2d. Owner controls (+21 more)

### Community 124 - "Requirements"
Cohesion: 0.50
Nodes (4): Future Considerations (P2), Must-Have (P0), Nice-to-Have (P1), Requirements

### Community 125 - "query.go"
Cohesion: 0.29
Nodes (11): queryToken, QueryFields, Source, parseQueryPattern(), renderPattern(), sanitizeTitle(), TestSanitizeTitle(), TestSourceBuildQuery_YearParsing() (+3 more)

### Community 126 - "Design handoff: Media detail stage layout"
Cohesion: 0.10
Nodes (20): 1. File joins the enrichment group, 2. More-with shelves break the width cap, 3. Visitor rail regains read-only field values, and the overview moves into it, 3a. Read-only values for visitors, 3b. The overview moves to the rail — unconditionally, 3c. Divergence from the film detail page — resolved in HOLODEX-364, 4. Measured tracks, 5. Why this no longer needs an ADR (+12 more)

### Community 127 - "Route"
Cohesion: 0.31
Nodes (9): Decision, Route(), TestRoute_BelowThreshold_RoutesToReview(), TestRoute_ExactMatchGate_AutoApplies(), TestRoute_FuzzyMatchNeverAutoApplies(), TestRoute_ManualOverrideAlwaysWins(), TestRoute_NonEntityField_AutoAppliesWithoutEntityMatch(), TestRoute_TierThresholds() (+1 more)

### Community 128 - "Store"
Cohesion: 0.46
Nodes (6): EnrichmentWriter, Store(), newRepo(), TestFilenameSourceResolvesWithNoResolverChange(), TestStore_EmptyFieldsIsNoop(), TestStore_RoundTripsThroughEntityEnrichment()

### Community 129 - "model.go"
Cohesion: 0.11
Nodes (11): CorePersonImageRole(), Category, EntityRef, PersonImageSet, Tag, ValidPersonImageRole(), Repo, Repo (+3 more)

### Community 130 - "Design handoff: Media detail Metadata — move, trim, fold"
Cohesion: 0.17
Nodes (12): 1. Final order (top to bottom), 2. Fields removed from the Metadata list, 3. The fold, 4. Anchors and deep links, 5. Accessibility, 6. Theming, 7. Verification, Design handoff: Media detail Metadata — move, trim, fold (+4 more)

### Community 131 - "Spec: Entity Completeness Score (F55)"
Cohesion: 0.08
Nodes (24): Access control & security, Artifacts to produce (project working agreements), Data, storage & serving (direction — finalized in the ADR), Excluded fields, Facet tables per entity type, Facet weight and source tier, Frontend / theming requirements, Functional requirements (+16 more)

### Community 132 - "nationality.ts"
Cohesion: 0.07
Nodes (27): Derivation (see the spec for detail), Design Handoff: Nationality flag beside the person name (HOLODEX-139), Placement & measurements, States, Theming notes (what bites these surfaces), 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — all three skins (+19 more)

### Community 133 - "routes/+layout.svelte"
Cohesion: 0.06
Nodes (33): fire(), guardKeydown(), hotkey(), apply(), HotkeyEntry, HotkeyParam, HotkeyRegistry, hotkeys (+25 more)

### Community 134 - "assetHostAllowed"
Cohesion: 0.17
Nodes (11): AssetClient, passthroughFetcher, net/http.Client, net/url.URL, assetHostAllowed(), assetRoleFor(), Source, newAssetClient() (+3 more)

### Community 135 - "newCompletenessHandlers"
Cohesion: 0.18
Nodes (21): FacetGroup, QueueRow, Handlers, sortFacetGroups(), sortRowsByName(), facetGroupByCanonical(), TestRemediationQueue_ActionableSplit(), TestRemediationQueue_GroupsByFacet() (+13 more)

### Community 136 - "Spec: Holodex Metadata Provider Contract (hand-off, protocol v1)"
Cohesion: 0.04
Nodes (52): §1 Setup, §2 Smoke — `[smoke]`, §3 Agent — live, all three skins — `[agent]`, §4 Human — `[human]`, QA Checklist: Revealable candidate detail in the Enrich picker (HOLODEX-380), Content spec, Design Handoff: Candidate thumbnail in the Enrich picker (HOLODEX-406 / F64), Edge cases (+44 more)

### Community 137 - "Decision"
Cohesion: 0.09
Nodes (23): 1 — Data model (migration 0043), 2 — Asserted-link invariant: zero relink participation (realizes spec RD1/P0-2), 3 — `filmBaseline`: the entity whose baseline is other entities (realizes spec RD2), 4 — Film resolver source: a dynamically-namespaced `provider:<name>` (resolves Q1), 5 — `films_enabled` suspend mechanism: reuse the existing "decided source currently unmatched" path (resolves Q2), 6 — `films_enabled` gate wiring, 7 — API surface (owner-gated mutations only; reads public when enabled), A — Caller-injected synthetic namespace into the existing `Enrichment` map, one new `resolveDecided` branch (chosen) (+15 more)

### Community 138 - "context.Context"
Cohesion: 0.03
Nodes (27): Noop, recordingSink, storedAsset, countingResolver, context.Context, fakePersonRepo, Handlers, Handlers (+19 more)

### Community 139 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.33
Nodes (6): 2026-07-10 · what happened this session, 2026-08-29 · session (1), 2026-08-29 · session (2), 2026-08-29 · session (3), 2026-08-29 · session (4), Session log — append-only (cap: last 8 sessions; older → archive/)

### Community 140 - "web/package.json"
Cohesion: 0.07
Nodes (26): ADR-0025, flag-icons, @fontsource/share-tech-mono, @fontsource-variable/archivo, @fontsource-variable/fraunces, @fontsource-variable/spline-sans-mono, @fontsource/vt323, svelte (+18 more)

### Community 141 - "Spec: Owner-authored person & studio ↔ media links, with file writeback (F40)"
Cohesion: 0.10
Nodes (20): API, Before implementation, Behavior detail, Future considerations (P2), Goals, Link derivation (RD2/RD3), Must-have (P0), Non-Goals (+12 more)

### Community 142 - "Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)"
Cohesion: 0.09
Nodes (22): 10. QA gate, 1. The remediation queue, 2. The completeness breakdown panel, 3. Components, 4. Tokens, 5. States, 6. Accessibility, 7. Edge cases (+14 more)

### Community 143 - "manifest.go"
Cohesion: 0.26
Nodes (15): axes, entry, filmAxes, imageAxes, imageSlotAxes, manifest, manifestDimension, nameAxes (+7 more)

### Community 144 - "generate"
Cohesion: 0.39
Nodes (13): breadthPool, idRange, assertPooled(), breadthCeiling(), bulkName(), newBreadthPool(), recordBulkDerived(), seedBreadthCategories() (+5 more)

### Community 145 - "MergePersons(canonical, merged) transaction"
Cohesion: 0.67
Nodes (3): MergePersons(canonical, merged) transaction, videos.deleted_at TEXT NULL column (migration 0010), person_image_suppressions table (person_id, source_url) + person_images.source_url (migration 0012)

### Community 147 - "queue"
Cohesion: 0.21
Nodes (6): newQueue(), drain(), TestQueueDedupAndDepth(), TestQueueDedupWhileInFlight(), TestQueueHighPriorityFirst(), queue

### Community 148 - "wireSvc"
Cohesion: 0.16
Nodes (16): TestSanitizeDetail(), TestServiceResolve_DetailIngest(), gateHint(), Source, intersectSearchFields(), sanitizeDetail(), sanitizeSearched(), boolPtr() (+8 more)

### Community 149 - "ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio"
Cohesion: 0.10
Nodes (20): A — One badge per stored external-id row (chosen), A — Provider-declared `link_templates`, resolved server-side (chosen), A — Read-only projection of the existing identity tables (chosen), Action Items, ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio, B — Frontend-hardcoded per-namespace URL map, B — Pick a single "primary" badge (first-inserted, or a namespace priority order), B — Promote to a resolver-backed registry facet (widen F55's Person/Studio tables) (+12 more)

### Community 150 - "confidence.go"
Cohesion: 0.20
Nodes (17): Agreement, EntityMatch, Specificity, Tier, AutoApplyThreshold(), IsEntityField(), IsMultiValueField(), scoreAgreement() (+9 more)

### Community 151 - "repo/studios_test.go"
Cohesion: 0.19
Nodes (14): TestFilmStudios_IncludesIconAndCount(), studioIDByName(), TestStudioMergeSurvivesRederivation(), studioByName(), studioNames(), TestGetStudio_NotFound(), TestListStudios_AttachesImageVersions(), TestReconcileVideoStudios_CreateReplacePrune() (+6 more)

### Community 152 - "Design handoff — Entity identity card (F60)"
Cohesion: 0.06
Nodes (32): 1. Reference chip — every entity page (HOLODEX-374), §1 Setup, 1a. Placement, 1b. Component: `RefChip.svelte` (new, `web/src/lib/components/entity/`), 1c. States, 1d. API / MCP contract (frontend-relevant part), §2–§3 as built (HOLODEX-377, 2026-09-14), 2. Edition field row — media page (HOLODEX-377) (+24 more)

### Community 153 - "ADR-086: Film provider enrichment — own `entity_type`, poster as an asset"
Cohesion: 0.18
Nodes (11): 1 — Film enrichment gets its own `entity_type: "film"`, 2 — Film poster is an asset (`film_images.role = 'poster'`), never a canonical field, 3 — TMDB is the first provider; it needs an entity-type-aware remap, not new endpoints, 4 — Lock the `"film:"` namespace-collision boundary ADR-085 flagged, Action Items, ADR-086: Film provider enrichment — own `entity_type`, poster as an asset, Consequences, Context (+3 more)

### Community 154 - "Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)"
Cohesion: 0.10
Nodes (21): API, Behavior detail, Future considerations (P2), Gate status, Goals, Must-have (P0), Non-Goals, Open Questions (+13 more)

### Community 155 - "Spec: Films as a first-class entity (F56)"
Cohesion: 0.11
Nodes (19): API, Asserted-link invariant (RD1/P0-2), Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions (+11 more)

### Community 156 - "Design Handoff: Tag & category create affordance (HOLODEX-243)"
Cohesion: 0.15
Nodes (13): 1. The "+ New" pill, 2. Expanded form, 3. Submit behavior — diverges by type (important asymmetry), 4. Empty-state wiring, 5. Interaction states, 6. Edge cases, Accessibility notes, Design Handoff: Tag & category create affordance (HOLODEX-243) (+5 more)

### Community 157 - "Spec: Revealable per-candidate record summary in the resolve picker (F61)"
Cohesion: 0.10
Nodes (21): Acceptance Criteria, FR1 — Contract: `candidates[].detail` (§2.3) and its caps (§5), FR2 — Sanitization sibling of `sanitizeSearched`, FR3 — Picker reveal: text toggle, inline expansion, FR4 — Auto-expand on label collision, FR5 — Unattended path: the auto-applied candidate's `detail` reaches the activity log, FR6 — Never stored, never written back, Functional Requirements (+13 more)

### Community 158 - "Spec: Poster View for the People list page (F55)"
Cohesion: 0.11
Nodes (19): API, Behavior detail, Conditional border — exact rule, Density formula, Future Considerations (P2), Gate status, Goals, Must-Have (P0) (+11 more)

### Community 159 - "api/person_images_test.go"
Cohesion: 0.35
Nodes (15): deleteReq(), fillGallery(), getStatus(), personImageServer(), personImageServerCfg(), pngUpload(), TestGetPersonImages(), TestPersonDetailImageSet() (+7 more)

### Community 160 - "Design handoff: Film Studio cascade edit affordance (F57)"
Cohesion: 0.11
Nodes (18): 1. Media detail page — no visual change, §1 Setup, 2. Film detail page — trigger affordance, §2 Smoke, §3 Agent live QA (preview tools against §1 stack), 3. New component: `FilmStudioCascadeDialog.svelte`, 3a. Step 1 — Picker (open state), 3b. Step 2 — Results (post-commit, same dialog, same `PickerShell`) (+10 more)

### Community 161 - "QA checklist: Responsive page width (HOLODEX-331)"
Cohesion: 0.22
Nodes (7): 1. Setup, 2. Smoke, 3. Agent-verifiable geometry, 4. Density ladder, 5. Human review, 6. Regression, QA checklist: Responsive page width (HOLODEX-331)

### Community 162 - "nameKeyExpr"
Cohesion: 0.05
Nodes (45): database/sql.Tx, EntityAlias, nameCollidesInTable(), queueFilmSameTitle(), attachExternalID(), canonicalTable(), lookupByNameKey(), nameKeyExpr() (+37 more)

### Community 163 - "Geometry assertion harness (HOLODEX-349)"
Cohesion: 0.20
Nodes (7): Files, Four things it refuses to do quietly, Geometry assertion harness (HOLODEX-349), The run matrix, What holds the page still, Why this and not screenshots, Writing an assertion

### Community 164 - "ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename"
Cohesion: 0.12
Nodes (17): A — Namespace-qualified scalar value (chosen), Action Items, ADR-082: `external_provider_id` is a namespace-qualified value, not a plain rename, B — `(provider, external_id)`-keyed schema change across the nine tables, C — Leave `external_provider_id` a bare scalar, disambiguate providers elsewhere, Consequences, Context, Decision (+9 more)

### Community 165 - "writebackJob.ts"
Cohesion: 0.16
Nodes (15): BatchStatus, JOB_POLL_TIMEOUT_MS, pollUntilSettled(), fast, ADR-0091, ADR-0041, ADR-0048, ADR-0077 (+7 more)

### Community 166 - "Write"
Cohesion: 0.14
Nodes (26): currentTagValue(), ReadCurrentValues(), snapshotValueToString(), TestReadCurrentValues_AbsentTagIsEmpty(), TestReadCurrentValues_RoundTrips(), TestReadCurrentValues_SkipsImageFields(), mustReadDir(), requireExiftool() (+18 more)

### Community 167 - "Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254)"
Cohesion: 0.20
Nodes (10): Accessibility Notes, Content specification (the string the owner actually sees), Design Handoff: Configurable provider search patterns — search box seeding (HOLODEX-254), Design-system fit (the `/design-system` check), Edge case: empty sanitization result, Measured contrast, Non-goals (explicitly out of this change), Optional P1: seeded-value transparency caption (+2 more)

### Community 168 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.10
Nodes (21): 2026-08-17 · Brainstormed the Films entity end-to-end, opened epic, wrote spec, 2026-08-18 · Wrote ADR-085, resolving spec Q1/Q2, 2026-08-21 · session, 2026-08-21 · session (cont.), 2026-08-21 · session (cont. 2), 2026-08-21 · session (cont. 3), 2026-08-21 · session (cont. 4), 2026-08-23 · session (+13 more)

### Community 169 - "keyed"
Cohesion: 0.40
Nodes (3): entity_aliases, entity_aliases_fts, keyed

### Community 170 - "ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation"
Cohesion: 0.12
Nodes (16): Action Items, ADR-081: Entity Completeness Score — Facet Criticality, Not-Applicable Status, and Score Computation, Consequences, Context, D1: Facet criticality is static metadata on `registry.FieldDef`, D2: not-applicable persistence, D2: Not-applicable persists in a new, dedicated table — not a 4th decision `source`, D3/D4: score computation and list consumption (+8 more)

### Community 171 - "Find"
Cohesion: 0.22
Nodes (16): ContentType(), entityDir(), Ext(), Find(), Path(), Remove(), Store(), TestExt_FollowsBytes() (+8 more)

### Community 172 - "adr-claims.mjs"
Cohesion: 0.17
Nodes (32): CLAIMS_FILENAME, collapseClaims(), collisions(), daysSince(), describeRivals(), git(), main(), mainWorktreeRoot() (+24 more)

### Community 173 - "Video"
Cohesion: 0.08
Nodes (18): fakeVideoLookup, Handlers, ExtraMetadata, Video, FacetValue, RelatedShelf, VideoFilter, VideoStat (+10 more)

### Community 174 - "scanner_test.go"
Cohesion: 0.18
Nodes (19): TestBuildVideoFromFileForcesExtractWithoutPersisting(), TestExtractionHook(), TestExtractionHook_ErrorDoesNotFailScan(), New(), activeCount(), newFakeRepo(), newTestScanner(), TestChangedFileIsReindexed() (+11 more)

### Community 175 - "settings.json"
Cohesion: 0.50
Nodes (3): hooks, PreToolUse, $schema

### Community 176 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.12
Nodes (16): 2026-07-10 · what happened this session, 2026-08-07 · Architecture gate closed — ADR-081 written, 2026-08-07 · Backend D1+D2 — facet criticality metadata + not-applicable mutation, 2026-08-07 · Backend D3 — score/actionability computation in internal/resolver, 2026-08-07 · Design gate closed — remediation queue + breakdown panel handoff written, 2026-08-07 · Spec written, Jira epic + stories created, branch/issue wired up, 2026-08-08 · Backend D4 — list-wide resolve-all predicate for browse sort/filter + remediation queue, 2026-08-08 · Item #4 — browse Completeness sort + Missing facet filter chip (F55.5/F55.6) (+8 more)

### Community 177 - "Decision"
Cohesion: 0.05
Nodes (36): ADR-011: Symlink Handling & Path Resolution, Configuration, Consequences, Context, Decision, Hardlinks, Loop protection, Resolution & dedup (+28 more)

### Community 179 - "Stress fixture seeder (HOLODEX-342)"
Cohesion: 0.14
Nodes (14): Derived kinds are addressed above the pool, not below it, Derived links go through the file layer, Films, scenes and the scene pool, Isolation, twice over (spec D5), `manifest.json`, Not every rung fits every field, Notes, Reverse cardinality is emergent, not addressed (+6 more)

### Community 180 - "field_source_decisions table"
Cohesion: 0.67
Nodes (3): field_source_decisions table, four-tier label/render/group/order resolution ladder, settings KV table + typed Registry (validation/UI schema)

### Community 181 - "architecture/README.md"
Cohesion: 0.04
Nodes (41): ADR-002: SvelteKit chosen as frontend framework (SPA/static-adapter mode), ADR-045 (owner-session, per promote-override-fields.md): owner gate / Admin mode / effectiveOwner, ADR-046 (Proposed, owner-session-persistence.md): Owner session persistence via HttpOnly token-exchange cookie, ADR-094: Local development credentials come from the environment, never a repo file, Alternatives considered, Consequences, Context, Decision (+33 more)

### Community 182 - "ResolvedField"
Cohesion: 0.05
Nodes (38): enrichRoute, entityCompletenessBatch, ExternalLink, relinkContext, FieldCandidate, FieldDecision, Handlers, injectSyntheticFacet() (+30 more)

### Community 183 - "Spec: Unified Studio edit affordance + Film-level cascade writeback (F57)"
Cohesion: 0.12
Nodes (16): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+8 more)

### Community 184 - "AutoRegisterFields"
Cohesion: 0.11
Nodes (33): AutoRegisterFields(), ClaimedKeys(), ResolvedField, newAutoAcc(), claimField(), fieldByKey(), ResolvedField, hintLookup() (+25 more)

### Community 185 - "Spec: People Images (F25)"
Cohesion: 0.08
Nodes (26): Access control & security, Addendum — configurable cap, owner override & enrichment suppression ([ADR-043](../architecture/ADR-043-gallery-cap-and-enrichment-suppression.md), 2026-06-25), Addendum — enrichment photos are deduplicated in the gallery ([ADR-050](../architecture/ADR-050-image-content-dedup.md), F34, 2026-06-29), Addendum — owner/admin cap bypass, gallery grid modal, image viewer (HOLODEX-174, 2026-07-08), Addendum — owner-set core images take precedence over enrichment ([ADR-049](../architecture/ADR-049-manual-image-precedence.md), F33, 2026-06-28), Artifacts to produce (project working agreements), Data, storage & serving (direction — finalized in the ADR), F25.26–30 — Person-page polish (follow-ups) (+18 more)

### Community 187 - "stub.js"
Cohesion: 0.07
Nodes (27): ADR-0039, ADR-0082, ALL_ENTITY_TYPES, BY_SLUG, candidate(), candidatesFor(), crc32(), CRC_TABLE (+19 more)

### Community 189 - "Design handoff — Media parts (`part` badge and pill)"
Cohesion: 0.10
Nodes (20): 1. Browse / search cards — `VideoCard.svelte`, §1 Setup, 1a. Placement and treatment, 1b. Coexistence, 1c. Data, 2. Media page — header pill + Metadata row, §2 Smoke — automated, 2a. Header pill (`routes/media/[id]/+page.svelte`) (+12 more)

### Community 193 - "person-detail-bio-header-handoff.md"
Cohesion: 0.07
Nodes (23): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist), Manual QA Checklist: Two-Tier Field Editing Model (F56), 2026-08-31 · design-handoff artifacts committed, 2026-08-31 · session, 2026-09-01 · code-review high --fix (+15 more)

### Community 194 - "Spec: Two-Tier Field Editing Model (F56)"
Cohesion: 0.08
Nodes (24): Access control & security, Artifacts to produce (project working agreements), At rest, Data, storage & serving, Expand, Frontend / theming requirements, Future considerations (P2), Goals (+16 more)

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

### Community 201 - "Design Handoff — People Images (F25)"
Cohesion: 0.14
Nodes (14): A. People list (`/people`) — headshot, Accessibility notes, Animation / motion, B. Person page (`/people/[id]`) — banner hero + gallery + owner tools, C. Video page (`/media/[id]`) — poster cards, Components, Design Handoff — People Images (F25), Design tokens used (+6 more)

### Community 202 - "Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating"
Cohesion: 0.08
Nodes (21): 1. Studio next to the title, 2. Commentary block, 3. Poster upload, 4. File metadata — owner only, Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating, QA checklist, Responsive / motion / a11y, 2026-07-10 · what happened this session (+13 more)

### Community 203 - "studio-link-card-handoff.md"
Cohesion: 0.33
Nodes (3): Gates — definition of done, HOLODEX-290 · StudioLinkCard (reusable Studio display), Up next — ordered (position = priority)

### Community 205 - "Design handoff: Entity Films row — card-height match + hover lift"
Cohesion: 0.11
Nodes (19): 10. Resolved decisions, 11. Scope, 1. Overview, 1a. The rule, 2. Design tokens, 3. Layout, 3a. Sizing formula, 3b. Caption block (+11 more)

### Community 206 - "Design Handoff: Writeback hides the target file tag (HOLODEX-216)"
Cohesion: 0.14
Nodes (12): Design Handoff: Writeback hides the target file tag (HOLODEX-216), Design-system fit (the `/design-system` check), Non-goals (explicitly out of this change), Problem, QA checklist, Row states (unchanged rows omitted — only the new branch), The "no dimming" rule, applied, 2026-08-13 · session (+4 more)

### Community 207 - "net/http/httptest.Server"
Cohesion: 0.22
Nodes (18): net/http/httptest.Server, net/http.Response, filmResolvedValue(), TestFilmFieldDecision(), filmPut(), TestFilmPeopleRolesCRUD(), filmDelete(), filmPatch() (+10 more)

### Community 208 - "Design Handoff: Two-Tier Field Editing Model (F56)"
Cohesion: 0.15
Nodes (13): Accessibility Notes, Animation / Motion, Design Handoff: Two-Tier Field Editing Model (F56), Design-system fit (the `/design-system` check), Design Tokens Used, Edge Cases, Layout, Open Question carried to implementation (+5 more)

### Community 209 - ".RelinkProviderIcon"
Cohesion: 0.36
Nodes (7): Handlers, Find(), Path(), Remove(), Store(), TestStorePNG_ExtensionFollowsBytes(), TestStoreRoundTrip()

### Community 213 - "Rules"
Cohesion: 0.40
Nodes (4): Enrichment components, Rules, The candidate slot's box is kind-shaped, always 60 px tall, never `aspect-*`, Ungated-aspect image slots use the plate with `object-contain`

### Community 214 - "Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)"
Cohesion: 0.13
Nodes (15): 1. `/tags` — unified type filter + search, 2. Category pill, 3. `/categories/{id}` detail page (new route), 4. Bulk "Add to category…" / "Remove from category…" (Manage-mode bar), 5. Browse-page "Categories" facet, Accessibility notes, Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240), Design-system-fit audit (+7 more)

### Community 215 - "ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field"
Cohesion: 0.20
Nodes (10): 1. `studio_images` replaces `studio_logos` — three core roles, no gallery, 2. `enrich.ImageSink` / `downloadAssets` become entity-generic, 3. The studio `logo` field is retired; TMDB emits it as an asset, 4. Serving, upload, delete — mirrors ADR-057 §4 with an owner-write path added, Action items, ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field, Consequences, Context (+2 more)

### Community 216 - "New"
Cohesion: 0.51
Nodes (13): TestQueue_RevertUnknownBatch(), New(), newRepo(), seedVideo(), testLogger(), TestQueue_EnqueueMany_EmptyIsNoop(), TestQueue_EnqueueMany_OneCallEnqueuesEveryJob(), TestQueue_FailureMarksFailedAndKeepsRow() (+5 more)

### Community 219 - "+layout.ts"
Cohesion: 0.50
Nodes (3): prerender, ssr, ADR-0002

### Community 220 - "CurationFieldRow.svelte"
Cohesion: 0.12
Nodes (14): commitEdit(), draft, editing, isProvider, onEditKey(), provenance, adding, busy (+6 more)

### Community 221 - "dependencies"
Cohesion: 0.29
Nodes (7): dependencies, flag-icons, @fontsource/share-tech-mono, @fontsource-variable/archivo, @fontsource-variable/fraunces, @fontsource-variable/spline-sans-mono, @fontsource/vt323

### Community 222 - "scripts"
Cohesion: 0.29
Nodes (7): scripts, build, check, dev, geometry, preview, test

### Community 236 - "Must-Have (P0)"
Cohesion: 0.09
Nodes (22): Acceptance Criteria, FR1 — Operator pattern config (`metadata-sources.yaml`), FR2 — Provider-advertised preference (`/describe.preferred_search_pattern`), FR3 — Token grammar, rendering, and precedence fallthrough, FR4 — Unconditional title sanitizer, FR5 — Wiring: choke point, response payload, zero picker changes, FR6 — Residue rule: `{title}` renders empty when it is only the other tokens *(ADR-095 D5)*, FR7 — `hint.query_source`: is the query Holodex's render or the owner's? *(ADR-095 D4)* (+14 more)

### Community 247 - "Spec: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.11
Nodes (18): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+10 more)

### Community 258 - "testing.T"
Cohesion: 0.05
Nodes (57): bytes.Buffer, testing.T, attachFilmVideo(), resolvedValue(), seedFilm(), TestFilmSourceInjection_SceneVsFullFilm(), filmReleaseYear(), TestFilmFields_NameSourcesAreBaselineThenProviderTitle() (+49 more)

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
Cohesion: 0.05
Nodes (38): ADR-003: SQLite (modernc.org/sqlite) + FTS5 chosen as database, WAL mode enabling concurrent reads during scanner writes, ADR-008: Caching Strategy — In-Process Cache with Redis-Ready Interface, Cache Interface, Configuration, Consequences, Context, Decision (+30 more)

### Community 291 - "newHandler"
Cohesion: 0.22
Nodes (14): newHandler(), classifyCredential(), main(), newTMDBClient(), newDiscardLogger(), TestDescribe(), TestDescribeAdvertisesStudio(), TestDescribeLinkTemplates() (+6 more)

### Community 293 - "Design handoff: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.25
Nodes (8): 1. `/studios` list — logo well data source change only, 2. `/studios/{id}` detail — role-generic image control, 3. Provenance (P1, non-blocking), 4. Accessibility & 3-skin QA checklist, Design handoff: Studio image roles — icon / logo / poster (F51), Empty vs. populated states (per role), Interaction, Layout

### Community 297 - "Design Handoff: Metadata Enrichment UI for People (F22)"
Cohesion: 0.12
Nodes (16): Accessibility Notes, Animation / Motion, Components, Confidence display, Design Handoff: Metadata Enrichment UI for People (F22), Design Tokens Used, Edge Cases, Layout (+8 more)

### Community 301 - "ADR-089: Film enrichment field vocabulary — where each provider value lands on a film"
Cohesion: 0.10
Nodes (21): Action Items, ADR-089: Film enrichment field vocabulary — where each provider value lands on a film, Consequences, Context, Current state (survey, 2026-09-04), D1 — Provider film cast is film-level and never cascades to attached videos; it is read from the shadow, not copied into `film_people_roles`, D1 — where a provider's film cast lands, D2 — how the two cast sources render (+13 more)

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

### Community 315 - "HOLODEX-342 · Dev-time stress fixture for UX and layout QA"
Cohesion: 0.20
Nodes (8): Gates — definition of done, HOLODEX-342 · Dev-time stress fixture for UX and layout QA, Up next — ordered (position = priority), 2026-09-10 · chevron gated on a real overflow measurement, Gates — definition of done, HOLODEX-361 · ExpandableText renders its chevron even when the text does not clamp, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 316 - "Design handoff: In-app promote / override affordance for auto-registered fields (F44)"
Cohesion: 0.15
Nodes (13): 10. Three-skin QA (required), 11. What is explicitly not in this handoff, 1. The Promote control (owner-only, on the auto row), 2. The inline editor (shared promote + edit — DD2), 3. After promotion — the partition move (shared by all treatments), 4. Edit / Remove promotion (owner-only, on the promoted row), 5. States, 6. Responsive behavior (+5 more)

### Community 317 - "Spec: Media parts — denote the files of a multi-file media (`part`)"
Cohesion: 0.11
Nodes (19): API, Behavior detail, Data model, Future considerations (P2), Goals, Marker lifter (RD4), Must-have (P0), Non-Goals (+11 more)

### Community 325 - "Spec: Film provider enrichment on the film detail page (F59)"
Cohesion: 0.11
Nodes (19): API, Behavior detail, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement (+11 more)

### Community 328 - "Functional Requirements"
Cohesion: 0.15
Nodes (13): Data Model Extensions (Phase 3), F14: People Enrichment, F15: Tag Enrichment, F16: Metadata Source Plugins, F17: Metadata Writeback, F18: Autogenerated Preview Trailers, Functional Requirements, In Scope (+5 more)

### Community 331 - "Spec: Entity identity card — reference handle, unified external ids, films in the spine, edition, display name (F60)"
Cohesion: 0.10
Nodes (21): API, Behavior detail, Data model, Display name (RD9/RD10), Edition (RD6–RD8), Future considerations (P2), Goals, Must-have (P0) (+13 more)

### Community 336 - "manifest.mjs"
Cohesion: 0.33
Nodes (8): DEFAULT_MANIFEST, describe(), entries(), here, load(), select(), m, plan()

### Community 337 - "Spec: Person Aliases (F23)"
Cohesion: 0.17
Nodes (12): API, Data model, Functional Requirements, In scope, Non-functional, Objective, Open questions, Out of scope (tracked follow-ups, not gaps) (+4 more)

### Community 348 - "Backfill"
Cohesion: 0.18
Nodes (13): image.Image, Backfill(), readStored(), discardLog(), TestBackfillHashesAndRemoves(), downscale(), Hash(), normalize() (+5 more)

### Community 351 - "Spec — Showcase Demo Corpus"
Cohesion: 0.05
Nodes (33): metadata.Extractor Go interface encapsulating tool implementations, ADR-004: layered ffprobe + exiftool + ffmpeg metadata extraction pipeline, ADR-006: REST + OpenAPI 3.1 API design under /api/v1, ADR-010: MKV (Matroska) Tag-Target Precedence, Consequences, Context, Decision, Rationale (+25 more)

### Community 353 - "Decision"
Cohesion: 0.12
Nodes (13): frontendFS(), frontendFS(), 1. Embed source lives in the `cmd/holodex` package, 2. SPA fallback handler, 3. BuildKit cache mounts, 4. `.dockerignore`, 5. Startup logs the URL, ADR-020: Frontend Embed Location, SPA Fallback Serving & BuildKit Caching (+5 more)

### Community 358 - "Spec: Candidate thumbnail in the resolve picker (F64)"
Cohesion: 0.11
Nodes (18): Acceptance Criteria, FR1 — Contract: `candidates[].image_url` (§2.3, §5, §6, §8), FR2 — Sanitization: host gate in `sanitizeCandidates`, FR3 — Picker row: kind-shaped thumb column, 60 px tall, FR4 — TMDB sidecar emits `image_url`, rendition per entity kind, Functional Requirements, Future Considerations (P2), Goals (+10 more)

### Community 374 - "Design handoff: Media detail page reorder"
Cohesion: 0.08
Nodes (23): 1. Films + People row, 2. Rejected during iteration, Accessibility / interaction, Design handoff: Media detail page reorder, Edge cases, Final order (top to bottom), Overview, Theming (+15 more)

### Community 375 - "filmYearServer"
Cohesion: 0.27
Nodes (17): billedCast(), seedBilled(), TestFilmBilledCast_DegenerateShapes(), TestFilmBilledCast_LinksKnownPeopleAndCreatesNone(), TestFilmBilledCast_MatchesByIdentityNotString(), TestFilmBilledCast_OnlyTheComplement(), filmYear(), filmYearServer() (+9 more)

### Community 378 - "Manual QA Checklist: Metadata Enrichment for People (F22)"
Cohesion: 0.29
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs a human's eye, Manual QA Checklist: Metadata Enrichment for People (F22)

### Community 381 - "Sink"
Cohesion: 0.25
Nodes (7): FilmRepo, personRepo, Sink, StudioRepo, filmImageSourceProvider(), ReplaceFilmImageFile(), ReplaceStudioImageFile()

### Community 383 - "Manual QA Checklist: Admin Mode (F29)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Admin Mode (F29)

### Community 385 - ".uploadVideoPoster"
Cohesion: 0.22
Nodes (6): Handlers, readAllLimited(), Handlers, chi.Router, PosterPath(), ThumbPath()

### Community 398 - "Repo"
Cohesion: 0.16
Nodes (27): backfillPersonLinks(), backfillStudioLinks(), main(), newLogger(), promoteEnrichmentAliases(), run(), runHealthcheck(), runMCPStdio() (+19 more)

### Community 434 - "Design handoff: Completeness panel — collapsible facet fold"
Cohesion: 0.07
Nodes (25): Accessibility, Chip anatomy, Collision review line, Design handoff — alias collapse (HOLODEX-306), Out of scope, States, Theming, What changes on screen (+17 more)

### Community 435 - "Design handoff: Fire-and-forget writeback"
Cohesion: 0.12
Nodes (16): Accessibility, Badge alignment and weight, Design handoff: Fire-and-forget writeback, Design tokens used, Dialog rows — what changes, Edge cases, Job-level, not per-field, Layout (+8 more)

### Community 453 - "ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field"
Cohesion: 0.15
Nodes (13): ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field, Alternatives considered, Consequences, Context, D1 — The two layers, and the boundary between them, D2 — Layer 1 is reachable from the entity's own detail page, D3 — Adoption must visibly land in layer 2, D4 — A new source is a namespace, not a subsystem (+5 more)

### Community 456 - "evaluate.mjs"
Cohesion: 0.44
Nodes (6): describeBound(), evaluate(), reconcileBlocked(), score(), base, within()

### Community 457 - ".getFilm"
Cohesion: 0.27
Nodes (6): filmImageURL(), setFilmAttachmentPosterURLs(), setFilmImageURLs(), Handlers, chi.Router, redactFileMetadataForVisitor()

### Community 458 - "SourceBadge.svelte"
Cohesion: 0.14
Nodes (17): badgeProvider, busy, cancelCustomEdit(), close(), commitCustomDraft(), confirm(), draft, editing (+9 more)

### Community 459 - "Load"
Cohesion: 0.16
Nodes (19): applyEnv(), Defaults(), envBool(), envInt(), envInt64(), envStr(), Config, Load() (+11 more)

### Community 461 - "ReadbackGaps"
Cohesion: 0.33
Nodes (9): LogReadbackGaps(), ReadbackGaps(), readKey(), TestLogReadbackGaps_MessageIsActionable(), TestLogReadbackGaps_SilentWhenClean(), TestReadbackGaps_DetectsWrongTag(), TestReadbackGaps_TitleReadsThroughTheFileTitleAlias(), writeTargetReadKeys() (+1 more)

### Community 462 - "HOLODEX-240.md"
Cohesion: 0.40
Nodes (4): 2026-07-31 · session, Gates — definition of done, Session log   (append-only), Up next   (ordered — position is the priority; top line is the next action)

### Community 463 - "ADR-096: Entity identity card — one reference, one external-id store, films in the spine, edition as a file field, display name as a decision"
Cohesion: 0.12
Nodes (17): Action Items, ADR-096: Entity identity card — one reference, one external-id store, films in the spine, edition as a file field, display name as a decision, Consequences, Context, D1 — A server-produced reference `kind:id` is the handle; slugs are cut, D1 — the handle, D2 — One polymorphic `entity_external_ids` table replaces two per-kind tables and the memo column, D2 — where provider ids live (+9 more)

### Community 465 - "holoShuffle"
Cohesion: 0.50
Nodes (3): holoShuffle(), registerShuffle(), TestHoloShuffle()

### Community 466 - "SanitizeLinkTemplates"
Cohesion: 0.29
Nodes (8): BuildLink(), SanitizeLinkTemplates(), TestBuildLink(), TestManifest_LinkTemplatesDecodeBackwardCompat(), TestSanitizeLinkTemplates_DropsInvalidNormalizesKeys(), TestSanitizeLinkTemplates_EmptyAndNil(), TestValidateLinkTemplate(), ValidateLinkTemplate()

### Community 467 - "Decisions"
Cohesion: 0.15
Nodes (13): D10 — The fixture owns its metadata mapping (HOLODEX-347)., D11 — The palette reaches every free-text field the app renders, and no others (HOLODEX-346)., D12 — Breadth is a pool, not a dimension (HOLODEX-350)., D1 — Injection: seed rows, drop images. Do not go through the scanner., D2 — The ladder runs from zero, not from the maximum., D3 — One factor at a time. Never cross-product., D4 — Addressing: reserved ID blocks, encoded names, machine-readable manifest., D5 — Isolation is simultaneously the teardown and the safety guard. (+5 more)

### Community 468 - "cascadeServer"
Cohesion: 0.49
Nodes (10): cascadePost(), cascadeServer(), seedCascadeVideo(), TestCascadeFilmStudio_AllCollide_EmptyBatch(), TestCascadeFilmStudio_PartialCollision_BestEffort(), TestCascadeFilmStudio_SameValueRedecide_NotACollision(), TestCascadeFilmStudio_UnmatchedProvider_PerVideoError(), TestCascadeFilmStudio_ZeroVideos_EmptyBatch() (+2 more)

### Community 469 - "extractReviewServer"
Cohesion: 0.28
Nodes (17): extractReviewGET(), extractReviewPOST(), extractReviewServer(), TestDismissExtractionReview(), TestExtractionQueue_Empty(), TestExtractionQueue_FilteredStillRequiresOwner(), TestExtractionQueue_ListsPendingRowsVideoJoined(), TestExtractionQueue_RejectsMalformedVideoID() (+9 more)

### Community 470 - "Handoff Spec: Person detail — bio in the header row"
Cohesion: 0.14
Nodes (14): Accessibility Notes, Animation / Motion, Design-system fit, Design Tokens Used, Edge Cases, Handoff Spec: Person detail — bio in the header row, Layout, Open Questions carried to implementation (+6 more)

### Community 471 - "authServer"
Cohesion: 0.32
Nodes (16): authServer(), exchange(), findCookie(), getCookie(), getTok(), TestCapabilities(), TestControlsUnauthenticatedFlag(), TestCookieAuthorizesGatedRoute() (+8 more)

### Community 473 - "HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film)"
Cohesion: 0.33
Nodes (5): 2026-08-25 · full implementation + simplify + verification, Gates — definition of done, HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 474 - "Design Handoff: People on the unified source-of-truth model (F37)"
Cohesion: 0.20
Nodes (10): Accessibility, Design Handoff: People on the unified source-of-truth model (F37), Design tokens, Entity-generic baseline label (dev notes), Layout, Long-text fields (`bio`) — P1-1 resolved, Overview, States and Interactions (delta to F36) (+2 more)

### Community 475 - "HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-08-13 · Implemented the SSRF perimeter fix end to end, 2026-08-13 · PR #238 opened, then a `/code-review --fix` pass found and closed a follow-on gap, Gates — definition of done, HOLODEX-212 · Close the SSRF allowlist gap on image writeback + resolved image_url fields, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 476 - "Spec: Quick Wins batch — Search history & "More with …" shelves"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, QW1 — Search history (client-only) (+8 more)

### Community 477 - "Design Handoff: Poster View for the People list page (F55)"
Cohesion: 0.11
Nodes (19): Accessibility, Component, CSS (new, additive — `app.css`, filed next to the `.portrait-frame` block), Design Handoff: Poster View for the People list page (F55), Design tokens used (all surfaces), Edge cases, Gate status (mirrors the spec), Load-in animation (+11 more)

### Community 479 - "Configuration Reference (holodex.yaml layers)"
Cohesion: 0.22
Nodes (9): admin_token / owner session authentication, ADR-046 (owner session persistence), default_source / provider_trust_order (F36, ADR-051), Configuration Reference (holodex.yaml layers), Metadata field mapping config (ADR-013), In-app field promotion (F44, ADR-062) precedence ladder, Metadata source plugins config (F22, ADR-033), Person images config (F25, ADR-038/ADR-043) (+1 more)

### Community 480 - "Design Handoff: Provider Link Badge — Multi-Badge States for Person/Studio (HOLODEX-266)"
Cohesion: 0.14
Nodes (14): 1. Placement on person and studio pages, 2. Cardinality states (0 / 1 / N), 3. Degraded state: id present, no link template, 4. Interaction and accessibility, 5. Film and media headers (F63 — HOLODEX-390, spec [provider-link-badge-coverage.md](../specs/provider-link-badge-coverage.md)), Badge anatomy (recap, unchanged), DD1 — Badges join the existing muted metadata line, not a new row, DD2 — Wrap, don't scroll or collapse (+6 more)

### Community 483 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.11
Nodes (17): ADR-091: Writeback is fire-and-forget; job status is a property of the video, not of the dialog, Alternatives considered, Consequences, Context, Decision, Scope, 2026-09-05 · pre-implementation gates: ADR + design handoff, 2026-09-06 · /code-review high --fix on PR #303 (+9 more)

### Community 484 - "thumbServer"
Cohesion: 0.25
Nodes (16): seedThumbVideo(), TestAdminStatus(), TestListEnqueuesVisibleAndExposesURL(), TestRegenerateDisabled(), TestRegenerateThumbnail(), TestServeThumbnailNotReadyThenReady(), thumbServer(), postersPNG() (+8 more)

### Community 485 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.14
Nodes (14): 2026-09-02 · ADR-088 + design handoff landed; direction set to a full collapse, 2026-09-02 · P0-1 migration 0044 landed, 2026-09-02 · spec landed — all four pre-implementation gates green, 2026-09-02 · testing gate closed, 2026-09-03 · alias-state seed generator + a bug it immediately found, 2026-09-03 · P0-2/P0-4/P0-5 enrich write path landed, 2026-09-03 · P0-6/P0-6b/P0-7 registry removal + completeness facet landed, 2026-09-03 · P0-8 model + API surface landed — backend complete (+6 more)

### Community 486 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.12
Nodes (17): 2026-09-16 · backend: extractor, 2026-09-16 · backend: formatMap, 2026-09-16 · backend: mapping + lifter, 2026-09-16 · backend: payload (gate closed), 2026-09-16 · brainstorm + spec + design handoff, 2026-09-16 · fixture step (#6), 2026-09-16 · frontend: header link (A) + queue rows — frontend gate closed, 2026-09-16 · frontend: media header + film row (+9 more)

### Community 487 - "reviewServer"
Cohesion: 0.28
Nodes (14): sendTok(), deleteServer(), TestDeleteEndpointsGated(), TestDeleteNotFoundPaths(), TestPurgeNowEndpoint(), TestSoftDeleteRestoreFlow(), doJSONTok(), reviewServer() (+6 more)

### Community 489 - ".ReconcileVideoPeopleLocked"
Cohesion: 0.22
Nodes (7): extIDFor(), foldedExtIDIndex(), foldNameKey(), PersonRoleName, Repo, personHasAuthoredIdentity(), personLinkKey

### Community 490 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.22
Nodes (9): 2026-08-25 · implementation landed — all seven gates green, 2026-08-25 · resolved three rounds of merge conflicts against a fast-moving main, 2026-08-25 · security-review landed — all five pre-implementation gates green, 2026-08-25 · spec, ADR, and design handoff landed, 2026-08-25 · testing-strategy landed; mockup persistence established as a standing rule, Gates — definition of done, HOLODEX-285 · Unified Studio edit affordance + Film-level cascade writeback, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 491 - "Repo"
Cohesion: 0.16
Nodes (5): fakeFilmRepo, ValidFilmImageRole(), FilmImage, FilmImageInsert, Repo

### Community 504 - "net/http.ResponseWriter"
Cohesion: 0.06
Nodes (28): net/http.Request, net/http.ResponseWriter, Handlers, Handlers, parseCategoryName(), Handlers, decodeJSON(), Handlers (+20 more)

### Community 505 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.14
Nodes (13): 2026-09-11 · contract text written, 2026-09-11 · design handoff for the caption, 2026-09-11 · epic refreshed from the filename probe, ADR-095 drafted, 2026-09-11 · F54 spec amended, 2026-09-11 · HOLODEX-368 built (request side), 2026-09-11 · HOLODEX-369 built (picker caption), 2026-09-11 · HOLODEX-372 — the caption's stressed state in the geometry harness, 2026-09-11 · security review of the design (+5 more)

### Community 506 - "Design Handoff: Studio relationship-edit popover (HOLODEX-271)"
Cohesion: 0.20
Nodes (10): Accessibility Notes, Components, Design Handoff: Studio relationship-edit popover (HOLODEX-271), Design Tokens Used, Edge Cases, Layout, Overview, QA (+2 more)

### Community 507 - "HOLODEX-288 · Fix film-studio cascade code-review findings"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-25 · session, Gates — definition of done, HOLODEX-288 · Fix film-studio cascade code-review findings, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 510 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.17
Nodes (12): 2026-07-10 · what happened this session, 2026-08-09 · Architecture gate closed — ADR-083 written, 2026-08-09 · Backend gate closed — LinkTemplates + external_links projection, 2026-08-09 · Design gate closed — multi-badge handoff written, 2026-08-09 · Frontend gate closed — ProviderLinkBadge.svelte + person/studio wiring, 2026-08-09 · Post-review hardening — high-effort /code-review pass, 6 fixes applied, 2026-08-09 · Security gate closed — LinkTemplates injection review, no findings, 2026-08-09 · Testing gate closed — external_links projection + BuildProviderLink coverage (+4 more)

### Community 511 - "Design handoff: Responsive page width — player column, metadata rail, intrinsic grid density"
Cohesion: 0.07
Nodes (28): 1. Overview, 1a. Most pages are already full width, 1b-i. Interim change already shipped: max density is now 8 columns, 1b. What's actually wrong, 1c. The rule, 2. Layout, 2a. The stage — IMPLEMENTED, 2b. The two-zone split (>= 1024px) — IMPLEMENTED (+20 more)

### Community 513 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.22
Nodes (9): 2026-08-11 · simplify + security-review, all gates green, ready to commit, 2026-08-11a · spec + design handoff written, 2026-08-11b · backend + frontend implementation, 3-skin live QA, 2026-08-11c · PR #231 code-review fixes + a second /simplify pass, 2026-08-11d · merged origin/main into PR #231, resolved 11-file conflict, Gates — definition of done, HOLODEX-271 · Studio relationship-edit popover (F56.4), Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 517 - "Design Handoff: Revealable candidate detail in the Enrich picker (HOLODEX-380 / F61)"
Cohesion: 0.17
Nodes (12): Batch path — Activity detail row (no new markup), Content spec, Design Handoff: Revealable candidate detail in the Enrich picker (HOLODEX-380 / F61), Design-system fit, Edge cases, Implementation notes, Keyboard and accessibility, Non-goals (+4 more)

### Community 518 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.25
Nodes (8): 2026-07-10 · what happened this session, 2026-08-26 · session (1), 2026-08-26 · session (2), 2026-08-26 · session (3), 2026-08-26 · session (4), 2026-08-26 · session (5), 2026-08-26 · session (6), Session log — append-only (cap: last 8 sessions; older → archive/)

### Community 520 - "extractServer"
Cohesion: 0.56
Nodes (8): extractPOST(), extractServer(), TestAdminExtractAllAccepted(), TestAdminExtractAllUnavailable(), TestExtractMediaMatch(), TestExtractMediaNotFound(), TestExtractMediaRequiresOwner(), TestExtractMediaUnavailable()

### Community 523 - "Spec: Owner tooling hub + visitor/owner nav split (F35)"
Cohesion: 0.17
Nodes (12): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, Requirements (+4 more)

### Community 527 - "studio-picker-handoff.md"
Cohesion: 0.33
Nodes (3): Gates — definition of done, HOLODEX-289 · Studio add-affordance on the media detail page, Up next — ordered (position = priority)

### Community 531 - "tag-link-chip-handoff.md"
Cohesion: 0.25
Nodes (6): 2026-08-29 · session, 2026-08-29 · session, Gates — definition of done, HOLODEX-292 · Shared TagLinkChip component, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 538 - "Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA"
Cohesion: 0.33
Nodes (6): Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA, Decision: empty-state CTA — "+ Add studio" text button, not a bare pencil, Decision: pencil position — trailing, not leading, Decision: visibility — always-visible, not hover-revealed, Do / Don't, States (trigger, superseding "States and Interactions" above)

### Community 540 - "New"
Cohesion: 0.49
Nodes (9): New(), jpegBytes(), TestSinkRefusesPortraitFilmBanner(), TestSinkRollsBackOnStoreFailure_Person(), TestSinkRollsBackOnStoreFailure_Studio(), TestSinkSkipsDuplicate(), TestSinkStoreAsset_Person_Normalizes(), TestSinkStoreAsset_Studio_Normalizes() (+1 more)

### Community 541 - "Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided""
Cohesion: 0.17
Nodes (11): Accessibility, Decided visual spec (Option 1), Design Handoff: Writeback dialog — poster comparison + enrichment/decision legibility gap, Fix options considered, Issue 1 — the dialog never shows the file's current poster next to the enriched candidate, Issue 2 — a field the owner just enriched doesn't pre-check / doesn't land in "decided", Layout, Overview (+3 more)

### Community 542 - "Spec: Tag Writeback Exclusion — per-tag Genre writeback control"
Cohesion: 0.17
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 544 - "Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369)"
Cohesion: 0.17
Nodes (12): Batch path — Activity detail row (no new markup), Content spec, Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369), Design-system fit, Edge cases, Implementation notes, Keyboard and accessibility, Non-goals (+4 more)

### Community 545 - "HOLODEX-102 · Video Credits → People + Headshots (F32)"
Cohesion: 0.29
Nodes (6): 2026-06-30 – 2026-08-06 · F32 implementation (4 slices), 2026-08-06 · code-review + simplify + doc/Jira sync, Gates — definition of done, HOLODEX-102 · Video Credits → People + Headshots (F32), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 546 - "HOLODEX-255 · <epic title>"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-05 · session, Gates — definition of done, HOLODEX-255 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 547 - "ADR-085-films-entity.md"
Cohesion: 0.09
Nodes (17): Decision, Film banner landscape guard — design handoff, Not in scope, QA, What the owner sees, 2026-08-27 · Implemented film_people_roles CRUD, Gates — definition of done, HOLODEX-281 · film_people_roles CRUD (film-level billing/role data) (+9 more)

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

### Community 563 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.13
Nodes (14): 2026-09-12 · brainstorm → epic → design handoff → spec, 2026-09-13 · 374 reference handle — coded, tested, live-QA'd, 2026-09-13 · 375 external-id unification — coded, tested, 2026-09-13 · ADR-096, 2026-09-13 · open-question check — no spikes needed, 2026-09-13 · tag casing is policy, 2026-09-14 · 376 films into the spine — coded, tested, live-QA'd, 2026-09-14 · 377 edition — coded, tested, live-QA'd, security-reviewed (+6 more)

### Community 566 - "filelayer.go"
Cohesion: 0.42
Nodes (7): fieldHint, fileField, fileSource(), fieldHint, loadFileField(), loadPersonField(), missingFieldErr()

### Community 568 - "Spec: Owner-mode video editing — Commentary, poster upload, studio placement, file-metadata gating (F52)"
Cohesion: 0.14
Nodes (14): API, Before implementation, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Problem Statement, Requirements (+6 more)

### Community 569 - "Design handoff: Manage block — destructive split button"
Cohesion: 0.20
Nodes (10): 1. What's wrong, 2. The rule, 2a. Why the default segment stays warn, 2b. Why split rather than a plain menu button, 2c. What this does not change, 3. Reuse, 4. Keyboard and screen reader, 5. Contrast (measured, not assumed) (+2 more)

### Community 572 - "ADR-098: Provider source URL — `_source_url` as the per-pill fallback behind link templates"
Cohesion: 0.13
Nodes (15): Action Items, ADR-098: Provider source URL — `_source_url` as the per-pill fallback behind link templates, Consequences, Context, D1 / D2 — where the URL rides and where it lives, D1 — Transport: `fields._source_url` on `/enrich`, riding the `_` sidecar channel, D2 — Ingest: validate like `profile_url`; garbage overwrites, absence leaves alone, D3 — how a stored URL composes with templates (+7 more)

### Community 573 - "New"
Cohesion: 0.09
Nodes (30): strings.Builder, time.Duration, formatFloat(), New(), newHistogram(), findLine(), scrape(), TestExposition() (+22 more)

### Community 575 - "httpClient"
Cohesion: 0.23
Nodes (6): httpClient, Source, Source, newHTTPClient(), TestHTTPClientContract(), TestHTTPClientNoCrossHostRedirect()

### Community 576 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.14
Nodes (14): 2026-09-04 · brainstorm corrected the premise, then all three pre-implementation gates written, 2026-09-04 · HOLODEX-309 wiring built and verified live end to end, 2026-09-04 · HOLODEX-310 cast coverage — ADR-089 D1's storage amended by one lookup, 2026-09-04 · HOLODEX-311 year fill shipped; ADR-089 D3 amended twice by building it, 2026-09-04 · HOLODEX-313 provider docs corrected — and one of my own claims retracted, 2026-09-04 · HOLODEX-317 — the year became a field instead of a message; backlink removed, 2026-09-04 · owner review of the 311 message → interim fix + a better direction (HOLODEX-317), 2026-09-04 · owner review round 2 — banner shipped (312), index poster bug fixed (318), owner-only gating (+6 more)

### Community 577 - "people-grid-handoff.md"
Cohesion: 0.29
Nodes (5): 2026-08-29 · session, Gates — definition of done, HOLODEX-294 · Reusable PeopleGrid component, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 578 - "Decision"
Cohesion: 0.15
Nodes (13): ADR-088: Provider aliases collapse into the canonical alias spine, Alternatives considered, Consequences, Context, D1 — `aliases` leaves the field registry, D2 — `entity_aliases` gains a `source` column, D3 — A provider alias is fully live on arrival, D4 — Deleting a provider alias records a suppression (+5 more)

### Community 579 - "QA: TMDB Provider Sidecar + ADR-039 Core Changes"
Cohesion: 0.13
Nodes (14): 0. Setup, 1. Provider contract — smoke (no real TMDB, no network), 2. ADR-039 core changes — `asset_hosts` allowlist, 3. End-to-end via Holodex + real TMDB provider, 4. Provider image (Docker), 5. Security checks, 6. Non-functional, 7. Film / Video enrichment (F26) (+6 more)

### Community 580 - "api.test.ts"
Cohesion: 0.15
Nodes (10): ENRICH_ENTITY_BASE, freshApi(), ADR-0038, ADR-0086, ADR-0088, ADR-0089, runEnrichRefresh(), runEnrichRefreshAll() (+2 more)

### Community 581 - "postTok"
Cohesion: 0.21
Nodes (21): aliasList(), aliasServer(), TestAddAliasConflict409(), TestAliasEndpointsGatedAndValidated(), TestGetPersonIncludesAliases(), TestMergeEndpoint(), TestMergeEndpoint_PropagatesWritebackToAffectedVideos(), TestPersonDetail_AliasSourceAndSkipped() (+13 more)

### Community 582 - "ADR-092: Flightplan repo extraction — relocate the plugin to its own repository"
Cohesion: 0.15
Nodes (12): A — standalone repo, hand-copied per consumer, own ADR trail (chosen), Action Items, ADR-092: Flightplan repo extraction — relocate the plugin to its own repository, B — package as an installable Claude Code plugin now, C — stay in Holodex; solve the numbering collision with a reserved ADR range instead, Consequences, Constraints / forces, Context (+4 more)

### Community 588 - "probe-edition-filenames.mjs"
Cohesion: 0.26
Nodes (12): classify(), hasKeyword(), KEYWORD_RE, KEYWORDS, keywordsIn(), main(), ADR-0096, printReport() (+4 more)

### Community 589 - "Match"
Cohesion: 0.29
Nodes (3): Match, Service, fakeEnricher

### Community 590 - "CurationRow"
Cohesion: 0.31
Nodes (3): curationNorm(), CurationRow, Repo

### Community 591 - "coverArtManager"
Cohesion: 0.36
Nodes (7): assertDecodedWidth(), coverArtManager(), Manager, pngOfWidth(), TestWriteCoverArtTiersScaling(), TestWriteCoverArtTiersWithinBothCaps(), TestGenerateFrameRealFfmpeg()

### Community 592 - "Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)"
Cohesion: 0.15
Nodes (13): Behaviour notes, Bulk bar (`tags/+page.svelte`), Component: `WritebackBatchDialog.svelte`, Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239), Design-system fit (the `/design-system` check), Details card (`tags/[id]/+page.svelte`), Layout, Measured contrast (all three skins, dialog + card surfaces) (+5 more)

### Community 594 - ".resolveMovie"
Cohesion: 0.19
Nodes (14): disambiguate(), movieThumbURL(), rankConfidence(), TestDisambiguate(), TestRankConfidence(), TestSlugifyConcurrent(), TestTMDBMovieURL(), tmdbEntityURL() (+6 more)

### Community 595 - "filmEntityServer"
Cohesion: 0.38
Nodes (10): decodeFilmItems(), filmEntityServer(), seedPlainVideo(), seedStudio(), seedTag(), TestFilmVideoCandidates(), TestFullFilmVideoHiddenFromListSurfaces(), TestListFilmsForEntity() (+2 more)

### Community 596 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.17
Nodes (11): 2026-09-06 (later) · Max density raised to 8 columns; remodel now blocked on a conflict, 2026-09-06 (later still) · Density model decided: column counts kept, ladder to be extended, 2026-09-06 (later still) · Ladder extended, slider range made viewport-aware, 2026-09-06 · Premise corrected, direction chosen, design package committed, 2026-09-07 (later) · Film detail rail; shared stage-grid utility, 2026-09-07 (later still) · Stage-aligned Scenes grid, 2026-09-07 · Stage token + two-zone rail (items 4–6), Gates — definition of done (+3 more)

### Community 597 - "run"
Cohesion: 0.16
Nodes (25): ladderDemands, TestRun_IsRepeatable(), TestRun_RefusesADifferentSeed(), TestRun_RefusesAForeignDatabase(), loadShippedPersonas(), loadTestMapping(), TestEnrichPlanRefusesAMappingWithNoProviderSources(), TestEnrichPlanRefusesAMultiField() (+17 more)

### Community 598 - "Design Handoff: Extract from filename on the media detail page (F48.5a)"
Cohesion: 0.17
Nodes (12): Accessibility, Backend change, Design Handoff: Extract from filename on the media detail page (F48.5a), Goals, How this fits with provider enrichment, Interaction detail, Non-goals, Problem (+4 more)

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

### Community 604 - "header-narrow-width-handoff.md"
Cohesion: 0.24
Nodes (6): 2026-09-09 · all three causes fixed, harness armed, 2026-09-09 · PR #315 merged — worklog closed out, Gates — definition of done, HOLODEX-356 · the header nav at 768px, and an unbroken name on a detail page, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 605 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (10): 2026-09-08 · brainstormed, reframed, specced, 2026-09-08 · HOLODEX-343 — seeder skeleton, isolated DATA_PATH, safety guard, 2026-09-08 · HOLODEX-344 — ladder table, OFAT, reserved blocks, manifest, 2026-09-08 · HOLODEX-345 — adversarial image set across four kinds, 2026-09-08 · HOLODEX-346 — text palette across every free-text field, 2026-09-08 · HOLODEX-347 — relationship cardinality, films, scenes, film cast, 2026-09-08 · HOLODEX-350 — collection breadth, and the first bugs it found, 2026-09-09 · HOLODEX-348 — ten fake providers, and the half that had to be seeded (+2 more)

### Community 606 - ".scaleToWidth"
Cohesion: 0.22
Nodes (10): io.Reader, absPath(), Manager, lastLine(), scaleArgs(), seekSeconds(), TestScaleArgs(), TestSeekSeconds() (+2 more)

### Community 607 - "HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list"
Cohesion: 0.33
Nodes (5): 2026-08-30 · Fixed empty candidate list in the film→video bulk attach dialog, Gates — definition of done, HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 608 - ".partsFor"
Cohesion: 0.10
Nodes (11): EnrichQueueProviderState, ExtractionCandidate, Handlers, SplitJoined(), EnrichQueueRow, Repo, ExtractionQueueRow, Repo (+3 more)

### Community 609 - "NewService"
Cohesion: 0.47
Nodes (8): Store, newTestStore(), TestFetchAllowedImage_AllowedViaBaseHost(), TestFetchAllowedImage_AssetHostsExtendTheAllowlist(), TestFetchAllowedImage_IgnoresDisabledProvider(), TestFetchAllowedImage_RefusesUnlistedHost(), TestFetchAllowedImage_UnionAcrossMultipleProviders(), NewService()

### Community 610 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (9): 2026-09-06 · `/code-review high`: one confirmed finding, fixed; PR marked ready, 2026-09-06 · Design settled from a hand sketch; handoff + mockup committed, 2026-09-06 · HOLODEX-329: film chips never showed the film's poster, 2026-09-06 · Implemented the design; three-skin QA caught a real AA failure, 2026-09-06 · `/simplify` caught a real perf regression the implementation introduced, Gates — definition of done, HOLODEX-328 · Media detail Films + People: collapse empty sections, port the Scenes pill, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 618 - "Spec: Fire-and-forget writeback with page-level status"
Cohesion: 0.15
Nodes (13): Acceptance criteria, Context, Goals, Non-goals, Open items, R1 — The dialog closes on the enqueue acknowledgement, R2 — Writeback status is a property of the video, R3 — Failed jobs persist and are actionable (+5 more)

### Community 619 - "media-detail-metadata-fold-handoff.md"
Cohesion: 0.25
Nodes (6): 2026-09-06 · design-critique mockups → scoping → implementation → handoff, Deferred, Gates — definition of done, HOLODEX-325 · Extract ExpandableText shared component, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 620 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.25
Nodes (8): 2026-09-04 · ADR-090 — generalize the two-layer model, 2026-09-04 · design critique → mockup rev 2, 2026-09-04 · gap analysis → option chosen → gate artifacts, 2026-09-04 · implementation + live QA + simplify, Gates — definition of done, HOLODEX-194 · Extract from filename on the media detail page (F48.5a), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 621 - "Field"
Cohesion: 0.06
Nodes (33): promotionBody, promotionView, Handlers, Handlers, chi.Router, Handlers, chi.Router, hasNonEmpty() (+25 more)

### Community 622 - "3. QA"
Cohesion: 0.25
Nodes (8): 1. The problem, 2. The decision, 2a. Why not collapse the nav into a menu, 3. QA, Design handoff: the header at narrow widths, Human, Setup, Smoke

### Community 623 - "loadEnrichPlan"
Cohesion: 0.50
Nodes (7): enrichPlan, persona, dedupe(), enrichableFields(), loadEnrichPlan(), namesOf(), seedEnrichment()

### Community 624 - "QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)"
Cohesion: 0.29
Nodes (6): ADR-068: Extraction entity-field chip refinement (D2), §1 Setup, §2 Smoke, §3 Agent live QA (all 3 skins), §4 Human, QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)

### Community 625 - "Repo"
Cohesion: 0.32
Nodes (3): Repo, ProviderIcon, ProviderIconInsert

### Community 626 - "Design handoff — `ExpandableText` shared component"
Cohesion: 0.29
Nodes (7): Accessibility, Call sites (v1 — plain-text only), Component, Deferred (explicit scope decision), Design handoff — `ExpandableText` shared component, Options considered, Verification

### Community 628 - "HOLODEX-362.md"
Cohesion: 0.29
Nodes (5): 2026-09-10 · split button, built out of the chip pattern that already existed, Gates — definition of done, HOLODEX-362 · Split-button the Manage block so permanent delete is not adjacent to Move to Trash, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 629 - "HOLODEX-321 · Broken in-page anchors in the provider hand-off specs"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-09-05 · three dead anchors fixed; found by writing the checker properly the second time, Gates — definition of done, HOLODEX-321 · Broken in-page anchors in the provider hand-off specs, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 630 - "HOLODEX-326 · Film/Media detail pages: no way to edit a scene number after attach"
Cohesion: 0.29
Nodes (6): 2026-09-06 · `/code-review high --fix` caught two real interaction regressions, 2026-09-06 · Implemented, tested, and live-verified both edit surfaces, Gates — definition of done, HOLODEX-326 · Film/Media detail pages: no way to edit a scene number after attach, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 631 - "HOLODEX-320 · Media detail: move, trim, and fold the Metadata section"
Cohesion: 0.29
Nodes (6): 2026-09-05 · mockup review → implementation → handoff, Deferred, Gates — definition of done, HOLODEX-320 · Media detail: move, trim, and fold the Metadata section, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 632 - "TestClassifyCredential"
Cohesion: 0.80
Nodes (4): hexKey(), jwt(), TestClassifyCredential(), TestCredentialKindsAreMutuallyExclusive()

### Community 634 - "Spec: Provider link badge coverage — person, studio, film, and media (F63)"
Cohesion: 0.14
Nodes (14): Component map, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement, Requirements (+6 more)

### Community 635 - "refreshServer"
Cohesion: 0.57
Nodes (7): stubFileExtractor, refreshPOST(), refreshServer(), seedRefreshVideo(), TestRefreshEndpointDisabled(), TestRefreshEndpointRequiresOwner(), TestRefreshEndpointStatuses()

### Community 636 - "searchedCaption.ts"
Cohesion: 0.32
Nodes (6): CaptionHide, moreLabel(), searchedCaption, shown, two, ADR-0095

### Community 637 - "Alias state seed (F58 / ADR-088)"
Cohesion: 0.18
Nodes (9): Deterministic fixture corpus + golden-file pattern (testdata/gen.sh), Alias state seed (F58 / ADR-088), Notes, What it stages, Why it goes through the repo, not SQL, Golden files, Regenerate, Test Fixtures (+1 more)

### Community 638 - "HOLODEX-355 · `stage-grid`'s single-column branch omits the `minmax(0, …)` guard"
Cohesion: 0.29
Nodes (7): 2026-09-09 · `code-review max --fix` over the whole stacked branch, 2026-09-09 · fixed, measured before/after against the stress fixture, 2026-09-10 · `code-review xhigh` over PR #314, all 14 findings applied, Gates — definition of done, HOLODEX-355 · `stage-grid`'s single-column branch omits the `minmax(0, …)` guard, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 639 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.22
Nodes (8): 2026-09-07 (last) · xhigh code review — found the same bug from a second cause, 2026-09-07 (later) · Live-config warning (D5) + Jira sync, 2026-09-07 (later still) · Documentation sweep — and a gate I had wrongly closed, 2026-09-07 · Root cause found, fix + ADR + guard test landed, Gates — definition of done, HOLODEX-335 · Writeback read-back: pair the write target with a baseline source, make `in_sync` tri-state, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 640 - "HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`)"
Cohesion: 0.29
Nodes (6): 2026-09-14 · root-caused, harness stops on a dead server, docs, confirmed on Node 24.19, Findings (2026-09-13/14), Gates — definition of done, HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 642 - "HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster"
Cohesion: 0.33
Nodes (5): 2026-09-16 · diagnosed, fixed, tested, Gates — definition of done, HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 643 - "format.ts"
Cohesion: 0.06
Nodes (21): ImageCandidate, showThumb(), SLOT_CLASS, slotShape, batchId(), revert(), calculatedFrom(), filterByTitle() (+13 more)

### Community 644 - "HOLODEX-280 · Film poster/thumbnail asset pipeline"
Cohesion: 0.33
Nodes (6): 2026-07-10 · what happened this session, 2026-08-25 · full implementation + live verification, Gates — definition of done, HOLODEX-280 · Film poster/thumbnail asset pipeline, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 645 - "QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)"
Cohesion: 0.29
Nodes (6): §1 Setup, §2 Smoke (`make test`), §3 Agent (live, one skin), §4 Human (all three skins — Cinémathèque, Broadcast, Brutalist), §5 Known gaps, QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)

### Community 646 - "buildEnrichResponse"
Cohesion: 0.18
Nodes (12): buildCompanyEnrichResponse(), buildEnrichResponse(), TestBioTrimAtSentence(), TestBuildEnrichResponseCapsAt20(), TestBuildEnrichResponseFallsBackToProfilePath(), TestBuildEnrichResponseMultiplePhotos(), TestBuildEnrichResponseSkipsEmptyFilePath(), TestDescribeAssetKindsCoverEveryEmittedKind() (+4 more)

### Community 647 - "HOLODEX-284 · Film provider enrichment (ADR-086)"
Cohesion: 0.33
Nodes (5): 2026-08-27 · ADR-086 implementation + code-review pass, Gates — definition of done, HOLODEX-284 · Film provider enrichment (ADR-086), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 648 - "HOLODEX-341 · Local dev credentials come from the environment; CI enforces it"
Cohesion: 0.33
Nodes (5): 2026-09-08 (last) · verified the premise, then hardened the mechanism, Gates — definition of done, HOLODEX-341 · Local dev credentials come from the environment; CI enforces it, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 649 - "Person"
Cohesion: 0.11
Nodes (9): FilmBilledCredit, holodex/internal/model.PersonAlias, Handlers, idOrZero(), normalizedName(), Person, Repo, resolveOrCreatePerson() (+1 more)

### Community 650 - "HOLODEX-358 · a long field value still widens the media page below ~440px"
Cohesion: 0.33
Nodes (6): 2026-09-09 · `code-review medium --fix` over PR #315, both findings applied, 2026-09-09 · PR #316 merged — worklog closed out, Gates — definition of done, HOLODEX-358 · a long field value still widens the media page below ~440px, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 651 - "HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram"
Cohesion: 0.33
Nodes (5): 2026-09-14 · diagnosed, fixed, live-verified, Gates — definition of done, HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 652 - "HOLODEX-386 · Refuse a portrait image for the film banner role (landscape guard)"
Cohesion: 0.33
Nodes (5): 2026-09-16 · implemented, tested, Gates — definition of done, HOLODEX-386 · Refuse a portrait image for the film banner role (landscape guard), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 654 - "Spec: Dev-time stress fixture for UX and layout QA"
Cohesion: 0.22
Nodes (9): Acceptance criteria, In scope, Ladder dimensions, Objective, Open questions, Out of scope, Related, Scope (+1 more)

### Community 655 - "HOLODEX-370 · Clear / Dismiss never revert a written-back provider — say so, point at Revert"
Cohesion: 0.33
Nodes (5): 2026-09-11 · premise corrected, option A built and verified, Gates — definition of done, HOLODEX-370 · Clear / Dismiss never revert a written-back provider — say so, point at Revert, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 656 - "HOLODEX-380 · F61 — `candidates[].detail`: revealable per-candidate record summary"
Cohesion: 0.33
Nodes (5): 2026-09-13 · proposal reviewed, decisions locked, spec + contract amendment written, Gates — definition of done, HOLODEX-380 · F61 — `candidates[].detail`: revealable per-candidate record summary, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 657 - "Design handoff: StudioLinkCard draws the studio logo"
Cohesion: 0.15
Nodes (13): 10. Out of scope (deliberately) and follow-ups, 11. QA checklist, 1. Resolved decisions, 2. Component change: `StudioLinkCard.svelte`, 3. Call sites — unchanged, 4. Backend — unchanged, 5. Design tokens used, 6. States (+5 more)

### Community 658 - "navSearch.svelte.ts"
Cohesion: 0.24
Nodes (5): navSearch, pageScopeFor(), SEARCH_TABS, SearchTab, SearchResponse

### Community 659 - "urlParamID"
Cohesion: 0.16
Nodes (7): Handlers, chi.Router, Handlers, chi.Router, writeSceneCollisionConflict(), urlParamID(), routeKind()

### Community 660 - "Store"
Cohesion: 0.25
Nodes (8): PortraitBannerError, CheckRoleAspect(), Find(), Remove(), Store(), TestCheckRoleAspect(), TestFind_DelegatesToEntityImage(), TestStoreRemove_Delegates()

### Community 661 - "QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore)"
Cohesion: 0.29
Nodes (6): Agent (verified this session via DOM inspection), F25.29 — Post-enrichment image freshness, Human (needs eyes — not capturable in the headless preview), QA Checklist: Person-page polish (parallax banner · inline poster · list scroll-restore), Setup, Smoke

### Community 662 - "newSourceURLService"
Cohesion: 0.52
Nodes (6): Service, newSourceURLService(), sourceURLRow(), TestEnrichSourceURL_OmittedKeepsMalformedClears(), TestEnrichSourceURL_StoresFirstValidValue(), TestProviderLink_Precedence()

### Community 663 - "downloadImageToTemp"
Cohesion: 0.62
Nodes (6): TestDownloadImageToTemp_PropagatesFetcherRefusal(), TestDownloadImageToTemp_RefusesNonHTTPS(), TestDownloadImageToTemp_RefusesWithNoFetcherConfigured(), TestDownloadImageToTemp_WritesAllowedBytesToTemp(), withImageFetcher(), downloadImageToTemp()

### Community 664 - "Enrich stub — fake metadata-source providers for manual QA"
Cohesion: 0.33
Nodes (6): Enrich stub — fake metadata-source providers for manual QA, Reaching each layer, Start it, Ten providers, one process, The bare routes still serve the original persona, Wire it to a running backend

### Community 665 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (9): 2026-09-15 · design gate, 2026-09-16 · frontend + live verification, 2026-09-16 · human QA pass → ready for review, 2026-09-16 · post-ready code-review medium --fix, 2026-09-16 · session, Gates — definition of done, HOLODEX-384 · Entity Films row: card-height match + hover lift, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 669 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (10): 2026-09-16 · brainstorm → epic + 4 stories → spec, 2026-09-16 · HOLODEX-391 sidecar templates + homepage unwind, 2026-09-17 · HOLODEX-392 `_source_url` ingest + per-pill fallback, 2026-09-17 · HOLODEX-393 film badge (design gate + code), 2026-09-17 · HOLODEX-394 media badge (backend + frontend), 2026-09-17 · website unwind folded into 391 → ADR-098, Gates — definition of done, HOLODEX-390 · Provider link badge coverage (F63) (+2 more)

### Community 670 - "resolvedByCanonical"
Cohesion: 0.23
Nodes (17): NewFilmBaseline(), filmTestFields(), TestFilmBaseline_NameResolvesFromRecord(), TestFilmBaseline_NilFilmIsEmptyBaseline(), TestFilmBaseline_RD6Additivity(), TestFilmBaseline_RecordBlankPinSuppressesProvider(), collectionField(), TestReplaceMarkers_FilmSourceOffersCandidateNamedAfterFilm() (+9 more)

### Community 671 - "NewStudioBaseline"
Cohesion: 0.42
Nodes (7): NewStudioBaseline(), studioTestFields(), TestStudioBaseline_NameResolvesFromRecord(), TestStudioBaseline_NilStudioIsEmptyBaseline(), TestStudioBaseline_RD6Additivity(), TestStudioBaseline_RecordBlankPinSuppressesProvider(), studioBaseline

### Community 672 - "HOLODEX-247 · Studio image roles: icon, logo, poster (F51)"
Cohesion: 0.33
Nodes (6): 2026-07-10 · what happened this session, 2026-08-04 · Full epic delivered end-to-end and merged, Gates — definition of done, HOLODEX-247 · Studio image roles: icon, logo, poster (F51), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 673 - "HOLODEX-413 · Writeback (ffmpeg path) attaches PNG covers as image/jpeg and never replaces cover.jpg"
Cohesion: 0.33
Nodes (5): 2026-09-18 · diagnosed, filed, fixed, verified live, Gates — definition of done, HOLODEX-413 · Writeback (ffmpeg path) attaches PNG covers as image/jpeg and never replaces cover.jpg, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 674 - "Decision"
Cohesion: 0.25
Nodes (8): ADR-097: Alpha-preserving image normalization — PNG for non-opaque images, extension derived from the bytes, Consequences, Context, D1 — Normalize keeps alpha: PNG when the decoded image is not opaque, JPEG otherwise, D2 — The on-disk extension is derived from the bytes, never stored, D3 — Serve with the Content-Type of the file that was found, D4 — Rejected alternatives, Decision

### Community 675 - "HOLODEX-406 · F64 — `candidates[].image_url`: candidate thumbnail in the resolve picker"
Cohesion: 0.25
Nodes (7): 2026-09-16 → 17 · brainstorm, story filed, spec + contract amendment written, 2026-09-17 · design handoff, 2026-09-17 · security review, §4.6 decided, mark ready, Gates — definition of done, HOLODEX-406 · F64 — `candidates[].image_url`: candidate thumbnail in the resolve picker, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 676 - "HOLODEX-415 · Replaced person poster stays stale on the Cast grids"
Cohesion: 0.33
Nodes (5): 2026-09-18 · diagnosed, reproduced, fixed, Gates — definition of done, HOLODEX-415 · Replaced person poster stays stale on the Cast grids, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 678 - "ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary)"
Cohesion: 0.29
Nodes (7): ADR-005: MCP Server Transport — HTTP/SSE (Primary) + stdio (Secondary), Client Configuration Examples, Configuration, Consequences, Context, Decision, Rationale

### Community 679 - "HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-09-05 · the guard landed and immediately caught a kind nobody had noticed, Gates — definition of done, HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 680 - "HOLODEX-397 · Studio logo on the Film and Media detail pages"
Cohesion: 0.29
Nodes (6): 2026-09-16 · design gate, 2026-09-17 · implemented, agent-QA'd, Gates — definition of done, HOLODEX-397 · Studio logo on the Film and Media detail pages, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 681 - "UI Vocabulary"
Cohesion: 0.29
Nodes (7): Audiences and branches, Content and chrome, Control patterns (by name), Field model, Saying it, Translations, UI Vocabulary

### Community 684 - "Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)"
Cohesion: 0.33
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43)

### Community 685 - "QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)"
Cohesion: 0.33
Nodes (6): 1. Overlay on playback (media detail page), 2. Search-history dropdown (header), 3. "More with …" shelves (media detail page), 4. Fluid Back (browse grid), 5. Cross-cutting, QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)

### Community 686 - "HOLODEX-283 · Films: real backend search integration"
Cohesion: 0.33
Nodes (5): 2026-08-27 · session, Gates — definition of done, HOLODEX-283 · Films: real backend search integration, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 687 - "HOLODEX-396 · Transparent entity images lose their alpha channel on upload"
Cohesion: 0.33
Nodes (5): 2026-09-16 · traced, fixed, tested, ADR written, Gates — definition of done, HOLODEX-396 · Transparent entity images lose their alpha channel on upload, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 688 - "HOLODEX-407 · F-number claims — `scripts/feature-claims.mjs`"
Cohesion: 0.33
Nodes (5): 2026-09-17 · branch cut, script + tests + wiring shipped, Gates — definition of done, HOLODEX-407 · F-number claims — `scripts/feature-claims.mjs`, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 689 - "HOLODEX-411 · Studio logo sits bare on the page background"
Cohesion: 0.33
Nodes (5): 2026-09-18 · filed, decided, implemented, QA'd, Gates — definition of done, HOLODEX-411 · Studio logo sits bare on the page background, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 690 - "HOLODEX-414 · Candidate slot shape per entity kind (media backdrop, studio logo)"
Cohesion: 0.33
Nodes (5): 2026-09-18 · session, Gates — definition of done, HOLODEX-414 · Candidate slot shape per entity kind (media backdrop, studio logo), Session log, Up next

### Community 692 - "Manual QA Checklist: Owner tooling hub + nav split (F35)"
Cohesion: 0.40
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: Owner tooling hub + nav split (F35)

### Community 693 - "Manual QA Checklist: People Images (F25)"
Cohesion: 0.40
Nodes (5): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (all three skins), Manual QA Checklist: People Images (F25)

### Community 694 - "gateTestHandlers"
Cohesion: 0.67
Nodes (3): gateTestHandlers(), Handlers, TestGateImageURL_MergedField()

### Community 697 - "applyGenreWriteback"
Cohesion: 0.67
Nodes (3): applyGenreWriteback(), genreWritebackFieldValues(), LabelAndDisplay()

## Knowledge Gaps
- **2985 isolated node(s):** `$schema`, `PreToolUse`, `holodex`, `Handlers`, `categoryTagIDsBody` (+2980 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 3436 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **231 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Canonical Field Registry (operator reference)` connect `ADR-090-two-layer-entity-metadata-management.md` to `Spec: Tag governance & video enrichment (F50)`, `Spec: Holodex Metadata Provider Contract (hand-off, protocol v1)`, `Configuration Reference (holodex.yaml layers)`, `Field`, `architecture/README.md`, `Spec: Derived/calculated person fields (F45)`?**
  _High betweenness centrality (0.366) - this node is a cross-community bridge._
- **Why does `Lookup()` connect `Field` to `service.go`, `Derive`, `Handlers`, `ResolvedField`, `ResolveFields`, `AutoRegisterFields`, `applyGenreWriteback`, `Complete`, `SanitizeValue`, `Mappings`?**
  _High betweenness centrality (0.254) - this node is a cross-community bridge._
- **Why does `TestCriticality()` connect `Field` to `testing.T`?**
  _High betweenness centrality (0.157) - this node is a cross-community bridge._
- **Are the 180 inferred relationships involving `newRepo()` (e.g. with `TestAliasesSurviveRescan()` and `TestMergePersons()`) actually correct?**
  _`newRepo()` has 180 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `PreToolUse`, `holodex` to the rest of the system?**
  _2985 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `types.ts` be split into smaller, more focused modules?**
  _Cohesion score 0.016145106637432727 - nodes in this community are weakly interconnected._
- **Should `media/[id]/+page.svelte` be split into smaller, more focused modules?**
  _Cohesion score 0.03374578177727784 - nodes in this community are weakly interconnected._