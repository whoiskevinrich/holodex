// F87 query language, client side (docs/specs/query-language.md). The grammar is parsed
// only on the server (browse-and-list-state.md, "Boolean queries"): this module keeps a
// lexer that tells nav search whether it is in query mode and which field the caret is
// in, the chip model drawn from GET /query/parse's display tree, and a printer that
// rejoins that tree's own canonical fragments after a chip is removed.

/** One node of /query/parse's display tree (internal/query DisplayNode). */
export interface QueryNode {
	kind: 'or' | 'and' | 'not' | 'cond' | 'word';
	expr: string;
	items?: QueryNode[];
	field?: string;
	op?: string;
	value?: string;
	id?: number;
	lo?: number;
	hi?: number;
	label?: string;
	/** A stored definition's entity that no longer exists; label is `#id` (F89). */
	missing?: boolean;
}

/** A refused query (internal/query Error). start/end are UTF-16 offsets into the text. */
export interface QueryError {
	code: string;
	message: string;
	start: number;
	end: number;
	field?: string;
	suggestions?: string[];
	candidates?: { id: number; name: string }[];
}

export interface ParsedQuery {
	canonical: string;
	tree: QueryNode;
}

/** api.parseQuery's answer: the parse, or the refusal with its offsets. */
export type QueryParseResult = ({ ok: true } & ParsedQuery) | { ok: false; error: QueryError };

/** The fields a condition may name; `matched` is the owner's only. */
export const QUERY_FIELDS = ['tag', 'person', 'studio', 'category', 'duration', 'year', 'added', 'resolution', 'playlist', 'matched'] as const;
export const OWNER_ONLY_FIELDS: readonly string[] = ['matched'];

/** Fields offered while typing, for this reader: a visitor is never offered `matched` (P0-5). */
export function fieldsFor(isOwner: boolean): string[] {
	return QUERY_FIELDS.filter((f) => isOwner || !OWNER_ONLY_FIELDS.includes(f));
}

const FIELD_TERM = /^-?\p{L}[\p{L}_]*:/u;

interface Term {
	text: string;
	start: number;
	end: number;
}

/** Splits text into terms the way the server's lexer does: whitespace and parentheses
 * separate, a quoted run stays whole (an unclosed quote runs to the end). */
function terms(text: string): Term[] {
	const out: Term[] = [];
	let i = 0;
	while (i < text.length) {
		const c = text[i];
		if (/\s/.test(c)) {
			i++;
			continue;
		}
		if (c === '(' || c === ')') {
			out.push({ text: c, start: i, end: i + 1 });
			i++;
			continue;
		}
		const start = i;
		while (i < text.length && !/\s/.test(text[i]) && text[i] !== '(' && text[i] !== ')') {
			if (text[i] === '"') {
				const close = text.indexOf('"', i + 1);
				i = close < 0 ? text.length : close + 1;
				continue;
			}
			i++;
		}
		out.push({ text: text.slice(start, i), start, end: i });
	}
	return out;
}

/** True once the text holds a field condition, an operator or a parenthesis (P0-4):
 * nav search then treats it as a video query instead of today's live search. */
export function isQueryMode(text: string): boolean {
	return terms(text).some(
		(t) =>
			t.text === '(' ||
			t.text === ')' ||
			t.text === 'OR' ||
			t.text === 'AND' ||
			t.text === 'NOT' ||
			FIELD_TERM.test(t.text) ||
			(t.text.length > 1 && t.text.startsWith('-'))
	);
}

/** What the caret is in, for suggestions: a field name being typed, a value after
 * `field:` (quotes stripped from the partial value), or nothing. start/end span the
 * part a suggestion replaces. */
export type CaretContext =
	| { kind: 'field'; prefix: string; start: number; end: number }
	| { kind: 'value'; field: string; prefix: string; start: number; end: number }
	| { kind: 'none' };

export function caretContext(text: string, caret: number): CaretContext {
	const t = terms(text).find((x) => x.start < caret && caret <= x.end);
	if (!t || t.text === '(' || t.text === ')') return { kind: 'none' };
	const neg = t.text.startsWith('-') ? 1 : 0;
	const body = t.text.slice(neg, caret - t.start);
	const colon = body.indexOf(':');
	if (colon < 0) {
		// A bare word or a closed quoted phrase: offer field names only for a plain word.
		if (body.includes('"') || !/^\p{L}[\p{L}_]*$/u.test(body)) return { kind: 'none' };
		return { kind: 'field', prefix: body.toLowerCase(), start: t.start + neg, end: t.end };
	}
	const field = body.slice(0, colon).toLowerCase();
	if (!/^\p{L}[\p{L}_]*$/u.test(field)) return { kind: 'none' };
	let value = body.slice(colon + 1);
	if (value.startsWith('"')) {
		if (value.length > 1 && value.endsWith('"')) return { kind: 'none' }; // after a closing quote
		value = value.slice(1);
	}
	return { kind: 'value', field, prefix: value, start: t.start + neg + colon + 1, end: t.end };
}

/** Quotes a value that would not stay one term. */
export function quoteValue(v: string): string {
	return /[\s():]/.test(v) || v.startsWith('-') ? `"${v.replaceAll('"', '')}"` : v;
}

/** Replaces [start, end) with insert and a trailing space; returns the text and caret. */
export function insertAt(text: string, start: number, end: number, insert: string): { text: string; caret: number } {
	const after = text.slice(end).replace(/^\S*/, '');
	const spaced = after.startsWith(' ') ? insert : insert + ' ';
	return { text: text.slice(0, start) + spaced + after, caret: start + spaced.length };
}

/** The error echo's three runs (before, bad part, after), sliced at the server's UTF-16
 * offsets. Rendered as three text nodes, never as HTML (P0-7, the perimeter). */
export function echoParts(text: string, err: Pick<QueryError, 'start' | 'end'>): [string, string, string] {
	const s = Math.max(0, Math.min(err.start, text.length));
	const e = Math.max(s, Math.min(err.end, text.length));
	return [text.slice(0, s), text.slice(s, e), text.slice(e)];
}

// ---- chips ----

/** A condition drawn as one chip. path locates its node in the display tree. */
export interface ChipCond {
	label: string;
	negated: boolean;
	path: number[];
	/** The condition's field and id, for marking the chip a refusal names (F89). */
	field?: string;
	id?: number;
	/** Draws the key glyph; absent otherwise. */
	ownerOnly?: boolean;
	missing?: boolean;
}

/** One entry in the chips row. A group is one OR level whose members are a chip or an
 * AND set of chips (the approved `( a + b ) or c` box). */
export type ChipItem = { kind: 'chip'; chip: ChipCond } | { kind: 'group'; members: ChipCond[][]; path: number[] };

export type ChipModel = { kind: 'chips'; items: ChipItem[] } | { kind: 'advanced'; text: string };

function fmtSeconds(s: number): string {
	if (s > 0 && s % 3600 === 0) return `${s / 3600} h`;
	if (s > 0 && s % 60 === 0) return `${s / 60} min`;
	return `${s} s`;
}

const OP_TEXT: Record<string, string> = { '<': '<', '<=': '≤', '>': '>', '>=': '≥' };

/** A condition's chip text, from the server's current names (never the typed spelling). */
export function condLabel(n: QueryNode): string {
	if (n.kind === 'word') return `${n.label} · title or Tag`;
	const name = n.label ?? n.value ?? '';
	switch (n.field) {
		case 'tag':
		case 'person':
			return name;
		case 'studio':
			return `Studio: ${name}`;
		case 'category':
			return `Category: ${name}`;
		case 'resolution':
			return `Resolution: ${n.value}`;
		case 'matched':
			return n.value === 'any' ? 'matched' : `matched: ${name}`;
		case 'added': // the server words it: "last 30 days", "2026-01 to 2026-06"
			return `added: ${name}`;
		case 'playlist':
			return `in: ${name}`;
		case 'duration':
		case 'year': {
			const fmt = n.field === 'duration' ? fmtSeconds : String;
			const title = n.field === 'duration' ? 'Duration' : 'Year';
			const lo = n.lo ?? 0;
			if (n.op === '..') return `${title} ${fmt(lo)} – ${fmt(n.hi ?? 0)}`;
			if (n.op && n.op !== '=') return `${title} ${OP_TEXT[n.op] ?? n.op} ${fmt(lo)}`;
			return `${title} ${fmt(lo)}`;
		}
	}
	return name;
}

const isLeaf = (n: QueryNode) => n.kind === 'cond' || n.kind === 'word';

/** A leaf or a NOT over a leaf as a chip, else null. */
function chipOf(n: QueryNode, path: number[]): ChipCond | null {
	const facts = (x: QueryNode) => ({
		...(x.field ? { field: x.field } : {}),
		...(x.id ? { id: x.id } : {}),
		...(x.field && OWNER_ONLY_FIELDS.includes(x.field) ? { ownerOnly: true } : {}),
		...(x.missing ? { missing: true } : {})
	});
	if (isLeaf(n)) return { label: condLabel(n), negated: false, path, ...facts(n) };
	if (n.kind === 'not' && n.items && isLeaf(n.items[0])) {
		return { label: `not ${condLabel(n.items[0])}`, negated: true, path, ...facts(n.items[0]) };
	}
	return null;
}

/** An OR group the chips can draw: every member is a chip or an AND set of chips. */
function groupOf(n: QueryNode, path: number[]): ChipItem | null {
	const members: ChipCond[][] = [];
	for (const [i, m] of (n.items ?? []).entries()) {
		const single = chipOf(m, [...path, i]);
		if (single) {
			members.push([single]);
			continue;
		}
		if (m.kind !== 'and') return null;
		const set: ChipCond[] = [];
		for (const [j, x] of (m.items ?? []).entries()) {
			const c = chipOf(x, [...path, i, j]);
			if (!c) return null;
			set.push(c);
		}
		members.push(set);
	}
	return { kind: 'group', members, path };
}

/** The chips row for a query (P0-6): plain and NOT chips, OR groups one level deep, or —
 * for anything deeper — the Advanced query row alone, never a partial chip set. */
export function toChips(tree: QueryNode): ChipModel {
	const top = tree.kind === 'and' ? (tree.items ?? []).map((n, i) => [n, [i]] as const) : [[tree, []] as const];
	const items: ChipItem[] = [];
	for (const [n, path] of top) {
		const chip = chipOf(n, [...path]);
		if (chip) {
			items.push({ kind: 'chip', chip });
			continue;
		}
		const group = n.kind === 'or' ? groupOf(n, [...path]) : null;
		if (!group) return { kind: 'advanced', text: tree.expr };
		items.push(group);
	}
	return { kind: 'chips', items };
}

/** Prints a display tree from its nodes' canonical fragments, with the server printer's
 * parenthesis rules (an OR inside an AND, and any composite under a NOT). */
export function printTree(n: QueryNode, group = false): string {
	switch (n.kind) {
		case 'and': {
			const s = (n.items ?? []).map((x) => printTree(x, x.kind === 'or')).join(' ');
			return group ? `(${s})` : s;
		}
		case 'or': {
			const s = (n.items ?? []).map((x) => printTree(x)).join(' OR ');
			return group ? `(${s})` : s;
		}
		case 'not': {
			const x = n.items![0];
			return '-' + printTree(x, x.kind === 'and' || x.kind === 'or');
		}
		default:
			return n.expr;
	}
}

const ENTITY_FIELDS = new Set(['tag', 'person', 'studio', 'category', 'playlist']);

/** printTree with each entity named by its current name instead of its id: the text
 * Edit as text puts in the nav box (the server re-binds it on the way back). */
export function printNames(n: QueryNode, group = false): string {
	const kids = n.items ?? [];
	switch (n.kind) {
		case 'and': {
			const s = kids.map((x) => printNames(x, x.kind === 'or')).join(' ');
			return group ? `(${s})` : s;
		}
		case 'or': {
			const s = kids.map((x) => printNames(x)).join(' OR ');
			return group ? `(${s})` : s;
		}
		case 'not':
			return '-' + printNames(kids[0], kids[0].kind === 'and' || kids[0].kind === 'or');
		case 'cond':
			if (n.field && ENTITY_FIELDS.has(n.field) && n.label && !n.missing) {
				const v = quoteValue(n.label);
				return `${n.field}:${/^\d+$/.test(v) ? `"${v}"` : v}`;
			}
			return n.expr;
		default:
			return n.expr;
	}
}

/** Removes the node at path; an AND or OR left with one item becomes that item, one left
 * with none disappears. Returns null when nothing is left. */
export function removeAt(tree: QueryNode, path: number[]): QueryNode | null {
	if (path.length === 0) return null;
	const [i, ...rest] = path;
	const items = [...(tree.items ?? [])];
	const kept = removeAt(items[i], rest);
	if (kept) items[i] = kept;
	else items.splice(i, 1);
	if (tree.kind === 'not') return kept ? { ...tree, items: [kept] } : null;
	if (items.length === 0) return null;
	if (items.length === 1) return items[0];
	return { ...tree, items };
}

/** The `expr` left after removing the node at path; '' when the query is now empty. */
export function exprWithout(tree: QueryNode, path: number[]): string {
	const left = removeAt(tree, path);
	return left ? printTree(left) : '';
}

/** Spoken label for a group (accessibility notes): "science-fiction and fantasy, or action". */
export function groupLabel(members: ChipCond[][]): string {
	return members.map((set) => set.map((c) => c.label).join(' and ')).join(', or ');
}
