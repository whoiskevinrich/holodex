// The server's parse of a list's `expr`, for its chips (F87 query-language-handoff.md):
// drawn from the display tree, never from the text. The previous parse stays until the
// new one returns. Browse and the playlist editor share it. Call during component init.
import { api } from './api';
import { toMessage } from './format';
import { exprWithout, toChips, type QueryParseResult } from './query';

/** `stored` reads expr as a saved definition (api.parseQuery). */
export function parsedExpr(expr: () => string | undefined, stored = false) {
	let parsed = $state<QueryParseResult | null>(null);
	let seq = 0;
	$effect(() => {
		const e = expr();
		const mine = ++seq;
		if (!e) {
			parsed = null;
			return;
		}
		api
			.parseQuery(e, stored)
			.then((r) => {
				if (mine === seq) parsed = r;
			})
			.catch((err) => {
				if (mine === seq) parsed = { ok: false, error: { code: 'error', message: toMessage(err), start: 0, end: 0 } };
			});
	});
	const chips = $derived(parsed?.ok ? toChips(parsed.tree) : null);
	return {
		get parsed() {
			return parsed;
		},
		get chips() {
			return chips;
		},
		/** A refused query runs nothing. */
		get refused() {
			return expr() && parsed && !parsed.ok ? parsed.error : null;
		},
		/** How many entries the chips row shows. */
		get count() {
			return chips ? (chips.kind === 'advanced' ? 1 : chips.items.length) : 0;
		},
		/** The expr left after removing the node at path. */
		without(path: number[]) {
			return parsed?.ok ? exprWithout(parsed.tree, path) || undefined : expr();
		}
	};
}
