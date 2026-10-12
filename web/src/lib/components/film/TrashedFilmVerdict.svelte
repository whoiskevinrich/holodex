<script lang="ts">
	// In-Trash verdict (F78 RD6, design handoff §E): a rename, year edit or create landed
	// on the name and year of a film in Trash. That film still holds its key, has no page
	// to link to and can't be merged into, so the one way forward is to restore it. The
	// card shell is the film page's year-collision card; the caller decides what a
	// restore leads to (go to the film, or pick it as the attach target).
	import { api } from '#lib/api.ts';
	import { refLabel, toMessage } from '#lib/format.ts';
	import type { Film, FilmYearCollision } from '#lib/types.ts';

	let {
		trashed,
		onrestored,
		oncancel
	}: {
		trashed: FilmYearCollision;
		onrestored: (film: Film) => void;
		oncancel: () => void;
	} = $props();

	let busy = $state(false);
	let error = $state('');

	const label = $derived(refLabel({ name: trashed.film_name, year: trashed.year }));

	async function restore() {
		busy = true;
		error = '';
		try {
			const { film } = await api.restoreFilm(trashed.film_id);
			onrestored(film);
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="mt-2 space-y-2 rounded-theme border border-rule bg-surface p-3" aria-live="polite">
	<p class="text-sm text-ink"><span class="font-semibold">{label}</span> is in Trash.</p>
	<p class="text-xs text-muted">A film in Trash keeps its name and year until it's purged.</p>
	<div class="flex flex-wrap gap-2">
		<button type="button" onclick={restore} disabled={busy} class="btn-accent px-3 py-1.5 text-sm">
			Restore that film
		</button>
		<button type="button" onclick={oncancel} class="btn-quiet px-3 py-1.5 text-sm">Cancel</button>
	</div>
	{#if error}
		<p class="text-sm text-warn">{error}</p>
	{/if}
</div>
