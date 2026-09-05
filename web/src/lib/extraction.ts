// Shared extraction-review logic for the two surfaces that render the same queue:
// the owner's Extraction tab (whole library, F48.6a) and the media detail page's
// inline panel (one video, F48.6i). ADR-090 D2 requires both to be one code path —
// a second copy of the staging/label logic is exactly how the two would drift into
// showing different labels or writing different values for the same row.
//
// Pure and DOM-free so it is unit-testable and neither page owns it.

import type { ExtractionPreviewItem, ExtractionQueueRow, ExtractionResolveAction } from '$lib/types';

/** reviewId -> the owner's staged-but-unwritten pick (F48.7a). */
export type StagedPicks = Record<number, { action: ExtractionResolveAction; value: string }>;

// The extraction package's field key ("people", from the {people} filename token —
// internal/extract/pattern.go) differs from the app's canonical field key for the same
// data ("actors": metadata-mappings.yaml.example declares filename:people as one of
// "actors"'s *sources*, not a canonical field of its own). Mirrors
// internal/extract.WritebackField on the backend: without this alias the facet-registry
// lookup misses and falls back to a titleized "People" instead of the app's configured
// label ("Actors"), showing a different name for the same field than every other surface.
export const FIELD_LABEL_ALIASES: Record<string, string> = { people: 'actors' };

/** Fields within a video render People → Studio → Title → Release date → other. */
const FIELD_ORDER: Record<string, number> = { people: 0, studio: 1, title: 2, release_date: 3 };

function fieldRank(key: string): number {
	return FIELD_ORDER[key] ?? 99;
}

/** People/Studio resolve to entities and render as chips; title/release_date are scalars. */
export function isEntityField(key: string): boolean {
	return key === 'people' || key === 'studio';
}

// makeFieldLabel builds the field-key -> label lookup both surfaces use, from the
// shared facet registry. Falls back to a titleized key for anything not registered
// as a facet.
export function makeFieldLabel(labelByField: Record<string, string>): (key: string) => string {
	return (key: string) => {
		const canonical = FIELD_LABEL_ALIASES[key] ?? key;
		return labelByField[canonical] ?? key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
	};
}

// sortRows orders one video's rows for render.
export function sortRows(rows: ExtractionQueueRow[]): ExtractionQueueRow[] {
	return [...rows].sort(
		(a, b) => fieldRank(a.field_key) - fieldRank(b.field_key) || a.field_key.localeCompare(b.field_key)
	);
}

export interface VideoGroup {
	videoId: number;
	videoTitle: string;
	filePath: string;
	rows: ExtractionQueueRow[];
}

// groupByVideo groups a flat queue by video, most-pending-fields first (clears the most
// backlog per click). Owner-tab only — the media page already has exactly one video and
// uses sortRows directly for the within-video ordering the two surfaces share.
export function groupByVideo(rows: ExtractionQueueRow[]): VideoGroup[] {
	const byVideo = new Map<number, ExtractionQueueRow[]>();
	for (const row of rows) {
		const list = byVideo.get(row.video_id);
		if (list) list.push(row);
		else byVideo.set(row.video_id, [row]);
	}
	const out: VideoGroup[] = [...byVideo.entries()].map(([videoId, items]) => {
		const sorted = sortRows(items);
		return { videoId, videoTitle: sorted[0].video_title, filePath: sorted[0].file_path, rows: sorted };
	});
	out.sort((a, b) => b.rows.length - a.rows.length || a.videoTitle.localeCompare(b.videoTitle));
	return out;
}

export function stagePick(
	staged: StagedPicks,
	reviewId: number,
	action: ExtractionResolveAction,
	value: string
): StagedPicks {
	return { ...staged, [reviewId]: { action, value } };
}

export function unstagePick(staged: StagedPicks, reviewId: number): StagedPicks {
	if (!(reviewId in staged)) return staged;
	const next = { ...staged };
	delete next[reviewId];
	return next;
}

// buildPreviewItems builds the old -> new diff rows ExtractionPreviewDialog renders. A
// staged pick whose row has since left the queue (resolved in another tab, dismissed)
// is skipped rather than rendered against a missing row.
//
// Takes a prebuilt index rather than the row array: this runs on every stage/unstage
// click, and the owner tab's array is the whole library — rebuilding the Map per click
// there was a regression over the `rowsById` memo this function replaced.
export function buildPreviewItems(
	staged: StagedPicks,
	byId: Map<number, ExtractionQueueRow>,
	fieldLabel: (key: string) => string
): ExtractionPreviewItem[] {
	const items: ExtractionPreviewItem[] = [];
	for (const [idStr, pick] of Object.entries(staged)) {
		const id = Number(idStr);
		const row = byId.get(id);
		if (!row) continue;
		items.push({
			reviewId: id,
			videoTitle: row.video_title,
			fieldLabel: fieldLabel(row.field_key),
			oldValue: row.tag_value,
			newValue: pick.value,
			action: pick.action
		});
	}
	return items;
}
