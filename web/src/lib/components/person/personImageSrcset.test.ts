import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import type { PersonImage } from '#lib/types.ts';
import { personImageSrcset } from './personImageSrcset.ts';
import PersonImageFrame from './PersonImageFrame.svelte';
import PersonBanner from './PersonBanner.svelte';
import PersonImageViewer from './PersonImageViewer.svelte';
import CropEditor from '../writeback/CropEditor.svelte';

// HOLODEX-332: small surfaces request a `?w=` tier through srcset/sizes; full-resolution
// surfaces (banner, viewer, crop editor) load the original only.
const url = (w?: number) => `/img${w ? `?w=${w}` : ''}`;

describe('personImageSrcset', () => {
	it('lists the three tiers, then the original at the 2000px storage cap', () => {
		expect(personImageSrcset(url)).toBe('/img?w=160 160w, /img?w=320 320w, /img?w=640 640w, /img 2000w');
	});

	it('drops tiers at or above a known original width', () => {
		expect(personImageSrcset(url, 320)).toBe('/img?w=160 160w, /img 320w');
	});
});

describe('PersonImageFrame', () => {
	const frame = (props: { version?: number; sizes?: string; imageId?: number }) =>
		render(PersonImageFrame, {
			props: { personId: 7, role: 'poster', name: 'Ana', frameClass: 'portrait-frame--2x3', ...props }
		}).body;

	it('a filled slot with sizes gets every tier, the original and the caller sizes', () => {
		const out = frame({ version: 3, sizes: '80px' });
		expect(out).toContain(
			'srcset="/api/v1/people/7/image/poster?v=3&amp;w=160 160w, /api/v1/people/7/image/poster?v=3&amp;w=320 320w, ' +
				'/api/v1/people/7/image/poster?v=3&amp;w=640 640w, /api/v1/people/7/image/poster?v=3 2000w"'
		);
		expect(out).toContain('sizes="80px"');
		expect(out).toContain('src="/api/v1/people/7/image/poster?v=3"');
	});

	it('a by-id gallery frame tiers the gallery URL', () => {
		expect(frame({ version: 3, sizes: '44px', imageId: 9 })).toContain('/api/v1/people/7/images/9?v=3&amp;w=160 160w');
	});

	it('an empty slot (placeholder SVG) gets no srcset', () => {
		const out = frame({ sizes: '80px' });
		expect(out).not.toContain('srcset');
		expect(out).not.toContain('sizes=');
	});

	it('no sizes means the original only', () => {
		expect(frame({ version: 3 })).not.toContain('srcset');
	});
});

describe('full-resolution surfaces', () => {
	const image: PersonImage = {
		id: 9,
		role: 'extra',
		source: 'upload',
		version: 4,
		width: 1200,
		height: 1800,
		sort_order: 0,
		created_at: ''
	};
	const noop = () => {};

	it('PersonBanner requests no width', () => {
		const out = render(PersonBanner, { props: { personId: 7, name: 'Ana', version: 3 } }).body;
		expect(out).toContain('/api/v1/people/7/image/banner?v=3');
		expect(out).not.toContain('srcset');
		expect(out).not.toContain('w=');
	});

	it('PersonImageViewer requests no width', () => {
		const out = render(PersonImageViewer, {
			props: { personId: 7, name: 'Ana', items: [image], index: 0, onclose: noop, onnavigate: noop }
		}).body;
		expect(out).toContain('/api/v1/people/7/images/9?v=4');
		expect(out).not.toContain('srcset');
		expect(out).not.toContain('w=');
	});

	it('CropEditor requests no width', () => {
		const out = render(CropEditor, {
			props: { personId: 7, image, role: 'poster', onclose: noop, onpromoted: noop }
		}).body;
		expect(out).toContain('/api/v1/people/7/images/9?v=4');
		expect(out).not.toContain('srcset');
		expect(out).not.toContain('w=');
	});
});
