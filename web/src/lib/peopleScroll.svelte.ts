// People-list scroll snapshot (mirrors the browse grid, ADR-032). The list page is
// destroyed on navigating into a person and re-created on Back; SvelteKit resets scroll
// to top for in-app (push) navigation, so we stash the scroll offset keyed by sort (a
// sort change reorders the list, invalidating it) and restore it once the re-fetched
// list paints. The list isn't paginated, so the snapshot carries only the offset — the
// keyed one-shot mechanics are shared via navSnapshot.
import { createNavSnapshot, type Keyed } from '$lib/navSnapshot.svelte';

export interface PeopleScrollSnapshot extends Keyed {
	scrollY: number;
}

export const peopleScroll = createNavSnapshot<PeopleScrollSnapshot>();
