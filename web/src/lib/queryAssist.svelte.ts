// Nav search query mode (F87, query-language-handoff.md "States and interactions"): the
// debounced server parse that drives the error echo, the suggestions for the field under
// the caret, and the highlighted row. The nav box owns `queryAssist`; a page-owned field
// (sort/QueryField, the playlist editor) makes its own, so it never touches the nav box's.
import { api } from './api';
import { toMessage } from './format';
import { caretContext, fieldsFor, insertAt, quoteValue, type QueryError, type QueryFieldInfo, type QueryParseResult } from './query';
import { matchedAlias } from './tagInput';

/** One suggestion row: `label` is shown, `insert` replaces [start, end). */
export interface QuerySuggestion {
	label: string;
	sub?: string;
	insert: string;
	start: number;
	end: number;
	/** A field name keeps the caret after its colon, no trailing space. */
	field?: boolean;
}

const PARSE_MS = 250;
const SUGGEST_MS = 150;
const CAP = 8;

const DURATION_HINTS = ['<2m', '>1h', '20m..1h'];
const DATE_HINTS = ['<30d', '<6mo', '>1y', '2026-01..2026-06'];
const RESOLUTIONS = ['SD', 'HD', 'FHD', '4K'];
const HEADINGS: Record<string, string> = {
	tag: 'Tags',
	person: 'People',
	studio: 'Studios',
	category: 'Categories',
	resolution: 'Resolution',
	duration: 'Duration',
	added: 'Added',
	updated: 'Updated',
	released: 'Released',
	playlist: 'Playlists',
	matched: 'Providers'
};

export class QueryAssist {
	parse = $state<QueryParseResult | null>(null);
	/** The text `parse` answered for: its offsets slice this text, not a newer one. */
	parsedText = $state('');
	suggestions = $state<QuerySuggestion[]>([]);
	highlight = $state(-1);
	/** The group header over the rows. */
	heading = $state('');
	/** Suggestions are "Did you mean" fixes for the parse error, not values. */
	fixing = $state(false);

	private parseTimer: ReturnType<typeof setTimeout> | undefined;
	private suggestTimer: ReturnType<typeof setTimeout> | undefined;
	private parseSeq = 0;
	private suggestSeq = 0;
	private providers: string[] | null = null;
	private playlists: string[] | null = null;
	private mapped: QueryFieldInfo[] | null = null;

	/** The parse error for text, or null while text's own parse hasn't answered. */
	errorFor(text: string): QueryError | null {
		return this.parsedText === text && this.parse && !this.parse.ok ? this.parse.error : null;
	}

	/** Every keystroke or caret move in query mode. */
	update(text: string, caret: number, isOwner: boolean) {
		clearTimeout(this.parseTimer);
		this.parseTimer = setTimeout(() => void this.runParse(text, caret, isOwner), PARSE_MS);
		clearTimeout(this.suggestTimer);
		this.suggestTimer = setTimeout(() => void this.suggest(text, caret, isOwner), SUGGEST_MS);
	}

	/** Parses now (Enter): the parse the caller acts on, never a stale one. */
	async parseNow(text: string): Promise<QueryParseResult> {
		clearTimeout(this.parseTimer);
		const seq = ++this.parseSeq;
		let r: QueryParseResult;
		try {
			r = await api.parseQuery(text);
		} catch (e) {
			r = { ok: false, error: { code: 'error', message: toMessage(e), start: 0, end: 0 } };
		}
		if (seq === this.parseSeq) {
			this.parse = r;
			this.parsedText = text;
		}
		return r;
	}

	private async runParse(text: string, caret: number, isOwner: boolean) {
		await this.parseNow(text);
		const err = this.errorFor(text);
		// An unknown or ambiguous name offers its fixes in place of value suggestions.
		if (err && (err.suggestions?.length || err.candidates?.length)) {
			this.suggestSeq++;
			this.fixing = true;
			this.highlight = -1;
			this.heading = err.candidates?.length ? 'Which one' : 'Did you mean';
			this.suggestions = err.candidates?.length
				? err.candidates.map((c) => ({ label: c.name, sub: `id ${c.id}`, insert: String(c.id), start: err.start, end: err.end }))
				: (err.suggestions ?? []).map((s) => ({ label: s, insert: quoteValue(s), start: err.start, end: err.end }));
		} else if (this.fixing) {
			this.fixing = false;
			void this.suggest(text, caret, isOwner);
		}
	}

	private async suggest(text: string, caret: number, isOwner: boolean) {
		const seq = ++this.suggestSeq;
		const ctx = caretContext(text, caret);
		let rows: QuerySuggestion[] = [];
		let heading = ctx.kind === 'field' ? 'Fields' : ctx.kind === 'value' ? (HEADINGS[ctx.field] ?? '') : '';
		const at = (label: string, insert = quoteValue(label), sub?: string): QuerySuggestion =>
			ctx.kind === 'none' ? { label, insert, start: 0, end: 0 } : { label, sub, insert, start: ctx.start, end: ctx.end };
		try {
			if (ctx.kind === 'field') {
				this.mapped ??= (await api.queryFields()).fields;
				rows = fieldsFor(isOwner, this.mapped.map((f) => f.field))
					.filter((f) => f.startsWith(ctx.prefix))
					.map((f) => ({ ...at(`${f}:`, `${f}:`), field: true }));
			} else if (ctx.kind === 'value') {
				const p = ctx.prefix.trim();
				switch (ctx.field) {
					case 'tag':
						if (p) {
							const tags = (await api.search(p)).tags ?? [];
							rows = tags.map((t) => {
								const alias = matchedAlias(t, p);
								const count = t.video_count != null ? String(t.video_count) : '';
								return at(t.name, undefined, [alias ? `alias: ${alias}` : '', count].filter(Boolean).join(' · '));
							});
						}
						break;
					case 'person':
						if (p) rows = ((await api.search(p)).people ?? []).map((x) => at(x.name));
						break;
					case 'studio':
						if (p) rows = ((await api.search(p)).studios ?? []).map((x) => at(x.name));
						break;
					case 'category':
						rows = (await api.listCategories({ q: p || undefined })).items.map((c) => at(c.name));
						break;
					case 'resolution':
						rows = RESOLUTIONS.filter((r) => r.toLowerCase().startsWith(p.toLowerCase())).map((r) => at(r));
						break;
					case 'duration':
						rows = DURATION_HINTS.filter((d) => d.startsWith(p)).map((d) => at(d));
						break;
					case 'added':
					case 'updated':
					case 'released':
						rows = DATE_HINTS.filter((d) => d.startsWith(p)).map((d) => at(d));
						break;
					case 'playlist':
						// The server lists what this reader may name: a visitor, public ones only.
						// Read once per query session: the list re-counts every smart playlist.
						this.playlists ??= (await api.listPlaylists()).items.map((x) => x.name);
						rows = this.playlists.filter((x) => x.toLowerCase().includes(p.toLowerCase())).map((x) => at(x));
						break;
					case 'matched':
						if (isOwner) {
							this.providers ??= (await api.providers()).providers.map((x) => x.name);
							rows = ['any', ...this.providers].filter((x) => x.startsWith(p.toLowerCase())).map((x) => at(x));
						}
						break;
					default: {
						this.mapped ??= (await api.queryFields()).fields;
						const info = this.mapped.find((f) => f.field === ctx.field);
						heading = info?.label ?? '';
						if (info?.kind === 'date') rows = DATE_HINTS.filter((d) => d.startsWith(p)).map((d) => at(d));
						else if (info && info.kind !== 'number') rows = (await api.queryValues(info.field, p)).values.map((x) => at(x));
					}
				}
			}
		} catch {
			rows = []; // a failed lookup offers nothing; the parse still reports any error
		}
		if (seq !== this.suggestSeq) return;
		this.fixing = false;
		this.heading = heading;
		this.suggestions = rows.slice(0, CAP);
		this.highlight = -1;
	}

	move(delta: number) {
		const n = this.suggestions.length;
		if (!n) return;
		this.highlight = this.highlight < 0 ? (delta > 0 ? 0 : n - 1) : (this.highlight + delta + n) % n;
	}

	/** Applies the highlighted (or first) suggestion to text; null when there is none. */
	accept(text: string, index = this.highlight): { text: string; caret: number } | null {
		const s = this.suggestions[index < 0 ? 0 : index];
		if (!s) return null;
		if (s.field) {
			return { text: text.slice(0, s.start) + s.insert + text.slice(s.end), caret: s.start + s.insert.length };
		}
		return insertAt(text, s.start, s.end, s.insert);
	}

	reset() {
		clearTimeout(this.parseTimer);
		clearTimeout(this.suggestTimer);
		this.parseSeq++;
		this.suggestSeq++;
		this.playlists = null;
		this.mapped = null;
		this.parse = null;
		this.parsedText = '';
		this.suggestions = [];
		this.highlight = -1;
		this.fixing = false;
	}
}

export const queryAssist = new QueryAssist();
