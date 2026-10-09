import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// F70 P0-4 / P0-5, asserted the way `routes/media/[id]/playerElement.test.ts` asserts
// element identity: a structural guarantee, pinned statically because this repo has no
// component-test harness (docs/testing-strategy.md §5).
//
// The panel repeats the ROW's verdict snippet rather than rendering its own pair. That
// one fact is what makes three separate criteria true at once and unbreakable rather
// than merely currently-true:
//   * the collapsed row and the open panel can never disagree about `busy`, or about
//     which verdict carries the accent (P0-4);
//   * a per-side fetch failure cannot disable a verdict, because the failure lives in
//     the panel and the verdicts are not the panel's to disable (P0-5);
//   * Merge stays two-step, because `choosing` is the row's state and the survivor
//     buttons render from the same snippet in both places.
// A future "just inline the buttons in the footer" refactor breaks all three silently.
// It fails here instead.
const read = (name: string) =>
	readFileSync(fileURLToPath(new URL(`./${name}`, import.meta.url)), 'utf8');

/** The markup after the script block, with HTML comments stripped — the comments
 *  discuss the verdicts at length and prose must not count as markup. */
function template(src: string): string {
	const end = src.lastIndexOf('</script>');
	return (end === -1 ? src : src.slice(end + '</script>'.length)).replace(/<!--[\s\S]*?-->/g, '');
}

describe('DuplicateComparePanel verdict ownership', () => {
	const panel = template(read('DuplicateComparePanel.svelte'));

	it('renders the verdicts through the snippet the row passed in', () => {
		expect(panel).toMatch(/\{@render verdicts\(\)\}/);
	});

	it('has no verdict control of its own — no second Keep separate or Merge', () => {
		expect(panel).not.toMatch(/Keep separate/);
		expect(panel).not.toMatch(/>\s*Merge\s*</);
	});

	it('carries no merge or dismiss call, so a panel state cannot resolve a pair', () => {
		expect(panel).not.toMatch(/mergeEntities|dismissDuplicate/);
	});
});

// F76: the same rule for the Videos group. The panel renders the row's snippets and calls
// the row's `label`; it never talks to the resolving endpoints itself.
describe('VideoComparePanel verdict ownership', () => {
	const panel = template(read('VideoComparePanel.svelte'));

	it('renders Keep both and Keep this one through the row snippets', () => {
		expect(panel).toMatch(/\{@render verdicts\(\)\}/);
		expect(panel).toMatch(/\{@render keepThis\(/);
		expect(panel).not.toMatch(/Keep both/);
		expect(panel).not.toMatch(/Keep this one/);
	});

	it('carries no resolving call of its own', () => {
		const src = read('VideoComparePanel.svelte');
		expect(src).not.toMatch(/keepVideo|keepBothVideos|labelVideoPair/);
	});
});

describe('VideoPairRow verdict ownership', () => {
	const src = read('VideoPairRow.svelte');
	const markup = template(src);

	it('defines each verdict once and hands the same snippets to the panel', () => {
		expect(src.match(/\{#snippet verdicts\(\)\}/g)).toHaveLength(1);
		expect(src.match(/\{#snippet keepThis\(/g)).toHaveLength(1);
		expect(markup).toMatch(/<VideoComparePanel[\s\S]*?\{verdicts\}[\s\S]*?\{keepThis\}[\s\S]*?\/>/);
	});

	it('styles Keep both as the bordered ghost and Keep this one as the accent pill', () => {
		expect(src).toMatch(/GHOST\s*=\s*'btn-row btn-ghost px-2'/);
		expect(src).toMatch(/PILL_ACTION\s*=\s*'btn-row btn-pill btn-accent'/);
	});
});

describe('DuplicatePairRow verdict ownership', () => {
	const src = read('DuplicatePairRow.svelte');
	const markup = template(src);

	it('defines the verdicts once, as a snippet', () => {
		expect(src.match(/\{#snippet verdicts\(\)\}/g)).toHaveLength(1);
	});

	it('passes that same snippet to the panel', () => {
		expect(markup).toMatch(/<DuplicateComparePanel[\s\S]*?\{verdicts\}[\s\S]*?\/>/);
	});

	it('renders the panel only inside the open gate, so a collapsed row fetches nothing', () => {
		const at = markup.indexOf('<DuplicateComparePanel');
		expect(at).toBeGreaterThan(-1);
		const before = markup.slice(0, at);
		const opens = (before.match(/\{#(if|each|await|key)\b/g) ?? []).length;
		const closes = (before.match(/\{\/(if|each|await|key)\}/g) ?? []).length;
		expect(opens - closes).toBeGreaterThan(0);
	});

	// Whole `<button …>…</button>` elements. Scanning to the tag's closing `>` does not
	// work here: an inline arrow handler contains `=>`, so a `[^>]*` attribute scan stops
	// inside the attribute. Taking the element also makes these assertions independent of
	// attribute ORDER — a formatter that swaps `class` and `onclick` changes nothing the
	// owner can see, and a test that goes red on that gets weakened rather than fixed.
	const buttons = markup.match(/<button[\s\S]*?<\/button>/g) ?? [];

	/** The one button whose source contains `handler`. */
	function buttonWith(handler: string): string {
		const found = buttons.filter((b) => b.includes(handler));
		expect(found, `expected exactly one <button> carrying ${handler}`).toHaveLength(1);
		return found[0];
	}

	// RD7, and `app.css` names these two buttons by name in its `.btn-ghost` /
	// `.btn-accent` doc comments. The swap is the whole point of P0-4: 185 dismissals
	// against 31 open merges, so accenting Merge optimised the page backwards.
	it('gives Keep separate the accent pill and Merge the bordered ghost', () => {
		expect(src).toMatch(/PILL_ACTION\s*=\s*'btn-row btn-pill btn-accent'/);
		expect(src).toMatch(/GHOST\s*=\s*'btn-row btn-ghost px-2'/);
		expect(buttonWith('doDismiss')).toContain('class={PILL_ACTION}');
		expect(buttonWith('choosing = true')).toContain('class={GHOST}');
	});

	// Merge is irreversible (ADR-061), so it is NOT `.btn-quiet` — app.css documents
	// that class as "a UI-only toggle with no side effect (Cancel, Undo)".
	it('never styles Merge as the no-side-effect toggle', () => {
		expect(src).toMatch(/TOGGLE\s*=\s*'btn-row btn-quiet'/);
		expect(buttonWith('choosing = true')).not.toContain('class={TOGGLE}');
	});
});
