import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import CoreImageEditButton, { coreEditLabel } from './CoreImageEditButton.svelte';

// HOLODEX-43 (F25.35): a core slot the owner set (upload/promoted) shows a lock in its Edit
// control and names it in the accessible name; an enrichment slot is today's control; a
// visitor gets no control at all.
const html = (props: { isOwner: boolean; locked?: boolean; compact?: boolean; role?: string }) =>
	render(CoreImageEditButton, {
		props: { role: props.role ?? 'poster', position: 'right-1 top-1', onclick: () => {}, ...props }
	}).body;

describe('coreEditLabel', () => {
	it('names the lock for an owner-set slot', () => {
		expect(coreEditLabel('poster', true)).toBe('Replace poster (yours — kept on re-enrich)');
	});

	it('keeps the plain label for an enrichment slot', () => {
		expect(coreEditLabel('poster', false)).toBe('Replace poster');
	});
});

describe('CoreImageEditButton remove (HOLODEX-610)', () => {
	const withRemove = (props: { isOwner: boolean; compact?: boolean }) =>
		render(CoreImageEditButton, {
			props: { role: 'banner', position: 'right-2 top-2', onclick: () => {}, onremove: () => {}, ...props }
		}).body;

	it('owner, filled slot: Remove beside Edit', () => {
		const out = withRemove({ isOwner: true });
		expect(out).toContain('data-remove');
		expect(out).toContain('aria-label="Remove banner"');
		expect(out).toContain('Remove');
	});

	it('compact: icon-only Remove', () => {
		const out = withRemove({ isOwner: true, compact: true });
		expect(out).toContain('data-remove');
		expect(out).not.toContain('>Remove<');
	});

	it('no onremove (empty slot): no Remove control', () => {
		expect(html({ isOwner: true })).not.toContain('data-remove');
	});

	it('visitor: nothing', () => {
		expect(withRemove({ isOwner: false })).not.toContain('data-remove');
	});
});

describe('CoreImageEditButton', () => {
	it('owner, locked: lock icon before Edit, label names the lock', () => {
		const out = html({ isOwner: true, locked: true });
		expect(out).toContain('data-lock');
		expect(out).toContain('aria-label="Replace poster (yours — kept on re-enrich)"');
		expect(out).toContain('title="Replace poster (yours — kept on re-enrich)"');
		expect(out).toContain('Edit');
	});

	it('owner, enrichment slot: no lock, plain label', () => {
		const out = html({ isOwner: true, locked: false });
		expect(out).not.toContain('data-lock');
		expect(out).toContain('aria-label="Replace poster"');
	});

	it('owner, locked compact headshot: lock beside the pencil, still 24px tall', () => {
		const out = html({ isOwner: true, locked: true, compact: true, role: 'headshot' });
		expect(out).toContain('data-lock');
		expect(out).toContain('h-6 gap-0.5 px-1');
		expect(out).not.toContain('h-6 w-6');
		expect(out).toContain('aria-label="Replace headshot (yours — kept on re-enrich)"');
	});

	it('owner, locked but uploading: lock withdrawn rather than dimmed', () => {
		expect(
			render(CoreImageEditButton, {
				props: { role: 'poster', position: '', onclick: () => {}, isOwner: true, locked: true, busy: true }
			}).body
		).not.toContain('data-lock');
	});

	it('visitor: renders nothing, locked or not', () => {
		expect(html({ isOwner: false, locked: true })).not.toContain('<button');
		expect(html({ isOwner: false, locked: false })).not.toContain('<button');
	});
});
