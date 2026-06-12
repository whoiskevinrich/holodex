import { describe, it, expect } from 'vitest';
import { formatYear, formatDuration, resolutionBucket } from './format';

describe('formatYear', () => {
	// Regression: recorded_at is a UTC instant. Reading the year in local time made
	// a midnight-UTC date (e.g. a Jan-1 release) roll back a year for viewers west
	// of UTC. getUTCFullYear keeps it stable regardless of the runner's timezone.
	it('reads a midnight-UTC date as its UTC year', () => {
		expect(formatYear('2021-01-01T00:00:00Z')).toBe('2021');
		expect(formatYear('2020-01-01T00:00:00Z')).toBe('2020');
	});

	it('returns empty for missing or unparseable input', () => {
		expect(formatYear(null)).toBe('');
		expect(formatYear(undefined)).toBe('');
		expect(formatYear('')).toBe('');
		expect(formatYear('not-a-date')).toBe('');
	});
});

describe('formatDuration', () => {
	it('renders M:SS under an hour and H:MM:SS at or over an hour', () => {
		expect(formatDuration(2899)).toBe('48:19');
		expect(formatDuration(7092)).toBe('1:58:12');
		expect(formatDuration(0)).toBe('0:00');
	});
});

describe('resolutionBucket', () => {
	it('classifies by width with the ADR-012 boundaries', () => {
		expect(resolutionBucket(854)).toBe('SD');
		expect(resolutionBucket(1280)).toBe('HD');
		expect(resolutionBucket(1920)).toBe('FHD');
		expect(resolutionBucket(3840)).toBe('4K');
	});
});
