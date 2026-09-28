// One list page's live state, wired to the ADR-114 contract in ./listState.
//
// SvelteKit's replaceState is shallow: it rewrites the address bar but leaves `page.url`
// as it was, so a page can't simply derive its state from `page.url`. Instead the state
// lives here, and is re-parsed from the URL on every real navigation into the page —
// arrival, Back, and a same-route nav click (`/people` while on `/people?…`), which must
// clear the filters (D2). `afterNavigate` fires for those and not for our own commits.
//
// Pages call `listController()` during component init: afterNavigate and the popstate
// listener both register against the component.
import { afterNavigate } from '$app/navigation';
import {
	chooseSort,
	commit,
	parseList,
	serialize,
	stateKey,
	type ListSchema,
	type ListState
} from './listState';

export class ListController<S extends string, Q> {
	state: ListState<S, Q> = $state() as ListState<S, Q>;
	#owner = false;

	constructor(
		private schema: ListSchema<S, Q>,
		private path: string
	) {
		this.#arrive(new URLSearchParams(location.search));
		afterNavigate(({ to }) => {
			if (to?.url.pathname === path) this.#arrive(to.url.searchParams);
		});
	}

	/**
	 * Back/Forward to an entry on this same page is invisible to afterNavigate: our
	 * replaceState writes are shallow, so SvelteKit records the entry's page URL as the one
	 * it originally loaded and treats the pop as a shallow state change — no navigation, no
	 * hook. This re-reads the address bar instead. Returns the unsubscribe; pages get it
	 * wired by `listController()` below.
	 */
	listen(): () => void {
		const onpop = () => {
			if (location.pathname === this.path) this.#arrive(new URLSearchParams(location.search));
		};
		window.addEventListener('popstate', onpop);
		return () => window.removeEventListener('popstate', onpop);
	}

	/** The canonical view key for ADR-032 snapshots (D5). */
	get key(): string {
		return stateKey(this.schema, this.state);
	}

	/**
	 * Owner capabilities load asynchronously; when they resolve the sort is re-resolved
	 * from the current URL so an owner-only sort in a link or preference applies.
	 */
	setOwner(owner: boolean): void {
		if (owner === this.#owner) return;
		this.#owner = owner;
		this.#arrive(new URLSearchParams(location.search));
	}

	/** The sort control's change handler — the only place the preference is written (D3). */
	setSort(sort: S): void {
		chooseSort(this.schema, sort);
		this.#update({ ...this.state, sort });
	}

	setQuery(query: Q): void {
		this.#update({ ...this.state, query });
	}

	#arrive(params: URLSearchParams): void {
		const { state, syncUrl } = parseList(this.schema, params, this.#owner);
		this.state = state;
		if (syncUrl) commit(this.path, serialize(this.schema, state));
	}

	#update(next: ListState<S, Q>): void {
		this.state = next;
		commit(this.path, serialize(this.schema, next));
	}
}

/** What a list page calls during init: the controller, with its popstate listener tied to the component. */
export function listController<S extends string, Q>(schema: ListSchema<S, Q>, path: string): ListController<S, Q> {
	const c = new ListController(schema, path);
	$effect(() => c.listen());
	return c;
}
