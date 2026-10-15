<script lang="ts">
	// Full-page gallery grid modal (HOLODEX-174): a read-only browse surface over a
	// person's full gallery, opened from PersonGallery's "Gallery (N)" trigger.
	// Modal (shared/Modal.svelte) owns the shell — backdrop/X close, focus trap, focus
	// return, and Escape. No owner editing here (promote/move/delete stay in the inline
	// row) — this is browse-only. Modal's stack routes a single Escape press to the
	// topmost of this modal and a stacked image viewer rather than closing both at once.
	// Tokens only; QA Cinémathèque.
	import { onMount } from 'svelte';
	import { api } from '#lib/api.ts';
	import type { PersonImage } from '#lib/types.ts';
	import Modal from '#lib/components/shared/Modal.svelte';

	let {
		personId,
		name,
		items,
		onclose,
		onselect
	}: {
		personId: number;
		name: string;
		items: PersonImage[];
		onclose: () => void;
		// Opens the image viewer at gallery index i.
		onselect: (i: number) => void;
	} = $props();

	let dialogEl = $state<HTMLElement | null>(null);

	onMount(() => {
		// The header's own Close button is the first <button> in DOM order — scope to
		// the grid so initial focus lands on the first grid item, per spec.
		dialogEl?.querySelector<HTMLElement>('ul button')?.focus();
	});

	function thumbSrc(img: PersonImage): string {
		return api.personGalleryImageURL(personId, img.id, { version: img.version });
	}
</script>

<Modal titleId="gallery-modal-title" {onclose} size="media" align="center" bind:panel={dialogEl}>
	<div class="flex min-h-0 flex-1 flex-col p-4">
		<div class="mb-3 flex items-start justify-between gap-3">
			<h2 id="gallery-modal-title" class="skin-title text-lg font-semibold text-ink">
				{name} — Gallery <span class="text-sm font-normal text-muted">({items.length})</span>
			</h2>
			<button
				onclick={onclose}
				aria-label="Close"
				class="rounded-theme px-2 py-0.5 text-muted hover:text-ink"
			>
				✕
			</button>
		</div>

		<ul
			class="grid grid-cols-3 gap-3 overflow-y-auto sm:grid-cols-4 md:grid-cols-6"
		>
			{#each items as img, i (img.id)}
				<li>
					<button
						onclick={() => onselect(i)}
						aria-label={`${name} — gallery image ${i + 1}`}
						class="block aspect-[3/4] w-full overflow-hidden rounded-theme border border-rule bg-surface-2"
					>
						<img
							src={thumbSrc(img)}
							alt={`${name} — gallery image ${i + 1}`}
							loading="lazy"
							decoding="async"
							class="h-full w-full object-contain"
						/>
					</button>
				</li>
			{/each}
		</ul>
	</div>
</Modal>
