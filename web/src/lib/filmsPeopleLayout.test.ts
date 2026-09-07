import { describe, expect, it } from 'vitest';
import { filmsPeopleLayout } from './filmsPeopleLayout';

// The state matrix from docs/design/media-detail-films-people-handoff.md §2: four link
// states x owner/visitor. Each case is named for the row it corresponds to in that table,
// so a failure points straight at the design doc.
const base = { filmsEnabled: true, isOwner: true, filmCount: 0, peopleCount: 0 };

describe('filmsPeopleLayout — owner', () => {
	it('1 · neither linked: two bare CTAs sharing one line, no headings', () => {
		expect(filmsPeopleLayout(base)).toEqual({ row: 'inline-ctas', films: 'cta', people: 'cta' });
	});

	it('2 · people only: Films degrades to a CTA above a full-width People section', () => {
		expect(filmsPeopleLayout({ ...base, peopleCount: 3 })).toEqual({
			row: 'stacked',
			films: 'cta',
			people: 'section'
		});
	});

	it('3 · film only: Films section above a bare People CTA', () => {
		expect(filmsPeopleLayout({ ...base, filmCount: 1 })).toEqual({
			row: 'stacked',
			films: 'section',
			people: 'cta'
		});
	});

	it('4 · both linked: the two sections sit side by side', () => {
		expect(filmsPeopleLayout({ ...base, filmCount: 1, peopleCount: 2 })).toEqual({
			row: 'side-by-side',
			films: 'section',
			people: 'section'
		});
	});
});

describe('filmsPeopleLayout — visitor', () => {
	const visitor = { ...base, isOwner: false };

	it('1 · neither linked: the whole row is absent, not an empty state', () => {
		expect(filmsPeopleLayout(visitor)).toEqual({ row: 'hidden', films: 'hidden', people: 'hidden' });
	});

	it('2 · people only: People alone, and Films leaves no trace', () => {
		expect(filmsPeopleLayout({ ...visitor, peopleCount: 3 })).toEqual({
			row: 'stacked',
			films: 'hidden',
			people: 'section'
		});
	});

	it('3 · film only: Films alone, no People CTA', () => {
		expect(filmsPeopleLayout({ ...visitor, filmCount: 1 })).toEqual({
			row: 'stacked',
			films: 'section',
			people: 'hidden'
		});
	});

	it('4 · both linked: same side-by-side layout as the owner sees', () => {
		expect(filmsPeopleLayout({ ...visitor, filmCount: 1, peopleCount: 2 })).toEqual({
			row: 'side-by-side',
			films: 'section',
			people: 'section'
		});
	});
});

describe('filmsPeopleLayout — films_enabled off', () => {
	// The capability suppresses Films for everyone, so state 1 collapses to the People
	// CTA alone (stacked, not inline — there is no second CTA to sit beside).
	it('hides Films from the owner even with attachments present', () => {
		expect(filmsPeopleLayout({ ...base, filmsEnabled: false, filmCount: 2 })).toEqual({
			row: 'stacked',
			films: 'hidden',
			people: 'cta'
		});
	});

	it('leaves a visitor with nothing when People is also empty', () => {
		expect(filmsPeopleLayout({ ...base, filmsEnabled: false, isOwner: false, filmCount: 2 })).toEqual({
			row: 'hidden',
			films: 'hidden',
			people: 'hidden'
		});
	});
});
