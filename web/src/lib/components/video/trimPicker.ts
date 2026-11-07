// The trim picker's rules (F88, docs/design/video-trimming-handoff.md). Pure, so they are pinned by
// trimPicker.test.ts without a real decoder. All times are frame start times, in seconds.
import { formatDuration } from '#lib/format.ts';

export type TrimEnd = 'in' | 'out';
export type TrimMode = 'replace' | 'edition';

// What the dialog hands the page to queue.
export interface TrimChoice {
	inTime: number;
	outTime: number;
	mode: TrimMode;
	edition: string;
	captureIn: File;
	captureOut: File;
}

export interface TrimRange {
	inTime: number;
	outTime: number;
	interval: number; // one frame
	duration: number;
}

// How far a handle can go: it stops one frame short of the other, so at least two frames are kept
// and the handles never cross. Each is bounded by the file's first and last frames.
export function handleBounds(end: TrimEnd, r: TrimRange, first: number, last: number): { min: number; max: number } {
	return end === 'in'
		? { min: first, max: Math.max(first, r.outTime - r.interval) }
		: { min: Math.min(last, r.inTime + r.interval), max: last };
}

// Both handles on the file's own ends: there's nothing to trim.
export function cutsNothing(r: Pick<TrimRange, 'inTime' | 'outTime' | 'interval'>, first: number, last: number): boolean {
	const near = (a: number, b: number) => Math.abs(a - b) < r.interval / 2;
	return near(r.inTime, first) && near(r.outTime, last);
}

const span = (seconds: number) => (seconds > 0 && seconds < 1 ? 'less than a second' : formatDuration(seconds));

// What the range keeps: up to the end of the last frame.
const kept = (r: TrimRange) => Math.max(0, Math.min(r.duration, r.outTime + r.interval) - r.inTime);

// "Cuts 0:12 from the start and 1:48 from the end. Keeps 40:10."
export function trimSummary(r: TrimRange, first: number, last: number): string {
	if (cutsNothing(r, first, last)) return 'Move a handle to choose what to cut.';
	const fromStart = Math.abs(r.inTime - first) >= r.interval / 2 ? r.inTime - first : 0;
	const fromEnd = Math.abs(r.outTime - last) >= r.interval / 2 ? r.duration - (r.outTime + r.interval) : 0;
	const cuts = [
		fromStart > 0 ? `${span(fromStart)} from the start` : '',
		fromEnd > 0 ? `${span(fromEnd)} from the end` : ''
	].filter(Boolean);
	return `Cuts ${cuts.join(' and ')}. Keeps ${span(kept(r))}.`;
}

// The review step's sentence: "Keeps 40:10 of 42:10. …".
export function reviewSummary(r: TrimRange): string {
	return `Keeps ${span(kept(r))} of ${formatDuration(r.duration)}. The video is re-encoded. Audio, subtitles and tags are kept, and chapters follow the cut.`;
}

const editionMarker = /\s*\{edition-[^{}]*\}\s*/g;

// The name as the server writes it (internal/maintenance/trim.go, SanitizeEditionName). A preview
// only: the server sanitizes again, and its answer is what lands.
export function sanitizeEditionName(name: string): string {
	return name
		.replace(/[/\\{}:*?"<>|\p{Cc}]/gu, '')
		.trim()
		.replace(/^\.+/, '')
		.trim();
}

// "The Long Road (2004) {edition-Trimmed}.mkv": the original's name with this edition's marker in
// place of any it already carries. Empty when the name is.
export function editionFileName(filename: string, name: string): string {
	const clean = sanitizeEditionName(name);
	if (!clean) return '';
	const dot = filename.lastIndexOf('.');
	const stem = dot > 0 ? filename.slice(0, dot) : filename;
	const ext = dot > 0 ? filename.slice(dot) : '';
	return `${stem.replace(editionMarker, ' ').trim()} {edition-${clean}}${ext}`;
}
