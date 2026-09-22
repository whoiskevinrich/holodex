// Shared refresh state for CompletenessRing (F65.8, HOLODEX-435), keyed by entity.
// The same entity can be mounted more than once on a page — the landing page's
// shelves repeat the grid's videos — so busy and the post-refresh bands live here,
// not in the component instance: every ring for the entity spins together, and
// every one redraws to the re-read bands, not just the one that was pressed.
import type { CompletenessSummary, EnrichEntityKind } from '$lib/types';

export interface RingRefreshState {
	busy: boolean;
	/** The list bands the refresh replaced — a later list fetch with different
	 * bands wins again, so `bands` only applies while the props still match. */
	over?: CompletenessSummary;
	bands?: CompletenessSummary;
}

export const ringRefresh = $state<Record<string, RingRefreshState>>({});

export function ringKey(entity: { kind: EnrichEntityKind; id: number }): string {
	return `${entity.kind}:${entity.id}`;
}

/** The bands a ring should draw: the shared post-refresh bands while the list
 * props are still the ones that refresh replaced, else the props. */
export function currentBands(state: RingRefreshState | undefined, props: CompletenessSummary): CompletenessSummary {
	if (state?.over && state.bands && state.over.required === props.required && state.over.extras === props.extras) {
		return state.bands;
	}
	return props;
}
