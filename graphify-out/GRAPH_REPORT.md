# Graph Report - holodex  (2026-10-18)

## Corpus Check
- 1598 files · ~2,213,073 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 19 file(s) not represented in the graph (top: (none) 8, .example 6, .provider-tmdb 1)

## Summary
- 14984 nodes · 70753 edges · 520 communities (386 shown, 134 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 7530 edges (avg confidence: 0.91)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `bd9007a9`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- types.ts
- people/[id]/+page.svelte
- whats-left.mjs
- deployment.md
- status/+page.svelte
- testing.T
- 2026-09-06 · implementation landed, verified live against real media
- time.Duration
- Holodex Project Working Agreements (CLAUDE.md)
- testMappings
- devDependencies
- time.Time
- imagetools.mjs
- newProviderIconEnv
- NameEditControl.svelte
- Service
- resolver.go
- writeback/writeback.go
- Handlers
- ResolveReviewAction
- Scanner
- ExtraMetadata
- placeholders
- Design handoff: StudioLinkCard (reusable Studio display)
- Design handoff: `?` keyboard-shortcuts sheet + hotkey firing feedback (F62)
- enrich/enrich_test.go
- Design Handoff: Unified name-edit mechanism (HOLODEX-269)
- tmdb.go
- .requireOwner
- StudioPicker.svelte
- handler
- Spec: Age-in-media (cross-entity derived field)
- media-page-extraction-handoff.md
- Server
- Spec: Tag governance & video enrichment (F50)
- extractor.go
- resolver_test.go
- Index
- api.ts
- people/+page.svelte
- jira-sync.mjs
- q
- args_test.go
- .RelinkVideoStudios
- Repo
- density.svelte.ts
- Queue
- Spec: Duplicates pair evidence — expand a flagged pair into a symmetric side-by-side compare panel (F70)
- imageVariant
- generate.mjs
- ADR-064-flightplan-plugin.md
- Video composite-key collision check (F56.3, HOLODEX-270)
- #lib
- nav.mjs
- stack.md
- matroska.go
- openAt
- Action Items
- tmdb_test.go
- newServer
- Manager
- SanitizeValue
- HeldPath
- Mappings
- extraction/+page.svelte
- WritebackFormDialog.svelte
- EntityImageSlot.svelte
- RunJumpPopover.svelte
- WriteBatch
- newRepoDB
- Video
- PopoverMenu
- navSnapshot.svelte.ts
- Quick Wins batch (overlay fix, search history, related shelves, fluid Back)
- ADR-058 (Jira transitions via direct REST API)
- TagAddInput.svelte
- commit-type-scope-check.mjs
- Spec: Sticky sort preferences + Random sort
- genre_integration_test.go
- NewService
- run.mjs
- PromoteFieldEditor.svelte
- Spec: Tag Detail — Hierarchy & Category Controls
- process.go
- Design Handoff: Shared Modal primitive (HOLODEX-215)
- media/[id]/+page.svelte
- itoa
- toMessage
- Normalize
- Decision
- QA Checklist: System Activity (F21)
- ADR-070: Canary the release candidate, then promote it by retagging the same digest
- video-file-maintenance.md
- generate
- verify.go
- planCarry
- Design handoff: Films entity (F56)
- flip_integration_test.go
- New
- ref_node_url
- .mergeEntity
- Design handoff: Media detail — Films + People sections
- deleted_at soft-delete column (orthogonal to active)
- Spec: Tag & Category Create Affordance — closing the /tags creation gap
- BatchRunner
- Spec: Tag Categories — grouping tags without merging them
- requireCoverTools
- PersonPicker.svelte
- Decision
- itoa
- Observability and jobs
- Spec: Duplicate videos — find two files of one scene, compare them, keep the right one (F76)
- Decision
- Options Considered
- Design handoff: completeness ring badge (F65)
- Ref
- ADR-066: Enrichment auto-apply-with-revert — provider-contract posture change + durable `enrichment_dismissals` store
- compilerOptions
- Design Handoff: Person Aliases ("Also known as") (F23)
- CompletenessRow
- preflight.go
- flip_mkv_integration_test.go
- Design handoff: Film provider enrichment (F59)
- genreWritebackServer
- Action Items
- Design handoff: Media detail stage layout
- showcase-demo-corpus.md
- Store
- ADR-039: Provider asset URLs — contract clarification + operator-configured asset-host allowlist
- Design handoff: Media detail Metadata — move, trim, fold
- Complete
- PersonLinkChip.svelte
- routes/+layout.svelte
- Gates — definition of done
- newCompletenessHandlers
- Spec: Holodex Metadata Provider Contract (hand-off, protocol v1)
- ADR-058: Jira issue transitions via REST API (retire the Automation rules)
- context.Context
- Swapper
- web/package.json
- ADR-067: Filename-based metadata extraction with confidence-gated auto-apply and rollback
- Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)
- Handoff Spec: Film Details parity (HOLODEX-95)
- Handoff Spec: Smart playlists, Play all and Shuffle (F75)
- NewService
- Handoff: Session-expired banner — hold the reload while editing
- Options Considered
- Design handoff: Person Details fold
- Handlers
- confidence.go
- ActivityState
- Design handoff — Entity identity card (F60)
- worklog-gate.mjs
- Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)
- title
- Design Handoff: Tag & category create affordance (HOLODEX-243)
- detail
- ADR-009: Cover Art & Thumbnail Strategy — Tiered Extraction with Throttled, Priority-Aware Backfill
- Decision
- Design handoff: Film Studio cascade edit affordance (F57)
- QA checklist: Responsive page width (HOLODEX-331)
- resolveOrCreateByName
- Geometry assertion harness (HOLODEX-349)
- ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field
- Unified name-edit mechanism (F56.2, HOLODEX-269)
- Write
- .maybeRenewSession
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Instance skin — Appearance tab & header picker removal — design handoff
- Session log — newest first (cap: last 8 sessions; older → archive/)
- Find
- feature-claims.mjs
- demo/package.json
- newFakeRepo
- settings.json
- .getPersonCard
- ADR-011: Symlink Handling & Path Resolution
- Decision
- Options Considered
- ADR-021-frontend-theming-and-skins.md
- 2026-08-08 · Backend D4 — list-wide resolve-all predicate for browse sort/filter + remediation queue
- .setFieldDecision
- Field
- Spec: People Images (F25)
- F38: Studio entity pages
- stub.js
- ADR-115: Cinémathèque is the only look — retire Broadcast, Brutalist, the custom palette and the skin setting
- Design handoff — Media parts (`part` badge and pill)
- Design handoff: Media page tag input (Enter, autocomplete, status sort)
- /owner/duplicates dense pair-row review queue
- Design handoff — `SourceEditModal` at paragraph length (HOLODEX-417)
- reference/ui-vocabulary.md
- Session log — append-only (cap: last 8 sessions; older → archive/)
- goto
- Spec: Collapse provider aliases into the canonical alias spine (F58)
- ref_node_test
- .compareVideoDuplicate
- Decision
- Options Considered
- ADR-114: One list-state model — the URL holds what you see, storage holds only preferences
- Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating
- Design handoff: no plate under an image — bare + halo, plate only under the monogram
- gen.sh
- Design handoff: Entity Films row — card-height match + hover lift
- TagForField
- net/http/httptest.Server
- Design handoff: Media detail page reorder
- .RelinkProviderIcon
- Design Handoff: People attach/detach + relationship picker (HOLODEX-272)
- HOLODEX-287 · Frontend component-reuse decision as a topic doc
- FacetGroup
- ADR-017: Search Architecture — Global Mixed-Entity Search + FTS5
- Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)
- Options Considered
- newRepo
- app.d.ts
- Q1 — how a film competes as a resolver source
- +layout.ts
- HOLODEX-405 · Page-scoped hotkeys (F62)
- Design handoff: studio image halo toggle (HOLODEX-463)
- scripts
- HOLODEX-404 · `/?q=` deep link keeps its search on Media
- HOLODEX-464 · MP4 genre writeback keeps only the last tag
- HOLODEX-465 · A genres writeback makes the file's tags match the UI
- Screenshot: Video detail page in the Cinémathèque skin — shows player, title card, metadata, people, and tags for a sample video 'Nightshade'
- HOLODEX-477 · List scroll position is lost across a full page reload
- HOLODEX-498 · Stress fixture seeds container "mp4", so no fixture video ever gets a write target
- Screenshot: Holodex browse/grid view in the 'Cinémathèque' skin. Dark theme with warm amber/gold accents. Top bar shows 'Holodex' logo, global 'Search everything... (Ctrl-K)' box, nav links (Media, People, Tags), and a three-way skin switcher (Cinémathèque selected/amber, Broadcast/blue, Brutalist/green). Filter row includes title search, Resolution toggle (All/SD/HD/FHD/4K), Duration (min) range, Year range, People multiselect, and Tags multiselect. Results header reads '18 videos'. Grid of movie-poster style cards (3 rows x 6 columns) each showing a colored gradient thumbnail with a resolution badge (4K/HD/FHD/SD) top-left, duration badge (e.g. 1:36:40) bottom-right, title overlay, and below the card a title repeated plus genre/tag chips (e.g. Vantablack - Horror, Thriller; Tin Soldier - Drama, War; The Long Saturday - Comedy; The Quiet Coast - Drama; The Cartographer - Drama; Static Bloom - Experimental, Short; Solar Drift - Adventure, Sci-Fi; Overgrowth - Documentary, Nature; Paper Moons - Animation, Family; Nightshade - Noir, Thriller; Neon Tide - Crime, Noir; Migration - Documentary, Nature; Harbor Lights - Drama, Romance; Glasshouse - Mystery, Thriller; Dust & Echoes - Drama, Western; Concrete Garden - Documentary; Amélie en Hiver - Drama, Romance; Ferrous - Sci-Fi, Thriller).
- Dependabot Config
- Deploy Landing Page Workflow
- HOLODEX-536 · MKV tag target-level selection
- HOLODEX-548 · Only uploaded images keep an orphan from the sweep
- HOLODEX-59 · Rename internal adminMode → ownerView
- SourceValueClamp.svelte
- .getMedia
- Decision (option A, approved 2026-09-27)
- HOLODEX-417 · Edit Description modal pushes Save/Cancel off screen when sources are long
- HOLODEX-330 · VideoCard's scene badge can fail AA contrast over a bright thumbnail
- HOLODEX-425 · F67 Instance skin — owner-set skin on an Appearance tab + custom palette
- HOLODEX-449 · CodeQL config — exclude test files from analysis
- HOLODEX-459 · Person profile never scrolls to the hover card's #videos / #films anchors
- HOLODEX-463 · Per-studio owner toggle for the studio image halo
- HOLODEX-478 · Trashed video lingers in the Media grid after the Back exit
- HOLODEX-479 · Writeback refuses fragmented MP4 with a remux hint
- HOLODEX-486 · Bundle MKVToolNix in the runtime image
- .downloadAssets
- HOLODEX-497 · Stress fixture: a replace-studio variant for exercising studio decisions
- holodex
- HOLODEX-507 · Tag aliases: tag page panel, alias search, lowercase alias text
- TMDB Brand Logo (tmdb-brand.png)
- HOLODEX-513 · The linked film decides a video's Film and Title by default
- HOLODEX-514 · Film-derived values beyond the media page (grids + picker)
- Detail page screenshot (Cinémathèque skin) — Holodex showcase site
- HOLODEX-515 · Completeness counts film-derived Film/Title
- HOLODEX-524 · Rules and gates for living spec / architecture / design docs
- Screenshot: Holodex browse/grid view in the 'Cinémathèque' skin, showing a 18-video poster grid with search, resolution/duration/year filters, and people/tag filters
- holodex/main.go
- extraction/CLAUDE.md
- ADR-014: Configuration and Data Layout
- Age-in-media derived field
- chips render mode (read-only pill list)
- system-activity.md
- HOLODEX-525 · Teach the doc skills their "never contains" boundaries
- F36: Per-field source-of-truth
- HOLODEX-528 · Retire the numbered ADRs
- F39: Provider render hints / auto-registered non-canonical fields
- F44: In-app promote/override affordance
- Spec: Studio as a first-class entity (F38)
- HOLODEX-529 · Media detail delete dialog names a different video during playlist playback
- HOLODEX-531 · Topic docs batch 1 (stack, config-and-settings, deployment, security-perimeter)
- Design Handoff: Owner tooling hub + nav split (F35)
- F47: Enrichment review workflow
- F48: On-demand metadata extraction from filenames & tags
- F49: Claimed provider keys
- HOLODEX-533 · Topic docs batch 2 (field-resolution, metadata-providers, entity-identity)
- Flightplan session-state plugin
- Job history digest, pagination, entity search
- Leading logo well (studio list monogram/logo)
- needsWriteback(field) selection predicate
- Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31)
- Three-skin QA contract (Cinémathèque · Broadcast · Brutalist, tokens-only)
- ADR-001: Backend
- Spec: Runtime owner-editable settings (F41)
- ADR-002: Frontend
- HOLODEX-534 · Topic docs batch 3 (entity-relationships, images, media-ingest)
- HOLODEX-537 · Topic docs batch 4 (writeback, observability-and-jobs, browse-and-list-state)
- HOLODEX-538 · Activity history paging: correct the topic doc
- ADR-018: Scan change detection
- ADR-019: Observability
- HOLODEX-540 · Rule move batch 1 (foundation ADR leftovers)
- Jira HOLODEX-166 (System Activity epic)
- Design handoff: Studio image roles — icon / logo / poster (F51)
- HOLODEX-541 · Rule move batches 2–4 (remaining ADR leftovers)
- HOLODEX-542 · Specs and design docs still describe retired skins and contain self-contradictions
- HOLODEX-544 · Make Trivy's SARIF upload honour the CRITICAL,HIGH severity filter
- provider
- HOLODEX-545 · Hotfix release lane: patch a shipped minor from release/vX.Y without releasing main
- HOLODEX-546 · Enforce the merge gate: activate the main ruleset with current checks, a bypass, and release/** coverage
- SweepRunner
- HOLODEX-547 · Permanent delete leaves a video's edits behind
- .showcase.md Portfolio Self-Report
- HOLODEX-549 · Sweep remaining retired-skin references and dead SourceSelect out of docs and code
- HOLODEX-550 · jira-sync docs-only guard strands docs tickets
- HOLODEX-551 · A person rename reaches the video files
- HOLODEX-552 · Person merge writes each credit to its own field
- dismissable
- HOLODEX-553 · A studio rename reaches the video files
- HOLODEX-557 · Tags automatically get a dashed alias for their multi-word name
- HOLODEX-572 · Restored item missing from the Media grid after returning from Trash
- HOLODEX-573 · List scroll snapshot restores on a plain link arrival, not only on Back
- HOLODEX-574 · Nav harness back-after-write-refreshes-grid hangs in route.fetch on Windows
- HOLODEX-582 · One shared Details section for the film and studio pages
- HOLODEX-583 · Person image crop squashes the saved image horizontally on short viewports
- Session log — append-only (cap: last 8 sessions; older → archive/)
- CurationFieldRow.svelte
- Spec: Media parts — denote the files of a multi-file media (`part`)
- HOLODEX-585 · Auto-refresh the committed graphify graph via a bot PR on push to main
- placeholder.go
- Media page restructure — one sync verb, render once
- ADR-010: MKV (Matroska) Tag-Target Precedence
- ADR-026: Prometheus metrics — hand-rolled exposition, no client library
- Options considered
- Film banner landscape guard — design handoff
- assetRoleFor
- 2. Layout
- HOLODEX-403 · Poster row as an image-tile chooser + ledger-witnessed image sync
- HOLODEX-418 · "Re-match…" auto-applies a lone strong candidate instead of showing the picker
- HOLODEX-43 · Owner-view provenance badge on core images
- HOLODEX-484 · MKV cover writeback fails when the existing cover has a different filename
- HOLODEX-485 · mkvpropedit cover path leaves a second cover.* attachment behind
- HOLODEX-489 · Read-back gap: ledger witness + owner-visible gap
- HOLODEX-490 · "+ Add overview" and "+ Set part" diverge from the page's "+ Add …" text CTA
- HOLODEX-491 · "+ Set edition": film-page text CTA, media-page header slot; "Add film" ghost tile
- HOLODEX-492 · Stress testbed: commit the :9300 preview entries and map `edition` in the fixture
- manifest.mjs
- HOLODEX-499 · Dev environment never emits audio
- HOLODEX-509 · Unattached-tag aliases written canonically in genre writeback
- HOLODEX-532 · Speed up the multi-arch image build
- HOLODEX-566 · Migrate web imports from $lib to #lib subpath imports
- HOLODEX-569 · Flip horizontally fails verification on real MP4s
- HOLODEX-570 · Flip drops cover art on MP4s carrying duplicate covr items
- HOLODEX-571 · hflip still fails verification on MP4s with no compressor name
- maintenance/snapshot_test.go
- idNamespaceFor
- ADR-027: Local `.env` loading for development config
- QA Checklist: Candidate slot shape per entity kind (HOLODEX-414)
- personimage.go
- HOLODEX-262 · Browse-page completeness sort + missing-facet filter chip
- Behavior detail
- Spec — Showcase Demo Corpus
- mkvTagsDoc
- go_pkg_io_fs
- States and interactions
- Requirements
- canon
- Handlers
- Archived ADRs
- Implementation notes
- fakeRescanner
- .SetBootDrain
- Admin mode header toggle (presentation-only gate)
- Age-in-media corner badge on cast poster card
- "Attach to…" pill + picker (F49 claim action)
- "Attached keys" owner-tooling list (/owner/fields)
- ClaimFieldEditor with outcome-before-commit copy
- rules/doc-types.md
- CodeQL Workflow
- routes/+page.svelte
- Trash view (/trash) with Restore / Delete permanently
- Derived Age / Age-at-death row, tooltip-only provenance
- EnrichProviderChips Refresh/Re-match/Clear split
- /owner/enrichment review queue tab
- /tags pill-native manage mode (rename/alias/merge)
- CurationChip radio mode (shared shell, dot vs ✕ glyph)
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
- completeness_boot.go
- ADR-030 (access control gating seam)
- Owner tooling hub (F35) follow-up rename
- Admin mode header toggle (visitor-preview, complete hide set)
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
- entity-completeness-score.md
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
- PlaceholderImage.svelte
- web/src/lib/peopleScroll.svelte.ts
- web/src/lib/searchHistory.ts
- SourceBadge.svelte
- Load
- ReadbackGaps
- holoShuffle
- cascadeServer
- extractReviewServer
- Spec: Quick Wins batch — Search history & "More with …" shelves
- Design Handoff: Poster View for the People list page (F55)
- Spec: Metadata Source Plugins (F22/F27/F28)
- Design Handoff: Provider Link Badge — Multi-Badge States for Person/Studio (HOLODEX-266)
- Session log — append-only (cap: last 8 sessions; older → archive/)
- net/http.ResponseWriter
- Session log — append-only (cap: last 8 sessions; older → archive/)
- Design Handoff: Studio relationship-edit popover (HOLODEX-271)
- HOLODEX-288 · Fix film-studio cascade code-review findings
- Design handoff: Responsive page width — player column, metadata rail, intrinsic grid density
- Spec: Owner tooling hub + visitor/owner nav split (F35)
- tag-set-writeback-handoff.md
- New
- .TagNamesForVideo
- Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369)
- HOLODEX-255 · <epic title>
- archive/README.md
- HOLODEX-114 · <epic title>
- HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values
- HOLODEX-244 · <epic title>
- HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip
- toSearchItem
- Design handoff: Manage block — destructive split button
- metrics.go
- Spec: Instance skin — owner-set skin on an Appearance tab + custom palette from `holodex.yaml` (F67)
- QA: TMDB Provider Sidecar + ADR-039 Core Changes
- films/[id]/+page.svelte
- CompletenessRing.svelte
- probe-edition-filenames.mjs
- coverart_test.go
- tmdbClient
- Design Handoff: Extract from filename on the media detail page (F48.5a)
- HOLODEX-298 · Film detail page: match media page's Tags section styling
- HOLODEX-302 · Person hero image hover-to-front
- HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise
- header-narrow-width-handoff.md
- images_test.go
- File maintenance
- HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list
- FilmBulkAttachDialog.svelte
- .promotionTarget
- Design Handoff: Delete a film (F78)
- HOLODEX-321 · Broken in-page anchors in the provider hand-off specs
- HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`)
- HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster
- EnrichPicker.svelte
- QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)
- HOLODEX-284 · Film provider enrichment (ADR-086)
- tmdb/main.go
- EnrichmentRow
- HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram
- Spec: Dev-time stress fixture for UX and layout QA
- Studio
- urlParamID
- ADR-038-person-images.md
- Session log — append-only (cap: last 8 sessions; older → archive/)
- HOLODEX-247 · Studio image roles: icon, logo, poster (F51)
- HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`
- HOLODEX-397 · Studio logo on the Film and Media detail pages
- Handlers

## God Nodes (most connected - your core abstractions)
1. `newRepo()` - 238 edges
2. `Repo` - 224 edges
3. `toMessage()` - 222 edges
4. `itoa()` - 219 edges
5. `sampleVideo()` - 153 edges
6. `writeError()` - 152 edges
7. `title()` - 141 edges
8. `Open()` - 115 edges
9. `New()` - 115 edges
10. `writeJSON()` - 113 edges

## Surprising Connections (you probably didn't know these)
- `Gates — definition of done` --references--> `TestEntityCompletenessSummary()`  [INFERRED]
  docs/plans/HOLODEX-435.md → internal/api/completeness_store_test.go
- `D3 — Serve with the Content-Type of the file that was found` --references--> `serveEntityImageFile()`  [INFERRED]
  docs/architecture/archive/ADR-097-alpha-preserving-image-normalization.md → internal/api/entity_images.go
- `D5: The list payload carries `completeness` for the owner; the badge rides it` --references--> `redactFileMetadataForVisitors()`  [INFERRED]
  docs/architecture/archive/ADR-099-completeness-score-required-band.md → internal/api/handlers.go
- `Access control & security` --references--> `redactFileMetadataForVisitors()`  [INFERRED]
  docs/specs/entity-completeness-score.md → internal/api/handlers.go
- `2026-09-13 · 374 reference handle — coded, tested, live-QA'd` --references--> `pathID()`  [INFERRED]
  docs/plans/HOLODEX-373.md → internal/api/handlers.go

## Import Cycles
- 4-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 4-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 4-file cycle: `internal/api/duplicates_test.go -> internal/api/duplicates_video_test.go -> internal/api/handlers.go -> internal/metadata/people_link_e2e_test.go -> internal/api/duplicates_test.go`
- 4-file cycle: `internal/api/duplicates_test.go -> internal/api/duplicates_video_test.go -> internal/api/smart_playlists.go -> internal/metadata/people_link_e2e_test.go -> internal/api/duplicates_test.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/activity_dismiss_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/aliases_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/auth_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/categories_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/completeness_browse_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/completeness_detail_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/curation_collision_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/decisions_collision_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/decisions_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart.go -> internal/metadata/people_link_e2e_test.go -> internal/api/delete_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/activity_dismiss_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/aliases_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/auth_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/categories_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/completeness_browse_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`
- 5-file cycle: `internal/api/activity.go -> internal/thumbnail/coverart_integration_test.go -> internal/metadata/people_link_e2e_test.go -> internal/api/completeness_detail_test.go -> internal/api/duplicates_test.go -> internal/api/activity.go`

## Hyperedges (group relationships)
- **F50 Tag Governance & Video Enrichment design package (plan + handoff + QA checklist)** — docs_plans_holodex_224, docs_design_tag_governance_and_video_enrichment_handoff, docs_design_tag_governance_and_video_enrichment_qa_checklist [EXTRACTED 1.00]
- **Client-only, zero-backend-state UI persistence/derivation pattern (localStorage/session, no migration)** — docs_specs_quick_wins, docs_specs_sort_persistence, docs_specs_people_nationality_flag [INFERRED 0.65]
- **curation/enrichment/entity component folders jointly implement the per-field source-of-truth + identity spine UI** — web_src_lib_components_curation_claude, web_src_lib_components_enrichment_claude, web_src_lib_components_entity_claude [INFERRED 0.75]
- **Video card/grid primitives and the shelves built on top of them** — web_src_lib_components_video_videocard, web_src_lib_components_video_videogrid, web_src_lib_components_video_recentlyaddedshelf, web_src_lib_components_video_relatedshelf [INFERRED 0.75]
- **ADR-070 canary + retag release pipeline** — github_workflows_image_yml, github_workflows_release_please_yml, github_workflows_release_yml, github_workflows_release_candidate_yml, concept_adr_070_canary_retag_promotion [INFERRED 0.85]
- **Entity-generic F36 decision model proven across person, studio, and derived links** — docs_specs_people_source_of_truth, docs_specs_studio_entity, docs_specs_person_media_linking [INFERRED 0.85]
- **Frontend token-discipline design + CI enforcement** — claude_rules_frontend_theming_md, github_workflows_ci_yml, concept_adr_021_frontend_theming_skins [INFERRED 0.85]
- **Jira status-transition pipeline (ADR-058/069)** — claude_claude_md, claude_flightplan_yaml, github_workflows_jira_sync_yml, github_workflows_release_yml, concept_adr_058_jira_rest_transitions [INFERRED 0.85]

## Communities (520 total, 134 thin omitted)

### Community 0 - "types.ts"
Cohesion: 0.01
Nodes (227): ADR-0006, ADR-0028, ADR-0036, ADR-0056, ADR-0080, ADR-0099, ADR-0107, ADR-0108 (+219 more)

### Community 1 - "people/[id]/+page.svelte"
Cohesion: 0.02
Nodes (138): Layout, Gates — definition of done, DismissableOptions, canEdit, claimingKey, entries, {
		fields,
		isOwner = false,
		entityType,
		entityNoun = '',
		onchanged
	}, promotingKey (+130 more)

### Community 2 - "whats-left.mjs"
Cohesion: 0.14
Nodes (24): emptyWorklog(), flipGate(), frontmatter(), logSkillRun(), maskComments(), ADR-0064, ADR-0092, parseGates() (+16 more)

### Community 3 - "deployment.md"
Cohesion: 0.02
Nodes (78): ADR-007: Docker Image Structure — Single Multi-Stage Image + Vite Dev Server, Base Image: Debian Slim (not Alpine), Build Stages, Consequences, Context, Decision, Development Workflow, Frontend Embedding (+70 more)

### Community 4 - "status/+page.svelte"
Cohesion: 0.04
Nodes (64): Design-system fit (the `/design-system` check), Accessibility, API contract (for the spec / ADR), Content, Decisions (locked), Design handoff: dismiss addressed failures on the status page (HOLODEX-416), Design tokens used, Edge cases (+56 more)

### Community 5 - "testing.T"
Cohesion: 0.02
Nodes (331): Action Items, 2. Smoke — automated (green in CI), 2. Smoke — automated (green in CI), 2026-07-31 · session, Session log   (append-only), Up next — ordered (position = priority), 2026-09-03 · P0-2/P0-4/P0-5 enrich write path landed, Gates — definition of done (+323 more)

### Community 6 - "2026-09-06 · implementation landed, verified live against real media"
Cohesion: 0.50
Nodes (9): 2026-09-06 · implementation landed, verified live against real media, getMediaWritebackStatus(), mediaWritebackURL(), seedFailedWriteback(), TestDismissWriteback_DeletesRow(), TestEnqueueWriteback_ClearsPriorFailedForVideo(), TestGetMedia_WritebackStatusRedactedForVisitor(), TestRetryDismissWriteback_OwnerGated() (+1 more)

### Community 7 - "time.Duration"
Cohesion: 0.04
Nodes (38): Noop, ADR-008: Caching Strategy — In-Process Cache with Redis-Ready Interface, Cache Interface, Configuration, Consequences, Context, Decision, Invalidation Strategy (+30 more)

### Community 8 - "Holodex Project Working Agreements (CLAUDE.md)"
Cohesion: 0.05
Nodes (44): Holodex Project Working Agreements (CLAUDE.md), Pre-commit checklist, Secrets & publishing rules, Jira task tracking (HOLODEX project), Flightplan Config (flightplan.yaml), Frontend Theming Rules, Database Migrations Rule, Provider Sidecars Rule (+36 more)

### Community 9 - "testMappings"
Cohesion: 0.07
Nodes (41): compiledPatterns, fakeEnrichmentCall, fakeEnrichmentWriter, fakeVideoLister, Pattern, patternFile, recordingJobRecorder, recordingReviewStore (+33 more)

### Community 10 - "devDependencies"
Cohesion: 0.13
Nodes (15): devDependencies, playwright, svelte, svelte-check, @sveltejs/adapter-static, @sveltejs/kit, @sveltejs/vite-plugin-svelte, tailwindcss (+7 more)

### Community 11 - "time.Time"
Cohesion: 0.04
Nodes (30): heldView, maintenanceView, trashItem, JobRun, New(), TestSweepQuietWhenNothingDue(), TestSweepRecordsError(), TestSweepRecordsRunWithCounts() (+22 more)

### Community 12 - "imagetools.mjs"
Cohesion: 0.10
Nodes (34): ADR-0035, execDetailed(), execFileAsync, IMAGES, inspectDigest(), inspectImageConfig(), isAbsent(), ADR-0070 (+26 more)

### Community 13 - "newProviderIconEnv"
Cohesion: 0.17
Nodes (22): filmImageServer(), solidJPEG(), TestFilmImage_BannerRequiresLandscape(), TestFilmImage_InvalidRole(), TestFilmImage_MutationsRequireOwner(), TestFilmImage_ReplaceAdvancesVersion(), TestFilmImage_UploadServeDelete(), uploadFilmImage() (+14 more)

### Community 14 - "NameEditControl.svelte"
Cohesion: 0.10
Nodes (24): Accessibility, `CollisionOfferCard.svelte`, Edge Cases, Props, Responsive, States and Interactions, busy, cancelEdit() (+16 more)

### Community 15 - "Service"
Cohesion: 0.02
Nodes (129): ExternalLink, refreshAllResult, 3. `RelinkProviderIcon` is the sole writer — triggered at boot and config-reload, Action Items, ADR-083: Provider-Link Badge — Extending Namespace-Qualified Display to Person and Studio, Consequences, Context, D1 — Data source: read-only projection, not a promoted registry facet (+121 more)

### Community 16 - "resolver.go"
Cohesion: 0.04
Nodes (111): 3 — Resolver consults the decision first, 1 — The `BaselineSource` interface, 2 — The video implementation (the file layer), 3 — The entity-agnostic core, 4 — What is explicitly *not* in this ADR, A — `BaselineSource` interface owning its namespace + entity-agnostic core (chosen), Action Items, ADR-052: `BaselineSource` contract — the resolver's entity-agnostic baseline seam (+103 more)

### Community 17 - "writeback/writeback.go"
Cohesion: 0.04
Nodes (90): Action items, ADR-116: Remux a fragmented MP4 inside the writeback job, restoring its tags, Consequences, Context, Decision, ADR-117: Bundle MKVToolNix in a trixie runtime image; MKV writeback edits tags in place, Alternatives considered, Consequences (+82 more)

### Community 18 - "Handlers"
Cohesion: 0.02
Nodes (100): FacetSummary, Health, purger, rescanner, scanStatusSource, searchMetrics, staleRef, thumbnailer (+92 more)

### Community 19 - "ResolveReviewAction"
Cohesion: 0.26
Nodes (12): ResolvedWrite, ReviewAction, ResolveReviewAction(), TestResolveReviewAction_Filename(), TestResolveReviewAction_FilenameRequiresValue(), TestResolveReviewAction_Manual(), TestResolveReviewAction_ManualMultiValue(), TestResolveReviewAction_ManualRequiresValue() (+4 more)

### Community 20 - "Scanner"
Cohesion: 0.03
Nodes (53): 1. A new aggregated read-model endpoint, 2. Scanner status accessor, 3. Durable job history (`job_runs`), 30-day retention, 4. Library counts behind the existing cache seam, ADR-028: User-facing activity surface & job-history persistence, Consequences, Context, Decision (+45 more)

### Community 21 - "ExtraMetadata"
Cohesion: 0.04
Nodes (41): stubFileExtractor, A — Post-write re-extract through the canonical scan path (chosen), A — Scalar row + per-value candidates in the read (chosen), ADR-068: Extraction resolve materializes entities via post-write re-extract, B — Insert + link the entity inline in the resolve handler, B — One review row per person (migration), C — Nothing; rely on the next scheduled scan, Consequences (+33 more)

### Community 22 - "placeholders"
Cohesion: 0.02
Nodes (85): ADR-031: Related-media endpoint — one combined call, random selection seam, Context, Rationale, Decision, 4. One read-path visibility seam; direct-by-id surfaces join it, Action Items, ADR-045: Seeded random ordering for paginated list endpoints, Consequences (+77 more)

### Community 23 - "Design handoff: StudioLinkCard (reusable Studio display)"
Cohesion: 0.20
Nodes (11): 1. Resolved decisions (open questions from the rough mockup), 2. New component: `StudioLinkCard.svelte`, 3. Call-site changes, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior, 8. Edge cases, 9. Accessibility notes (+3 more)

### Community 24 - "Design handoff: `?` keyboard-shortcuts sheet + hotkey firing feedback (F62)"
Cohesion: 0.20
Nodes (11): Accessibility, Cinémathèque QA (numbered per `docs/design` convention), Content, Decision, Design handoff: `?` keyboard-shortcuts sheet + hotkey firing feedback (F62), Edge cases, Layout, Motion (+3 more)

### Community 25 - "enrich/enrich_test.go"
Cohesion: 0.09
Nodes (36): F25.29 — Post-enrichment image freshness, gateTestHandlers(), TestGateImageURL_MergedField(), TestServiceRecordSearched_AppliedDetail(), TestServiceResolveGatesImageURL(), newSvc(), TestDownloadAssetsFirstSuccessPerRole(), TestEnrichAssetFailureIsNonFatal() (+28 more)

### Community 26 - "Design Handoff: Unified name-edit mechanism (HOLODEX-269)"
Cohesion: 0.14
Nodes (12): Accessibility Notes, Animation / Motion, Cinémathèque QA checklist (before this ships), Component contract (resolves the spec's open question), Components, Cross-context notes, Design Handoff: Unified name-edit mechanism (HOLODEX-269), Design Tokens Used (+4 more)

### Community 27 - "tmdb.go"
Cohesion: 0.07
Nodes (44): Implementation notes, Problem Statement, buildCompanyEnrichResponse(), buildMovieEnrichResponse(), buildPeopleCredits(), headshotFor(), movieAliases(), movieDisambiguate() (+36 more)

### Community 28 - ".requireOwner"
Cohesion: 0.02
Nodes (93): sessionClaims, ADR-030: Access-control / "Pro mode" gating seam for owner-only surfaces, Consequences, Context, Decision, Frontend capability flag, One choke point: a `requireOwner` middleware, Rationale (+85 more)

### Community 29 - "StudioPicker.svelte"
Cohesion: 0.05
Nodes (56): Role choice — resolved direction, States and Interactions, 1. Decisions, 2. Component change, save(), Entity components, Frame follows source aspect, never config or role name, Keyboard lists: roving tabindex in pickers, aria-activedescendant in a text combobox (+48 more)

### Community 30 - "handler"
Cohesion: 0.28
Nodes (6): decode(), isSupportedEntity(), writeJSON(), writeRateLimited(), enrichRequest, handler

### Community 31 - "Spec: Age-in-media (cross-entity derived field)"
Cohesion: 0.20
Nodes (9): 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — Cinémathèque, QA Checklist: Age-in-media badge on the cast poster card (HOLODEX-173), ADR-063 (derived/computed fields) reused arithmetic, Spec: Age-in-media (cross-entity derived field), age_in_media cross-entity computed field (bespoke API-layer join), HOLODEX-176 (F46 queryable-field substrate) scope overlap (+1 more)

### Community 32 - "media-page-extraction-handoff.md"
Cohesion: 0.17
Nodes (9): ADR-068: Extraction entity-field chip refinement (D2), 1. Setup, 2. Smoke, 3. Agent, 4. Human, Manual QA Checklist: Extract from filename on the media detail page (F48.5a), Gates — definition of done, HOLODEX-194 · Extract from filename on the media detail page (F48.5a) (+1 more)

### Community 33 - "Server"
Cohesion: 0.21
Nodes (8): 2026-08-25 · session (cont. 3), filterNamed(), isOwner(), jsonResult(), mapSlice(), withOwner(), namedCount, Server

### Community 34 - "Spec: Tag governance & video enrichment (F50)"
Cohesion: 0.13
Nodes (19): F50: Tag governance & video enrichment, Design Handoff: Tag Governance & Video Enrichment (F50), QA Checklist: Tag Governance & Video Enrichment (F50), HOLODEX-224 · F50 Tag governance & video enrichment, 2026-07-10 · what happened this session, 2026-10-08 · reproduced the studio prune data loss; designed the shared orphan sweep, Dropped — newest first (the reason is the point), HOLODEX-535 · Guarded prune-on-empty for studios and tags (+11 more)

### Community 35 - "extractor.go"
Cohesion: 0.07
Nodes (35): ADR-004: Metadata Extraction — ffprobe + exiftool + ffmpeg, Consequences, Context, Decision, Extraction Pipeline (per file), Field Mapping Reference, Rationale, 2026-09-17 · debug → repro → fix (+27 more)

### Community 36 - "resolver_test.go"
Cohesion: 0.07
Nodes (79): Gates — definition of done, decide(), providerCandidate(), TestResolve_CandidatesListFileAndMatchedProviders(), TestResolve_DecisionAdoptProvider(), TestResolve_DecisionKeepFileOverridesMappingOrder(), TestResolve_DecisionManualLiteral(), TestResolve_DeclaredBaseline_StaysKnowable() (+71 more)

### Community 37 - "Index"
Cohesion: 0.05
Nodes (49): Consequences, Decision, ADR-113: Offer empty replace fields to the owner, one field at a time, through the resolver, Alternatives considered, Consequences, Context, D1: The resolver offers an empty field when asked to; it doesn't decide which fields, D2: Which fields are offered is per-field registry metadata, adopted one component at a time (+41 more)

### Community 38 - "api.ts"
Cohesion: 0.02
Nodes (130): ADR-0073, ADR-0081, Context, Interaction detail, 2026-07-10 · what happened this session, 2026-09-04 · full implementation + live verification, Gates — definition of done, HOLODEX-307 · Film detail: poster becomes the header image; remove the Images section (+122 more)

### Community 39 - "people/+page.svelte"
Cohesion: 0.02
Nodes (114): 1. A per-epic worklog is the ground truth; Jira status is a projection of it, List state: the URL holds the view, storage holds only preferences, Seeded shuffle: `holo_shuffle(id, seed)` for paged lists, client PRNG for unpaged, Components, Persistence module (P0-2, RD1), 2026-09-10 · `code-review xhigh` over PR #314, all 14 findings applied, 2026-10-17 · session, Dropped — newest first (the reason is the point) (+106 more)

### Community 40 - "jira-sync.mjs"
Cohesion: 0.08
Nodes (35): 2026-09-26 · session, Gates — definition of done, HOLODEX-462 · jira-sync: never move an issue backwards, Session log, Up next, docsOnlyExemptKeys(), log, main() (+27 more)

### Community 41 - "q"
Cohesion: 0.04
Nodes (57): D1. Split list state into *preferences* and *queries*, D2. The URL is the single source of truth for what is on screen, D3. Only an explicit user choice writes to storage, D4. Leaving a list after a destructive action is `history.back()` when in-app, else the bare list, D5. Snapshots are keyed by the list's URL query string, Decision, Amendments approved 2026-09-27, Animation and motion (+49 more)

### Community 42 - "args_test.go"
Cohesion: 0.09
Nodes (39): flag(), hasTrackStatistics(), inputArgs(), isQTChapterTrack(), MKVEncodeArgs(), MKVMergeArgs(), mkvTrackID(), MP4Args() (+31 more)

### Community 43 - ".RelinkVideoStudios"
Cohesion: 0.02
Nodes (190): relinkContext, ADR-036: Person aliases & merge — an alias is a name→canonical routing rule (search-time and scan-time), Consequences, Context, Rationale, Consequences, 1 — Data model (migration 0018), 2 — Capture: an internal sidecar enrichment field (+182 more)

### Community 44 - "Repo"
Cohesion: 0.05
Nodes (176): clearEnv, iconEnv, partsFixture, backfillDashedTagAliases(), backfillPersonLinks(), backfillStudioLinks(), main(), newLogger() (+168 more)

### Community 45 - "density.svelte.ts"
Cohesion: 0.10
Nodes (27): 7. Edge cases, 2c-i. The film detail page — same shell, different zoning, 2e. Grid density — IMPLEMENTED: column counts kept, ladder extended, 2026-09-06 (later) · Max density raised to 8 columns; remodel now blocked on a conflict, 2026-09-06 (later still) · Density model decided: column counts kept, ladder to be extended, 2026-09-06 (later still) · Ladder extended, slider range made viewport-aware, 2026-09-06 · Premise corrected, direction chosen, design package committed, 2026-09-07 (later) · Film detail rail; shared stage-grid utility (+19 more)

### Community 46 - "Queue"
Cohesion: 0.02
Nodes (128): Action Items, ADR-071: Job runs attribute to an entity; history reads through a keyset cursor, Consequences, Context, D1 — Attribution is a polymorphic `(entity_type, entity_id)` pair with no foreign key, D1 — how a run says what it touched, D2 — `batch_id` becomes a column; the Revert control reads it structurally, D2 — where the batch id lives (+120 more)

### Community 47 - "Spec: Duplicates pair evidence — expand a flagged pair into a symmetric side-by-side compare panel (F70)"
Cohesion: 0.04
Nodes (46): Accessibility, Build checklist, Cinémathèque QA, Decision, Design calls made here (not in the spec), Design tokens used, Duplicates pair evidence — expand a person pair into a symmetric compare panel — design handoff, Edge cases (+38 more)

### Community 48 - "imageVariant"
Cohesion: 0.22
Nodes (11): aspect, imageSlot, imageVariant, imageWriter, drawLabel(), monogram(), renderNormalized(), slotsFor() (+3 more)

### Community 49 - "generate.mjs"
Cohesion: 0.11
Nodes (21): ADR-0004, ADR-0017, buildItem(), ensureFfmpeg(), here, hms(), main(), ADR-0009 (+13 more)

### Community 50 - "ADR-064-flightplan-plugin.md"
Cohesion: 0.03
Nodes (65): ADR-052: BaselineSource contract, 1. Changelog from Conventional Commits — git-cliff, 2. GHCR publish as a GitHub Deployment — the `environment:` key, 3. Package link in the body, ADR-034: Release notes (Conventional-Commit changelog) + GHCR deployment linkage, Consequences, Context, Decision (+57 more)

### Community 51 - "Video composite-key collision check (F56.3, HOLODEX-270)"
Cohesion: 0.12
Nodes (15): A required precondition: generalize `NameEditControl`'s conflict type, Design Handoff: Video composite-key collision verdict (HOLODEX-270), New type: `VideoCollisionRef`, Overview, Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1) (+7 more)

### Community 52 - "#lib"
Cohesion: 0.01
Nodes (163): Keyboard and accessibility, Accessibility notes, Keyboard and accessibility, 6. Accessibility, 2026-08-08 · Item #4 follow-up — shared segmented-toggle class helper, svelte, d, { title, children } (+155 more)

### Community 53 - "nav.mjs"
Cohesion: 0.09
Nodes (26): ADR-032: Browse-state preservation across SPA navigation, Consequences, Context, Decision, Rationale, playwright, launch(), ADR-0030 (+18 more)

### Community 54 - "stack.md"
Cohesion: 0.05
Nodes (38): ADR-001: Backend Language — Go, Consequences, Context, Decision, Rationale, Rejected Alternatives, ADR-002: Frontend Framework — SvelteKit, Consequences (+30 more)

### Community 55 - "matroska.go"
Cohesion: 0.10
Nodes (39): Gates — definition of done, HOLODEX-296 · Extract shared poster-tile component for Films + People chips, Session log, Up next, 2026-10-11 · exiftool 13.25 flattens Matroska tags last-wins, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-565 · Fix TestExtract_MatroskaTargetLevels under exiftool 13.25 (+31 more)

### Community 56 - "openAt"
Cohesion: 0.14
Nodes (36): 2026-09-02 · testing gate closed, seedAliasCollapse(), TestMigration0044Down(), TestMigration0044PromotesAliasCuration(), TestMigration0044SuppressionsDieWithTheirEntity(), TestMigration0045ReviewQueueDetail(), TestMigration0061CollapseTagWhitespace(), completenessTriggerTables() (+28 more)

### Community 57 - "Action Items"
Cohesion: 0.06
Nodes (41): Action Items, D4 — materialization hook point, Action Items, ADR-095: Structured resolve hints — `hint.fields`, `hint.filename`, `hint.query_source`, and `searched[]`, Consequences, D1 — Opt-in via the manifest: `/describe.resolve_hints`, D3 — `hint.filename`: the basename, verbatim, D4 — `hint.query_source`: `"pattern"` | `"user"`, derived server-side (+33 more)

### Community 58 - "tmdb_test.go"
Cohesion: 0.10
Nodes (43): newHandler(), buildEnrichResponse(), newTMDBClient(), clientWith(), fakeTMDB(), newDiscardLogger(), TestBuildEnrichResponseCapsAt20(), TestBuildEnrichResponseFallsBackToProfilePath() (+35 more)

### Community 59 - "newServer"
Cohesion: 0.14
Nodes (17): TestSearchFilmsDisabled(), newServer(), seedVideo(), TestAdminActivity(), TestAdminActivityDigest(), TestAdminActivityHistory(), TestAdminActivityNoSecrets(), TestAdminRescanUnavailable() (+9 more)

### Community 60 - "Manager"
Cohesion: 0.05
Nodes (30): Must-have (P0), Open Questions, Problem Statement, ThumbnailCandidate, writeAtomic(), absPath(), lastLine(), scaleArgs() (+22 more)

### Community 61 - "SanitizeValue"
Cohesion: 0.10
Nodes (12): 3. `RelinkStudioLogo(ctx, studioID)` is the sole writer — same trigger set as the field, Action Items, Gates — definition of done, Handlers, Handlers, personDecisionSource(), TestPersonDecisionSource_EdgeMapping(), recordDecisionSource() (+4 more)

### Community 62 - "HeldPath"
Cohesion: 0.14
Nodes (38): TestRebuild_ToolFailureLeavesNoCandidate(), TestProbeWritable_LeavesNoTrace(), HeldPath(), NewPath(), NewSwapper(), failNth(), newMemStore(), newRig() (+30 more)

### Community 63 - "Mappings"
Cohesion: 0.07
Nodes (30): Action Items, 2026-09-16 · backend: extractor, 2026-09-16 · backend: mapping + lifter, 2026-09-16 · brainstorm + spec + design handoff, 2026-09-16 · fixture step (#6), 2026-09-16 · frontend: media header + film row, 2026-09-16 · frontend: types + helper + VideoCard, 2026-09-16 · human QA pass 2 — 4.3 re-ruled: the control lives at the click (+22 more)

### Community 64 - "extraction/+page.svelte"
Cohesion: 0.04
Nodes (58): Reuse map (don't build from scratch), 2026-09-16 · frontend: header link (A) + queue rows — frontend gate closed, acceptTag(), busy, castValue, Chip, chips, doDismiss() (+50 more)

### Community 65 - "WritebackFormDialog.svelte"
Cohesion: 0.01
Nodes (240): ADR-0101, ADR-111: Record the file's tag set at every scan; tags get a set-valued in_sync, Alternatives rejected, Consequences, Context, Consequences, D7. Wire and SPA, Accessibility (+232 more)

### Community 66 - "EntityImageSlot.svelte"
Cohesion: 0.06
Nodes (34): 4. Anchors and deep links, busy, candidateReady, detailHref, doApply(), error, FACET_ANCHOR, {
		row,
		facetCanonical,
		facetLabel,
		apply,
		onhandled
	} (+26 more)

### Community 67 - "RunJumpPopover.svelte"
Cohesion: 0.08
Nodes (36): active, caption, details, drawn, focusRow(), input, list, loadFailed (+28 more)

### Community 68 - "WriteBatch"
Cohesion: 0.06
Nodes (38): A — Cross-source dedup union + persistent curation store (chosen), A — Durable DB-backed queue + bounded worker pool (chosen), Action Items, ADR-048: Granular Metadata Curation — cross-source merge + durable batch-writeback queue, B — In-memory queue (replay nothing on restart), B — Keep first-source-wins; manual edits as a high-precedence override only, C — Materialize merged values into a real column / write-through on edit, C — Synchronous per-field writes (status quo, no queue) (+30 more)

### Community 69 - "newRepoDB"
Cohesion: 0.04
Nodes (124): filmVideoRow, 2. Smoke — automated (green in CI), 2026-08-23 · session (cont.), 2026-09-04 · HOLODEX-310 cast coverage — ADR-089 D1's storage amended by one lookup, readFilmVideos(), seedFilmVideo(), TestFilmVideosSurviveFullRelinkCycle(), storedQuery() (+116 more)

### Community 70 - "Video"
Cohesion: 0.03
Nodes (98): A — Registry flag + pure `Derive(resolved, now)` post-pass + `computed:` provenance (chosen), Action Items, ADR-063: Derived / computed fields — a compute-on-read field genre, B — Compute at enrich time and persist as a shadow enrichment value, C — Compute in the SPA from `birthdate`, Consequences, Context, Current state (survey, 2026-07-08) (+90 more)

### Community 71 - "PopoverMenu"
Cohesion: 0.06
Nodes (27): Accessibility, Build notes (2026-09-20), Decision, Design tokens used, Interaction, Layout & placement, Not in scope, Person hover card — floating preview with a link row on text-only person links — design handoff (+19 more)

### Community 72 - "navSnapshot.svelte.ts"
Cohesion: 0.08
Nodes (25): ADR-0118, A snapshot that holds list data is checked against one app-wide write count, Snapshots keyed by the canonical query string; scroll survives reload, 2026-10-13 · launch profiles for the nav harness, 2026-10-13 · merged, 2026-10-13 · write-count freshness for list snapshots, Dropped — newest first (the reason is the point), Gates — definition of done (+17 more)

### Community 75 - "TagAddInput.svelte"
Cohesion: 0.07
Nodes (31): 2026-10-03 · mockups, design handoff, spec amendment, crossing into build, 2026-10-04 · build, review, QA, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-519 · Media page tag input: Enter adds and clears, autocomplete, status-sorted chips, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), active (+23 more)

### Community 76 - "commit-type-scope-check.mjs"
Cohesion: 0.09
Nodes (29): 1. Type source: PR title, not individual commits, 2. Scope signal: changed-file globs, with a threshold, 3. Advisory, not blocking, 4. New workflow, not a job in `jira-sync.yml`, 5. Script shape, 6. Allowlist, not the `docs/**`-denylist `jira-sync.yml` uses, Action Items, ADR-076: Advisory CI check — `docs`/`chore`-typed PRs that touch non-doc code (+21 more)

### Community 77 - "Spec: Sticky sort preferences + Random sort"
Cohesion: 0.18
Nodes (11): Future Considerations (P2), Goals, Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, Requirements, Spec: Sticky sort preferences + Random sort (+3 more)

### Community 78 - "genre_integration_test.go"
Cohesion: 0.14
Nodes (29): ClassifyResolution(), ParseResolutionBucket(), ResolutionWidthRange(), TestClassifyResolution(), TestResolutionWidthRange(), ClearTagNames(), tagPart(), TestClearTagNames() (+21 more)

### Community 79 - "NewService"
Cohesion: 0.05
Nodes (37): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs a human's eye, Manual QA Checklist: Metadata Enrichment for People (F22), fileConfig, Registry, Store (+29 more)

### Community 80 - "run.mjs"
Cohesion: 0.08
Nodes (34): 2026-09-09 (last) · HOLODEX-349 — the regression mechanism, and the three bugs it found, ASSERTIONS, LIST_PAGES, LIST_WORST, ADR-0095, ADR-0114, openPersonCard, stressedPicker (+26 more)

### Community 82 - "PromoteFieldEditor.svelte"
Cohesion: 0.07
Nodes (28): {
		field,
		entityType,
		entityNoun,
		isOwner,
		onchanged
	}, open, busy, {
		entityType,
		fieldKey,
		mode,
		label = '',
		render = '',
		group = 'extended',
		order = 0,
		inheritedLabel = '',
		entityNoun,
		onchanged,
		onclose
	}, error, groupDraft, labelDraft, labelEl (+20 more)

### Community 83 - "Spec: Tag Detail — Hierarchy & Category Controls"
Cohesion: 0.05
Nodes (41): Forced single-file extract seam, Tag hierarchy is a parent pointer; the deny-list is its own table checked in the resolve spine, 1. Decision logic (when the dialog appears), 2. The confirm dialog, 3. States and interactions, 4. Edge cases, 5. Accessibility, 6. Visual reference (+33 more)

### Community 84 - "process.go"
Cohesion: 0.06
Nodes (42): Resolved Decisions, cachingResolver, countingResolver, Deps, Enqueuer, ExtractionReviewCall, fakeManualSource, fakeReviewStore (+34 more)

### Community 85 - "Design Handoff: Shared Modal primitive (HOLODEX-215)"
Cohesion: 0.07
Nodes (28): ADR-018: Scanner Change Detection — Incremental Scan by (path, size, mtime), Consequences, Context, Decision, Mid-copy protection, Rationale, Scan algorithm, Stored fields (+20 more)

### Community 86 - "media/[id]/+page.svelte"
Cohesion: 0.01
Nodes (226): 10. Verification (as-built), 1. Resolved decisions, 2. New component: `PeopleGrid.svelte`, 3. Call-site changes, 4. Backend requirement, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior (+218 more)

### Community 87 - "itoa"
Cohesion: 0.02
Nodes (235): briefRow, externalLinksEnv, mediaIDsBody, 2026-08-09 · Testing gate closed — external_links projection + BuildProviderLink coverage, Gates — definition of done, Must-have (P0), TestDismissJobRuns_OwnerGatedAndIdempotent(), aliasList() (+227 more)

### Community 88 - "toMessage"
Cohesion: 0.01
Nodes (222): 3. Submit behavior — diverges by type (important asymmetry), 6. Edge cases, Open Questions, PopoverMenuOptions, active, applying, createAndAssign(), dialogEl (+214 more)

### Community 89 - "Normalize"
Cohesion: 0.10
Nodes (33): ADR-097: Alpha-preserving image normalization — PNG for non-opaque images, extension derived from the bytes, Consequences, Context, D1 — Normalize keeps alpha: PNG when the decoded image is not opaque, JPEG otherwise, D2 — The on-disk extension is derived from the bytes, never stored, D3 — Serve with the Content-Type of the file that was found, D4 — Rejected alternatives, Decision (+25 more)

### Community 90 - "Decision"
Cohesion: 0.07
Nodes (29): 2. Orientation is pushed as a digest at session start; detail is pulled on demand, 3. Idea capture is split from triage, and capture never depends on the connector, A — a per-epic worklog file on disk, `docs/plans/<KEY>.md` (chosen), A — hooks stay strictly mechanical; judgment lives only in `/handoff`, invoked by the agent (chosen), A — offline `INBOX.md` append (capture) + deliberate `/triage` (filing), chosen, A — `SessionStart` hook pushes a compact digest; full detail stays behind a pointer, pulled on demand (chosen), Action Items, ADR-064: Flightplan — a portable session-state plugin (mechanical worklog + hooks over judgment) (+21 more)

### Community 91 - "QA Checklist: System Activity (F21)"
Cohesion: 0.17
Nodes (11): F21: System Activity — Under the Hood, Accessibility, Controls (owner), Gating (F21.7) — needs `ADMIN_TOKEN` to exercise, Header activity indicator, Job history, QA Checklist: System Activity (F21), Reachability & shell (+3 more)

### Community 92 - "ADR-070: Canary the release candidate, then promote it by retagging the same digest"
Cohesion: 0.07
Nodes (29): 1. An advisory release-candidate comment on the release-please PR, 2. The canary pins by digest, not by tag, 3. `release.yml` promotes by retagging the canaried digest, 4. The ancestry assertion replaces the tag-time CI re-run, Action Items, ADR-070: Canary the release candidate, then promote it by retagging the same digest, Consequences, Context (+21 more)

### Community 93 - "video-file-maintenance.md"
Cohesion: 0.08
Nodes (24): 2026-07-10 · what happened this session, 2026-10-10 · F77 design package: spec, design, architecture, testing, security, 2026-10-11 · maintenance package: probe, encoder table, argv, verifier, 2026-10-12 · design sign-off re-pinned, 2026-10-12 · merged; Decided-in links, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-556 · F77 Video file maintenance — flip horizontally (+16 more)

### Community 94 - "generate"
Cohesion: 0.02
Nodes (166): Frontend component tests in Vitest browser mode, Frontend components: one sibling per data shape, over shared chrome, Go backend in one process, golang-migrate with embedded versioned SQL, applied at boot, MCP over Streamable HTTP, with a stdio entrypoint, REST/JSON on chi under `/api/v1`, Semantic CSS tokens mapped by Tailwind v4 `@theme inline`, SQLite (modernc) with WAL and FTS5 (+158 more)

### Community 95 - "verify.go"
Cohesion: 0.16
Nodes (22): TestTrackTagsXML_CarriesOnlyTheTracksOwnNonStatTags(), TestTrackTagsXML_NothingToCarry(), trackTagsXML(), diffInts(), diffTags(), fail(), hasTagBase(), isEncodedStat() (+14 more)

### Community 96 - "planCarry"
Cohesion: 0.14
Nodes (14): videoCompareSide, Duplicate videos are computed on demand, not queued, pairIDs(), filmLinks(), CarryPreview, VideoFileFacts, VideoPair, VideoWork (+6 more)

### Community 97 - "Design handoff: Films entity (F56)"
Cohesion: 0.05
Nodes (38): 1. `/films` — list, §1 Setup, 2. `/films/{id}` — detail, §2 Smoke, 2a. Header, 2b. Full-film file section (RD4, P0-10), 2c. Scenes list (RD4), 2d. Film → video attach entry point (+30 more)

### Community 98 - "flip_integration_test.go"
Cohesion: 0.15
Nodes (20): buildM1(), clearCompressorName(), countCovers(), exifTagDiff(), psnrMirror(), sh(), TestFlip_RealFiles_MP4(), TestFlip_RealFiles_MP4_DuplicateCovers() (+12 more)

### Community 99 - "New"
Cohesion: 0.12
Nodes (24): heldRig(), readOr(), TestPurge_HeldOriginal_MismatchedRowWaits(), TestPurge_HeldOriginal_RemoveFilesOffPutsTheOriginalBack(), TestPurge_HeldOriginal_RemoveFilesOnDeletesBoth(), TestPurge_HeldOriginal_SymlinkNotFollowed(), TestPurge_HeldOriginal_UnsettledWaits(), TestPurge_WaitsForAMaintenanceJob() (+16 more)

### Community 100 - "ref_node_url"
Cohesion: 0.12
Nodes (12): BOUNDARIES, contextFor(), decideWrite(), docTypeOfSkill(), main(), readStdin(), GUARD, ADR-0061 (+4 more)

### Community 101 - ".mergeEntity"
Cohesion: 0.18
Nodes (12): identityRoutes, 2026-08-10 · Backend audit: no new Go code needed, 2026-08-10 · Design handoff landed, 2026-08-10 · Frontend landed, testing + security gates closed — all gates green, 2026-08-10 · Started HOLODEX-269, worktree + Jira set up, research dispatched, 2026-08-10 · Wrote the spec, then caught and fixed a wrong premise in it, Gates — definition of done, HOLODEX-269 · Unified name-edit mechanism across Video Title, Person, Studio, and Tag (+4 more)

### Community 102 - "Design handoff: Media detail — Films + People sections"
Cohesion: 0.12
Nodes (16): 10. Scope, 1. Overview, 1a. What's wrong, 1b. The rule, 2. State matrix, 3. Design tokens, 4. Layout, 4a. Tile sizing decision (+8 more)

### Community 104 - "Spec: Tag & Category Create Affordance — closing the /tags creation gap"
Cohesion: 0.15
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 105 - "BatchRunner"
Cohesion: 0.08
Nodes (24): Auto-apply routing and the `enrichment_dismissals` store, Future considerations (P2), In scope, Must-have (P0), Nice-to-have (P1), Objective, Open questions, Out of scope (Non-Goals) (+16 more)

### Community 106 - "Spec: Tag Categories — grouping tags without merging them"
Cohesion: 0.15
Nodes (12): Goals, Non-Goals, Open Questions, P0 — Must-Have, P1 — Nice-to-Have, P2 — Future Considerations, Problem Statement, Requirements (+4 more)

### Community 107 - "requireCoverTools"
Cohesion: 0.09
Nodes (22): 2026-09-28 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-488 · MKV writeback silently drops tags beside per-track tags, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), 2026-07-10 · what happened this session, 2026-10-03 · diagnose + fix (+14 more)

### Community 108 - "PersonPicker.svelte"
Cohesion: 0.04
Nodes (54): {
		label,
		detail = '',
		removeLabel,
		busy = false,
		disabled = false,
		onremove
	}, active, availableRoles(), candidates, closePicker(), commitAttach(), commitDetach(), commitError (+46 more)

### Community 109 - "Decision"
Cohesion: 0.09
Nodes (23): 1 — The decision primitive, 2 — Storage: a dedicated `field_source_decisions` table (migration 0016), 4 — Default-when-undecided is file-first, 5 — One decision drives writeback too; convergence is explicit (Open Q1), 6 — Performers are entities, not strings (Open Q4), 7 — API & auth, 8 — Multiple providers per entity, 9 — Generalization to People and Studios (entity-agnostic by construction) (+15 more)

### Community 110 - "itoa"
Cohesion: 0.05
Nodes (75): actorsItem, 2026-08-11a · spec + design handoff written, 2026-08-11b · backend + frontend implementation, 3-skin live QA, 2026-08-11c · PR #231 code-review fixes + a second /simplify pass, Session log — append-only (cap: last 8 sessions; older → archive/), Gates — definition of done, TestCurationAPI_NonPersonFieldSkipsCollisionGate(), postCurationNoFatal() (+67 more)

### Community 111 - "Observability and jobs"
Cohesion: 0.10
Nodes (18): ADR-100: Job-run dismissals — a sibling table with a cascading foreign key, never a column on the audit row, Consequences, Context, Decision, Option A: `dismissed_at` column on `job_runs` (rejected — D4 of the handoff), Option B: Sibling table `job_run_dismissals` + `ON DELETE CASCADE` (chosen), Option C: Sibling table + explicit orphan delete in each sweep, Option D: Per-kind watermark — `dismissed_before(kind, started_at)` (+10 more)

### Community 112 - "Spec: Duplicate videos — find two files of one scene, compare them, keep the right one (F76)"
Cohesion: 0.10
Nodes (19): 2026-10-08 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-521 · Duplicate videos v1: shared provider match queue, side-by-side compare, keep/label actions, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), Future Considerations (P2), Goals (+11 more)

### Community 113 - "Decision"
Cohesion: 0.15
Nodes (20): A — Unified refresh service over reusable seams, with plan/apply split (chosen), Activity recording: flat `job_runs` row, no FK, ADR-047: Per-item Metadata Refresh — forced re-extract + re-enrich as one owner action, B — Composed in the handler (rejected), C — Reuse the full-library scanner (force a whole rescan) + re-enrich (rejected), Concurrency, Consequences, Constraints (+12 more)

### Community 114 - "Options Considered"
Cohesion: 0.10
Nodes (20): A — basename verbatim (chosen), A — manifest opt-in list `resolve_hints` (chosen), A — residue rule at §4.9 render (chosen), A — resolved values, as-is, ∩ provider vocabulary (chosen), A — `searched[]` on the response, caption + activity detail (chosen), B — a stem-vs-tag provenance marker on `title`, B — Holodex logs what it *sent* and calls that transparency, B — redundancy-filtered (drop values already inside `title`) (+12 more)

### Community 115 - "Design handoff: completeness ring badge (F65)"
Cohesion: 0.11
Nodes (20): Accessibility, Accessibility, Content, Decision, Design handoff: completeness ring badge (F65), Edge cases, F65.8 — the ring as a button, Hoisting — one rule, four mounts (+12 more)

### Community 116 - "Ref"
Cohesion: 0.12
Nodes (12): RefKindError, `kind:id` is the entity reference handle, ParseRef(), TestListItemsCarryRef(), TestParseRef(), redactFileMetadataForVisitor(), toIDName(), toVideoDetail() (+4 more)

### Community 117 - "ADR-066: Enrichment auto-apply-with-revert — provider-contract posture change + durable `enrichment_dismissals` store"
Cohesion: 0.12
Nodes (19): A — Dedicated `enrichment_dismissals` table, mirrors `entity_keep_separate` (chosen), A — Reuse the existing frontend `matchLabel` cutoff, server-triggered (chosen), Action Items, ADR-066: Enrichment auto-apply-with-revert — provider-contract posture change + durable `enrichment_dismissals` store, B — A `dismissed` boolean/state column on `entity_enrichment`, B — Per-provider configurable threshold (operator setting), C — Confidence-weighted partial auto-apply (apply high-confidence fields, review the rest), C — Reuse `entity_keep_separate` directly (generalize its `(id_lo, id_hi)` shape) (+11 more)

### Community 118 - "compilerOptions"
Cohesion: 0.14
Nodes (13): $app/tsconfig, compilerOptions, allowJs, checkJs, esModuleInterop, forceConsistentCasingInFileNames, moduleResolution, resolveJsonModule (+5 more)

### Community 119 - "Design Handoff: Person Aliases ("Also known as") (F23)"
Cohesion: 0.10
Nodes (20): Accessibility notes, Animation / motion, Chip treatment (decisive), Cinémathèque QA checklist (required before merge — CLAUDE.md), Collision prompt (person page, inline), Components, Design Handoff: Person Aliases ("Also known as") (F23), Design-system fit (the `/design-system` check) (+12 more)

### Community 120 - "CompletenessRow"
Cohesion: 0.13
Nodes (15): 2026-09-16 → 2026-09-18 · brainstorm → epic → ADR, 2026-09-18 · backend, 2026-09-18 · frontend, 2026-09-18 · gates → ready, 2026-09-18 · post-ready code review (xhigh), Gates — definition of done, HOLODEX-412 · Completeness score v2 (F65), Session log — append-only (cap: last 8 sessions; older → archive/) (+7 more)

### Community 121 - "preflight.go"
Cohesion: 0.17
Nodes (11): fakeMaint, CheckFlip(), Preflight, probeWritable(), refused(), requireTools(), TestCheckFlip_CheapRefusals(), TestCheckFlip_NotEnoughSpaceReportsBothNumbers() (+3 more)

### Community 122 - "flip_mkv_integration_test.go"
Cohesion: 0.29
Nodes (17): buildK1(), dirNames(), flipMKV(), mkvFixture(), requireMKVTools(), statTag(), TestFlip_RealFiles_MKV_AV1(), TestFlip_RealFiles_MKV_K1() (+9 more)

### Community 123 - "Design handoff: Film provider enrichment (F59)"
Cohesion: 0.07
Nodes (30): 1. Details section — provider chips, §1 Setup, 2. Header — banner behind the poster row, §2 Smoke — automated (green in CI), 2a. Resolved: the banner sits behind an otherwise-unchanged header (spec Q1), 2b. Band geometry, 2c. Empty states (spec Q2) — as built, 2d. Owner controls (+22 more)

### Community 124 - "genreWritebackServer"
Cohesion: 0.13
Nodes (17): tagSetMedia, 2026-08-13 · session, Gates — definition of done, genreWritebackServer(), TestGenreWritebackEndpoint(), TestGenreWritebackValues(), TestGenreWritebackValues_AliasCollapsesIntoCanonicalTag(), TestGenreWritebackValues_AliasOfUnattachedTagWrittenCanonically() (+9 more)

### Community 125 - "Action Items"
Cohesion: 0.05
Nodes (45): A — core renders a string, `/resolve` contract unchanged (chosen), A — embed in the existing entity payload (chosen), A — operator > provider > global default > raw title (chosen), A — strip bracket punctuation + resolution tokens, collapse whitespace (chosen), Action Items, ADR-080: Configurable per-provider metadata search query patterns, B — leave the floor tier literal; rely on operator-configured patterns to work around messy titles, B — new endpoint, picker fetches on open (+37 more)

### Community 126 - "Design handoff: Media detail stage layout"
Cohesion: 0.10
Nodes (20): 1. File joins the enrichment group, 2. More-with shelves break the width cap, 3. Visitor rail regains read-only field values, and the overview moves into it, 3a. Read-only values for visitors, 3b. The overview moves to the rail — unconditionally, 3c. Divergence from the film detail page — resolved in HOLODEX-364, 4. Measured tracks, 5. Why this no longer needs an ADR (+12 more)

### Community 127 - "showcase-demo-corpus.md"
Cohesion: 0.12
Nodes (14): ADR-012: Resolution Classification — Width-Based Buckets with 10% Tolerance, Consequences, Context, Decision, Effective cutoffs (nominal − 10%), Nominal tier widths, Rationale, Worked examples (+6 more)

### Community 128 - "Store"
Cohesion: 0.50
Nodes (4): EnrichmentWriter, Store(), TestStore_EmptyFieldsIsNoop(), TestStore_RoundTripsThroughEntityEnrichment()

### Community 129 - "ADR-039: Provider asset URLs — contract clarification + operator-configured asset-host allowlist"
Cohesion: 0.12
Nodes (17): 1. Pin the asset object schema; ignore unknown kinds and keys (ratify shipped behaviour), 2. Advertise asset kinds separately in `/describe` (`asset_kinds`), 3. URL host trust — operator-configured `asset_hosts` allowlist (the amendment), 4. Fetch-soon, no long-lived URL storage (ratify) — `expires_at` deferred, 5. Multiple assets — preference-ordered, first success per role wins, 6. Empty = omit; shed assets before fields (bless as provider-side rules), Action items, ADR-039: Provider asset URLs — contract clarification + operator-configured asset-host allowlist (+9 more)

### Community 130 - "Design handoff: Media detail Metadata — move, trim, fold"
Cohesion: 0.08
Nodes (23): 1. Final order (top to bottom), 2. Fields removed from the Metadata list, 3. The fold, 5. Accessibility, 6. Theming, 7. Verification, Design handoff: Media detail Metadata — move, trim, fold, Overview (+15 more)

### Community 131 - "Complete"
Cohesion: 0.05
Nodes (47): D1: The score is the required band alone; extras are a separate number, never blended, D2: The completeness sort is the composite key `(required, extras)`, D3: The score is materialized in an entity-generic store — superseding ADR-081 D4, D5: The list payload carries `completeness` for the owner; the badge rides it, D6: Noise facets are muted by demoting them to `optional` in the registry, Decision, Implementation notes (2026-09-18, recorded at build time; the decisions above stand), 2026-08-07 · Backend D3 — score/actionability computation in internal/resolver (+39 more)

### Community 132 - "PersonLinkChip.svelte"
Cohesion: 0.03
Nodes (88): Data, 2026-09-26 · session, Gates — definition of done, HOLODEX-439 · a hover preparation so the person card's clamp becomes a harness rung, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), 2026-09-21 · found while triaging "hover cards not working", fixed, verified live, Gates — definition of done (+80 more)

### Community 133 - "routes/+layout.svelte"
Cohesion: 0.02
Nodes (109): Accessibility notes (summary), Design Handoff: Unified nav search — live, tabbed, in-place filtering panel, Design-system fit, Mobile (< 640px, the primary complaint driving this spec), Overview, Part A — The tab row lives with the box, not inside the dropdown, Part B — `SearchResultsPanel.svelte` (NS1), Part C — Per-page removal (NS4) (+101 more)

### Community 134 - "Gates — definition of done"
Cohesion: 0.04
Nodes (60): providerInfo, 1. The studio logo is a derived, self-hosted cache of the resolved `logo` field — not a new curation surface, 2. Data model — one logo per studio (migration 0020), 4. Serving — one public typed route, immutable cache, 5. Reuse, not a parallel spine, Action items, ADR-057: Self-hosted studio logo — derived, normalized cache of the resolved `logo` field, Consequences (+52 more)

### Community 135 - "newCompletenessHandlers"
Cohesion: 0.24
Nodes (16): facetGroupByCanonical(), TestRemediationQueue_ActionableSplit(), TestRemediationQueue_GroupsByFacet(), TestRemediationQueue_VideoRowThumbnailURL(), bandsOf(), facetByCanonical(), linkPeopleT(), newCompletenessHandlers() (+8 more)

### Community 136 - "Spec: Holodex Metadata Provider Contract (hand-off, protocol v1)"
Cohesion: 0.07
Nodes (27): Derivation (see the spec for detail), Design Handoff: Nationality flag beside the person name (HOLODEX-139), Placement & measurements, States, Theming notes (what bites these surfaces), 1. Setup / smoke, 2. Agent-verified (this session), 3. Human eyeball — Cinémathèque (+19 more)

### Community 137 - "ADR-058: Jira issue transitions via REST API (retire the Automation rules)"
Cohesion: 0.12
Nodes (17): 1. Transition-trigger split, 2. Mechanism (ported from Bookshelf's `scripts/lib/jira-sync.mjs`), 3. Relationship to Bookshelf's status model — converge the mechanism, not the semantics, 4. Credential storage & rotation — scripted two-repo fan-out, Action Items, ADR-058: Jira issue transitions via REST API (retire the Automation rules), Consequences, Context (+9 more)

### Community 138 - "context.Context"
Cohesion: 0.01
Nodes (82): A generic `settings` key/value table for library-owned values, Merge registers the loser's name as an alias; keep-separate is the durable negative, Incremental scan keyed on (canonical path, size, mtime), 1. Resolved decisions, 2026-07-10 · what happened this session, 2026-08-25 · full implementation + live verification, Gates — definition of done, HOLODEX-280 · Film poster/thumbnail asset pipeline (+74 more)

### Community 139 - "Swapper"
Cohesion: 0.26
Nodes (5): refuseSymlink(), exists(), HeldStore, rig, Swapper

### Community 140 - "web/package.json"
Cohesion: 0.06
Nodes (28): ADR-0025, flag-icons, @fontsource-variable/archivo, @fontsource-variable/fraunces, svelte-check, @sveltejs/adapter-static, @sveltejs/kit, @sveltejs/vite-plugin-svelte (+20 more)

### Community 141 - "ADR-067: Filename-based metadata extraction with confidence-gated auto-apply and rollback"
Cohesion: 0.12
Nodes (16): 1. Confidence-Gated Auto-Apply for Extraction (Generalizes ADR-066), 2. Snapshot-Based Rollback (Amends ADR-041), Action Items, ADR-067: Filename-based metadata extraction with confidence-gated auto-apply and rollback, Auto-apply design options, Confidence model trade-offs, Consequences, Context (+8 more)

### Community 142 - "Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)"
Cohesion: 0.09
Nodes (22): 10. QA gate, 1. The remediation queue, 2. The completeness breakdown panel, 3. Components, 4. Tokens, 5. States, 6. Accessibility, 7. Edge cases (+14 more)

### Community 143 - "Handoff Spec: Film Details parity (HOLODEX-95)"
Cohesion: 0.12
Nodes (14): Accessibility, Animation / motion, Cinémathèque QA checklist, Components, Edge cases, Handoff Spec: Film Details parity (HOLODEX-95), Layout, Mockup (+6 more)

### Community 144 - "Handoff Spec: Smart playlists, Play all and Shuffle (F75)"
Cohesion: 0.12
Nodes (16): 1 · Count line (all query-backed grids), 2 · Smart chip, 3 · Run strip (`NextUpStrip.svelte`), 4 · Playlist page header (`routes/playlists/[id]`), 5 · Browse edit mode, Animation / Motion, Divergences from shipped / approved F69 (approved 2026-09-30), Edge Cases (+8 more)

### Community 145 - "NewService"
Cohesion: 0.15
Nodes (15): The redundancy, NewService(), sourceByName(), TestReExtract_FileOnlyAndRelinks(), TestReExtract_TargetErrorPropagates(), TestRefreshFileErrorDoesNotPersist(), TestRefreshForcesReExtractAndPersists(), TestRefreshNoMatchesIsFileOnly() (+7 more)

### Community 146 - "Handoff: Session-expired banner — hold the reload while editing"
Cohesion: 0.15
Nodes (12): ADR-046 (Proposed, owner-session-persistence.md): Owner session persistence via HttpOnly token-exchange cookie, Component, Decision, Handoff: Session-expired banner — hold the reload while editing, Interaction, Layout and tokens, States, 2026-10-09 · held re-auth + session-expired banner (+4 more)

### Community 147 - "Options Considered"
Cohesion: 0.14
Nodes (14): A — additive `field_hints` map keyed by field key (chosen), A — hints for non-canonical keys only; code registry wins for canonical (chosen), A — persist a `provider_field_hints` table (chosen), A — presence-driven, display-only (chosen), B — advertised-driven and/or editable, B — hints may override canonical fields too, B — in-memory manifest cache, B — polymorphic `fields[]` (`string | object`) (+6 more)

### Community 148 - "Design handoff: Person Details fold"
Cohesion: 0.14
Nodes (12): Accessibility, Decisions, Design handoff: Person Details fold, Mechanics, Overview, QA (as run, 2026-10-04, Cinémathèque, `backend-films` + `web`), 2026-10-04 · designed, built, verified, Dropped — newest first (the reason is the point) (+4 more)

### Community 149 - "Handlers"
Cohesion: 0.24
Nodes (3): Maintenance, preflightView, Handlers

### Community 150 - "confidence.go"
Cohesion: 0.11
Nodes (31): §1 Setup, §2 Smoke, §3 Agent live QA (Cinémathèque), §4 Human, QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48), Agreement, Decision, EntityMatch (+23 more)

### Community 151 - "ActivityState"
Cohesion: 0.21
Nodes (3): 6 — `films_enabled` gate wiring, Reuse map (don't build from scratch), ActivityState

### Community 152 - "Design handoff — Entity identity card (F60)"
Cohesion: 0.06
Nodes (32): 1. Reference chip — every entity page (HOLODEX-374), §1 Setup, 1a. Placement, 1b. Component: `RefChip.svelte` (new, `web/src/lib/components/entity/`), 1c. States, 1d. API / MCP contract (frontend-relevant part), §2–§3 as built (HOLODEX-377, 2026-09-14), 2. Edition field row — media page (HOLODEX-377) (+24 more)

### Community 153 - "worklog-gate.mjs"
Cohesion: 0.17
Nodes (10): ADR-0001, artifacts, gEnd, gStart, keyRe, ADR-0002, ADR-0007, open (+2 more)

### Community 154 - "Spec: Two-tier video poster resolution — sharp detail page, small list thumbnails (F53)"
Cohesion: 0.15
Nodes (14): API, Future considerations (P2), Gate status, Goals, Non-Goals, Requirements, Resolved Decisions, Should-have (P1) (+6 more)

### Community 155 - "title"
Cohesion: 0.02
Nodes (125): Option A — Custom deterministic scalar function `holo_shuffle(id, seed)` (CHOSEN, for Media), Option B — Inline arithmetic hash in SQL (no custom function), Option C — Materialize a per-seed shuffle (random column / temp ordering table), Option D — Client-side shuffle for Media too (fetch all ids, page in the client), Options Considered, D1 — Core renders a string; the `/resolve` wire contract is untouched, D2 — Config carriage: operator YAML override, provider-advertised preference, one global default, D3 — Token grammar: space-joined, optional-aware, video-scoped (+117 more)

### Community 156 - "Design Handoff: Tag & category create affordance (HOLODEX-243)"
Cohesion: 0.17
Nodes (12): 1. The "+ New" pill, 2. Expanded form, 4. Empty-state wiring, 5. Interaction states, Accessibility notes, Design Handoff: Tag & category create affordance (HOLODEX-243), Design-system-fit audit, Measured contrast (+4 more)

### Community 157 - "detail"
Cohesion: 0.02
Nodes (120): resolveCounter, Action Items, ADR-055: Universal enrichment unique-key invariant — every source supplies a namespaced id, and it is the identity, Conformance table (the invariant applied per entity), Consequences, Context, Current state (survey, 2026-07-02), D1 — Mandatory: no name fallback (+112 more)

### Community 158 - "ADR-009: Cover Art & Thumbnail Strategy — Tiered Extraction with Throttled, Priority-Aware Backfill"
Cohesion: 0.17
Nodes (12): ADR-009: Cover Art & Thumbnail Strategy — Tiered Extraction with Throttled, Priority-Aware Backfill, Backfill posture: Eager by default, user-configurable, Configuration, Consequences, Context, Cost Reference (for capacity reasoning), Decision, Implementation status (2026-06-11) (+4 more)

### Community 159 - "Decision"
Cohesion: 0.17
Nodes (12): A — Explicit (chosen), ADR-041: Metadata Writeback — explicit per-field write-back to media files, API endpoint, Audit table, Auth, B — Rule-based (backlog), C — Batch (backlog), Consequences (+4 more)

### Community 160 - "Design handoff: Film Studio cascade edit affordance (F57)"
Cohesion: 0.12
Nodes (17): 1. Media detail page — no visual change, §1 Setup, 2. Film detail page — trigger affordance, §2 Smoke, §3 Agent live QA (preview tools against §1 stack), 3. New component: `FilmStudioCascadeDialog.svelte`, 3a. Step 1 — Picker (open state), 3b. Step 2 — Results (post-commit, same dialog, same `PickerShell`) (+9 more)

### Community 161 - "QA checklist: Responsive page width (HOLODEX-331)"
Cohesion: 0.13
Nodes (14): Load-in animation, Responsive behavior, Surface 3 — `PersonPosterGrid`, 1. Setup, 2. Smoke, 3. Agent-verifiable geometry, 4. Density ladder, 5. Human review (+6 more)

### Community 162 - "resolveOrCreateByName"
Cohesion: 0.01
Nodes (272): seedIdentityReviewQueue(), ADR-006: API Design — REST + OpenAPI 3.1, API Shape (Phase 1), Consequences, Context, Decision, Filter Query Parameters (`GET /api/v1/media`), Rationale (+264 more)

### Community 163 - "Geometry assertion harness (HOLODEX-349)"
Cohesion: 0.12
Nodes (14): 2026-07-10 · what happened this session, 2026-09-28 · nav harness built, mutation-proven, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-475 · Navigation behaviour harness, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), Files (+6 more)

### Community 164 - "ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field"
Cohesion: 0.17
Nodes (12): ADR-090: Two-layer entity metadata management — adoption at the entity, precedence per field, Alternatives considered, Consequences, Context, D1 — The two layers, and the boundary between them, D2 — Layer 1 is reachable from the entity's own detail page, D3 — Adoption must visibly land in layer 2, D4 — A new source is a namespace, not a subsystem (+4 more)

### Community 165 - "Unified name-edit mechanism (F56.2, HOLODEX-269)"
Cohesion: 0.17
Nodes (12): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Open Questions, Problem Statement, Requirements, Resolved Decisions (+4 more)

### Community 166 - "Write"
Cohesion: 0.15
Nodes (24): 4. Failure path — original untouched, TestReadCurrentValues_RoundTrips(), mustReadDir(), requireExiftool(), requireMkvpropedit(), TestBuildFFmpegArgs_AlwaysMapsAllStreams(), TestBuildFFmpegArgs_KeepsNonCoverAttachments(), TestBuildFFmpegArgs_ReplacesCoverOfOtherFormat() (+16 more)

### Community 167 - ".maybeRenewSession"
Cohesion: 0.33
Nodes (5): HttpOnly signed session cookie, no server-side store, ttlForClass(), baseSessionCookie(), Handlers, secureCookie()

### Community 168 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.18
Nodes (11): 2026-08-17 · Brainstormed the Films entity end-to-end, opened epic, wrote spec, 2026-08-18 · Wrote ADR-085, resolving spec Q1/Q2, 2026-08-23 · session, 2026-08-23 · session (cont. 3), 2026-08-24 · session, 2026-08-25 · session, 2026-08-25 · session (cont.), Gates — definition of done (+3 more)

### Community 169 - "Instance skin — Appearance tab & header picker removal — design handoff"
Cohesion: 0.18
Nodes (11): Accessibility, Custom card — three states (spec stories 7–8), Decision, Design tokens used, Header (spec R6), Instance skin — Appearance tab & header picker removal — design handoff, Interaction, Layout & placement (+3 more)

### Community 170 - "Session log — newest first (cap: last 8 sessions; older → archive/)"
Cohesion: 0.18
Nodes (10): 2026-10-11 · backend whole-run details read, 2026-10-11 · crossed into build, 2026-10-11 · design handoff for option A, 2026-10-11 · frontend jump list + security review, 2026-10-11 · handoff, build matches the approved mockup, 2026-10-11 · spec edit, RD14 + P0-13, Gates — definition of done, HOLODEX-564 · Jump to any position in a run from the run strip (+2 more)

### Community 171 - "Find"
Cohesion: 0.14
Nodes (26): Typed serve routes with an immutable, version-stamped cache, PortraitBannerError, TestFilmImage_DeleteClearsProviderRow(), ContentType(), entityDir(), Ext(), Find(), Path() (+18 more)

### Community 172 - "feature-claims.mjs"
Cohesion: 0.14
Nodes (36): 2026-09-17 · branch cut, script + tests + wiring shipped, Gates — definition of done, HOLODEX-407 · F-number claims — `scripts/feature-claims.mjs`, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), daysSince(), describeRivals(), git() (+28 more)

### Community 173 - "demo/package.json"
Cohesion: 0.18
Nodes (10): sharp, dependencies, sharp, description, name, private, scripts, generate (+2 more)

### Community 174 - "newFakeRepo"
Cohesion: 0.06
Nodes (63): Goals, Non-Goals, Open Questions, P0: Must-have, P1: Nice-to-have, P2: Future considerations (design for, don't build), Problem Statement, Requirements (+55 more)

### Community 175 - "settings.json"
Cohesion: 0.50
Nodes (3): hooks, PreToolUse, $schema

### Community 176 - ".getPersonCard"
Cohesion: 0.22
Nodes (8): 2026-09-19 · brainstorm → story → spec → design, 2026-09-20 · #373 merged → rebase → OQ1 prototype, 2026-09-20 · build, 2026-09-20 · design rev 2 after main moved, 2026-09-20 · review → ready, 2026-09-20 · scope: grid tiles join v1, Session log — append-only (cap: last 8 sessions; older → archive/), Handlers

### Community 177 - "ADR-011: Symlink Handling & Path Resolution"
Cohesion: 0.20
Nodes (10): ADR-011: Symlink Handling & Path Resolution, Configuration, Consequences, Context, Decision, Hardlinks, Loop protection, Resolution & dedup (+2 more)

### Community 179 - "Decision"
Cohesion: 0.20
Nodes (10): ADR-013: Configurable Metadata Field Mapping, Consequences, Context, Data capture (Phase 1), Decision, Discoverability (mapping authoring aid), Display (Phase 2), Filtering (Phase 2) (+2 more)

### Community 180 - "Options Considered"
Cohesion: 0.20
Nodes (10): A — One badge per stored external-id row (chosen), A — Provider-declared `link_templates`, resolved server-side (chosen), A — Read-only projection of the existing identity tables (chosen), B — Frontend-hardcoded per-namespace URL map, B — Pick a single "primary" badge (first-inserted, or a namespace priority order), B — Promote to a resolver-backed registry facet (widen F55's Person/Studio tables), D1 — where person/studio badge data comes from, D2 — where the outbound URL is built (+2 more)

### Community 181 - "ADR-021-frontend-theming-and-skins.md"
Cohesion: 0.02
Nodes (71): 1. Setup / preconditions, 2. Smoke — automated (green in CI), 3. Agent — drive the running app, 4. Human — needs your eyes (Cinémathèque), Manual QA Checklist: Admin Mode (F29), 1. Setup / preconditions, 3. Agent — drive the running app, 4. Human — needs your eyes (Cinémathèque) (+63 more)

### Community 182 - "2026-08-08 · Backend D4 — list-wide resolve-all predicate for browse sort/filter + remediation queue"
Cohesion: 0.03
Nodes (73): entityCompletenessBatch, PersonCard, PersonCompleteness, StudioCompleteness, VideoCompleteness, D4 — Materialization hooks into the existing `afterEnrichApply` dispatcher; it is a write, not a pure pass, ADR-112: Re-score completeness at boot only when its inputs changed, and never on the request path, Alternatives considered (+65 more)

### Community 183 - ".setFieldDecision"
Cohesion: 0.06
Nodes (36): decisionBody, filmStudioCascadeResult, Action Items, D1 — Extract the single-video Studio-decide logic into a shared helper; the cascade calls it once per attached video, D2 — `CascadeFilmStudio`: per-video decide (best-effort), then one shared-batch enqueue for every video that succeeded, D3 — Endpoint: `POST /films/{id}/studio/cascade`, Film-scoped, owner-gated; per-video results returned synchronously, writeback progress polled via the existing batch-status endpoint, Decision, D1. A cleared field is a manual decision with no value, recorded only by an explicit `clear` (+28 more)

### Community 184 - "Field"
Cohesion: 0.02
Nodes (112): D1 — Hint carriage: an additive `field_hints` map in `/describe`, D2 — The four-tier resolution ladder (who wins label / render / order), D3 — Persist provider hints (read-path availability), D4 — Presence-driven, display-only auto-registration; all three entities, Decision, Mechanism (the seams), Security (untrusted hints; the `image_url`/`url` widening), Action Items (+104 more)

### Community 185 - "Spec: People Images (F25)"
Cohesion: 0.06
Nodes (36): 2. Owner over-cap override (explicit flag, not a tier), 3. Enrichment URL suppression on delete, ADR-043: Person gallery — configurable cap, owner over-cap override & enrichment URL suppression, Alternatives considered, Consequences, Context, Decision, ADR-049: Owner-set person images take precedence over enrichment (+28 more)

### Community 187 - "stub.js"
Cohesion: 0.09
Nodes (16): ADR-0039, ADR-0082, ALL_ENTITY_TYPES, BY_SLUG, crc32(), CRC_TABLE, http, ADR-0033 (+8 more)

### Community 188 - "ADR-115: Cinémathèque is the only look — retire Broadcast, Brutalist, the custom palette and the skin setting"
Cohesion: 0.20
Nodes (10): Action Items (HOLODEX-476), ADR-115: Cinémathèque is the only look — retire Broadcast, Brutalist, the custom palette and the skin setting, Consequences, Context, D1 — Cinémathèque is the only look, and nothing chooses it, D2 — The token layer and hook classes stay, ungated, D3 — Leftover state degrades to nothing; no migration, D4 — QA is Cinémathèque (+2 more)

### Community 189 - "Design handoff — Media parts (`part` badge and pill)"
Cohesion: 0.10
Nodes (20): 1. Browse / search cards — `VideoCard.svelte`, §1 Setup, 1a. Placement and treatment, 1b. Coexistence, 1c. Data, 2. Media page — header pill + Metadata row, §2 Smoke — automated, 2a. Header pill (`routes/media/[id]/+page.svelte`) (+12 more)

### Community 190 - "Design handoff: Media page tag input (Enter, autocomplete, status sort)"
Cohesion: 0.20
Nodes (10): Accessibility, Animation / motion, Decisions (owner, 2026-10-03), Design handoff: Media page tag input (Enter, autocomplete, status sort), Design tokens used, Edge cases, Layout, Overview (+2 more)

### Community 192 - "Design handoff — `SourceEditModal` at paragraph length (HOLODEX-417)"
Cohesion: 0.20
Nodes (10): Components, `ConfirmDialog` (shared), Deferred, Design handoff — `SourceEditModal` at paragraph length (HOLODEX-417), Options considered, Root cause, `SourceEditModal`, `SourceValueClamp` (curation, new) (+2 more)

### Community 193 - "reference/ui-vocabulary.md"
Cohesion: 0.02
Nodes (92): UI vocabulary: translate, link, record, ADR-118: List scroll snapshots survive a full reload (sessionStorage), Consequences, Decision, Rationale, Accessibility Notes, Animation / Motion, Design-system fit (+84 more)

### Community 194 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (9): 2026-09-18 · ADR, 2026-09-18 · backend, 2026-09-18 · design handoff, 2026-09-18 · frontend, 2026-09-18 · ready, 2026-09-18 · spec, HOLODEX-416 · Owner can dismiss addressed failures from the status page's Recent failures callout, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 195 - "goto"
Cohesion: 0.22
Nodes (10): 2026-09-27 · session, 2026-09-27 · session (palette too), 2026-09-27 · session (spec), 2026-09-28 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-476 · Cinémathèque is the only look — retire the extra skins, the custom palette and the Appearance tab, Session log — newest first (+2 more)

### Community 196 - "Spec: Collapse provider aliases into the canonical alias spine (F58)"
Cohesion: 0.13
Nodes (15): API, Future considerations (P2), Goals, Must-have (P0), Non-Goals, Open Questions, Problem Statement, Requirements (+7 more)

### Community 197 - "ref_node_test"
Cohesion: 0.10
Nodes (14): config, tracked, cmp(), parseRelease(), promotionTags(), everyPersona, ADR-0095, ADR-0098 (+6 more)

### Community 199 - "Decision"
Cohesion: 0.22
Nodes (9): 1. Semantic design tokens as CSS custom properties, 2. Tailwind maps utilities → tokens, 3. All visual difference lives in CSS — components are skin-agnostic, 4. Fonts bundled offline via `@fontsource`, 5. Skin selection persists; supersedes the dark/light toggle, ADR-021: Frontend Theming — Semantic Design Tokens & Multi-Skin System, Consequences, Context (+1 more)

### Community 200 - "Options Considered"
Cohesion: 0.22
Nodes (9): A — new DB-backed `field_promotions` store, resolver tier-0 (chosen), A — promotion outranks YAML (tier-0, chosen), B — operator YAML still wins (promotion below tier-1), B — store a source list on the promotion row, B — the app writes `metadata-mappings.yaml`, D1 — where the override lives, D3 — precedence vs. operator YAML, D-candidate — where a promoted field's F36 candidates come from (+1 more)

### Community 201 - "ADR-114: One list-state model — the URL holds what you see, storage holds only preferences"
Cohesion: 0.22
Nodes (9): ADR-114: One list-state model — the URL holds what you see, storage holds only preferences, Consequences, Context, D4 alternatives: how a removal redirect finds the list, Option A: URL as source of truth, storage for preferences only (chosen), Option B: Storage as source of truth, URL as a mirror (People today, generalised), Option C: Everything in both (Media today, generalised), Option D: URL only, no storage at all (+1 more)

### Community 202 - "Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating"
Cohesion: 0.12
Nodes (14): 1. Studio next to the title, 2. Commentary block, 3. Poster upload, 4. File metadata — owner only, Design handoff: video owner-mode editing (F52) — studio placement, commentary, poster upload, file-metadata gating, QA checklist, Responsive / motion / a11y, 2026-07-10 · what happened this session (+6 more)

### Community 203 - "Design handoff: no plate under an image — bare + halo, plate only under the monogram"
Cohesion: 0.22
Nodes (9): 1. Decisions, 2. Component changes, 3. States, 4. Design tokens used, 5. Edge cases, 6. Accessibility, 7. QA checklist, Design handoff: no plate under an image — bare + halo, plate only under the monogram (+1 more)

### Community 205 - "Design handoff: Entity Films row — card-height match + hover lift"
Cohesion: 0.11
Nodes (18): 10. Resolved decisions, 11. Scope, 1. Overview, 1a. The rule, 2. Design tokens, 3. Layout, 3a. Sizing formula, 3b. Caption block (+10 more)

### Community 206 - "TagForField"
Cohesion: 0.05
Nodes (45): Design Handoff: Writeback hides the target file tag (HOLODEX-216), Design-system fit (the `/design-system` check), Non-goals (explicitly out of this change), Problem, QA checklist, The "no dimming" rule, applied, Gates — definition of done, HOLODEX-216 · Writeback hides the target file tag and reports success for fields it silently skipped (+37 more)

### Community 207 - "net/http/httptest.Server"
Cohesion: 0.15
Nodes (21): exchange(), findCookie(), getCookie(), TestCookieAuthorizesGatedRoute(), TestRememberDeviceLongerLifetime(), TestSessionExchangeNoopWhenGateOpen(), TestSessionExchangeSetsCookie(), TestSignOut() (+13 more)

### Community 208 - "Design handoff: Media detail page reorder"
Cohesion: 0.22
Nodes (9): 1. Films + People row, 2. Rejected during iteration, Accessibility / interaction, Design handoff: Media detail page reorder, Edge cases, Final order (top to bottom), Overview, Theming (+1 more)

### Community 209 - ".RelinkProviderIcon"
Cohesion: 0.62
Nodes (6): Find(), Path(), Remove(), Store(), TestStorePNG_ExtensionFollowsBytes(), TestStoreRoundTrip()

### Community 210 - "Design Handoff: People attach/detach + relationship picker (HOLODEX-272)"
Cohesion: 0.22
Nodes (9): Accessibility Notes, Animation / Motion, Components, Design Handoff: People attach/detach + relationship picker (HOLODEX-272), Design Tokens Used, Edge Cases, Layout, Overview (+1 more)

### Community 211 - "HOLODEX-287 · Frontend component-reuse decision as a topic doc"
Cohesion: 0.22
Nodes (8): 2026-10-15 · session, 2026-10-15 · session (2), 2026-10-15 · session (3), Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-287 · Frontend component-reuse decision as a topic doc, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 212 - "FacetGroup"
Cohesion: 0.36
Nodes (6): FacetGroup, QueueRow, Handlers, sortFacetGroups(), sortRowsByName(), TestSortFacetGroups_CriticalFirstThenCountDesc()

### Community 213 - "ADR-017: Search Architecture — Global Mixed-Entity Search + FTS5"
Cohesion: 0.25
Nodes (8): 1. Global search (command-palette style) — primary search box, 2. In-grid filtering — browse narrowing, ADR-017: Search Architecture — Global Mixed-Entity Search + FTS5, Consequences, Context, Decision, FTS configuration, Rationale

### Community 214 - "Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240)"
Cohesion: 0.12
Nodes (16): 1. `/tags` — unified type filter + search, 2. Category pill, 3. `/categories/{id}` detail page (new route), 4. Bulk "Add to category…" / "Remove from category…" (Manage-mode bar), 5. Browse-page "Categories" facet, Accessibility notes, Design Handoff: Tag categories — grouping tags without merging them (HOLODEX-240), Design-system-fit audit (+8 more)

### Community 215 - "Options Considered"
Cohesion: 0.25
Nodes (8): A — Mandatory, no name fallback (chosen), A — Shared namespace, cross-provider convergence (chosen), B — Preferred, name fallback quarantined, B — Provider-scoped keys `(provider, external_id)`, D1 — enforcement level, D2 — key scope, D3 — rollout, Options Considered

### Community 216 - "newRepo"
Cohesion: 0.19
Nodes (27): clearHarness(), deletes(), TestClear_DeletesEveryStudioTag(), TestClear_NoSourcesDeletesTargetOnly(), TestClear_RequiresExplicitFlagAndNoValues(), TestClear_WorkerRejectsHostileNames(), TestQueue_RevertUnknownBatch(), byTag() (+19 more)

### Community 218 - "Q1 — how a film competes as a resolver source"
Cohesion: 0.25
Nodes (8): A — Suspension is injection non-participation; reuse the existing "decided source unmatched → empty" path (chosen), B — A status/availability column on `field_source_decisions` rows, B — Compile films into `metadata-sources.yaml` as a pseudo-provider, C — Delete `field_source_decisions` rows naming a suspended film source, re-derive on re-enable, C — Direct writeback bypass (film page writes Album/Title straight to the file, no resolver involvement), Options Considered, Q1 — how a film competes as a resolver source, Q2 — suspend semantics for `films_enabled=false`

### Community 219 - "+layout.ts"
Cohesion: 0.50
Nodes (3): prerender, ssr, ADR-0002

### Community 220 - "HOLODEX-405 · Page-scoped hotkeys (F62)"
Cohesion: 0.25
Nodes (5): 2026-09-16 · brainstorm → story → spec, Gates — definition of done, HOLODEX-405 · Page-scoped hotkeys (F62), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 221 - "Design handoff: studio image halo toggle (HOLODEX-463)"
Cohesion: 0.25
Nodes (8): 1. What changes, 2. The switch, 3. Behaviour, 4. QA, [agent], Design handoff: studio image halo toggle (HOLODEX-463), [human], [smoke]

### Community 222 - "scripts"
Cohesion: 0.25
Nodes (8): scripts, build, check, dev, geometry, nav, preview, test

### Community 223 - "HOLODEX-404 · `/?q=` deep link keeps its search on Media"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-10-13 · fix same-route + Back/Forward ?q= loss, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-404 · `/?q=` deep link keeps its search on Media, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 224 - "HOLODEX-464 · MP4 genre writeback keeps only the last tag"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-09-26 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-464 · MP4 genre writeback keeps only the last tag, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 225 - "HOLODEX-465 · A genres writeback makes the file's tags match the UI"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-09-26 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-465 · A genres writeback makes the file's tags match the UI, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 227 - "HOLODEX-477 · List scroll position is lost across a full page reload"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-09-28 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-477 · List scroll position is lost across a full page reload, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 228 - "HOLODEX-498 · Stress fixture seeds container "mp4", so no fixture video ever gets a write target"
Cohesion: 0.25
Nodes (7): 2026-09-29 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-498 · Stress fixture seeds container "mp4", so no fixture video ever gets a write target, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), TestGenerate_VideosCarryAWritableContainer()

### Community 232 - "HOLODEX-536 · MKV tag target-level selection"
Cohesion: 0.25
Nodes (7): 2026-10-09 · Go Matroska Tags reader replaces incidental first-wins, 2026-10-09 · merged, deciding commit recorded, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-536 · MKV tag target-level selection, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 233 - "HOLODEX-548 · Only uploaded images keep an orphan from the sweep"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-10-08 · owner chose uploads-only; spec edited, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-548 · Only uploaded images keep an orphan from the sweep, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 234 - "HOLODEX-59 · Rename internal adminMode → ownerView"
Cohesion: 0.25
Nodes (7): 2026-07-10 · what happened this session, 2026-10-18 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-59 · Rename internal adminMode → ownerView, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 235 - "SourceValueClamp.svelte"
Cohesion: 0.25
Nodes (6): clamps, el, expanded, { text, label }, textId, width

### Community 236 - ".getMedia"
Cohesion: 0.04
Nodes (58): filmVideoCandidate, Context, D5 — Consumption: existing choke points, existing payloads, zero picker changes, Forces, Context, D2 — `hint.fields`: resolved canonical values, as-is, intersected with the provider's vocabulary, Forces, 2026-09-04 · ADR-090 — generalize the two-layer model (+50 more)

### Community 237 - "Decision (option A, approved 2026-09-27)"
Cohesion: 0.29
Nodes (7): Behaviour, Decision (option A, approved 2026-09-27), Design Handoff: Undismiss from the Enrich picker (HOLODEX-467), Out of scope, Problem, QA (done 2026-09-27, backend-films + tmdb sidecar), States

### Community 238 - "HOLODEX-417 · Edit Description modal pushes Save/Cancel off screen when sources are long"
Cohesion: 0.29
Nodes (5): 2026-09-18 · critique → fix, all three priorities shipped, Gates — definition of done, HOLODEX-417 · Edit Description modal pushes Save/Cancel off screen when sources are long, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 239 - "HOLODEX-330 · VideoCard's scene badge can fail AA contrast over a bright thumbnail"
Cohesion: 0.29
Nodes (6): 2026-10-13 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-330 · VideoCard's scene badge can fail AA contrast over a bright thumbnail, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 240 - "HOLODEX-425 · F67 Instance skin — owner-set skin on an Appearance tab + custom palette"
Cohesion: 0.29
Nodes (7): 2026-09-19 · brainstorm → epic → spec → ADR → design → S1 → S2 → S3, 2026-09-19 · human QA → ready for review, 2026-09-19 · merge main — F66→F67, migration 0048→0050, Gates — definition of done, HOLODEX-425 · F67 Instance skin — owner-set skin on an Appearance tab + custom palette, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 241 - "HOLODEX-449 · CodeQL config — exclude test files from analysis"
Cohesion: 0.29
Nodes (6): 2026-09-22 · config + guard, 2026-09-22 · verified on a real CodeQL run, Gates — definition of done, HOLODEX-449 · CodeQL config — exclude test files from analysis, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 242 - "HOLODEX-459 · Person profile never scrolls to the hover card's #videos / #films anchors"
Cohesion: 0.29
Nodes (6): 2026-10-13 · land entity pages on #videos / #films, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-459 · Person profile never scrolls to the hover card's #videos / #films anchors, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 243 - "HOLODEX-463 · Per-studio owner toggle for the studio image halo"
Cohesion: 0.29
Nodes (7): 2026-09-26 · session, Decisions — do not re-litigate, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-463 · Per-studio owner toggle for the studio image halo, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 244 - "HOLODEX-478 · Trashed video lingers in the Media grid after the Back exit"
Cohesion: 0.29
Nodes (6): 2026-10-13 · drop a trashed video from the pending grid snapshot, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-478 · Trashed video lingers in the Media grid after the Back exit, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 245 - "HOLODEX-479 · Writeback refuses fragmented MP4 with a remux hint"
Cohesion: 0.29
Nodes (6): 2026-09-28 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-479 · Writeback refuses fragmented MP4 with a remux hint, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 246 - "HOLODEX-486 · Bundle MKVToolNix in the runtime image"
Cohesion: 0.29
Nodes (6): 2026-09-28 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-486 · Bundle MKVToolNix in the runtime image, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 247 - ".downloadAssets"
Cohesion: 0.05
Nodes (50): 1. `studio_images` replaces `studio_logos` — three core roles, no gallery, 2. `enrich.ImageSink` / `downloadAssets` become entity-generic, 3. The studio `logo` field is retired; TMDB emits it as an asset, 4. Serving, upload, delete — mirrors ADR-057 §4 with an owner-write path added, Action items, ADR-079: Studio image roles — entity-generic asset orchestration, retiring the `logo` field, Consequences, Context (+42 more)

### Community 248 - "HOLODEX-497 · Stress fixture: a replace-studio variant for exercising studio decisions"
Cohesion: 0.29
Nodes (6): 2026-09-29 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-497 · Stress fixture: a replace-studio variant for exercising studio decisions, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 250 - "HOLODEX-507 · Tag aliases: tag page panel, alias search, lowercase alias text"
Cohesion: 0.29
Nodes (7): 2026-07-10 · what happened this session, 2026-10-01 · design, build and QA in one session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-507 · Tag aliases: tag page panel, alias search, lowercase alias text, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 252 - "HOLODEX-513 · The linked film decides a video's Film and Title by default"
Cohesion: 0.29
Nodes (7): 2026-07-10 · what happened this session, 2026-10-02 · linked film decides the Film/Title rows, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-513 · The linked film decides a video's Film and Title by default, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 253 - "HOLODEX-514 · Film-derived values beyond the media page (grids + picker)"
Cohesion: 0.29
Nodes (7): 2026-07-10 · what happened this session, 2026-10-02 · film values on tiles and in the picker, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-514 · Film-derived values beyond the media page (grids + picker), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 255 - "HOLODEX-515 · Completeness counts film-derived Film/Title"
Cohesion: 0.29
Nodes (7): 2026-07-10 · what happened this session, 2026-10-02 · completeness scores film values, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-515 · Completeness counts film-derived Film/Title, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 256 - "HOLODEX-524 · Rules and gates for living spec / architecture / design docs"
Cohesion: 0.29
Nodes (6): 2026-10-04 · brainstorm → epic → rules, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-524 · Rules and gates for living spec / architecture / design docs, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 258 - "holodex/main.go"
Cohesion: 0.32
Nodes (20): categoryTagIDsBody, claimBody, claimView, enrichRoute, promotionBody, promotionView, readbackGapView, targetView (+12 more)

### Community 263 - "system-activity.md"
Cohesion: 0.02
Nodes (90): ADR-019: Observability & Operational Conventions, Consequences, Context, Decision, Graceful shutdown, Health & readiness endpoints, Prometheus metrics (Phase 2), Rationale (+82 more)

### Community 264 - "HOLODEX-525 · Teach the doc skills their "never contains" boundaries"
Cohesion: 0.29
Nodes (6): 2026-10-04 · rule + guard hook, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-525 · Teach the doc skills their "never contains" boundaries, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 266 - "HOLODEX-528 · Retire the numbered ADRs"
Cohesion: 0.29
Nodes (6): 2026-10-04 · archive + redirect, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-528 · Retire the numbered ADRs, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 269 - "Spec: Studio as a first-class entity (F38)"
Cohesion: 0.04
Nodes (53): 1 — Data model (migration 0017), 2 — Resolved-value link derivation (RD1), 4 — Facet + search, 5 — One-time backfill, A — Links follow the *resolved* `studio` value (chosen), Action Items, ADR-053: Studio entity — data model + resolved-value link derivation, B — Links follow the raw file value at scan time (people's pattern) (+45 more)

### Community 270 - "HOLODEX-529 · Media detail delete dialog names a different video during playlist playback"
Cohesion: 0.29
Nodes (6): 2026-10-04 · fixed the delete dialog carrying over a run advance, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-529 · Media detail delete dialog names a different video during playlist playback, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 271 - "HOLODEX-531 · Topic docs batch 1 (stack, config-and-settings, deployment, security-perimeter)"
Cohesion: 0.29
Nodes (6): 2026-10-04 · topic list agreed, batch 1 written, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-531 · Topic docs batch 1 (stack, config-and-settings, deployment, security-perimeter), Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 272 - "Design Handoff: Owner tooling hub + nav split (F35)"
Cohesion: 0.10
Nodes (22): Accessibility, Cinémathèque QA (load-bearing), Design Handoff: Owner tooling hub + nav split (F35), Design-system fit (the `/design-system` check), Design tokens used (header), Design tokens used (hub), Edge cases, Implementation pointers (non-binding) (+14 more)

### Community 276 - "HOLODEX-533 · Topic docs batch 2 (field-resolution, metadata-providers, entity-identity)"
Cohesion: 0.29
Nodes (6): 2026-10-04 · batch 2 written, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-533 · Topic docs batch 2 (field-resolution, metadata-providers, entity-identity), Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 281 - "Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31)"
Cohesion: 0.12
Nodes (16): Accessibility notes, Agent / human (Cinémathèque), Animation / motion, Cinémathèque QA checklist (required before merge — CLAUDE.md), Design Handoff: Refresh Metadata (per-item re-extract + re-enrich) (F31), Design-system fit (the `/design-system` check), Design tokens used, Edge cases (+8 more)

### Community 284 - "Spec: Runtime owner-editable settings (F41)"
Cohesion: 0.06
Nodes (35): 1. Configurable cap (`PERSON_GALLERY_MAX`, default 20), 2. Precedence — a runtime override layer *above* startup config, 3. Hot-reload — DB is durable truth, the in-memory field is the hot cache, 4. Registry — a typed allowlist of settable keys (not arbitrary KV), 5. API — owner-gated, consistent with the existing `/admin/*` surface, Action Items, ADR-060: Runtime owner-editable settings — DB-backed override over startup config, debuting with the person gallery cap, Consequences (+27 more)

### Community 286 - "HOLODEX-534 · Topic docs batch 3 (entity-relationships, images, media-ingest)"
Cohesion: 0.29
Nodes (6): 2026-10-04 · batch 3 written, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-534 · Topic docs batch 3 (entity-relationships, images, media-ingest), Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 287 - "HOLODEX-537 · Topic docs batch 4 (writeback, observability-and-jobs, browse-and-list-state)"
Cohesion: 0.29
Nodes (6): 2026-10-04 · batch 4 written; folding complete, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-537 · Topic docs batch 4 (writeback, observability-and-jobs, browse-and-list-state), Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 288 - "HOLODEX-538 · Activity history paging: correct the topic doc"
Cohesion: 0.29
Nodes (6): 2026-10-09 · decided: drop, not build, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-538 · Activity history paging: correct the topic doc, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 291 - "HOLODEX-540 · Rule move batch 1 (foundation ADR leftovers)"
Cohesion: 0.29
Nodes (6): 2026-10-04 · batch 1, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-540 · Rule move batch 1 (foundation ADR leftovers), Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 293 - "Design handoff: Studio image roles — icon / logo / poster (F51)"
Cohesion: 0.25
Nodes (8): 1. `/studios` list — logo well data source change only, 2. `/studios/{id}` detail — role-generic image control, 3. Provenance (P1, non-blocking), 4. Accessibility & Cinémathèque QA checklist, Design handoff: Studio image roles — icon / logo / poster (F51), Empty vs. populated states (per role), Interaction, Layout

### Community 294 - "HOLODEX-541 · Rule move batches 2–4 (remaining ADR leftovers)"
Cohesion: 0.29
Nodes (6): 2026-10-04 · batches 2–4 combined, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-541 · Rule move batches 2–4 (remaining ADR leftovers), Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 295 - "HOLODEX-542 · Specs and design docs still describe retired skins and contain self-contradictions"
Cohesion: 0.29
Nodes (6): 2026-10-08 · retired-skin text and four contradictions fixed, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-542 · Specs and design docs still describe retired skins and contain self-contradictions, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 296 - "HOLODEX-544 · Make Trivy's SARIF upload honour the CRITICAL,HIGH severity filter"
Cohesion: 0.29
Nodes (6): 2026-10-07 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-544 · Make Trivy's SARIF upload honour the CRITICAL,HIGH severity filter, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 297 - "provider"
Cohesion: 0.04
Nodes (50): 1. A new `deleted_at` timestamp column — orthogonal to `active`, 2. The scanner treats a soft-deleted row as untouchable, 3. A dedicated `internal/purge` ticker — not folded into the scanner, 5. Owner-gated mutations; reads stay public but blind, 6. Hard-delete relies on existing cascades; disk removal degrades gracefully, ADR-037: Soft-delete media — a `deleted_at` axis orthogonal to `active`, swept by a dedicated purge job, Consequences, Context (+42 more)

### Community 298 - "HOLODEX-545 · Hotfix release lane: patch a shipped minor from release/vX.Y without releasing main"
Cohesion: 0.29
Nodes (6): 2026-10-07 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-545 · Hotfix release lane: patch a shipped minor from release/vX.Y without releasing main, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 299 - "HOLODEX-546 · Enforce the merge gate: activate the main ruleset with current checks, a bypass, and release/** coverage"
Cohesion: 0.29
Nodes (6): 2026-10-07 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-546 · Enforce the merge gate: activate the main ruleset with current checks, a bypass, and release/** coverage, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 300 - "SweepRunner"
Cohesion: 0.06
Nodes (33): activityResponse, activitySystem, stubThumbs, 2026-09-19 · architecture gate (ADR-103) — after a premature merge, 2026-09-19 · backend 4 — `SweepRunner` + endpoint + activity block (AI 6), 2026-09-19 · backend 5 — TMDB sidecar `429` pass-through (AI 7) — backend gate closed, 2026-09-19 · design handoff + decisions, 2026-09-19 · spec gate (F66) (+25 more)

### Community 301 - "HOLODEX-547 · Permanent delete leaves a video's edits behind"
Cohesion: 0.29
Nodes (6): 2026-10-09 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-547 · Permanent delete leaves a video's edits behind, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 302 - ".showcase.md Portfolio Self-Report"
Cohesion: 0.40
Nodes (5): .showcase.md Portfolio Self-Report, Incremental indexing (mtime/size change detection), Metadata writeback, Open enrichment protocol (provider sidecars), Unified field resolution

### Community 303 - "HOLODEX-549 · Sweep remaining retired-skin references and dead SourceSelect out of docs and code"
Cohesion: 0.29
Nodes (6): 2026-10-08 · SourceSelect deleted; retired-skin sweep across ~200 docs and 30 comments, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-549 · Sweep remaining retired-skin references and dead SourceSelect out of docs and code, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 304 - "HOLODEX-550 · jira-sync docs-only guard strands docs tickets"
Cohesion: 0.29
Nodes (6): 2026-10-09 · docsOnlyExempt added, reviewed, security-reviewed, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-550 · jira-sync docs-only guard strands docs tickets, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 305 - "HOLODEX-551 · A person rename reaches the video files"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-10-09 · rename propagates writeback, Dropped — newest first (the reason is the point), HOLODEX-551 · A person rename reaches the video files, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 306 - "HOLODEX-552 · Person merge writes each credit to its own field"
Cohesion: 0.29
Nodes (6): 2026-10-09 · merge propagation made role-aware, 2026-10-09 · what happened this session, Dropped — newest first (the reason is the point), HOLODEX-552 · Person merge writes each credit to its own field, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 307 - "dismissable"
Cohesion: 0.11
Nodes (23): Behaviour notes, Contrast, Design Handoff: Media page — one sync verb, render-once fields (F36 / F39), Design-system fit (the `/design-system` check), Layout, Not in scope, Overview, Part A — One sync verb (+15 more)

### Community 308 - "HOLODEX-553 · A studio rename reaches the video files"
Cohesion: 0.29
Nodes (6): 2026-10-09 · studio rename propagates writeback, 2026-10-09 · what happened this session, Dropped — newest first (the reason is the point), HOLODEX-553 · A studio rename reaches the video files, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 309 - "HOLODEX-557 · Tags automatically get a dashed alias for their multi-word name"
Cohesion: 0.29
Nodes (6): 2026-10-10 · spec and build the dashed tag alias, 2026-10-11 · merged; gaps spun out, Gates — definition of done, HOLODEX-557 · Tags automatically get a dashed alias for their multi-word name, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 310 - "HOLODEX-572 · Restored item missing from the Media grid after returning from Trash"
Cohesion: 0.29
Nodes (6): 2026-10-13 · clear the grid snapshot on a Trash restore, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-572 · Restored item missing from the Media grid after returning from Trash, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 311 - "HOLODEX-573 · List scroll snapshot restores on a plain link arrival, not only on Back"
Cohesion: 0.29
Nodes (6): 2026-10-13 · restore a list's place only on a return, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-573 · List scroll snapshot restores on a plain link arrival, not only on Back, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 312 - "HOLODEX-574 · Nav harness back-after-write-refreshes-grid hangs in route.fetch on Windows"
Cohesion: 0.29
Nodes (6): 2026-10-13 · fix the vacuous wait in back-after-write-refreshes-grid, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-574 · Nav harness back-after-write-refreshes-grid hangs in route.fetch on Windows, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 313 - "HOLODEX-582 · One shared Details section for the film and studio pages"
Cohesion: 0.29
Nodes (6): 2026-10-17 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-582 · One shared Details section for the film and studio pages, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 314 - "HOLODEX-583 · Person image crop squashes the saved image horizontally on short viewports"
Cohesion: 0.29
Nodes (6): 2026-10-17 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-583 · Person image crop squashes the saved image horizontally on short viewports, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 315 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.10
Nodes (17): 2026-09-08 · brainstormed, reframed, specced, 2026-09-08 · HOLODEX-343 — seeder skeleton, isolated DATA_PATH, safety guard, 2026-09-08 · HOLODEX-344 — ladder table, OFAT, reserved blocks, manifest, Gates — definition of done, HOLODEX-342 · Dev-time stress fixture for UX and layout QA, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), 2026-09-10 · chevron gated on a real overflow measurement (+9 more)

### Community 316 - "CurationFieldRow.svelte"
Cohesion: 0.05
Nodes (52): 8. Edge cases, 10. Cinémathèque QA (required), 11. What is explicitly not in this handoff, 1. The Promote control (owner-only, on the auto row), 2. The inline editor (shared promote + edit — DD2), 3. After promotion — the partition move (shared by all treatments), 4. Edit / Remove promotion (owner-only, on the promoted row), 5. States (+44 more)

### Community 317 - "Spec: Media parts — denote the files of a multi-file media (`part`)"
Cohesion: 0.11
Nodes (18): API, Behavior detail, Data model, Future considerations (P2), Goals, Marker lifter (RD4), Must-have (P0), Non-Goals (+10 more)

### Community 318 - "HOLODEX-585 · Auto-refresh the committed graphify graph via a bot PR on push to main"
Cohesion: 0.29
Nodes (6): 2026-10-18 · session, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-585 · Auto-refresh the committed graphify graph via a bot PR on push to main, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 319 - "placeholder.go"
Cohesion: 0.48
Nodes (6): TestGenderBucket(), GenderBucket(), minF(), Placeholder(), placeholderDims(), shoulderWidth()

### Community 321 - "ADR-010: MKV (Matroska) Tag-Target Precedence"
Cohesion: 0.33
Nodes (6): ADR-010: MKV (Matroska) Tag-Target Precedence, Consequences, Context, Decision, Rationale, Tool Mapping

### Community 322 - "ADR-026: Prometheus metrics — hand-rolled exposition, no client library"
Cohesion: 0.33
Nodes (5): ADR-026: Prometheus metrics — hand-rolled exposition, no client library, Consequences, Context, Decision, Rationale

### Community 323 - "Options considered"
Cohesion: 0.33
Nodes (6): A: Remux on write, with tag restore *(chosen)*, B: Background sweep that remuxes every fragmented file (flag at scan, then queue), C: Flag at scan time and warn in the UI (HOLODEX-481 as filed), D: Refuse only (status quo after HOLODEX-479), E: Remux without `-TagsFromFile`, Options considered

### Community 324 - "Film banner landscape guard — design handoff"
Cohesion: 0.33
Nodes (5): Decision, Film banner landscape guard — design handoff, Not in scope, QA, What the owner sees

### Community 325 - "assetRoleFor"
Cohesion: 0.04
Nodes (57): Action Items, ADR-089: Film enrichment field vocabulary — where each provider value lands on a film, Consequences, Context, Current state (survey, 2026-09-04), D1 — Provider film cast is film-level and never cascades to attached videos; it is read from the shadow, not copied into `film_people_roles`, D2 — The film page renders the scene union, then only the set difference, D4 — `banner` becomes the film's second image role, replacing `thumb`; no migration (+49 more)

### Community 326 - "2. Layout"
Cohesion: 0.33
Nodes (6): 2. Layout, 2a. The stage — IMPLEMENTED, 2b. The two-zone split (>= 1024px) — IMPLEMENTED, 2c-ii. Stage-aligned grids, 2c. Resulting geometry — measured, 2d. The field grid

### Community 327 - "HOLODEX-403 · Poster row as an image-tile chooser + ledger-witnessed image sync"
Cohesion: 0.33
Nodes (5): 2026-09-19 (b) · backend + frontend + tests, 2026-09-19 · gates, HOLODEX-403 · Poster row as an image-tile chooser + ledger-witnessed image sync, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 328 - "HOLODEX-418 · "Re-match…" auto-applies a lone strong candidate instead of showing the picker"
Cohesion: 0.33
Nodes (5): 2026-09-18 · diagnosed, fixed, verified, Gates — definition of done, HOLODEX-418 · "Re-match…" auto-applies a lone strong candidate instead of showing the picker, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 329 - "HOLODEX-43 · Owner-view provenance badge on core images"
Cohesion: 0.33
Nodes (6): 2026-10-14 · spec + design, sign-off, build, verify, Dropped — newest first (the reason is the point), Gates — definition of done, HOLODEX-43 · Owner-view provenance badge on core images, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 330 - "HOLODEX-484 · MKV cover writeback fails when the existing cover has a different filename"
Cohesion: 0.33
Nodes (5): 2026-09-28 · session, Dropped — newest first (the reason is the point), HOLODEX-484 · MKV cover writeback fails when the existing cover has a different filename, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 331 - "HOLODEX-485 · mkvpropedit cover path leaves a second cover.* attachment behind"
Cohesion: 0.33
Nodes (5): 2026-09-28 · session, Dropped — newest first (the reason is the point), HOLODEX-485 · mkvpropedit cover path leaves a second cover.* attachment behind, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 332 - "HOLODEX-489 · Read-back gap: ledger witness + owner-visible gap"
Cohesion: 0.33
Nodes (6): 2026-09-28 · session, 2026-09-29 · session, Dropped — newest first (the reason is the point), HOLODEX-489 · Read-back gap: ledger witness + owner-visible gap, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 333 - "HOLODEX-490 · "+ Add overview" and "+ Set part" diverge from the page's "+ Add …" text CTA"
Cohesion: 0.33
Nodes (5): 2026-09-29 · session, Gates — definition of done, HOLODEX-490 · "+ Add overview" and "+ Set part" diverge from the page's "+ Add …" text CTA, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 334 - "HOLODEX-491 · "+ Set edition": film-page text CTA, media-page header slot; "Add film" ghost tile"
Cohesion: 0.33
Nodes (5): 2026-09-29 · session, Gates — definition of done, HOLODEX-491 · "+ Set edition": film-page text CTA, media-page header slot; "Add film" ghost tile, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 335 - "HOLODEX-492 · Stress testbed: commit the :9300 preview entries and map `edition` in the fixture"
Cohesion: 0.33
Nodes (5): 2026-09-29 · session, Gates — definition of done, HOLODEX-492 · Stress testbed: commit the :9300 preview entries and map `edition` in the fixture, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 336 - "manifest.mjs"
Cohesion: 0.36
Nodes (7): DEFAULT_MANIFEST, describe(), entries(), here, load(), select(), m

### Community 337 - "HOLODEX-499 · Dev environment never emits audio"
Cohesion: 0.33
Nodes (5): 2026-09-30 · session, Gates — definition of done, HOLODEX-499 · Dev environment never emits audio, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 338 - "HOLODEX-509 · Unattached-tag aliases written canonically in genre writeback"
Cohesion: 0.33
Nodes (5): 2026-10-09 · canonical spelling for unattached-tag aliases, Gates — definition of done, HOLODEX-509 · Unattached-tag aliases written canonically in genre writeback, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 339 - "HOLODEX-532 · Speed up the multi-arch image build"
Cohesion: 0.33
Nodes (5): 2026-10-04 · session, Gates — definition of done, HOLODEX-532 · Speed up the multi-arch image build, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 340 - "HOLODEX-566 · Migrate web imports from $lib to #lib subpath imports"
Cohesion: 0.33
Nodes (5): 2026-10-12 · migrate $lib → #lib, Gates — definition of done, HOLODEX-566 · Migrate web imports from $lib to #lib subpath imports, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 341 - "HOLODEX-569 · Flip horizontally fails verification on real MP4s"
Cohesion: 0.33
Nodes (5): 2026-10-12 · flip tag-verification fix, Gates — definition of done, HOLODEX-569 · Flip horizontally fails verification on real MP4s, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 342 - "HOLODEX-570 · Flip drops cover art on MP4s carrying duplicate covr items"
Cohesion: 0.33
Nodes (5): 2026-10-12 · CoverArt restore skipped for mp4, Gates — definition of done, HOLODEX-570 · Flip drops cover art on MP4s carrying duplicate covr items, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 343 - "HOLODEX-571 · hflip still fails verification on MP4s with no compressor name"
Cohesion: 0.33
Nodes (5): 2026-10-12 · CompressorName exemption, Gates — definition of done, HOLODEX-571 · hflip still fails verification on MP4s with no compressor name, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 344 - "maintenance/snapshot_test.go"
Cohesion: 0.40
Nodes (5): parseStreamHash(), parseXMP(), TestParseStreamHash_MapsOutputToInputIndexes(), TestParseStreamHash_RefusesShortOrGarbledOutput(), TestParseXMP_DropsSourceFileKeepsJSONValues()

### Community 345 - "idNamespaceFor"
Cohesion: 0.40
Nodes (6): candidate(), candidatesFor(), describeFor(), idNamespaceFor(), namespaceOf(), thumbURL()

### Community 346 - "ADR-027: Local `.env` loading for development config"
Cohesion: 0.40
Nodes (5): ADR-027: Local `.env` loading for development config, Consequences, Context, Decision, Rationale

### Community 347 - "QA Checklist: Candidate slot shape per entity kind (HOLODEX-414)"
Cohesion: 0.40
Nodes (5): §1 Setup, §2 Smoke — `[smoke]`, §3 Agent — live, Cinémathèque — `[agent]`, §4 Human — `[human]`, QA Checklist: Candidate slot shape per entity kind (HOLODEX-414)

### Community 348 - "personimage.go"
Cohesion: 0.08
Nodes (25): A — Per-entity `normalize()` function + unique expression index (chosen), ADR-061: Unified entity name-identity — normalized key + aliases + merge across People, Studios, Tags, B — `COLLATE NOCASE` on the canonical column, C — A single `entity_names` table holding canonical + alias rows uniformly, Consequences, Context, Current state, D1 — A normalized name key is the identity, spanning canonical ∪ aliases (+17 more)

### Community 349 - "HOLODEX-262 · Browse-page completeness sort + missing-facet filter chip"
Cohesion: 0.40
Nodes (4): 2026-10-12 · found shipped; leftovers removed, HOLODEX-262 · Browse-page completeness sort + missing-facet filter chip, Session log — newest first (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 350 - "Behavior detail"
Cohesion: 0.40
Nodes (5): Behavior detail, Serving & fallback (RD6 mechanism), Tier 1 sizing logic, Tier 2 sizing logic, Why this doesn't touch enrichment or writeback

### Community 351 - "Spec — Showcase Demo Corpus"
Cohesion: 0.17
Nodes (12): Acceptance criteria, Approach, Content requirements, Follow-ups (tracked separately), Goal, Non-goals, Problem, Showcase surfaces built on this corpus (+4 more)

### Community 353 - "go_pkg_io_fs"
Cohesion: 0.18
Nodes (3): frontendFS(), frontendFS(), spaHandler

### Community 354 - "States and interactions"
Cohesion: 0.50
Nodes (4): Autocomplete (D3), Input and form (D1, D2), States and interactions, Status sort (D4)

### Community 355 - "Requirements"
Cohesion: 0.50
Nodes (4): Future Considerations (P2), Must-Have (P0), Nice-to-Have (P1), Requirements

### Community 356 - "canon"
Cohesion: 0.50
Nodes (3): canon(), permutations(), TestCanonicalPlaylistQueryTable()

### Community 359 - "Archived ADRs"
Cohesion: 0.67
Nodes (3): Archived ADRs, Cross-cutting, Phase specs

### Community 360 - "Implementation notes"
Cohesion: 0.67
Nodes (3): Implementation notes, 2026-09-11 · HOLODEX-369 built (picker caption), searchedFor()

### Community 374 - "routes/+page.svelte"
Cohesion: 0.01
Nodes (248): mediaBrief, 1. Media — seeded deterministic ordering via a custom SQLite function, 2. The `sort` / `seed` query contract, 3. People / Tags — client-side seeded shuffle, Decision, Action items, D6. One parser/serialiser per page, one shared module, D7 — A run is client-held state over an id list fetched once; shuffle is client-side and seeded (+240 more)

### Community 398 - "completeness_boot.go"
Cohesion: 0.21
Nodes (6): completenessInputsFingerprint(), hashFile(), markCompletenessDirtyIfInputsChanged(), TestMarkCompletenessDirtyIfInputsChanged(), TestMarkCompletenessDirtyIfInputsChanged_FailuresReScore(), completenessStore

### Community 434 - "entity-completeness-score.md"
Cohesion: 0.06
Nodes (29): Accessibility, Chip anatomy, Collision review line, Design handoff — alias collapse (HOLODEX-306), Out of scope, States, Theming, What changes on screen (+21 more)

### Community 458 - "SourceBadge.svelte"
Cohesion: 0.04
Nodes (66): 1. Resolved decisions, 2. New component: `TagLinkChip.svelte`, 3. Call-site changes, 4. Backend requirement, 5. Design tokens used, 6. States and interactions, 7. Responsive behavior, 8. Edge cases (+58 more)

### Community 459 - "Load"
Cohesion: 0.10
Nodes (29): Context, A local `.env` feeds the environment layer for development, A single `DATA_PATH` root holds all persistent state, Configuration and settings, One immutable `Config`, resolved once at startup by layered precedence, Parallel YAML files for structured, per-deployment config, Promoting a config value to an owner-editable runtime setting, 2026-08-21 · session (cont. 2) (+21 more)

### Community 461 - "ReadbackGaps"
Cohesion: 0.09
Nodes (29): ADR-119: A text field writeback cannot read back is witnessed by the write ledger, and the gap is shown to the owner, Consequences, Context, Decision, Rejected alternatives, 1. Written gap rows read `=` (no UI change), 2. The hint line: decided, unknown, never written, 3. System Activity: Mapping checks (+21 more)

### Community 465 - "holoShuffle"
Cohesion: 0.67
Nodes (3): holoShuffle(), registerShuffle(), TestHoloShuffle()

### Community 468 - "cascadeServer"
Cohesion: 0.27
Nodes (10): 2026-08-25 · implementation landed — all seven gates green, cascadePost(), seedCascadeVideo(), TestCascadeFilmStudio_AllCollide_EmptyBatch(), TestCascadeFilmStudio_PartialCollision_BestEffort(), TestCascadeFilmStudio_SameValueRedecide_NotACollision(), TestCascadeFilmStudio_UnmatchedProvider_PerVideoError(), TestCascadeFilmStudio_ZeroVideos_EmptyBatch() (+2 more)

### Community 469 - "extractReviewServer"
Cohesion: 0.23
Nodes (15): extractReviewGET(), extractReviewPOST(), extractReviewServer(), TestDismissExtractionReview(), TestExtractionQueue_Empty(), TestExtractionQueue_ListsPendingRowsVideoJoined(), TestExtractionQueue_RejectsMalformedVideoID(), TestExtractionQueue_VideoIDFilter() (+7 more)

### Community 476 - "Spec: Quick Wins batch — Search history & "More with …" shelves"
Cohesion: 0.12
Nodes (16): Future Considerations (P2), Goals, Must-Have (P0), Nice-to-Have (P1), Non-Goals, Open Questions, Problem Statement, QW1 — Search history (client-only) (+8 more)

### Community 477 - "Design Handoff: Poster View for the People list page (F55)"
Cohesion: 0.13
Nodes (15): Accessibility, Component, CSS (new, additive — `app.css`, filed next to the `.portrait-frame` block), Design Handoff: Poster View for the People list page (F55), Design tokens used (all surfaces), Edge cases, Gate status (mirrors the spec), Markup (+7 more)

### Community 479 - "Spec: Metadata Source Plugins (F22/F27/F28)"
Cohesion: 0.05
Nodes (35): 2026-10-14 · session, Dropped — newest first (the reason is the point), HOLODEX-86 · Success toast after enrich, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), 2026-07-10 · what happened this session, 2026-10-13 · picker names an unreachable provider, Dropped — newest first (the reason is the point) (+27 more)

### Community 480 - "Design Handoff: Provider Link Badge — Multi-Badge States for Person/Studio (HOLODEX-266)"
Cohesion: 0.12
Nodes (17): 1. Placement on person and studio pages, 2. Cardinality states (0 / 1 / N), 3. Degraded state: id present, no link template, 4. Interaction and accessibility, 5. Film and media headers (F63 — HOLODEX-390, spec [provider-link-badge-coverage.md](../specs/provider-link-badge-coverage.md)), 6. Media header: the provider match is a pill too (HOLODEX-424), Badge anatomy (recap, unchanged), DD1 — Badges join the existing muted metadata line, not a new row (+9 more)

### Community 490 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.29
Nodes (7): 2026-08-25 · resolved three rounds of merge conflicts against a fast-moving main, 2026-08-25 · security-review landed — all five pre-implementation gates green, 2026-08-25 · spec, ADR, and design handoff landed, 2026-08-25 · testing-strategy landed; mockup persistence established as a standing rule, Gates — definition of done, HOLODEX-285 · Unified Studio edit affordance + Film-level cascade writeback, Session log — append-only (cap: last 8 sessions; older → archive/)

### Community 504 - "net/http.ResponseWriter"
Cohesion: 0.04
Nodes (55): curationBody, playlistBody, 2026-08-12 · Root-caused and fixed the nil-slice marshaling bug, audited for siblings, 2026-08-25 · full implementation + simplify + verification, Gates — definition of done, HOLODEX-286 · Generalize the entity-image pipeline (Person → Studio → Film), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority) (+47 more)

### Community 505 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.17
Nodes (11): 2026-09-11 · contract text written, 2026-09-11 · design handoff for the caption, 2026-09-11 · epic refreshed from the filename probe, ADR-095 drafted, 2026-09-11 · F54 spec amended, 2026-09-11 · HOLODEX-368 built (request side), 2026-09-11 · HOLODEX-372 — the caption's stressed state in the geometry harness, 2026-09-11 · security review of the design, 2026-09-11 · testing strategy (+3 more)

### Community 506 - "Design Handoff: Studio relationship-edit popover (HOLODEX-271)"
Cohesion: 0.06
Nodes (30): Accessibility Notes, Addendum (HOLODEX-289): trigger position, visibility, and empty-state CTA, Components, Decision: empty-state CTA — "+ Add studio" text button, not a bare pencil, Decision: pencil position — trailing, not leading, Decision: visibility — always-visible, not hover-revealed, Design Handoff: Studio relationship-edit popover (HOLODEX-271), Design Tokens Used (+22 more)

### Community 507 - "HOLODEX-288 · Fix film-studio cascade code-review findings"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-25 · session, Gates — definition of done, HOLODEX-288 · Fix film-studio cascade code-review findings, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 511 - "Design handoff: Responsive page width — player column, metadata rail, intrinsic grid density"
Cohesion: 0.10
Nodes (20): 1. Overview, 1a. Most pages are already full width, 1b-i. Interim change already shipped: max density is now 8 columns, 1b. What's actually wrong, 1c. The rule, 3. Design tokens, 4. Decisions and their reasons, 4a. Why a rail rather than simply removing the cap (+12 more)

### Community 523 - "Spec: Owner tooling hub + visitor/owner nav split (F35)"
Cohesion: 0.10
Nodes (21): §1 Setup, §3 Agent live QA (preview tools against §1 stack), §4 Human eyes — Cinémathèque QA, QA Checklist: Derived / calculated person fields — Age & Age at death (F45), 1. Setup / preconditions, 3. Agent — drive the running app, 4. Human — needs your eyes (Cinémathèque), Manual QA Checklist: Entity name-identity — merge, alias & duplicate review (F43) (+13 more)

### Community 531 - "tag-set-writeback-handoff.md"
Cohesion: 0.06
Nodes (27): 1. Dialog — the tag-set row (`WritebackFormDialog.svelte`), 2. Media page — on-file glyph (`TagLinkChip.svelte`), 3. Tokens and skins, 4. Edge cases, Design handoff — Tag set writeback: dialog Tags row + on-file chip glyphs (HOLODEX-401), Owner decisions (2026-09-26, question cards over an inline mockup), 2026-08-29 · session, 2026-08-29 · session (+19 more)

### Community 540 - "New"
Cohesion: 0.39
Nodes (9): New(), jpegBytes(), TestSinkRefusesPortraitFilmBanner(), TestSinkRollsBackOnStoreFailure_Person(), TestSinkRollsBackOnStoreFailure_Studio(), TestSinkSkipsDuplicate(), TestSinkStoreAsset_Person_Normalizes(), TestSinkStoreAsset_Studio_Normalizes() (+1 more)

### Community 542 - ".TagNamesForVideo"
Cohesion: 0.06
Nodes (48): API & auth, Action Items, ADR-077: Tag writeback exclusion — per-tag Genre writeback flag + manual sync batch seam, Consequences, Context, Current state (survey, 2026-07-31), D1 — `tags.writeback_enabled` column; filtered at `TagNamesForVideo`, uniformly per name regardless of how it was reached, D1 — where the flag is enforced (+40 more)

### Community 544 - "Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369)"
Cohesion: 0.20
Nodes (10): Batch path — Activity detail row (no new markup), Content spec, Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369), Design-system fit, Edge cases, Non-goals, Overview, Placement (+2 more)

### Community 546 - "HOLODEX-255 · <epic title>"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-05 · session, Gates — definition of done, HOLODEX-255 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 547 - "archive/README.md"
Cohesion: 0.07
Nodes (21): ADR-045 (owner-session, per promote-override-fields.md): owner gate / Admin mode / effectiveOwner, ADR-046 (per qa-metadata-curation.md): Metadata curation and write queue, ADR-065: Typed field registry and relationship-scoped computed fields (Superseded), Architecture Decision Records, Topic docs (current truth), ADR-056 (provider field render hints, F39), ADR-074 (claimed provider keys), Claiming a provider key (F49) cookbook (+13 more)

### Community 548 - "HOLODEX-114 · <epic title>"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-05 · session, Gates — definition of done, HOLODEX-114 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 553 - "HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values"
Cohesion: 0.24
Nodes (11): 2026-08-13 · code-review xhigh --fix, 2026-08-13 · implementation + tests + docs sync + PR opened, Gates — definition of done, HOLODEX-258 · Reject malformed `_studio_external_ids` sidecar values, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), Timeline / routing, TestSanitizeFieldsCaps() (+3 more)

### Community 555 - "HOLODEX-244 · <epic title>"
Cohesion: 0.33
Nodes (5): 2026-07-10 · what happened this session, Gates — definition of done, HOLODEX-244 · <epic title>, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 556 - "HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip"
Cohesion: 0.33
Nodes (5): 2026-08-29 · session, Gates — definition of done, HOLODEX-293 · Migrate categories/[id] tag chips to shared TagLinkChip, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 568 - "toSearchItem"
Cohesion: 0.22
Nodes (9): Future considerations (P2), Must-have (P0), Requirements, Should-have (P1), thumbnailURL(), toSearchItem(), HasThumbnailImage(), searchItem (+1 more)

### Community 569 - "Design handoff: Manage block — destructive split button"
Cohesion: 0.20
Nodes (10): 1. What's wrong, 2. The rule, 2a. Why the default segment stays warn, 2b. Why split rather than a plain menu button, 2c. What this does not change, 3. Reuse, 4. Keyboard and screen reader, 5. Contrast (measured, not assumed) (+2 more)

### Community 573 - "metrics.go"
Cohesion: 0.21
Nodes (11): formatFloat(), New(), newHistogram(), findLine(), scrape(), TestExposition(), TestHistogramBucketing(), writeCounter() (+3 more)

### Community 575 - "Spec: Instance skin — owner-set skin on an Appearance tab + custom palette from `holodex.yaml` (F67)"
Cohesion: 0.06
Nodes (36): Action Items, ADR-102: Instance skin — the skin as owner-set instance identity, a `settings` store for library-owned settings, and a derived custom palette applied as inline custom properties, Applying a custom palette, Consequences, Context, D1 — The skin is instance identity: one server-held value, applied to every viewer, D2 — A generic `settings` key/value table holds library-owned settings; YAML/env holds the deployment, D3 — Read through `/capabilities`, write through one owner-gated endpoint (+28 more)

### Community 579 - "QA: TMDB Provider Sidecar + ADR-039 Core Changes"
Cohesion: 0.10
Nodes (23): 0. Setup, 1. Provider contract — smoke (no real TMDB, no network), 2. ADR-039 core changes — `asset_hosts` allowlist, 3. End-to-end via Holodex + real TMDB provider, 4. Provider image (Docker), 5. Security checks, 6. Non-functional, 7. Film / Video enrichment (F26) (+15 more)

### Community 580 - "films/[id]/+page.svelte"
Cohesion: 0.02
Nodes (149): Gates — definition of done, busy, {
		completeness,
		videoId,
		onchanged
	}, error, expanded, GROUPS, headline, showExtras (+141 more)

### Community 584 - "CompletenessRing.svelte"
Cohesion: 0.15
Nodes (20): Completeness components, Rules, bands, busy, key, reading, refresh(), {
		required,
		extras,
		size = 'card',
		entity,
		onrefreshed
	} (+12 more)

### Community 588 - "probe-edition-filenames.mjs"
Cohesion: 0.26
Nodes (12): classify(), hasKeyword(), KEYWORD_RE, KEYWORDS, keywordsIn(), main(), ADR-0096, printReport() (+4 more)

### Community 591 - "coverart_test.go"
Cohesion: 0.06
Nodes (38): ADR-110: A genres writeback makes the file's tag keys match the UI — filter the extra keys, ignored tags are UI-only, Consequences, Context, Decision, Rejected, 2026-10-01 · root-caused the dropped MKV tags; extended -ee to the writeback readers, Dropped — newest first (the reason is the point), Gates — definition of done (+30 more)

### Community 594 - "tmdbClient"
Cohesion: 0.14
Nodes (19): 3 — TMDB is the first provider; it needs an entity-type-aware remap, not new endpoints, 2026-09-18 · session, Gates — definition of done, HOLODEX-414 · Candidate slot shape per entity kind (media backdrop, studio logo), Session log, Up next, disambiguate(), movieThumbURL() (+11 more)

### Community 598 - "Design Handoff: Extract from filename on the media detail page (F48.5a)"
Cohesion: 0.20
Nodes (10): Accessibility, Backend change, Design Handoff: Extract from filename on the media detail page (F48.5a), Goals, How this fits with provider enrichment, Non-goals, Problem, QA (+2 more)

### Community 599 - "HOLODEX-298 · Film detail page: match media page's Tags section styling"
Cohesion: 0.29
Nodes (6): 2026-08-30 · Matched film detail Tags section to the media page's styling, 2026-08-30 · Repositioned Tags back into the header column, underneath Studio, Gates — definition of done, HOLODEX-298 · Film detail page: match media page's Tags section styling, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 601 - "HOLODEX-302 · Person hero image hover-to-front"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-08-31 · implementation + verification + ticket/worklog, Gates — definition of done, HOLODEX-302 · Person hero image hover-to-front, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 602 - "HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise"
Cohesion: 0.29
Nodes (6): 2026-09-01 · session, 2026-09-01 · session (code-review), Gates — definition of done, HOLODEX-305 · Person hero: bio hidden behind banner; remove banner hover-raise, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 604 - "header-narrow-width-handoff.md"
Cohesion: 0.07
Nodes (27): 1. The problem, 2. The decision, 2a. Why not collapse the nav into a menu, 3. QA, Design handoff: the header at narrow widths, Human, Setup, Smoke (+19 more)

### Community 605 - "images_test.go"
Cohesion: 0.12
Nodes (20): 2026-09-08 · HOLODEX-345 — adversarial image set across four kinds, 2026-09-19 · root-caused against live API data, fixed, verified end to end, Gates — definition of done, HOLODEX-433 · Poster picker: undecided provider tile never writes + tiles crop the image, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), imageTargets, storedImage (+12 more)

### Community 606 - "File maintenance"
Cohesion: 0.09
Nodes (26): A fixed encoder table: same codec, constant quality, source colour carried through, A separate topic from writeback, sharing its queue, File maintenance, One `writeback_queue` with a `kind`, two worker lanes, and one running job per file, Preflight is computed on request, not cached, The re-encode: ffmpeg makes the picture, the container's own tool rebuilds the file, The swap is two same-directory renames, journaled in `held_originals`, The verification gate compares probes, not exit codes (+18 more)

### Community 607 - "HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list"
Cohesion: 0.33
Nodes (5): 2026-08-30 · Fixed empty candidate list in the film→video bulk attach dialog, Gates — definition of done, HOLODEX-299 · Film→video bulk attach dialog: fix empty candidate list, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 610 - "FilmBulkAttachDialog.svelte"
Cohesion: 0.03
Nodes (92): 2026-09-06 · Design settled from a hand sketch; handoff + mockup committed, 2026-09-06 · Implemented the design; three-skin QA caught a real AA failure, 2026-09-06 · `/simplify` caught a real perf regression the implementation introduced, Gates — definition of done, HOLODEX-328 · Media detail Films + People: collapse empty sections, port the Scenes pill, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority), 2026-10-13 · session (+84 more)

### Community 628 - "Design Handoff: Delete a film (F78)"
Cohesion: 0.07
Nodes (27): A · Manage section (film detail rail), Accessibility, B and C · Confirm dialogs, D · Trash page, Design Handoff: Delete a film (F78), E · In-Trash verdict, Edge cases, Overview (+19 more)

### Community 629 - "HOLODEX-321 · Broken in-page anchors in the provider hand-off specs"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-09-05 · three dead anchors fixed; found by writing the checker properly the second time, Gates — definition of done, HOLODEX-321 · Broken in-page anchors in the provider hand-off specs, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 640 - "HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`)"
Cohesion: 0.29
Nodes (6): 2026-09-14 · root-caused, harness stops on a dead server, docs, confirmed on Node 24.19, Findings (2026-09-13/14), Gates — definition of done, HOLODEX-381 · Full geometry matrix crashes the Vite dev server (`0xC0000409`), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 642 - "HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster"
Cohesion: 0.29
Nodes (5): 2026-09-16 · diagnosed, fixed, tested, Gates — definition of done, HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 643 - "EnrichPicker.svelte"
Cohesion: 0.03
Nodes (88): Content spec, Design Handoff: Candidate slot shape per entity kind (HOLODEX-414), Edge cases, Keyboard and accessibility, Non-goals, Overview, Placement, Responsive (+80 more)

### Community 645 - "QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)"
Cohesion: 0.40
Nodes (5): §1 Setup, §3 Agent (live, one skin), §4 Human (Cinémathèque), §5 Known gaps, QA Checklist: Claimed provider keys — the Attach affordance and the Attached keys list (F49)

### Community 647 - "HOLODEX-284 · Film provider enrichment (ADR-086)"
Cohesion: 0.50
Nodes (3): HOLODEX-284 · Film provider enrichment (ADR-086), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 648 - "tmdb/main.go"
Cohesion: 0.09
Nodes (28): ADR-094: Local development credentials come from the environment, never a repo file, Alternatives considered, Consequences, Decision, 2026-09-08 (last) · verified the premise, then hardened the mechanism, Gates — definition of done, HOLODEX-341 · Local dev credentials come from the environment; CI enforces it, Session log — append-only (cap: last 8 sessions; older → archive/) (+20 more)

### Community 649 - "EnrichmentRow"
Cohesion: 0.12
Nodes (11): FilmBilledCredit, Constraints, Context, A film's cast, tags and studios are read-time unions; nothing on a film writes to its videos, Handlers, idOrZero(), normalizedName(), TestPersonExternalIDsFromRows() (+3 more)

### Community 651 - "HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram"
Cohesion: 0.33
Nodes (5): 2026-09-14 · diagnosed, fixed, live-verified, Gates — definition of done, HOLODEX-383 · Person/Studio/Tag Films row always draws the monogram, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 654 - "Spec: Dev-time stress fixture for UX and layout QA"
Cohesion: 0.11
Nodes (14): Acceptance criteria, In scope, Ladder dimensions, Objective, Open questions, Out of scope, Related, Scope (+6 more)

### Community 657 - "Studio"
Cohesion: 0.09
Nodes (30): 4. Backend requirement (blocking), 4. Backend — unchanged, 11. QA checklist, 2. Component change: `StudioLinkCard.svelte`, 3. Call sites — unchanged, 4. Backend — unchanged, 5. Design tokens used, 6. States (+22 more)

### Community 659 - "urlParamID"
Cohesion: 0.06
Nodes (33): Film membership is an asserted link with no reconciler, 2026-08-21 · session (cont. 4), 2026-08-24 · session (cont.), 2026-08-27 · Implemented film_people_roles CRUD, Gates — definition of done, HOLODEX-281 · film_people_roles CRUD (film-level billing/role data), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority) (+25 more)

### Community 661 - "ADR-038-person-images.md"
Cohesion: 0.04
Nodes (48): ADR-014: Configuration Strategy & Data Directory Layout, Configuration precedence (highest wins), Consequences, Context, Data directory layout, Decision, Key path variables, ADR-015: Media File Serving — HTTP Range Requests (+40 more)

### Community 665 - "Session log — append-only (cap: last 8 sessions; older → archive/)"
Cohesion: 0.20
Nodes (9): 2026-09-15 · design gate, 2026-09-16 · frontend + live verification, 2026-09-16 · human QA pass → ready for review, 2026-09-16 · post-ready code-review medium --fix, 2026-09-16 · session, Gates — definition of done, HOLODEX-384 · Entity Films row: card-height match + hover lift, Session log — append-only (cap: last 8 sessions; older → archive/) (+1 more)

### Community 672 - "HOLODEX-247 · Studio image roles: icon, logo, poster (F51)"
Cohesion: 0.33
Nodes (6): 2026-07-10 · what happened this session, 2026-08-04 · Full epic delivered end-to-end and merged, Gates — definition of done, HOLODEX-247 · Studio image roles: icon, logo, poster (F51), Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 679 - "HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`"
Cohesion: 0.29
Nodes (6): 2026-07-10 · what happened this session, 2026-09-05 · the guard landed and immediately caught a kind nobody had noticed, Gates — definition of done, HOLODEX-315 · TMDB `/describe` under-declares `asset_kinds`, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

### Community 680 - "HOLODEX-397 · Studio logo on the Film and Media detail pages"
Cohesion: 0.29
Nodes (6): 2026-09-16 · design gate, 2026-09-17 · implemented, agent-QA'd, Gates — definition of done, HOLODEX-397 · Studio logo on the Film and Media detail pages, Session log — append-only (cap: last 8 sessions; older → archive/), Up next — ordered (position = priority)

## Knowledge Gaps
- **413 isolated node(s):** `holodex`, `Handlers`, `Handlers`, `Handlers`, `Handlers` (+408 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 5359 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **134 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Index` connect `Index` to `Gates — definition of done`, `Spec: Studio as a first-class entity (F38)`, `Service`, `resolver.go`, `Handlers`, `ExtraMetadata`, `placeholders`, `title`, `.requireOwner`, `detail`, `.TagNamesForVideo`, `resolveOrCreateByName`, `extractor.go`, `provider`, `q`, `.RelinkVideoStudios`, `SweepRunner`, `Find`, `newFakeRepo`, `Queue`, `.setFieldDecision`, `Field`, `Action Items`, `WritebackFormDialog.svelte`, `assetRoleFor`, `Video`, `newRepoDB`, `Load`, `ReadbackGaps`, `genre_integration_test.go`, `Normalize`, `Archived ADRs`, `.getMedia`, `Observability and jobs`, `Decision`, `.downloadAssets`, `Action Items`?**
  _High betweenness centrality (0.122) - this node is a cross-community bridge._
- **Are the 220 inferred relationships involving `newRepo()` (e.g. with `TestAliasesSurviveRescan()` and `TestMergePersons()`) actually correct?**
  _`newRepo()` has 220 INFERRED edges - model-reasoned connections that need verification._
- **What connects `holodex`, `Handlers`, `Handlers` to the rest of the system?**
  _413 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `types.ts` be split into smaller, more focused modules?**
  _Cohesion score 0.013195978961127883 - nodes in this community are weakly interconnected._
- **Why does `title()` connect `title` to `types.ts`, `Design handoff: Media detail Metadata — move, trim, fold`, `Complete`, `holodex/main.go`, `PersonLinkChip.svelte`, `Spec: Holodex Metadata Provider Contract (hand-off, protocol v1)`, `context.Context`, `Design Handoff: Entity Completeness Score — Remediation Queue & Breakdown Panel (HOLODEX-260)`, `Service`, `resolver.go`, `Design Handoff: Owner tooling hub + nav split (F35)`, `Handoff Spec: Smart playlists, Play all and Shuffle (F75)`, `Studio`, `Handlers`, `ADR-038-person-images.md`, `placeholders`, `Design handoff — Entity identity card (F60)`, `Design handoff: `?` keyboard-shortcuts sheet + hotkey firing feedback (F62)`, `Design Handoff: Unified name-edit mechanism (HOLODEX-269)`, `tmdb.go`, `detail`, `StudioPicker.svelte`, `Decision`, `Design Handoff: Enrich picker "Searched" caption from `/resolve` `searched[]` (HOLODEX-369)`, `resolveOrCreateByName`, `Index`, `api.ts`, `Unified name-edit mechanism (F56.2, HOLODEX-269)`, `q`, `Queue`, `Spec: Duplicates pair evidence — expand a flagged pair into a symmetric side-by-side compare panel (F70)`, `newFakeRepo`, `Video composite-key collision check (F56.3, HOLODEX-270)`, `#lib`, `stack.md`, `.setFieldDecision`, `Action Items`, `SanitizeValue`, `reference/ui-vocabulary.md`, `WritebackFormDialog.svelte`, `assetRoleFor`, `Video`, `ReadbackGaps`, `TagForField`, `itoa`, `Design handoff: studio image halo toggle (HOLODEX-463)`, `generate`, `Spec — Showcase Demo Corpus`, `Design handoff: Films entity (F56)`, `Requirements`, `.mergeEntity`, `Design handoff: Media detail — Films + People sections`, `BatchRunner`, `.getMedia`, `Options Considered`, `Design Handoff: Delete a film (F78)`, `.downloadAssets`, `HOLODEX-513 · The linked film decides a video's Film and Title by default`, `Action Items`, `Design handoff: Media detail stage layout`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **Should `people/[id]/+page.svelte` be split into smaller, more focused modules?**
  _Cohesion score 0.016011138183083886 - nodes in this community are weakly interconnected._
- **Why does `sweep()` connect `newRepoDB` to `holodex/main.go`, `status/+page.svelte`, `Index`, `api.ts`, `testing.T`, `BatchRunner`, `SweepRunner`, `Repo`, `Service`, `Action Items`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._