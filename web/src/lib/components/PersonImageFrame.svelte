<script lang="ts">
	// Shared person-image frame (F25, ADR-038) backing PersonAvatar/Banner/Poster.
	// Builds the role-serving URL with the active skin (so the backend's empty-slot
	// placeholder matches the current skin) and the `?v=` cache-buster. The backend
	// always returns a usable image (real or themed placeholder), so an `img` error
	// just keeps the framed well — never a broken-image glyph. The caller supplies the
	// aspect/size via `frameClass` (e.g. "portrait-frame--1x1 w-20").
	import { api } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import type { PersonImageRole } from '$lib/types';

	let {
		personId,
		role,
		name,
		frameClass,
		version,
		eager = false
	}: {
		personId: number;
		role: PersonImageRole;
		name: string;
		frameClass: string;
		version?: number;
		eager?: boolean;
	} = $props();

	// The URL re-derives when the skin flips so the placeholder re-themes live.
	const src = $derived(api.personImageURL(personId, role, { version, skin: theme.current }));

	let loaded = $state(false);
</script>

<div class="portrait-frame {frameClass}">
	<img
		{src}
		alt={name}
		loading={eager ? 'eager' : 'lazy'}
		decoding="async"
		class={loaded ? 'is-loaded' : ''}
		onload={() => (loaded = true)}
		onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')}
	/>
</div>
