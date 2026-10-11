// Video file maintenance (F77, docs/design/video-file-maintenance-handoff.md): the owner-only
// Maintenance section's state, labels and copy. No I/O and no Svelte, so it unit-tests in
// isolation like the other pure modules here.

import { formatBytes } from './format';

export type MaintenanceOp = 'hflip' | 'restore' | 'discard';

// The media payload's owner-only `maintenance` block (internal/api/maintenance.go).
export interface MaintenanceState {
	pending: boolean;
	// Claimed by the worker; pending and not running means queued.
	running?: boolean;
	started_at?: string;
	failed: boolean;
	error?: string;
	op?: MaintenanceOp;
	held: { created_at: string; expires_at: string } | null;
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
}

// The running-state pill (handoff §2 state 2): "queued" until the worker claims the job.
export function pillLabel(op?: MaintenanceOp, running = true): string {
	if (!running) return 'queued';
	switch (op) {
		case 'restore':
			return 'restoring';
		case 'discard':
			return 'removing original';
		default:
			return 'flipping';
	}
}

// refusalCopy says why a flip can't run and what to do about it (handoff §3). One sentence
// pattern per reason: what happened, then what to do. Never a raw server string.
export function refusalCopy(pf: Pick<MaintenancePreflight, 'reason' | 'detail' | 'need_bytes' | 'free_bytes'>): string {
	switch (pf.reason) {
		case 'read_only':
			return 'The media library is mounted read-only. Remount it read-write, then try again.';
		case 'no_space':
			return `Not enough free space. This needs ${formatBytes(pf.need_bytes)}; ${formatBytes(pf.free_bytes)} is available.`;
		case 'unsupported_codec':
		case 'encoder_missing':
			return `There's no encoder for this file's video codec${pf.detail ? ` (${pf.detail})` : ''}, so it can't be re-encoded.`;
		case 'dolby_vision':
			return "This file's Dolby Vision picture can't survive a re-encode, so it can't be flipped.";
		case 'no_video':
			return 'This file has no video stream to flip.';
		case 'file_missing':
			return "The file isn't on disk. Rescan the library, then try again.";
		case 'tool_missing':
			return `A media tool this file needs isn't installed${pf.detail ? ` (${pf.detail})` : ''}.`;
		case 'unsupported_container':
			return 'Only .mp4 and .mkv files can be flipped.';
		case 'in_flight':
			return 'This video already has maintenance queued. Wait for it to finish.';
		default:
			return "This file can't be flipped right now.";
	}
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
	const verb = op === 'restore' ? 'restore the original' : op === 'discard' ? 'remove the original' : 'flip';
	const refused = /maintenance refused: (\w+)/.exec(error);
	if (refused) {
		// A refusal is already a whole sentence, with its own remedy.
		const why = refusalCopy({ reason: refused[1], need_bytes: 0, free_bytes: 0 });
		return `Couldn't ${verb}. ${why} Your file wasn't changed.`;
	}
	let reason = 'something went wrong (see job history)';
	const rule = /verification failed \((\w+)\)/.exec(error);
	if (rule && ruleCopy[rule[1]]) reason = ruleCopy[rule[1]];
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
