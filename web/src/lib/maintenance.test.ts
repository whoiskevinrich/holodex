import { describe, expect, it } from 'vitest';
import { clockTime, failureCopy, fileName, pillLabel, refusalCopy, shortDate } from './maintenance';

describe('pillLabel', () => {
	it('names the running operation', () => {
		expect(pillLabel('hflip')).toBe('flipping');
		expect(pillLabel('restore')).toBe('restoring');
		expect(pillLabel('discard')).toBe('removing original');
		expect(pillLabel(undefined)).toBe('flipping');
	});
	it('says queued until the worker claims the job', () => {
		expect(pillLabel('hflip', false)).toBe('queued');
		expect(pillLabel('restore', false)).toBe('queued');
		expect(pillLabel('hflip', true)).toBe('flipping');
	});
});

describe('refusalCopy', () => {
	const none = { need_bytes: 0, free_bytes: 0 };
	it('covers every reason the server refuses with', () => {
		for (const reason of [
			'read_only', 'no_space', 'unsupported_codec', 'encoder_missing', 'dolby_vision',
			'no_video', 'file_missing', 'tool_missing', 'unsupported_container', 'in_flight'
		]) {
			const copy = refusalCopy({ reason, ...none });
			expect(copy, reason).not.toBe("This file can't be flipped right now.");
			expect(copy, reason).toMatch(/\.$/);
		}
	});
	it('gives both numbers for a full disk', () => {
		expect(refusalCopy({ reason: 'no_space', need_bytes: 6.2 * 1024 ** 3, free_bytes: 3.1 * 1024 ** 3 })).toBe(
			'Not enough free space. This needs 6.2 GB; 3.1 GB is available.'
		);
	});
	it('names the codec', () => {
		expect(refusalCopy({ reason: 'unsupported_codec', detail: 'vc1', ...none })).toContain('(vc1)');
	});
	it('falls back for an unknown reason', () => {
		expect(refusalCopy({ reason: 'new_reason', ...none })).toBe("This file can't be flipped right now.");
	});
});

describe('failureCopy', () => {
	it('names the broken rule in plain words, never the raw message', () => {
		const raw = 'verification failed (tags): tag "PUBLISHER" missing at /media/library/a.mkv.holodex-new';
		const copy = failureCopy('hflip', raw);
		expect(copy).toBe("Couldn't flip: the new file's tags didn't match the original. Your file wasn't changed.");
		expect(copy).not.toContain('/media');
	});
	it('reads an encoder failure', () => {
		expect(failureCopy('hflip', 're-encode: ffmpeg: exit status 1: Error while opening encoder')).toContain(
			'the encoder stopped'
		);
	});
	it('reads a refusal raised at run time', () => {
		expect(failureCopy('hflip', 'maintenance refused: read_only')).toBe(
			"Couldn't flip. The media library is mounted read-only. Remount it read-write, then try again. Your file wasn't changed."
		);
	});
	it('says which operation failed', () => {
		expect(failureCopy('restore', 'boom')).toMatch(/^Couldn't restore the original:/);
		expect(failureCopy('discard', 'boom')).toMatch(/^Couldn't remove the original:/);
		expect(failureCopy('hflip', 'boom')).toBe(
			"Couldn't flip: something went wrong (see job history). Your file wasn't changed."
		);
	});
});

describe('shortDate', () => {
	const now = new Date(2026, 9, 11);
	it('omits the current year', () => {
		expect(shortDate(new Date(2026, 9, 17, 12).toISOString(), now)).toBe('17 Oct');
	});
	it('adds another year', () => {
		expect(shortDate(new Date(2027, 0, 3, 12).toISOString(), now)).toBe('3 Jan 2027');
	});
	it('is empty for garbage', () => {
		expect(shortDate('not a date', now)).toBe('');
	});
});

describe('fileName', () => {
	it('takes the last segment of either separator', () => {
		expect(fileName('/media/films/Example (1999).mkv')).toBe('Example (1999).mkv');
		expect(fileName('C:\\media\\a.mp4')).toBe('a.mp4');
		expect(fileName(undefined)).toBe('');
	});
});

describe('clockTime', () => {
	it('renders 24-hour local time', () => {
		expect(clockTime(new Date(2026, 9, 11, 14, 2).toISOString())).toBe('14:02');
		expect(clockTime(new Date(2026, 9, 11, 9, 5).toISOString())).toBe('09:05');
	});
	it('is empty without a time', () => {
		expect(clockTime(undefined)).toBe('');
		expect(clockTime('nope')).toBe('');
	});
});
