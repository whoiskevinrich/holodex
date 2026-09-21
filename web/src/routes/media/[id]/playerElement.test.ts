import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// F69 P0-9 / ADR-104 D4: the media page's <video> must be the SAME node across
// /media/A -> /media/B, which means it is rendered outside every {#if} in the page —
// the loading gate, the codec-failure branch, any per-item block. A live check
// (document.querySelector('video') identity before/after next-up) is the proof; this
// test pins the structural invariant that makes it true, so a refactor that wraps the
// element again fails here rather than in a PiP window.
const source = readFileSync(fileURLToPath(new URL('./+page.svelte', import.meta.url)), 'utf8');

// The markup after the script block, with HTML comments removed (they talk about the
// element too, and a `{#if}` quoted in prose must not count as a block).
function template(src: string): string {
	const end = src.lastIndexOf('</script>');
	return (end === -1 ? src : src.slice(end + '</script>'.length)).replace(/<!--[\s\S]*?-->/g, '');
}

function blockDepthAt(markup: string, index: number): number {
	const before = markup.slice(0, index);
	const opens = (before.match(/\{#(if|each|await|key)\b/g) ?? []).length;
	const closes = (before.match(/\{\/(if|each|await|key)\}/g) ?? []).length;
	return opens - closes;
}

describe('media page player element', () => {
	const markup = template(source);

	it('renders exactly one <video>', () => {
		expect(markup.match(/<video\b/g)).toHaveLength(1);
	});

	it('renders the <video> outside every {#if}/{#each}/{#key} block', () => {
		const at = markup.indexOf('<video');
		expect(at).toBeGreaterThan(-1);
		expect(blockDepthAt(markup, at)).toBe(0);
	});
});
