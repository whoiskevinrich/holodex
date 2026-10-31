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

// Both entity panels (person, and studio since 598) render the row's footer `verdicts` and
// its per-column `mergeInto`, and never a verdict of their own.
describe.each(['DuplicateComparePanel.svelte', 'DuplicateStudioPanel.svelte'])('%s verdict ownership', (file) => {
	const src = read(file);
	const panel = template(src);

	it('renders the verdicts through the snippet the row passed in', () => {
		expect(panel).toMatch(/\{@render verdicts\(\)\}/);
	});

	it('renders Merge into this one through the row snippet, once per column', () => {
		expect(panel).toMatch(/\{@render mergeInto\(ref, other\)\}/);
		expect(panel).toMatch(/\{@render column\('a', pair\.a, pair\.b\)\}/);
		expect(panel).toMatch(/\{@render column\('b', pair\.b, pair\.a\)\}/);
	});

	it('has no verdict control of its own — no second Keep separate or Merge', () => {
		expect(panel).not.toMatch(/Keep separate/);
		expect(panel).not.toMatch(/Merge into this one/);
		expect(panel).not.toMatch(/>\s*Merge\s*</);
	});

	it('carries no merge or dismiss call, so a panel state cannot resolve a pair', () => {
		expect(src).not.toMatch(/mergeEntities|dismissDuplicate/);
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

	it('defines the verdicts and Merge into this one once each, as snippets', () => {
		expect(src.match(/\{#snippet verdicts\(\)\}/g)).toHaveLength(1);
		expect(src.match(/\{#snippet mergeInto\(/g)).toHaveLength(1);
	});

	it('passes those same snippets to both panels', () => {
		for (const panel of ['DuplicateComparePanel', 'DuplicateStudioPanel']) {
			expect(markup).toMatch(new RegExp(`<${panel}[\\s\\S]*?\\{verdicts\\}[\\s\\S]*?\\{mergeInto\\}[\\s\\S]*?/>`));
		}
	});

	// C2/C5: on a person or studio pair the merge is the accent, named for screen readers,
	// and it only opens the row's confirm — it never merges on one click.
	it('styles Merge into this one as the accent pill, opening the confirm', () => {
		const snippet = src.slice(src.indexOf('{#snippet mergeInto('));
		const button = snippet.slice(0, snippet.indexOf('</button>'));
		expect(button).toContain('class={PILL_ACTION}');
		expect(button).toContain('openConfirm(keep, other)');
		expect(button).toMatch(/aria-label=\{`Merge \$\{refLabel\(other\)\} into \$\{refLabel\(keep\)\}`\}/);
	});

	// P0-11: Show as is a studio-only radio, and its pick reaches the merge call.
	it('sends the Show as pick on a studio merge only', () => {
		expect(src).toMatch(/merge\(confirm\.keep\.id, confirm\.other\.id, isStudio \? showAs : undefined\)/);
		expect(markup).toMatch(/<fieldset[\s\S]*?Show the studio as[\s\S]*?bind:group=\{showAs\}[\s\S]*?<\/fieldset>/);
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

	/** The one button whose source contains every one of `parts`. */
	function buttonWith(...parts: string[]): string {
		const found = buttons.filter((b) => parts.every((p) => b.includes(p)));
		expect(found, `expected exactly one <button> carrying ${parts.join(' + ')}`).toHaveLength(1);
		return found[0];
	}

	// RD7, and `app.css` names these buttons in its `.btn-ghost` / `.btn-accent` doc
	// comments. On a tag or film pair the swap is the whole point of P0-4: 185 dismissals
	// against 31 open merges, so accenting Merge optimised the page backwards. On a person
	// or studio pair (C2) Keep separate is the ghost, as Keep both is in the Videos lane,
	// because the accent has moved to Merge into this one in the panel.
	it('styles Keep separate by grammar: accent on tag/film pairs, ghost on person/studio', () => {
		expect(src).toMatch(/PILL_ACTION\s*=\s*'btn-row btn-pill btn-accent'/);
		expect(src).toMatch(/GHOST\s*=\s*'btn-row btn-ghost px-2'/);
		expect(buttons.filter((b) => b.includes('doDismiss'))).toHaveLength(2);
		buttonWith('doDismiss', 'class={PILL_ACTION}');
		buttonWith('doDismiss', 'class={GHOST}');
		expect(buttonWith('choosing = true')).toContain('class={GHOST}');
	});

	it('offers only Keep separate on a comparable row: Merge lives in the panel', () => {
		const verdicts = markup.slice(markup.indexOf('{#snippet verdicts()}'), markup.indexOf('{/snippet}'));
		const comparable = verdicts.slice(verdicts.indexOf('{#if comparable}'), verdicts.indexOf('{:else if choosing}'));
		expect(comparable.match(/<button/g)).toHaveLength(1);
		expect(comparable).toContain('Keep separate');
	});

	// Merge is irreversible (ADR-061), so it is NOT `.btn-quiet` — app.css documents
	// that class as "a UI-only toggle with no side effect (Cancel, Undo)".
	it('never styles Merge as the no-side-effect toggle', () => {
		expect(src).toMatch(/TOGGLE\s*=\s*'btn-row btn-quiet'/);
		expect(buttonWith('choosing = true')).not.toContain('class={TOGGLE}');
	});
});
