// The Videos group's rules (F76, HOLODEX-521), held here so `videoPairs.test.ts` can pin
// them without a component harness — the same reason `queue.ts` exists. The row, the
// panel and the page derive their per-pair ids from here; the fact table, the confirm's
// carry-over list and the label validation are pure functions of the API payload.
import { formatBitrate, formatBytes, formatDuration, resolutionBucket } from '$lib/format';
import type { CarryPreview, VideoCompareSide, VideoDuplicatePair, VideoPairSide, VideoWork } from '$lib/types';
import { groupId, QUEUE_ID } from './queue';

export function videoPairKey(p: VideoDuplicatePair): string {
	return `video-${p.a.id}-${p.b.id}`;
}
export const videoDisclosureId = (p: VideoDuplicatePair): string => `dup-disclosure-${videoPairKey(p)}`;
export const videoPanelId = (p: VideoDuplicatePair): string => `dup-panel-${videoPairKey(p)}`;

/** Focus ladder when a video pair is removed: the next row's disclosure, then the
 *  Videos heading, then the queue (which always renders). */
export function videoFocusLandingIds(pair: VideoDuplicatePair, siblings: VideoDuplicatePair[]): string[] {
	const next = siblings[siblings.indexOf(pair) + 1];
	return [...(next ? [videoDisclosureId(next)] : []), groupId('video'), QUEUE_ID];
}

/** The row's per-side summary: "4K · 34:14". */
export function sideSummary(s: Pick<VideoPairSide, 'width' | 'duration_sec'>): string {
	return `${resolutionBucket(s.width)} · ${formatDuration(s.duration_sec)}`;
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/** The compact "Your work" cell: "2 playlists · film link · 3 edits", '' when none. */
export function workSummary(w: VideoWork): string {
	const parts: string[] = [];
	if (w.playlists) parts.push(plural(w.playlists, 'playlist', 'playlists'));
	if (w.films.length) parts.push(w.films.length === 1 ? 'film link' : `${w.films.length} film links`);
	if (w.edits) parts.push(plural(w.edits, 'edit', 'edits'));
	if (w.manual_tags) parts.push(plural(w.manual_tags, 'tag you added', 'tags you added'));
	return parts.join(' · ');
}

/** The confirm's "Moving to the copy you keep:" lines — only what will actually move. */
export function carryLines(c: CarryPreview): string[] {
	const lines: string[] = [];
	if (c.playlists) lines.push(plural(c.playlists, 'playlist place', 'playlist places'));
	for (const f of c.films) {
		lines.push(`Film link — ${f.name}${f.scene_number != null ? `, scene ${f.scene_number}` : ''}`);
	}
	if (c.edits) lines.push(plural(c.edits, 'field edit', 'field edits'));
	if (c.manual_tags) lines.push(plural(c.manual_tags, 'tag you added', 'tags you added'));
	return lines;
}

export interface FactRow {
	key: string;
	label: string;
	a: string;
	b: string;
}

/** The fact table's rows, in the media page File section's order and formats. A row
 *  renders when either side has a value; a side without one is '' (absent-is-absent per
 *  cell), so the two columns never shift against each other. */
export function factRows(a: VideoCompareSide, b: VideoCompareSide): FactRow[] {
	const res = (s: VideoCompareSide) => (s.width ? `${resolutionBucket(s.width)} · ${s.width}×${s.height}` : '');
	const dur = (s: VideoCompareSide) => (s.duration_sec ? formatDuration(s.duration_sec) : '');
	const size = (s: VideoCompareSide) => (s.file_size ? formatBytes(s.file_size) : '');
	const rate = (s: VideoCompareSide) => (s.bitrate_kbps ? formatBitrate(s.bitrate_kbps) : '');
	const rows: FactRow[] = [
		{ key: 'file', label: 'File', a: a.file_name, b: b.file_name },
		{ key: 'folder', label: 'Folder', a: a.folder, b: b.folder },
		{ key: 'resolution', label: 'Resolution', a: res(a), b: res(b) },
		{ key: 'duration', label: 'Duration', a: dur(a), b: dur(b) },
		{ key: 'size', label: 'File size', a: size(a), b: size(b) },
		{ key: 'codec', label: 'Video codec', a: a.video_codec ?? '', b: b.video_codec ?? '' },
		{ key: 'bitrate', label: 'Bitrate', a: rate(a), b: rate(b) },
		{ key: 'container', label: 'Container', a: a.container ?? '', b: b.container ?? '' },
		{ key: 'edition', label: 'Edition', a: a.edition ?? '', b: b.edition ?? '' },
		{ key: 'part', label: 'Part', a: a.part ?? '', b: b.part ?? '' },
		{ key: 'work', label: 'Your work', a: workSummary(a.work), b: workSummary(b.work) }
	];
	return rows.filter((r) => r.a !== '' || r.b !== '');
}

export type LabelField = 'edition' | 'part';

/** The label editor's validation copy (design handoff), or '' when the labels may save. */
export function labelError(field: LabelField, a: string, b: string): string {
	const va = a.trim();
	const vb = b.trim();
	if (field === 'edition') return va || vb ? '' : 'Give at least one file an edition.';
	const isPart = (v: string) => /^\d+$/.test(v) && Number(v) > 0;
	if (!isPart(va) || !isPart(vb)) return 'Give both files a part number.';
	if (Number(va) === Number(vb)) return 'The two files need different part numbers.';
	return '';
}
