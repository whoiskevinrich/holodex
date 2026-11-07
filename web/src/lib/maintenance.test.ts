import { describe, expect, it } from 'vitest';
import {
	TRIM,
	clockTime,
	editionOutcomeCopy,
	failureCopy,
	fileName,
	heldVerb,
	nameRefusalCopy,
	pillLabel,
	refusalCopy,
	shortDate,
	skippedCopy
} from './maintenance';

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

describe('trim copy (F88)', () => {
	const none = { need_bytes: 0, free_bytes: 0 };
	it('labels the pill and the held notice', () => {
		expect(pillLabel('trim')).toBe('trimming');
		expect(pillLabel('trim', false)).toBe('queued');
		expect(heldVerb('trim')).toBe('Trimmed');
		expect(heldVerb('hflip')).toBe('Flipped');
	});
	it('says trim, not flip, in its refusals', () => {
		for (const reason of ['dolby_vision', 'no_video', 'unsupported_container', 'multiple_video_tracks', 'flac_in_matroska']) {
			const copy = refusalCopy({ reason, ...none }, TRIM);
			expect(copy, reason).toMatch(/trim/);
			expect(copy, reason).not.toMatch(/flip/);
		}
		expect(refusalCopy({ reason: 'dolby_vision', ...none })).toMatch(/flipped/);
	});
	it("refuses the edition's name under the file name", () => {
		expect(nameRefusalCopy({ reason: 'edition_name_empty' })).toBe('Enter a name for this edition.');
		expect(nameRefusalCopy({ reason: 'edition_same', edition: 'Trimmed' })).toBe('This video is already the Trimmed edition. Choose another name.');
		expect(nameRefusalCopy({ reason: 'target_exists', target: 'a {edition-T}.mkv' })).toBe('A file named a {edition-T}.mkv already exists. Choose another name.');
		expect(nameRefusalCopy({ reason: 'no_space' })).toBe('');
	});
	it('says what the screen check skipped', () => {
		expect(skippedCopy([])).toBe('');
		expect(skippedCopy([{ end: 'last', reason: 'still' }])).toBe(
			"The last frame is a still image, so it couldn't be compared with your screen. The cut itself was checked."
		);
		expect(skippedCopy([{ end: 'first', reason: 'black' }])).toMatch(/first frame is a black frame/);
		expect(skippedCopy([{ end: 'first', reason: 'slow_fade' }])).toMatch(/slow fade/);
		expect(skippedCopy([{ end: 'first', reason: 'black' }, { end: 'last', reason: 'still' }])).toBe(
			"Neither frame could be compared with your screen, because they barely differ from their neighbours. The cut itself was checked."
		);
	});
	it("words an edition's outcome", () => {
		expect(editionOutcomeCopy([])).toBe("Both frames were checked. The original wasn't changed.");
		expect(editionOutcomeCopy([{ end: 'last', reason: 'still' }])).toMatch(/still image.*The original wasn't changed\.$/);
	});
	it('names the check, the end and the offset in a failure', () => {
		expect(failureCopy('trim', 'verification failed (cut_check): last:2')).toBe(
			"Couldn't trim: the cut stayed 2 frames from your last frame after a correction. Your file wasn't changed."
		);
		expect(failureCopy('trim', 'verification failed (cut_check): first:1')).toMatch(/stayed 1 frame from your first/);
		expect(failureCopy('trim', 'verification failed (screen_check): first:3')).toBe(
			"Couldn't trim: the file and your screen disagreed by 3 frames about your first frame after a correction. Your file wasn't changed."
		);
		expect(failureCopy('trim', 'verification failed (screen_check): last:no_match')).toBe(
			"Couldn't trim: your last frame on screen didn't match the file at that point. Your file wasn't changed."
		);
		expect(failureCopy('trim', 'verification failed (chapters): chapter 1 moved')).toMatch(/what was expected/);
		expect(failureCopy('trim', 'maintenance refused: target_exists (a.mkv)')).toBe(
			"Couldn't trim. A file named a.mkv already exists. Choose another name. Your file wasn't changed."
		);
		expect(failureCopy('trim', 'maintenance refused: flac_in_matroska')).toMatch(/^Couldn't trim\. .*FLAC/);
		expect(failureCopy('trim', 're-encode: exit 1 /secret/path')).not.toMatch(/secret/);
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
