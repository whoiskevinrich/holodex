// Films row (F56, design handoff §5): shared best-effort fetch of an entity's inherited
// film union, used by the Person/Studio/Tag detail pages' FilmsRow footer — a non-fatal
// fetch alongside each page's own main load, separate from the getPerson/getStudio/getTag
// response. Re-runs on both films_enabled becoming available and id changing (navigating
// between entities of the same type). The three pages differ only in which filter key
// they pass to listFilms.
import { api } from './api';
import { activity } from './activity.svelte';
import type { Film } from './types';

export function filmsRow(entityId: () => number, filterKey: 'personId' | 'studioId' | 'tagId') {
	let films = $state<Film[]>([]);
	let requestId = 0;
	$effect(() => {
		if (activity.caps?.films_enabled) {
			const id = ++requestId;
			api
				.listFilms({ [filterKey]: entityId() })
				.then((res) => {
					if (id === requestId) films = res.items ?? [];
				})
				.catch(() => {});
		}
	});
	return {
		get films() {
			return films;
		}
	};
}
