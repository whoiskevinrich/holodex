import { describe, it, expect } from 'vitest';
import { render, summary, exitCode } from './report.mjs';
import { ASSERTIONS, validate } from './assertions.mjs';

const assertion = {
	key: 'person-tiles-stay-legible',
	finds: 'Headshot tiles collapsing as the cast grows.',
	selector: 'li.curation-chip .portrait-frame--2x3',
	measure: 'width',
	expect: { min: 40 }
};

const stats = { cells: 6, pages: 12, elapsedMs: 4200 };

describe('render', () => {
	// The AC: a failure names the page, the variant, the selector, and measured vs
	// expected. All four, on one screen, or the report costs more time than it saves.
	it('names the page, the variant, the selector and measured vs expected', () => {
		const out = render(
			[
				{
					assertion,
					url: '/media/104',
					label: 'people/25 (video 104, /media/104)',
					cell: 'brutalist/narrow',
					status: 'fail',
					detail: '3/25 elements have width outside >= 40',
					offenders: [{ index: 7, value: 31.5, tag: 'div.portrait-frame', text: 'Ada Lovelace' }]
				}
			],
			stats
		);
		expect(out).toContain('/media/104');
		expect(out).toContain('people/25');
		expect(out).toContain('li.curation-chip .portrait-frame--2x3');
		expect(out).toContain('brutalist/narrow');
		expect(out).toContain('= 31.5');
		expect(out).toContain('want >= 40');
		expect(out).toContain('Ada Lovelace');
	});

	it('collapses a passing assertion to a single line', () => {
		const pass = { assertion, url: '/media/104', label: 'people/25', cell: 'broadcast/wide', status: 'pass', detail: 'ok' };
		const out = render([pass, { ...pass, cell: 'brutalist/wide' }], stats);
		expect(out.split('\n').filter((l) => l.includes('person-tiles-stay-legible'))).toHaveLength(1);
		expect(out).toContain('2 checks');
	});

	it('truncates a long offender list rather than flooding the terminal', () => {
		const offenders = Array.from({ length: 9 }, (_, i) => ({ index: i, value: 1, tag: 'div', text: '' }));
		const out = render(
			[{ assertion, url: '/u', label: 'l', cell: 'c', status: 'fail', detail: 'd', offenders }],
			stats
		);
		expect(out).toContain('… and 4 more');
	});

	it('marks a known-open failure with its ticket', () => {
		const out = render(
			[
				{
					assertion: { ...assertion, blockedBy: 'HOLODEX-354' },
					url: '/people',
					label: '/people',
					cell: 'broadcast/wide',
					status: 'blocked',
					detail: 'matched 2064 elements, want <= 200'
				}
			],
			stats
		);
		expect(out).toContain('[HOLODEX-354]');
	});

	it('prints why an assertion was skipped instead of pretending it passed', () => {
		const out = render(
			[{ assertion, url: '—', label: '—', cell: '—', status: 'skipped', detail: 'needs a large fixture' }],
			stats
		);
		expect(out).toContain('skip');
		expect(out).toContain('needs a large fixture');
	});
});

describe('summary and exit code', () => {
	const mk = (status) => ({ assertion, url: '/u', label: 'l', cell: 'c', status, detail: 'd' });

	it('counts each outcome by name', () => {
		const line = summary([mk('pass'), mk('fail'), mk('blocked'), mk('skipped')], stats);
		expect(line).toContain('1 passed');
		expect(line).toContain('1 failed');
		expect(line).toContain('1 known-open');
		expect(line).toContain('1 skipped');
		expect(line).toContain('6 skin/width cells');
	});

	it('stays green for a known-open bug and a skip, red for anything unknown', () => {
		expect(exitCode([mk('pass'), mk('blocked'), mk('skipped')])).toBe(0);
		expect(exitCode([mk('pass'), mk('vacuous')])).toBe(1);
		expect(exitCode([mk('pass'), mk('fixed')])).toBe(1);
	});
});

describe('the shipped assertion table', () => {
	it('is well formed', () => {
		expect(validate()).toEqual([]);
	});

	it('says what each assertion finds, so a failure explains itself', () => {
		for (const a of ASSERTIONS) expect(a.finds.length).toBeGreaterThan(40);
	});
});

describe('validate', () => {
	const ok = { key: 'k', finds: 'f', selector: '.x', measure: 'width', expect: { min: 1 }, urls: ['/u'] };

	it('catches a metric typo before it fails every page incomprehensibly', () => {
		expect(validate([{ ...ok, measure: 'widht' }])[0]).toMatch(/measure must be one of/);
	});

	it('refuses an assertion that targets nothing, or both kinds of target', () => {
		expect(validate([{ ...ok, urls: undefined }])[0]).toMatch(/exactly one of/);
		expect(validate([{ ...ok, when: () => true }])[0]).toMatch(/exactly one of/);
	});

	it('refuses an unbounded expectation and a duplicate key', () => {
		expect(validate([{ ...ok, expect: {} }])[0]).toMatch(/needs a min, a max/);
		expect(validate([ok, ok]).some((p) => /duplicate key/.test(p))).toBe(true);
	});
});
