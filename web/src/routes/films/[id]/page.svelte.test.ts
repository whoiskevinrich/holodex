import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { FilmDetailResponse, ResolvedField } from '#lib/types.ts';
import Page from './+page.svelte';

// F59 P0-13 (HOLODEX-95): visitor vs. owner is a control gate, not a content gate. A visitor
// sees the film's Details whenever a row has a value, unboxed, with one "Enriched from …" note
// when a single provider supplies every row; the owner keeps the boxed section. Before this,
// the whole section was owner-only, so an owner-only test would have passed throughout.

const { api, activity } = vi.hoisted(() => ({
	api: {
		getFilm: vi.fn<() => Promise<FilmDetailResponse>>(),
		enrichSources: vi.fn()
	},
	activity: {
		data: null,
		caps: null,
		error: '',
		loading: false,
		effectiveOwner: false,
		needToken: false,
		start: () => {},
		stop: () => {}
	}
}));

vi.mock('#lib/api.ts', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.ts')>()),
	api
}));
vi.mock('#lib/activity.svelte.ts', () => ({ activity }));
vi.mock('$app/state', () => ({ page: { params: { id: '7' }, url: new URL('http://localhost/films/7') } }));
vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	afterNavigate: vi.fn(),
	replaceState: vi.fn(),
	pushState: vi.fn()
}));

function releaseDate(winning_source: string): ResolvedField {
	return {
		canonical: 'release_date',
		label: 'Released',
		values: ['2019-03-14'],
		winning_source
	} as ResolvedField;
}

function detail(resolved: ResolvedField[]): FilmDetailResponse {
	return {
		film: { id: 7, ref: 'film:7', name: 'Film title', year: 2019 },
		resolved,
		scenes: [],
		full_films: [],
		cast: [],
		tags: [],
		studios: []
	} as FilmDetailResponse;
}

function detailsSection(container: HTMLElement): HTMLElement | null {
	return container.querySelector('#field-release_date')?.closest('section') ?? null;
}

beforeEach(() => {
	activity.effectiveOwner = false;
	api.getFilm.mockReset();
	api.enrichSources.mockReset().mockResolvedValue({ sources: [] });
});

describe('/films/[id] Details (F59 P0-13, HOLODEX-95)', () => {
	it('shows a visitor the release date, unboxed, with one provider note and no controls', async () => {
		api.getFilm.mockResolvedValue(detail([releaseDate('provider1:release_date')]));
		const screen = await render(Page);

		await expect.element(screen.getByText('2019-03-14')).toBeInTheDocument();
		await expect.element(screen.getByText(/Enriched from\s*provider1/)).toBeInTheDocument();
		const section = detailsSection(screen.container)!;
		expect(section).not.toBeNull();
		// Unboxed. The harness loads no app.css, so the computed box is checked live (handoff
		// QA 1); here the box classes must simply be absent.
		expect(section.className).not.toMatch(/\bborder\b|\bbg-surface\b|\bp-4\b/);
		// No owner controls: no source picker, no "custom" or per-row badge under the note.
		expect(section.querySelector('[role="radiogroup"]')).toBeNull();
		expect(section.querySelector('button')).toBeNull();
		expect(api.enrichSources).not.toHaveBeenCalled();
	});

	it('drops the note and badges the row when the value is custom', async () => {
		api.getFilm.mockResolvedValue(detail([releaseDate('manual:release_date')]));
		const screen = await render(Page);

		await expect.element(screen.getByText('2019-03-14')).toBeInTheDocument();
		await expect.element(screen.getByText('custom')).toBeInTheDocument();
		expect(screen.container.textContent).not.toContain('Enriched from');
	});

	it('shows a visitor no Details section when nothing has a value', async () => {
		api.getFilm.mockResolvedValue(detail([]));
		const screen = await render(Page);

		await expect.element(screen.getByRole('heading', { name: 'Film title' })).toBeInTheDocument();
		expect(screen.container.querySelector('#enrich-providers')).toBeNull();
	});

	it('keeps the boxed section for the owner', async () => {
		activity.effectiveOwner = true;
		api.getFilm.mockResolvedValue(detail([releaseDate('provider1:release_date')]));
		const screen = await render(Page);

		await expect.element(screen.getByText('Released:')).toBeInTheDocument();
		const section = detailsSection(screen.container)!;
		expect(section.className).toMatch(/\bborder\b.*\bbg-surface\b.*\bp-4\b/);
		expect(screen.container.textContent).not.toContain('Enriched from');
	});
});
