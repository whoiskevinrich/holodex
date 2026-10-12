// The run strip's jump list (spec RD14 / P0-13, design run-position-jump-handoff.md): what
// the query box means, which rows a title search keeps, and which rows a virtualised list
// draws. Pure, so the popover is just wiring.

/** A row is h-11; the list shows 8½ of them so the half row says there's more. */
export const ROW_PX = 44;
export const VISIBLE_ROWS = 8.5;
/** Rows drawn beyond the viewport each side, so a fast wheel doesn't show blanks. */
const BUFFER = 6;

export type JumpQuery =
	| { kind: 'empty' }
	/** A 0-based index into the run's order. */
	| { kind: 'number'; index: number }
	| { kind: 'out-of-range' }
	| { kind: 'words'; text: string };

/** Digits only is a position (1-based on screen); anything else searches titles. */
export function parseJumpQuery(raw: string, total: number): JumpQuery {
	const q = raw.trim();
	if (!q) return { kind: 'empty' };
	if (/^\d+$/.test(q)) {
		const n = Number(q);
		return n >= 1 && n <= total ? { kind: 'number', index: n - 1 } : { kind: 'out-of-range' };
	}
	return { kind: 'words', text: q.toLowerCase() };
}

/** The run indices whose title contains `text` (already lowercased), in run order. */
export function matchIndices(
	order: readonly number[],
	titleOf: (id: number) => string | undefined,
	text: string
): number[] {
	const out: number[] = [];
	order.forEach((id, i) => {
		if (titleOf(id)?.toLowerCase().includes(text)) out.push(i);
	});
	return out;
}

/** Split a title around its first match so the match can be underlined. */
export function splitMatch(title: string, text: string): [string, string, string] {
	const at = text ? title.toLowerCase().indexOf(text) : -1;
	if (at < 0) return [title, '', ''];
	return [title.slice(0, at), title.slice(at, at + text.length), title.slice(at + text.length)];
}

/** The slice of rows to draw, [start, end), for a list scrolled to `scrollTop`. */
export function windowRange(scrollTop: number, viewportPx: number, count: number): [number, number] {
	const start = Math.max(0, Math.floor(scrollTop / ROW_PX) - BUFFER);
	const end = Math.min(count, Math.ceil((scrollTop + viewportPx) / ROW_PX) + BUFFER);
	return [start, Math.max(start, end)];
}

/** The scrollTop that puts row `row` in the middle, clamped to the list's ends. */
export function centredScrollTop(row: number, viewportPx: number, count: number): number {
	const max = Math.max(0, count * ROW_PX - viewportPx);
	return Math.min(max, Math.max(0, row * ROW_PX + ROW_PX / 2 - viewportPx / 2));
}

/** The list's height: 8½ rows, or fewer when the run is shorter. */
export function viewportPx(count: number): number {
	return Math.min(count, VISIBLE_ROWS) * ROW_PX;
}
