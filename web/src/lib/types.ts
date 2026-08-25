// Shape of the REST payloads (mirrors internal/model + handlers, ADR-006).

// PersonAlias is one owner-curated alternate name (F23, ADR-036). The id is the
// stable handle the delete control uses. Shared verbatim by studio + tag aliases
// (F43, ADR-061) — the server's EntityAlias has the same `{ id, alias }` shape.
export interface PersonAlias {
	id: number;
	alias: string;
}

// EntityKind names the three identity entities that share the alias/merge/rename
// spine (F43, ADR-061). Maps to the REST base (people | studios | tags) in the client.
export type EntityKind = 'person' | 'studio' | 'tag';

// EntityRef is the minimal shape Person/Studio/Tag all satisfy — used by the generic
// identity surfaces (F43): the merge-picker rows and the collision/conflict card, and
// the 409 conflict body of add-alias / rename.
export interface EntityRef {
	id: number;
	name: string;
	video_count?: number;
}

// VideoCollisionRef is the minimal shape the composite-key collision 409 body returns for the
// OTHER (colliding) video — enough for CollisionOfferCard to render without a follow-up fetch
// (HOLODEX-270). Deliberately separate from EntityRef: a video isn't on the identity spine and
// carries different identifying fields (people/date/studio, not a single name).
export interface VideoCollisionRef {
	id: number;
	title: string;
	people: string[]; // display names, already resolved server-side — no extra lookup needed
	recorded_at: string | null; // ISO date, same format as Video.recorded_at
	studios: string[]; // display names; empty when the video has no studio linked
}

// DuplicatePair is one flagged possible-duplicate (F43 S5, ADR-061): two entities that
// are a loose-key near-miss (not an exact-nameKey match) and the variation kind. Served
// by GET /owner/duplicates, grouped tags-first.
export interface DuplicatePair {
	entity_type: EntityKind;
	a: EntityRef;
	b: EntityRef;
	variation: string; // 'internal-whitespace' | 'punctuation'
}

export interface Person {
	id: number;
	name: string;
	video_count?: number;
	// Headshot image id on the people-list read — the avatar's ?v= cache-buster so the
	// list refreshes when the headshot changes (e.g. after enrichment) instead of showing
	// the stale cached image (F25.29). Absent/0 = no headshot (placeholder).
	headshot_version?: number;
	// Poster image id on the people-list read (F55 P0-6) — independent of headshot_version;
	// a person can have one role without the other. Absent/0 = no poster (placeholder).
	poster_version?: number;
	aliases?: PersonAlias[]; // present on the person-detail read (F23)
	// role is the video_people link role this person holds on the video being read
	// (HOLODEX-272). Present only on a video's People list — a dual-role attachment
	// surfaces as two Person entries sharing the same id. Absent elsewhere.
	role?: 'actor' | 'director';
}

// ResolvedPerson narrows Person.role to required — the shape a video's People list
// (video.people) always carries, vs. Person's other read contexts where it's absent.
export type ResolvedPerson = Person & { role: 'actor' | 'director' };

// Person image roles (F25, ADR-038). Three single-slot core roles plus the
// free-form `extra` gallery. Mirrors model.ValidPersonImageRole on the server.
export type PersonImageRole = 'headshot' | 'banner' | 'poster' | 'extra';

// The three single-slot core roles (the croppable, promotable slots — not the `extra`
// gallery). Canonical list + derived type, so the role set lives in exactly one place;
// `satisfies` guarantees every entry is a real non-extra role.
export const CORE_ROLES = ['headshot', 'banner', 'poster'] as const satisfies readonly Exclude<
	PersonImageRole,
	'extra'
>[];
export type CoreRole = (typeof CORE_ROLES)[number];

// PersonImage is one stored gallery image in the person-detail read model (F25).
// Version mirrors the id and drives the `?v=` cache-buster on serving URLs.
export interface PersonImage {
	id: number;
	role: PersonImageRole;
	source: string; // 'upload' | 'enrichment' | 'promoted'
	version: number;
	width: number;
	height: number;
	sort_order: number;
	created_at: string;
}

// PersonImageSet is the person-detail image read model (F25): which core roles are
// filled (with the `?v=` version) and the ordered gallery. Empty core roles are
// simply absent from `roles`.
export interface PersonImageSet {
	roles: Record<string, { present: boolean; version: number }>;
	gallery: PersonImage[];
}

export interface Tag {
	id: number;
	name: string;
	video_count?: number;
	// Owner-curated alternate names (F43, ADR-061), each searchable. Present on the
	// /tags list and the tag-detail read; omitted (undefined) elsewhere.
	aliases?: PersonAlias[];
	// Per-video provenance — "file" | "manual" | "provider:<name>" (F50, ADR-075 D3).
	// Present only on Video.tags; absent elsewhere (no single video context for it).
	source?: string;
	// The broader tag this tag sits under, or absent at the root (F50, ADR-075 D1).
	// Present on both the /tags list and the tag-detail read.
	parent_tag_id?: number;
	// Ancestor chain, root-first (F50, ADR-075 D1 P1-3) — the tag-detail breadcrumb.
	// Present only on the tag-detail read (getTag); absent on the /tags list.
	ancestors?: string[];
	// Direct children (one level, not the full subtree), name-ordered (HOLODEX-259).
	// Present only on the tag-detail read (getTag); absent on the /tags list.
	children?: EntityRef[];
	// Category memberships, name-ordered (HOLODEX-259, ADR-078). Present only on
	// the tag-detail read (getTag); absent on the /tags list.
	categories?: EntityRef[];
	// Whether this tag's name contributes to a video's Genre writeback value
	// (HOLODEX-239, ADR-077 D1) — defaults true. Present and accurate on both
	// the /tags list (ListTags batch-attaches it, same as parent_tag_id) and
	// the tag-detail read.
	writeback_enabled?: boolean;
}

// Category groups tags for browsing without merging or altering them
// (HOLODEX-240, ADR-078) — hand-curated, no provenance/alias/merge. TagCount
// is present on both the /categories list (the pill's count badge) and the
// detail read; Tags (the member list) only on the detail read.
export interface Category {
	id: number;
	name: string;
	tag_count: number;
	// Member tag ids — present on the /categories list only (drives the "Remove
	// from category…" picker's client-side filter to categories that actually
	// contain one of the selected tags); absent on the detail read, which has
	// `tags` (full objects) instead.
	tag_ids?: number[];
	tags?: Tag[];
}

// DeniedTag is one globally blocked term (F50, ADR-075 D2) — exact-match,
// case-insensitive, unconditioned on any entity or provider. Served by the
// owner's /owner/tags "Deny-list" tab.
export interface DeniedTag {
	term: string;
	created_at: string;
}

// Studio is a first-class entity (F38, ADR-053). Same read shape as Tag; its name is
// derived identity (video_studios follows the resolved studio field, no rename/merge).
export interface Studio {
	id: number;
	name: string;
	video_count?: number;
	// Self-hosted image roles (F51, ADR-079): icon (studios list well), logo (detail
	// page header), poster (no consumer yet). Each is independently owner-editable
	// (upload/replace/remove) and provider-sourced by default; present only when that
	// role's slot is filled. Always populated on both list and detail reads.
	icon_url?: string;
	logo_url?: string;
	poster_url?: string;
	// Owner-curated alternate names (F43, ADR-061), each searchable. Present on the
	// studio-detail read; omitted (undefined) elsewhere.
	aliases?: PersonAlias[];
}

// StudioImageRole is the enum of editable studio image slots (F51, ADR-079).
export type StudioImageRole = "icon" | "logo" | "poster";

export interface ExtraMetadata {
	source_key: string;
	value: string;
}

export interface Video {
	id: number;
	file_path: string;
	file_size: number;
	title: string;
	duration_sec: number;
	width: number;
	height: number;
	video_codec?: string; // ffprobe stream/format details (F12.4)
	audio_codec?: string;
	bitrate_kbps?: number;
	container?: string;
	recorded_at?: string | null;
	indexed_at: string;
	thumbnail_url?: string | null; // present once an image exists (ADR-009)
	poster_url?: string | null; // larger detail-page poster tier (F53); falls back to thumbnail bytes server-side until generated
	poster_uploaded?: boolean; // true when the poster is an owner upload (F52)
	people?: Person[];
	tags?: Tag[];
}

export interface MediaListResponse {
	items: Video[];
	total: number;
	limit: number;
	offset: number;
}

// MappedField is a configurable canonical field resolved for one video (F20.3).
export interface MappedField {
	canonical: string;
	label: string;
	values: string[];
}

// ResolvedValue is one surviving value of a field with its provenance + curation
// state (F30). A value present in multiple sources is reported once, with every
// contributing source namespace listed in `sources`.
export interface ResolvedValue {
	value: string;
	sources: string[]; // contributing namespaces, e.g. ["tmdb","file"], ["manual"]
	manual?: boolean; // owner-added value
	no_write?: boolean; // shown but excluded from the file write
}

// ResolvedField is one canonical field merged from all configured sources (F27/F30).
// `values` holds the surviving display values; `items` carries the per-value
// provenance + curation state the curation UI consumes. winning_source is the
// namespace:key that supplied the (first) value. display mirrors the registry hint.
export interface ResolvedField {
	canonical: string;
	label: string;
	display?: 'long_text' | 'image_url' | 'url' | 'chips';
	values: string[];
	items?: ResolvedValue[];
	multi?: boolean; // merge-mode (set) field: UI shows add + per-value remove
	winning_source?: string; // e.g. "tmdb:title" | "file:Title" | "manual:genres"
	// F39 (ADR-056) — a display-only, presence-driven auto-registered non-canonical
	// field: rendered read-only (label + values + provenance) with NO source-decision
	// or curation controls, in the "Additional details" group after the curatable
	// fields. Canonical/mapped fields omit it.
	auto_registered?: boolean;
	// F44 (ADR-062) — this non-canonical field became first-class curatable via an in-app
	// promotion (not a native mapping). It is otherwise a normal mapped field; the flag only
	// tells the SPA to offer the owner-only Edit / Remove-promotion affordance on this row.
	promoted?: boolean;
	// F45 (ADR-063) — a computed-on-read, source-less, read-only derived field (e.g. Age).
	// Renders a bare value + the muted "calculated" provenance icon; carries NO
	// decision/candidates/in_sync and is never adoptable/curatable. winning_source is
	// "computed:<canonical>".
	computed?: boolean;
	// F45 — the human LABELS of the inputs this value was derived from (e.g. ["Born"]),
	// for the "calculated from …" provenance copy. Backend-supplied so the SPA needs no registry.
	derived_from?: string[];
	// F36 (ADR-051) — per-field source-of-truth, present on replace (scalar) fields only.
	// `decision` is the standing source choice (absent ⇒ implicit file default); `in_sync`
	// is false when the decided value differs from the value embedded in the file; the
	// `candidates` feed the SourceSelect segments + the candidates line.
	decision?: FieldDecision;
	in_sync?: boolean;
	candidates?: FieldCandidate[];
	// F40 (ADR-072) — marks a field whose resolved value(s) name a linkable entity;
	// mirrors registry.FieldDef.EntityKind. CurationFieldRow's "+ Add" opens the
	// entity-search LinkPicker instead of a bare text input when this is set.
	entity_kind?: 'person' | 'studio' | '';
	// write_target (HOLODEX-216) — the destination file tag this field maps to for
	// the video's current container (e.g. "QuickTime:Artist"), absent/empty when the
	// container has no writeback mapping for this canonical. Video-only.
	write_target?: string;
}

// F36 — Per-field source-of-truth decisions (ADR-051). A standing, per-item, per-field
// choice naming which source is *true* for a replace (scalar) field: it overrides global
// mapping precedence and drives both display and writeback. Merge (set) fields are
// unchanged — they keep F30 per-value curation; the source decision is replace-only.

// DecisionSource names the pinned source: the entity's baseline — `file` for videos,
// `record` for persons (F37, RD4: the baseline label is per-entity) — a specific matched
// provider (`provider:<name>`), or a frozen manual literal.
export type DecisionSource = 'file' | 'record' | `provider:${string}` | 'manual';

// FieldDecision is the per-field decision marker on a ResolvedField. `standing` is true for
// an explicit stored decision and false for the implicit file default (undecided).
// `manual_value` is present only when source === 'manual'.
export interface FieldDecision {
	source: DecisionSource;
	standing: boolean;
	manual_value?: string;
}

// FieldCandidate is one selectable source value for a replace field — the file value or a
// matched provider's value — feeding the SourceSelect `Adopt` segments and the candidates
// line. A provider candidate with an empty `value` yields no `Adopt` segment (you cannot
// adopt an empty value).
export interface FieldCandidate {
	source: DecisionSource; // 'file' | 'provider:<name>'
	provider?: string; // provider name when source is 'provider:<name>'
	value: string;
}

// DecisionRequest is the body of PUT …/decision (F36, ADR-051 §7). `manual_value` is
// required iff source === 'manual'; a `provider:<name>` must be a currently-matched provider.
// `override` bypasses the Video Title composite-key collision check (HOLODEX-270) — set only
// on a resubmit after the owner has already seen and dismissed a collision verdict.
export interface DecisionRequest {
	source: DecisionSource;
	manual_value?: string;
	override?: boolean;
}

// F44 (ADR-062) — the render-mode vocabulary a promotion may set (F39's five modes; no
// new modes). Empty/`text` is inline text.
export type PromotionRender = '' | 'long_text' | 'chips' | 'url' | 'image_url';

// FieldPromotionRequest is the body of PUT /admin/field-promotions/{entity_type}/{field_key}
// (F44). Every field is optional; an omitted presentation column inherits from the lower
// tiers (provider hint → title-case). `order` positions the field within its group.
export interface FieldPromotionRequest {
	label?: string;
	render?: PromotionRender;
	group?: 'primary' | 'attributes' | 'extended';
	order?: number;
}

// The entity types a promotion may target — a promotion is global per (entity_type,
// field_key), so the affordance passes the row's type, not the entity id being viewed.
export type PromotionEntityType = 'video' | 'person' | 'studio';

// FieldPromotionView is one stored promotion as returned by GET /admin/field-promotions/
// {entity_type} — the Edit editor loads it to pre-fill group/order (not carried on the
// resolved field) so a Save doesn't reset them.
export interface FieldPromotionView {
	field_key: string;
	label?: string;
	render?: PromotionRender;
	group?: 'primary' | 'attributes' | 'extended';
	order?: number;
}

// FieldClaim is one stored claim as returned by GET /admin/field-claims/{entity_type}
// (F49, ADR-074): the provider key `provider:field_key` is a candidate source of
// `canonical` rather than a display-only row of its own. Keyed per (provider, key) — one
// provider's spelling can be claimed while another provider's identical key is not.
export interface FieldClaim {
	provider: string;
	field_key: string;
	canonical: string;
}

// FieldTarget is one field a claim may target, from GET /admin/field-targets/{entity_type}.
// The entity type's EFFECTIVE (post-promotion) field set — the SPA cannot derive it from
// the page, because undecided empty fields never render and are exactly the ones an owner
// needs to attach to. `merge` decides the editor's outcome sentence: a merge field folds
// the attached values in immediately, a replace field keeps its winner and takes the new
// value as a lowest-precedence candidate.
export interface FieldTarget {
	canonical: string;
	label: string;
	merge: boolean;
}

// CurationAction is a value-level owner decision (F30, ADR-048).
export type CurationAction = 'add' | 'suppress' | 'nowrite';

// CurationRequest records or clears one value-level decision for a field. override
// bypasses the People composite-key collision gate (HOLODEX-272) on a resubmit after
// the owner has already seen and dismissed a collision verdict for this exact edit —
// harmless (ignored server-side) for any field other than a person-typed one.
export interface CurationRequest {
	field: string;
	value: string;
	action: CurationAction;
	override?: boolean;
}

export interface MediaDetailResponse {
	video: Video;
	metadata: ExtraMetadata[] | null;
	fields: MappedField[] | null;
	resolved?: ResolvedField[] | null; // unified merged view (F27); supersedes fields when present
	enriched?: EnrichedField[] | null;
	// Studio entities linked to this video (F38, ADR-053): the resolved studio value
	// links to its /studios/{id} page; the link target always matches the displayed
	// value because video_studios is derived from that same resolution (RD1).
	studios?: Studio[] | null;
	// Films this video is attached to (F56, design handoff §3a) — non-nil (empty array,
	// never null) when films_enabled is on; empty/omitted when off (server-side gate,
	// same read-suppression as the resolver-source injection at getMedia).
	films?: FilmAttachment[];
	// enrich_queries maps provider name -> the rendered /resolve search query the
	// Enrich picker should default to (F54, ADR-080): each enabled video-capable
	// provider's own precedence chain (operator pattern -> provider-advertised
	// preference -> operator default -> sanitized-title floor) applied server-side to
	// this video's already-resolved fields. A provider absent from the map (e.g.
	// enrichment disabled) falls back to the plain resolved/raw title client-side.
	enrich_queries?: Record<string, string> | null;
	// completeness is the F55.13 per-entity breakdown panel's data, owner-gated
	// like enrich_queries — null for a visitor.
	completeness?: Completeness | null;
}

// WritebackRequest asks the server to embed a batch of resolved field values
// into the media file's tags in a single exiftool pass (F28, ADR-041).
export interface WritebackRequest {
	fields: Array<{
		field: string;
		values: string[];
		source: string;
	}>;
}

// A soft-deleted item in the owner's Trash view (F24, ADR-037). purge_at is null
// when auto-purge is disabled (grace = 0) — it lingers until purged manually.
export interface TrashEntry {
	id: number;
	title: string;
	path: string;
	deleted_at: string;
	purge_at: string | null;
}

export interface FacetValue {
	value: string;
	count: number;
}

// Facet is a filterable mapped field plus its distinct values (F20.4).
export interface Facet {
	canonical: string;
	label: string;
	multi: boolean;
	values: FacetValue[];
}

// MetadataKey is one row of the library-wide key-discovery view (F20.9).
export interface MetadataKey {
	source_key: string;
	count: number;
	samples: string[];
	mapped: boolean;
}

export interface SearchResponse {
	videos: Video[] | null;
	people: Person[] | null;
	tags: Tag[] | null;
	studios: Studio[] | null;
	// films is spliced in client-side (F56) — the backend Search() aggregation has no
	// films branch, so callers fetch it separately via api.listFilms(q) and merge it
	// onto the response themselves. Absent/undefined when films_enabled is off.
	films?: Film[] | null;
}

// "More with …" related-media shelves (ADR-031, QW2/QW3). Either block is null when
// the item has no people / no tags; items is [] when the entity has no other siblings.
export interface RelatedShelf {
	id: number;
	name: string;
	items: Video[];
}

export interface RelatedResponse {
	person: RelatedShelf | null;
	tag: RelatedShelf | null;
}

// System Activity — "under the hood" read-model (F21, ADR-028).
export interface ScanSummary {
	trigger: string;
	finished_at: string;
	duration_ms: number;
	seen: number;
	added: number;
	updated: number;
	removed: number;
	skipped: number;
	errors: number;
}

export interface ScanStatus {
	state: 'idle' | 'running';
	trigger?: string;
	started_at?: string | null;
	last_run: ScanSummary | null;
	next_scheduled_at?: string | null;
}

export interface ThumbnailStats {
	depth: number;
	high: number;
	normal: number;
	in_flight: number;
	workers: number;
}

export interface LibraryCounts {
	videos_active: number;
	videos_inactive: number;
	people: number;
	tags: number;
}

export interface ActivitySystem {
	ready: boolean;
	version: string;
	media_path_present: boolean;
	controls_unauthenticated: boolean;
	uptime_seconds?: number;
}

export interface Activity {
	scan: ScanStatus;
	thumbnails: ThumbnailStats;
	library: LibraryCounts;
	system: ActivitySystem;
}

// JobRun is one row of the 30-day activity history (F21.3).
export interface JobRun {
	id: number;
	kind: string;
	trigger: string;
	status: string;
	started_at: string;
	finished_at: string;
	duration_ms: number;
	seen: number;
	added: number;
	updated: number;
	removed: number;
	skipped: number;
	errors: number;
	error_message?: string;
	detail?: string; // short description for non-scan jobs, e.g. enrich (F22.6b)
	// What this run acted on (ADR-071). Absent for library-wide kinds (scan, the
	// backfills). Not a foreign key — job_runs outlives what it describes, so the
	// id may no longer resolve to a live entity.
	entity_type?: 'video' | 'person' | 'studio';
	entity_id?: number;
	// Writeback snapshot batch (ADR-067) this run belongs to; drives Revert.
	batch_id?: string;
}

// One kind's roll-up in the activity digest (ADR-071). last_status is the status
// of the most recent run, so a kind whose latest pass failed reads as failing
// even if older passes in the window succeeded.
export interface JobKindDigest {
	kind: string;
	runs: number;
	errors: number;
	last_run: string;
	last_status: string;
}

// The activity digest (ADR-071): a per-kind summary plus the window's failed
// runs. Fixed-size — its length tracks the number of job kinds and the (capped)
// failure count, never the number of runs in the window.
export interface JobDigest {
	kinds: JobKindDigest[];
	failures: JobRun[];
}

// Capabilities tells the SPA whether it may act as owner (F21.7).
export interface Capabilities {
	owner: boolean;
	auth_required: boolean;
	// Soft-delete grace window in seconds (F24); 0 = auto-purge disabled.
	delete_grace_period_seconds: number;
	// card_layout is the operator's preferred browse-grid aspect ratio:
	// "wide" (16:9, default) for personal/AMV libraries, "poster" (2:3) for film libraries.
	card_layout: 'wide' | 'poster';
	// person_gallery_max is the per-person 'extra' gallery cap (F25), so the gallery
	// can warn at the limit and offer the owner an explicit over-cap "add anyway".
	person_gallery_max: number;
	// films_enabled gates the Films entity (F56, ADR-085) — routes, nav, video-list
	// hiding, and the resolver-source injection are all suspended when false.
	films_enabled: boolean;
}

// Metadata source plugins — People enrichment (F22, ADR-033).

// EnrichSource is one configured provider (no base_url or secrets — F22.9d). Also the
// shape of the public /providers directory (ADR-059), so both the owner enrich controls
// and visitor provenance badges resolve a provider name to its brand icon the same way.
export interface EnrichSource {
	name: string;
	entity_types: string[];
	// icon_url is the served brand-icon URL (ADR-059), present only when a normalized
	// icon is cached for this provider; absent → the SPA renders a monogram.
	icon_url?: string;
}

// EnrichCandidate is one provider match the owner confirms (F22.5b). Confidence stays
// provider-native/advisory; auto_apply is the server-computed verdict derived from it
// (ADR-066 D1, internal/enrich.StrongMatchThreshold) — the client renders auto_apply
// but never re-derives it from a confidence cutoff of its own.
export interface EnrichCandidate {
	external_id: string;
	namespace: string;
	label: string;
	confidence: number;
	disambiguation?: string;
	auto_apply: boolean;
	// profile_url is an optional, provider-supplied link to its own page for this
	// candidate (F47 P1-1/RD6). Scheme-validated server-side, but the client still
	// gates it through isHttpUrl() before rendering as a link (format.ts's standing
	// convention for any provider-supplied URL — belt and suspenders). Absent when
	// the provider doesn't offer one.
	profile_url?: string;
}

// EnrichedField is a resolved field with provenance (F22.7). Provider is the
// source name ("from <provider>"); an empty Provider would denote a file value.
// display hints the render mode: "image_url" → <img>, "long_text" → block paragraph,
// "url" → link (opens in a new tab), absent/other → inline text.
export interface EnrichedField {
	canonical: string;
	label: string;
	display?: 'text' | 'long_text' | 'image_url' | 'url';
	values: string[];
	provider: string;
	external_id?: string;
	fetched_at?: string;
}

// Enrichment review queue (F47 S2, ADR-066). EnrichEntityKind is the enrichment
// entity spine — person | studio | video — distinct from EntityKind (F43's
// alias/merge/rename spine), which has no `video`.
export type EnrichEntityKind = 'person' | 'studio' | 'video';

// EnrichQueueProviderState is one row's per-provider status (RD9 — never a single
// collapsed flag). 'unreviewed' | 'not_matched' come from the server (GET
// /owner/enrich-queue); 'auto_applied' only ever exists client-side, set once a
// /resolve+/enrich round-trip actually applies a candidate. Whether the owner has
// merely *opened* an unreviewed provider's picker this session is ephemeral UI
// state, not domain state — it stays local to EnrichQueueRow, not here.
export interface EnrichQueueProviderState {
	provider: string;
	state: 'unreviewed' | 'not_matched' | 'auto_applied';
}

// EnrichQueueRow is one review-queue entity: still missing at least one supporting
// provider's data. `providers` lists only outstanding providers — a linked one is
// simply absent (mirrors internal/repo/enrich_queue.go's EnrichQueueRow).
export interface EnrichQueueRow {
	entity_type: EnrichEntityKind;
	entity_id: number;
	name: string;
	providers: EnrichQueueProviderState[];
}

// RefreshAllResult is one provider's outcome from POST .../enrich/refresh-all (RD8/P1-2):
// a linked provider refreshes directly; an unlinked one resolves and either auto-applies a
// single strong match or comes back needs_review — never silently dropped.
export interface RefreshAllResult {
	provider: string;
	status: 'refreshed' | 'auto_applied' | 'needs_review' | 'no_candidates';
	enriched?: EnrichedField[];
}

// Per-item metadata refresh outcome (F31, ADR-047). One entry per attempted
// source (file first, then each linked provider). sources_disagree is reserved
// (populated by the future batch op, F31.11); single-item it is false.
export interface RefreshSourceResult {
	source: string; // "file" or the provider name
	ok: boolean;
	changed: boolean;
	error?: string;
}

export interface RefreshReport {
	video_id: number;
	sources: RefreshSourceResult[];
	changed: boolean;
	sources_disagree: boolean;
}

// Filename metadata extraction (F48, ADR-067).

// Result of the on-demand/batch extraction triggers (F48.5a/b) — mirrors
// internal/extract.Result. matched=false means no configured pattern matched
// the filename; fields is then empty.
export interface ExtractionResult {
	matched: boolean;
	fields: Array<{ field: string; outcome: 'noop' | 'auto_applied' | 'logged_only' | 'queued' }>;
}

// ExtractionQueueRow is one pending metadata_extraction_review row,
// video-joined — served by GET /owner/extraction-queue (F48.6). Mirrors
// internal/repo/extraction_review.go's ExtractionQueueRow. suggested_entity_*
// is present only for People/Studio fields carrying a Jaro-Winkler advisory
// match (F48.3d) — never a value the owner has already accepted.
export interface ExtractionQueueRow {
	id: number;
	video_id: number;
	video_title: string;
	file_path: string;
	field_key: string;
	filename_value: string;
	tag_value: string;
	confidence: number;
	suggested_entity_id?: number;
	suggested_entity_name?: string;
	// Per-value entity candidates for People/Studio fields (HOLODEX-196 #1):
	// the filename value split into individual names, each resolved against the
	// identity spine. People yields one per person; Studio yields one. Absent
	// for non-entity fields (title, release_date), which stay scalar.
	candidates?: ExtractionCandidate[];
}

// One parsed name from an entity field's filename value, resolved against the
// identity spine — mirrors internal/repo.ExtractionCandidate. entity_id is
// present when the name already exists (entity_name is then its canonical
// spelling, which may differ in case); absent means the name will be created.
export interface ExtractionCandidate {
	name: string;
	entity_id?: number;
	entity_name?: string;
}

// The owner's choice when resolving one ExtractionQueueRow field (F48.6c):
// keep the filename-derived value, keep the file's existing tag value, or
// supply one manually (freeform edit, or an entity picked from a search).
export type ExtractionResolveAction = 'filename' | 'tag' | 'manual';

// One staged-and-not-yet-written change shown in ExtractionPreviewDialog
// (F48.7a) — a queue row's Stage / "Stage edit" action stages one of these;
// "Keep tag"/"Dismiss" never do (they touch no file).
export interface ExtractionPreviewItem {
	reviewId: number;
	videoTitle: string;
	fieldLabel: string;
	oldValue: string;
	newValue: string;
	action: ExtractionResolveAction;
}

export interface PersonDetailResponse {
	person: Person;
	items: Video[];
	total: number;
	// F37: unified resolved view (same shape as media detail's resolved[], baseline `record`;
	// `in_sync` is always absent — persons have no file). Supersedes the retired enriched[].
	resolved?: ResolvedField[] | null;
	images?: PersonImageSet; // F25: per-role presence + version + ordered gallery
	// completeness is the F55.13 per-entity breakdown panel's data, owner-gated
	// like getMedia's enrich_queries — null for a visitor.
	completeness?: Completeness | null;
	// external_links is the HOLODEX-266/ADR-083 provider-link badge projection — one
	// entry per stored person_external_ids row (0..N), read-only, visitor-visible.
	external_links?: ExternalLink[] | null;
}

// StudioDetailResponse is GET /studios/{id} (F38, ADR-053): the studio, its videos,
// and resolved[] in the record vocabulary (in_sync always absent — studios have no
// file). Details render only when a field beyond `name` has a value or a decision.
export interface StudioDetailResponse {
	studio: Studio;
	items: Video[];
	total: number;
	resolved?: ResolvedField[] | null;
	// completeness is the F55.13 per-entity breakdown panel's data, owner-gated
	// like getMedia's enrich_queries — null for a visitor.
	completeness?: Completeness | null;
	// external_links is the HOLODEX-266/ADR-083 provider-link badge projection — one
	// entry per stored studio_external_ids row (0..N), read-only, visitor-visible.
	external_links?: ExternalLink[] | null;
}

// Film is a first-class entity (F56, ADR-085): unlike Studio/Tag, its video membership
// (film_videos) is an owner ASSERTION, never derived from resolved fields — a film may
// be created and hold zero videos indefinitely. Name+year is its identity key (year
// collisions across different releases are the common case), not a bare unique name.
export interface Film {
	id: number;
	name: string;
	year?: number;
	video_count?: number;
	// Self-hosted image roles (F56/HOLODEX-280, ADR-086): poster (detail page header,
	// search results), thumb (no consumer yet). Owner-editable (upload/replace/
	// remove); present only when that role's slot is filled. Always populated on
	// both list and detail reads. Mirrors Studio's icon/logo/poster_url fields.
	poster_url?: string;
	thumb_url?: string;
}

// FilmImageRole is the enum of editable film image slots (F56/HOLODEX-280, ADR-086).
export type FilmImageRole = "poster" | "thumb";

// FilmVideo is one scene/full-film row on a film's detail page (F56): the video plus
// its film_videos attachment attributes. scene_number is null for an unnumbered scene
// (legal, non-colliding); is_full_film splits the detail page's two regions.
export interface FilmVideo {
	video: Video;
	scene_number: number | null;
	is_full_film: boolean;
}

// FilmAttachment is one film a video is linked to — the "Also in: X" badge on the
// film→video candidates picker, and the resolver-source injection's provenance.
export interface FilmAttachment {
	film_id: number;
	film_name: string;
	is_full_film: boolean;
	// null for an unnumbered scene or a full-film attachment (which carries no scene
	// number) — the media detail page's Films section badge shows "#N" or "Full film".
	scene_number: number | null;
}

// FilmDetailResponse is GET /films/{id} (F56): the film, its resolved[] fields (record
// vocabulary), the two detail-page regions (scenes vs. full_films), and the read-only
// inherited cast/tags/studios (set union over its videos, no relink/prune semantics).
export interface FilmDetailResponse {
	film: Film;
	resolved?: ResolvedField[] | null;
	scenes: FilmVideo[];
	full_films: FilmVideo[];
	cast: Person[];
	tags: Tag[];
	studios: Studio[];
}

// One video's outcome from POST /films/{id}/studio/cascade's best-effort per-video
// decide-then-enqueue (F57, HOLODEX-285, ADR-087 D2) -- a collision or error excludes
// only that video, never the whole batch.
export interface FilmStudioCascadeResult {
	video_id: number;
	status: 'enqueued' | 'collision' | 'error';
	conflict?: VideoCollisionRef;
	error?: string;
}

// FilmSceneCollision names the video already occupying a requested scene number
// (409 payload from attach/bulk-attach) — no silent swap, no auto-bump renumbering.
export interface FilmSceneCollision {
	video_id: number;
	video_title: string;
}

// FilmVideoCandidate is one row in the film→video attach picker's result list
// (design handoff §4): the video plus, when it's already linked to a film other than
// the one being edited, which film(s) — so the picker can show an "Also in: X" badge.
export interface FilmVideoCandidate {
	video: Video;
	already_attached: FilmAttachment[];
}

// ExternalLink is one badge-ready outbound link (HOLODEX-266, ADR-083 D2/D3), mirroring
// the Go api.ExternalLink struct: `provider` is the id's namespace (e.g. "imdb"), `label`
// its display text (e.g. "IMDb"), and `url` the server-built outbound link — absent when
// no provider currently advertises a link_templates entry for this (namespace, entity
// kind), the ADR-083 D2 degraded "known to, but nowhere to click through to" state.
export interface ExternalLink {
	provider: string;
	label: string;
	url?: string;
}

export type Resolution = 'All' | 'SD' | 'HD' | 'FHD' | '4K';

// Sort keys accepted by GET /media?sort= (F12.1). Mirrors repo.VideoFilter.orderBy.
// "random" is a seeded shuffle paired with a ?seed= param (ADR-045). completeness_asc/
// desc (F55.5) are owner-only — the server 401s a non-owner request using them, so the
// frontend must only ever offer/send them when isOwner is true.
export type SortOrder =
	| 'added_desc'
	| 'added_asc'
	| 'title_asc'
	| 'title_desc'
	| 'duration_desc'
	| 'duration_asc'
	| 'resolution_desc'
	| 'resolution_asc'
	| 'random'
	| 'completeness_asc'
	| 'completeness_desc';

// Sort options for the unpaged People/Tags indexes — the single source of truth for
// both pages. 'name'/'count' map to the server toggle; 'random' is a client-side
// seeded shuffle of the name-ordered list (ADR-045 §3).
export const PEOPLE_TAG_SORTS = ['name', 'count', 'random'] as const;
export type PeopleTagSort = (typeof PEOPLE_TAG_SORTS)[number];

// CompletenessFacet is one scored facet's tier/status on the per-entity
// breakdown panel (F55.13-15) — mirrors internal/resolver.FacetScore.
// not_applicable is still listed (muted status), but excluded from the
// parent Completeness's score/actionability. provider is present only when
// actionable is true.
export interface CompletenessFacet {
	canonical: string;
	label: string;
	criticality: string;
	tier: 'missing' | 'provider' | 'curated';
	not_applicable?: boolean;
	actionable?: boolean;
	provider?: string;
}

// Completeness is the F55 completeness score plus the separate actionability
// signal for one entity — mirrors internal/resolver.Completeness. Present
// only on an owner-authorized detail response (video/person/studio); null for
// a visitor, mirroring enrich_queries' access-control shape.
export interface Completeness {
	score: number;
	// undefined when there are no missing scored facets — the ratio is
	// undefined, not zero.
	actionability?: number;
	facets: CompletenessFacet[];
}

// FacetSummary is one row of GET /completeness/facets (F55.6, ADR-081 D4) —
// mirrors internal/api.FacetSummary. Feeds the Missing-facet filter chip's
// option list: canonical (the value sent as ?missing_facet=), a display label,
// and how many entities of this type are currently missing it.
export interface FacetSummary {
	canonical: string;
	label: string;
	criticality: string;
	missing_count: number;
}

// CompletenessQueueRow is one (entity, missing facet) pair in the facet-first
// remediation queue (F55.7, GET /owner/completeness-queue) — mirrors
// internal/api.QueueRow. Exactly one of thumbnail_url/headshot_version/icon_url
// is set, matching entity_type. provider is present only on candidate-ready
// rows (F55.8 DD3).
export interface CompletenessQueueRow {
	entity_type: EnrichEntityKind;
	entity_id: number;
	name: string;
	thumbnail_url?: string;
	headshot_version?: number;
	icon_url?: string;
	provider?: string;
}

// CompletenessFacetGroup is one missing-facet group, pre-split into
// candidate-ready and needs-research rows — mirrors internal/api.FacetGroup.
export interface CompletenessFacetGroup {
	canonical: string;
	label: string;
	criticality: string;
	candidate_ready: CompletenessQueueRow[];
	needs_research: CompletenessQueueRow[];
}

export interface MediaFilters {
	q?: string;
	person?: number[];
	tag?: number[];
	// Studio-entity filter (F38, HOLODEX-120): repeatable ?studio_id, filtering by the
	// derived video_studios link. Distinct from the legacy mapped `studio` string filter
	// (still under `mapped`, kept for REST/MCP back-compat but no longer surfaced in the UI).
	studio_id?: number[];
	// Category filter (HOLODEX-240, ADR-078): repeatable ?category_id, expanded
	// server-side to the category's member tag ids and ORed into the same TagIDs
	// matching the existing `tag` filter uses (no new client-side expansion).
	category?: number[];
	duration_min?: number; // minutes
	duration_max?: number; // minutes
	resolution?: Resolution;
	year_min?: number;
	year_max?: number;
	sort?: SortOrder;
	// seed parameterizes the "random" sort's deterministic shuffle (ADR-045); sent
	// with the API request (paging param) but not the shareable URL. Ignored unless
	// sort==='random'.
	seed?: number;
	mapped?: Record<string, string>; // configurable mapped-field filters (F20.5)
	// Missing-facet filter (F55.6): canonical facet keys the video must be missing
	// (AND semantics across multiple). Owner-only, like the completeness sorts above.
	missing_facet?: string[];
	limit?: number;
	offset?: number;
}
