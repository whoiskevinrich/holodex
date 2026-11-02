// File names (F86, HOLODEX-602): the wire types and the pure wording the rename surfaces share.
// The plan, the template card and the video page's File name row all read from here, so a reason
// is spelled the same wherever it shows (handoff calls 3, 7, 13).

import { shortDate } from './maintenance.ts';

export type RenameMode = 'off' | 'plan' | 'manual' | 'writeback' | 'job';

export type RenameReason =
	| 'will-rename'
	| 'out-of-date'
	| 'already-named'
	| 'no-template'
	| 'wont-read-back'
	| 'clash'
	| 'bad-name'
	| 'locked'
	| 'held';

export type LockSource = 'auto' | 'owner' | 'undo';

export interface RenameChange {
	token: string;
	recorded: string[];
	current: string[];
}

export interface RenamePlanRow {
	video_id: number;
	title: string;
	current: string;
	name?: string;
	template: number | null;
	reason: RenameReason;
	bad_name?: string;
	parsed?: Record<string, string[]>;
	written?: Record<string, string[]>;
	holder_id?: number;
	changes?: RenameChange[];
	lock_source?: LockSource;
	locked_at?: string;
}

export interface RenamePlan {
	mode: RenameMode;
	templates: string[];
	providers: string[];
	total: number;
	counts: Record<RenameReason, number>;
	template_counts: { names: number; wont_read_back: number }[];
	reason: RenameReason | 'all';
	filtered: number;
	rows: RenamePlanRow[];
}

export interface RenameStatus {
	queued: boolean;
	failed: boolean;
	error?: string;
}

export interface RenameHistoryEntry {
	old_name: string;
	new_name: string;
	trigger: string;
	created_at: string;
}

export const MODES: { value: RenameMode; label: string }[] = [
	{ value: 'off', label: 'Off' },
	{ value: 'plan', label: 'Plan' },
	{ value: 'manual', label: 'Manual' },
	{ value: 'writeback', label: 'On writeback' },
	{ value: 'job', label: 'Job' }
];

const MODE_RANK: Record<RenameMode, number> = { off: 0, plan: 1, manual: 2, writeback: 3, job: 4 };

// atLeastManual reports whether a mode may touch files (the spec's P0-7 ladder).
export function atLeastManual(mode: RenameMode): boolean {
	return MODE_RANK[mode] >= MODE_RANK.manual;
}

export function parseMode(v: string | undefined): RenameMode {
	return v && v in MODE_RANK ? (v as RenameMode) : 'off';
}

export const MODE_LINES: Record<RenameMode, string> = {
	off: 'Off: Holodex never renames a file. Choose Plan to see what your files would be called.',
	plan: 'Plan: see what every file would be called. Nothing on disk changes.',
	manual: 'Manual: preview every name, then rename one video, a selection or everything. Nothing renames on its own.',
	writeback:
		'On writeback: when Holodex writes into a file whose name is out of date, it takes its new name too. Includes Manual.',
	job: "Job: a sweep renames every unlocked video whose new name hasn't changed for the settle window. Includes On writeback."
};

// The filter row's order and labels (handoff call 13; the spec's P1-1 list).
export const FILTERS: { value: RenameReason | 'all'; label: string }[] = [
	{ value: 'all', label: 'All' },
	{ value: 'will-rename', label: 'Will rename' },
	{ value: 'out-of-date', label: 'Out of date' },
	{ value: 'already-named', label: 'Already named' },
	{ value: 'no-template', label: 'No template' },
	{ value: 'wont-read-back', label: "Won't read back" },
	{ value: 'clash', label: 'Clash' },
	{ value: 'bad-name', label: 'Bad name' },
	{ value: 'locked', label: 'Locked' },
	{ value: 'held', label: 'Held' }
];

export function parseFilter(v: string | null): RenameReason | 'all' | null {
	return FILTERS.some((f) => f.value === v) ? (v as RenameReason | 'all') : null;
}

// renames reports whether an apply acts on a row's reason.
export function renames(reason: RenameReason): boolean {
	return reason === 'will-rename' || reason === 'out-of-date';
}

// renameableCount is how many videos a filter can rename: the server's totals, never the page.
export function renameableCount(counts: Record<string, number>, filter: RenameReason | 'all'): number {
	if (filter === 'all') return (counts['will-rename'] ?? 0) + (counts['out-of-date'] ?? 0);
	return renames(filter) ? (counts[filter] ?? 0) : 0;
}

const BAD_NAME_CHIPS: Record<string, string> = {
	reserved: 'reserved name',
	trailing: 'trailing dot',
	'too-long': 'too long',
	'invalid-text': 'invalid text',
	'bad-char': 'bad character',
	empty: 'empty name',
	'not-local': 'not local'
};

// A row's chip, and whether it is a refusal (warn border) or a state that is fine (muted border)
// (handoff call 4). Rows that rename carry no chip.
export function reasonChip(row: Pick<RenamePlanRow, 'reason' | 'bad_name'>): { label: string; refusal: boolean } | null {
	switch (row.reason) {
		case 'wont-read-back':
			return { label: "won't read back", refusal: true };
		case 'clash':
			return { label: 'clash', refusal: true };
		case 'bad-name':
			return { label: BAD_NAME_CHIPS[row.bad_name ?? ''] ?? 'bad name', refusal: true };
		case 'locked':
			return { label: 'locked', refusal: false };
		case 'held':
			return { label: 'held', refusal: false };
		case 'no-template':
			return { label: 'no template', refusal: false };
		default:
			return null;
	}
}

// tokenLabel names a template token in a sentence: `title`, `studio`, `provider-1 id`.
export function tokenLabel(token: string): string {
	return token.endsWith('-id') ? `${token.slice(0, -3)} id` : token;
}

function capitalize(s: string): string {
	return s.charAt(0).toUpperCase() + s.slice(1);
}

// changedFields lists the fields an out-of-date name changed on: "title", "title, studio".
export function changedFields(changes: RenameChange[] | undefined): string {
	return (changes ?? []).map((c) => tokenLabel(c.token)).join(', ');
}

// changedSince is the short out-of-date line: "Title changed since Holodex named it."
export function changedSince(changes: RenameChange[] | undefined): string {
	const fields = changedFields(changes);
	return fields ? `${capitalize(fields)} changed since Holodex named it.` : 'Its metadata changed since Holodex named it.';
}

// lockLine says why a video is locked (handoff call 7).
export function lockLine(source: LockSource | undefined, at: string | undefined): string {
	const when = at ? ` on ${shortDate(at)}` : '';
	switch (source) {
		case 'auto':
			return `Locked when you renamed it outside Holodex${when}.`;
		case 'undo':
			return `Locked when you undid a rename${when}.`;
		default:
			return 'Locked by you.';
	}
}

// plainName strips the id marker a clash settled with: "Ada (2023) {hx-412}.mkv" → "Ada (2023).mkv".
export function plainName(name: string): string {
	return name.replace(/ \{hx-\d+\}(?=\.[^.]*$|$)/, '');
}

const joinValues = (v: string[] | undefined) => (v ?? []).join(', ');

// readBack explains a won't-read-back row: which token's value didn't survive, and when the value
// holds the character that ends its token in the template, which character (handoff, plan row 4).
export interface ReadBackLine {
	token: string;
	value: string;
	breaker?: string;
	template?: number;
	readsAs: string;
}

export function readBack(row: RenamePlanRow, templates: string[]): ReadBackLine | null {
	const written = row.written ?? {};
	const parsed = row.parsed ?? {};
	for (const token of Object.keys(written)) {
		const value = joinValues(written[token]);
		const readsAs = joinValues(parsed[token]);
		if (value === readsAs) continue;
		const line: ReadBackLine = { token, value, readsAs };
		const text = row.template != null ? templates[row.template] : undefined;
		if (text) {
			// The literal that ends the token: everything up to the next token or the end.
			const at = text.indexOf(`{${token}}`);
			const rest = at >= 0 ? text.slice(at + token.length + 2) : '';
			const brace = rest.indexOf('{');
			const literal = brace >= 0 ? rest.slice(0, brace) : rest;
			const breaker = [literal, literal.trim()].find((s) => s && value.includes(s));
			if (breaker) {
				line.breaker = breaker;
				line.template = row.template! + 1;
			}
		}
		return line;
	}
	return null;
}

// badNameLine spells out a bad-name refusal (spec P0-4).
export function badNameLine(row: RenamePlanRow): string {
	switch (row.bad_name) {
		case 'too-long':
			return row.name
				? `The name would be ${new TextEncoder().encode(row.name).length} bytes. The limit is 255.`
				: 'The name would be longer than 255 bytes.';
		case 'reserved':
			return 'The name is reserved on Windows.';
		case 'trailing':
			return 'The name would end in a dot or a space, which Windows drops.';
		case 'invalid-text':
			return "The name would hold characters that aren't valid text.";
		case 'bad-char':
			return "The name would hold a character a file name can't.";
		case 'empty':
			return 'The name would be empty.';
		case 'not-local':
			return 'The name would point outside its folder.';
		default:
			return "The name isn't valid on Windows or Linux.";
	}
}

// staleFailure reports a rename the worker refused because the plan moved under it: its action is
// Refresh plan, not Retry (handoff edge case "The plan changes under the owner").
export function staleFailure(error: string | undefined): boolean {
	return !!error && error.includes('changed since the plan');
}

// failureLine is a failed rename's reason, as the owner reads it.
export function failureLine(error: string | undefined): string {
	if (staleFailure(error)) return 'The name changed since you looked.';
	return error ? `Not renamed: ${error}.` : 'Not renamed.';
}

// The draft template list survives a reload and leaving the tab until it is saved or discarded
// (handoff "Folded card": it stays open on reload until saved).
const DRAFT_KEY = 'holodex.rename.templateDraft';

export function loadDraft(): string[] | null {
	try {
		const raw = localStorage.getItem(DRAFT_KEY);
		const v = raw ? JSON.parse(raw) : null;
		return Array.isArray(v) && v.every((s) => typeof s === 'string') ? v : null;
	} catch {
		return null;
	}
}

export function storeDraft(draft: string[] | null): void {
	try {
		if (draft) localStorage.setItem(DRAFT_KEY, JSON.stringify(draft));
		else localStorage.removeItem(DRAFT_KEY);
	} catch {
		// best-effort: a blocked store only loses the draft on reload
	}
}

// sentence turns a server refusal ("add {title} or …: without one, …") into owner prose ("Add
// {title} or …. Without one, ….").
export function sentence(msg: string): string {
	const s = msg
		.split(': ')
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join('. ');
	return s && !/[.!?]$/.test(s) ? `${s}.` : s;
}

export function sameList(a: string[], b: string[]): boolean {
	return a.length === b.length && a.every((s, i) => s === b[i]);
}

export const nf = new Intl.NumberFormat('en-US');
