import { describe, it, expect } from 'vitest';
import {
	formatYear,
	formatDuration,
	resolutionBucket,
	providerFromWinningSource,
	calculatedFrom
} from './format';

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

describe('providerFromWinningSource', () => {
	it('strips baseline namespaces (incl. the F45 computed guard) and keeps provider names', () => {
		expect(providerFromWinningSource('tmdb:bio')).toBe('tmdb');
		expect(providerFromWinningSource('file:Title')).toBe('');
		expect(providerFromWinningSource('record:name')).toBe('');
		expect(providerFromWinningSource('manual:genres')).toBe('');
		// F45: a computed winning source must never resolve to a phantom "computed" provider.
		expect(providerFromWinningSource('computed:age')).toBe('');
		expect(providerFromWinningSource(undefined)).toBe('');
	});
});

describe('calculatedFrom', () => {
	it('builds the "calculated from …" phrase with a serial-comma join', () => {
		expect(calculatedFrom(['Born'])).toBe('calculated from Born');
		expect(calculatedFrom(['Born', 'Died'])).toBe('calculated from Born and Died');
		expect(calculatedFrom(['A', 'B', 'C'])).toBe('calculated from A, B, and C');
		expect(calculatedFrom([])).toBe('');
	});
});
