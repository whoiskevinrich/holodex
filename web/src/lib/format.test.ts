import { describe, it, expect } from 'vitest';
import {
	releaseYear,
	formatYear,
	formatDuration,
	resolutionBucket,
	providerFromWinningSource,
	calculatedFrom,
	filterByTitle,
	sortExternalLinks,
	isHttpUrl
} from './format';
import type { ExternalLink } from './types';

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

describe('filterByTitle', () => {
	// NS6 (HOLODEX-249): the video-list twin of filterByName, keyed on `title`
	// instead of `name` — same case-insensitive substring match.
	const videos = [{ title: 'Jackson Interview 2024' }, { title: "Jack & Jackson: The Reunion" }, { title: 'Unrelated Vlog' }];

	it('matches case-insensitively on a substring of title', () => {
		expect(filterByTitle(videos, 'jackson')).toEqual([videos[0], videos[1]]);
		expect(filterByTitle(videos, 'REUNION')).toEqual([videos[1]]);
	});

	it('returns every item for an empty or whitespace-only query', () => {
		expect(filterByTitle(videos, '')).toEqual(videos);
		expect(filterByTitle(videos, '   ')).toEqual(videos);
	});

	it('returns an empty array when nothing matches', () => {
		expect(filterByTitle(videos, 'nonexistent')).toEqual([]);
	});
});

describe('sortExternalLinks', () => {
	// HOLODEX-266/ADR-083 D3: the multi-badge row orders by display label, not
	// insertion/backend row order, so it's stable across reloads regardless of
	// which provider ran last.
	it('orders by label alphabetically, independent of input order', () => {
		const links: ExternalLink[] = [
			{ provider: 'x', label: 'X-Provider', url: 'https://x.example/1' },
			{ provider: 'imdb', label: 'IMDb', url: 'https://imdb.example/1' },
			{ provider: 'tmdb', label: 'TMDB', url: 'https://tmdb.example/1' }
		];
		expect(sortExternalLinks(links).map((l) => l.label)).toEqual(['IMDb', 'TMDB', 'X-Provider']);
	});

	it('does not mutate the input array', () => {
		const links: ExternalLink[] = [
			{ provider: 'tmdb', label: 'TMDB' },
			{ provider: 'imdb', label: 'IMDb' }
		];
		const original = [...links];
		sortExternalLinks(links);
		expect(links).toEqual(original);
	});

	it('handles a degraded (no-url) badge the same as a resolved one', () => {
		const links: ExternalLink[] = [
			{ provider: 'tmdb', label: 'TMDB', url: 'https://tmdb.example/1' },
			{ provider: 'unknown', label: 'Unknown' }
		];
		expect(sortExternalLinks(links).map((l) => l.label)).toEqual(['TMDB', 'Unknown']);
	});
});

describe('isHttpUrl', () => {
	// The provider-link badge's XSS gate (ProviderLinkBadge.svelte): only http(s)
	// values may become an `href`, since Svelte doesn't sanitize it and a
	// provider-declared link_template could otherwise emit javascript:/data:/etc.
	it('accepts http and https, case-insensitively', () => {
		expect(isHttpUrl('https://example.com/x')).toBe(true);
		expect(isHttpUrl('HTTP://example.com/x')).toBe(true);
	});

	it('rejects non-http(s) schemes and non-URLs', () => {
		expect(isHttpUrl('javascript:alert(1)')).toBe(false);
		expect(isHttpUrl('data:text/html,x')).toBe(false);
		expect(isHttpUrl('mailto:a@b.com')).toBe(false);
		expect(isHttpUrl('not a url')).toBe(false);
		expect(isHttpUrl('')).toBe(false);
	});
});

// releaseYear must stay in step with filmReleaseYear (internal/api/film_year.go) — the
// server parses release_date to FILL films.year, the client parses it to decide whether
// the year and the release date DISAGREE. If the two ever diverge, the page would report
// a disagreement the server does not see, or stay silent about one it does. The cases
// below mirror TestFilmReleaseYear one for one.
describe('releaseYear', () => {
	const cases: Array<[string, string | null | undefined, number]> = [
		['contract-preferred full date', '2001-07-20', 2001],
		['bare year', '2001', 2001],
		['leading and trailing space', '  1999-01-01  ', 1999],
		['slash-separated date still leads with the year', '2001/07/20', 2001],

		// Everything below must yield 0, not a plausible-looking number.
		['empty', '', 0],
		['undefined', undefined, 0],
		['null', null, 0],
		['too short to hold a year', '201', 0],
		['day-first date would otherwise parse the day as a year', '20-07-2001', 0],
		['non-numeric', 'soon', 0],
		['partially numeric', '20x1-07-20', 0],
		['explicit zero is not a release year', '0000-01-01', 0]
	];

	for (const [name, input, want] of cases) {
		it(name, () => {
			expect(releaseYear(input)).toBe(want);
		});
	}
});
