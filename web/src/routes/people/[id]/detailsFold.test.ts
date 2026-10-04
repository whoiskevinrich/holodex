import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// HOLODEX-539: the Person Details field list is a fold, closed at rest for owner and visitor.
// No component harness exists in web/, so these pin the source shape (as media/[id]'s
// deleteConfirmReset.test.ts does); the open/close and deep-link landing are live QA.
const source = readFileSync(fileURLToPath(new URL('./+page.svelte', import.meta.url)), 'utf8');

describe('person Details fold', () => {
	it('starts closed and is not gated on owner', () => {
		expect(source).toMatch(/let detailsExpanded = \$state\(false\);/);
		expect(source).not.toMatch(/isOwner \? detailsExpanded/);
	});

	it('clips the field list rather than unmounting it, inert while closed', () => {
		const fold = source.slice(source.indexOf('id="person-details-fields"'));
		expect(fold).toMatch(/^[^>]*inert=\{!detailsExpanded\}/);
		expect(fold).toMatch(/^[^>]*max-height: \{detailsExpanded \? '6000px' : '0px'\}/);
	});

	it('toggles from a labelled disclosure button', () => {
		expect(source).toMatch(/aria-expanded=\{detailsExpanded\}/);
		expect(source).toMatch(/aria-controls="person-details-fields"/);
	});

	it('closes again when the route component is reused for another person', () => {
		const load = source.indexOf('load(id);');
		const effect = source.slice(source.lastIndexOf('$effect(', load), load);
		expect(effect).toMatch(/detailsExpanded = false;/);
	});

	it('opens for a #field-* deep link that targets a row inside the fold', () => {
		const fn = source.slice(source.indexOf('async function openDeepLinkedDetail'));
		const body = fn.slice(0, fn.indexOf('\n\t}\n'));
		expect(body).toMatch(/row\?\.closest\('#person-details-fields'\)/);
		expect(body).toMatch(/detailsExpanded = true;/);
		expect(source).toMatch(/const hash = \$page\.url\.hash;/);
	});
});
