// Owner-gated fetch of the "Missing facet" filter's option list (F55.6) — shared
// by the video/people/studio browse pages, which each needed the exact same
// fetch-once effect. `fetched` is a plain (non-$state) guard: the API can
// legitimately return an empty facet list, and assigning a fresh empty array on
// every response is itself a $state write, so gating the fetch on
// `options.length === 0` would refire it forever once isOwner resolves true.
import { api } from '$lib/api';
import type { FacetSummary } from '$lib/types';

export function createMissingFacetOptions(entityType: 'video' | 'person' | 'studio') {
	let options = $state<FacetSummary[]>([]);
	let fetched = false;

	return {
		get options() {
			return options;
		},
		// Call from the page's own $effect, reading `isOwner` there so this stays
		// reactive to the caller's derived value without this module owning an effect.
		ensureFetched(isOwner: boolean) {
			if (!isOwner || fetched) return;
			fetched = true;
			api
				.completenessFacets(entityType)
				.then((r) => (options = r.facets ?? []))
				.catch(() => (fetched = false));
		}
	};
}
