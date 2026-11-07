// Video file maintenance (F77, docs/design/video-file-maintenance-handoff.md): the owner-only
// Maintenance section's state, labels and copy. No I/O and no Svelte, so it unit-tests in
// isolation like the other pure modules here.

import { formatBytes } from './format';

export type MaintenanceOp = 'hflip' | 'trim' | 'restore' | 'discard';

// One frame the screen check couldn't run on (F88).
export interface SkippedCheck {
	end: 'first' | 'last';
	reason: 'still' | 'black' | 'slow_fade';
}

// The last trim's outcome, until dismissed.
export interface TrimOutcome {
	mode: 'replace' | 'edition';
	edition_name?: string;
	edition_video_id?: number;
	skipped: SkippedCheck[];
	created_at: string;
}

// The media payload's owner-only `maintenance` block (internal/api/maintenance.go).
export interface MaintenanceState {
	pending: boolean;
	// Claimed by the worker; pending and not running means queued.
	running?: boolean;
	started_at?: string;
	failed: boolean;
	error?: string;
	op?: MaintenanceOp;
	held: { op?: string; created_at: string; expires_at: string } | null;
	trim?: TrimOutcome;
	// An edition's source: its first history entry.
	trimmed_from?: { video_id: number; title: string };
}

export const MAINTENANCE_IDLE: MaintenanceState = { pending: false, failed: false, held: null };

// The confirm dialog's preflight answer.
export interface MaintenancePreflight {
	ok: boolean;
	reason?: string;
	detail?: string;
	need_bytes: number;
	free_bytes: number;
	expires_at: string;
	// Trim only: the edition's file name and the name as it will be written.
	target?: string;
	edition?: string;
}

// The running-state pill (handoff §2 state 2): "queued" until the worker claims the job.
export function pillLabel(op?: MaintenanceOp, running = true): string {
	if (!running) return 'queued';
	switch (op) {
		case 'restore':
			return 'restoring';
		case 'discard':
			return 'removing original';
		case 'trim':
			return 'trimming';
		default:
			return 'flipping';
	}
}

// refusalCopy says why a flip can't run and what to do about it (handoff §3). One sentence
// pattern per reason: what happened, then what to do. Never a raw server string.
export function refusalCopy(
	pf: Pick<MaintenancePreflight, 'reason' | 'detail' | 'need_bytes' | 'free_bytes'>,
	action: { verb: string; done: string } = FLIP
): string {
	switch (pf.reason) {
		case 'read_only':
			return 'The media library is mounted read-only. Remount it read-write, then try again.';
		case 'no_space':
			return `Not enough free space. This needs ${formatBytes(pf.need_bytes)}; ${formatBytes(pf.free_bytes)} is available.`;
		case 'unsupported_codec':
		case 'encoder_missing':
			return `There's no encoder for this file's video codec${pf.detail ? ` (${pf.detail})` : ''}, so it can't be re-encoded.`;
		case 'dolby_vision':
			return `This file's Dolby Vision picture can't survive a re-encode, so it can't be ${action.done}.`;
		case 'no_video':
			return `This file has no video stream to ${action.verb}.`;
		case 'file_missing':
			return "The file isn't on disk. Rescan the library, then try again.";
		case 'tool_missing':
			return `A media tool this file needs isn't installed${pf.detail ? ` (${pf.detail})` : ''}.`;
		case 'unsupported_container':
			return `Only .mp4 and .mkv files can be ${action.done}.`;
		case 'in_flight':
			return 'This video already has maintenance queued. Wait for it to finish.';
		case 'multiple_video_tracks':
			return "This file has a second video track, which can't be cut on the frames you choose, so it can't be trimmed.";
		case 'flac_in_matroska':
			return "This file's FLAC audio can't be cut yet, so it can't be trimmed.";
		default:
			return `This file can't be ${action.done} right now.`;
	}
}

const FLIP = { verb: 'flip', done: 'flipped' };
export const TRIM = { verb: 'trim', done: 'trimmed' };

// The edition name's own refusals, shown under the file name rather than replacing the choice.
// Empty when the reason isn't about the name.
export function nameRefusalCopy(pf: Pick<MaintenancePreflight, 'reason' | 'target' | 'edition'>): string {
	switch (pf.reason) {
		case 'edition_name_empty':
			return 'Enter a name for this edition.';
		case 'edition_same':
			return `This video is already the ${pf.edition} edition. Choose another name.`;
		case 'target_exists':
			return pf.target ? `A file named ${pf.target} already exists. Choose another name.` : 'A file with that name already exists. Choose another name.';
	}
	return '';
}

const framesOff = (n: string) => (n === '1' ? '1 frame' : `${n} frames`);

// A trim's own failures: the two frame checks, whose detail is "<first|last>:<frames>". Empty when
// the error isn't one of them.
function frameCheckCopy(error: string): string {
	const m = /verification failed \((cut_check|screen_check)\): (first|last):(\w+)/.exec(error);
	if (!m) return '';
	const [, check, end, n] = m;
	if (n === 'no_match') return `Couldn't trim: your ${end} frame on screen didn't match the file at that point. Your file wasn't changed.`;
	return check === 'cut_check'
		? `Couldn't trim: the cut stayed ${framesOff(n)} from your ${end} frame after a correction. Your file wasn't changed.`
		: `Couldn't trim: the file and your screen disagreed by ${framesOff(n)} about your ${end} frame after a correction. Your file wasn't changed.`;
}

const skipWords: Record<SkippedCheck['reason'], string> = {
	still: 'still image',
	black: 'black frame',
	slow_fade: 'slow fade'
};

// skippedCopy says which frames the screen check couldn't compare, and that the cut was still
// checked. Empty when it ran on both.
export function skippedCopy(skipped: SkippedCheck[] = []): string {
	if (skipped.length === 0) return '';
	if (skipped.length > 1) {
		return "Neither frame could be compared with your screen, because they barely differ from their neighbours. The cut itself was checked.";
	}
	const [s] = skipped;
	return `The ${s.end} frame is a ${skipWords[s.reason]}, so it couldn't be compared with your screen. The cut itself was checked.`;
}

// The line under "Saved as the {name} edition."
export function editionOutcomeCopy(skipped: SkippedCheck[] = []): string {
	const why = skippedCopy(skipped);
	return why ? `${why} The original wasn't changed.` : "Both frames were checked. The original wasn't changed.";
}

// The held notice's first sentence names the operation that set the original aside.
export function heldVerb(op?: string): string {
	return op === 'trim' ? 'Trimmed' : 'Flipped';
}

// Verifier rules (internal/maintenance/verify.go) in plain words.
const ruleCopy: Record<string, string> = {
	streams: 'a stream was missing or out of place in the new file',
	copied: 'a stream that should have been copied untouched changed',
	tags: "the new file's tags didn't match the original",
	chapters: "the new file's chapters didn't match the original",
	picture: "the new picture's shape didn't match the original",
	duration: "the new file's length didn't match the original"
};

// failureCopy turns a failed job's error into the failure line (handoff §2 state 6): the
// operation, a plain reason (never raw stderr — the full message is in job history), and
// the reassurance that matters most.
export function failureCopy(op: MaintenanceOp | undefined, error = ''): string {
	const verb = op === 'restore' ? 'restore the original' : op === 'discard' ? 'remove the original' : op === 'trim' ? 'trim' : 'flip';
	const refused = /maintenance refused: (\w+)(?: \((.*)\))?/.exec(error);
	if (refused) {
		// A refusal is already a whole sentence, with its own remedy.
		const pf = { reason: refused[1], need_bytes: 0, free_bytes: 0 };
		const why = nameRefusalCopy({ ...pf, target: refused[2] }) || refusalCopy(pf, op === 'trim' ? TRIM : FLIP);
		return `Couldn't ${verb}. ${why} Your file wasn't changed.`;
	}
	if (op === 'trim') {
		const frames = frameCheckCopy(error);
		if (frames) return frames;
	}
	let reason = 'something went wrong (see job history)';
	const rule = /verification failed \((\w+)\)/.exec(error);
	if (op === 'trim' && rule?.[1] === 'chapters') reason = "the new file's chapters didn't match what was expected";
	else if (op === 'trim' && rule?.[1] === 'duration') reason = "the new file's length didn't match what you chose";
	else if (rule && ruleCopy[rule[1]]) reason = ruleCopy[rule[1]];
	else if (error.includes('symlink')) reason = 'a working file beside it is a link, so nothing was touched';
	else if (/re-encode:|rebuild:/.test(error)) reason = 'the encoder stopped';
	return `Couldn't ${verb}: ${reason}. Your file wasn't changed.`;
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

// shortDate renders "17 Oct", adding the year only when it isn't the current one (handoff §3).
export function shortDate(iso: string, now: Date = new Date()): string {
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return '';
	const base = `${d.getDate()} ${MONTHS[d.getMonth()]}`;
	return d.getFullYear() === now.getFullYear() ? base : `${base} ${d.getFullYear()}`;
}

// clockTime renders "14:02" (24-hour, local), the running line's "Started 14:02".
export function clockTime(iso?: string): string {
	if (!iso) return '';
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return '';
	return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

// fileName is the last path segment, for dialog copy ("Example (1999).mkv").
export function fileName(path?: string): string {
	return path?.split(/[\\/]/).pop() ?? '';
}
