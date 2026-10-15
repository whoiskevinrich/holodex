<script lang="ts">
	// Full-page image viewer modal (HOLODEX-174), opened by clicking/Entering any
	// gallery thumbnail (inline row or grid modal). Modal (shared/Modal.svelte) owns the
	// shell, focus trap/return and Escape; its stack gives the darker stacked backdrop
	// when this opens over the grid modal, so it reads "on top". Prev/next with no
	// wraparound, a position counter, and a fit-to-modal image that never upscales past
	// native resolution. The arrow keys are handled by the parent (PersonGallery), not
	// this component. Tokens only; QA Cinémathèque.
	import { onMount, tick } from 'svelte';
	import { api } from '#lib/api.ts';
	import type { PersonImage } from '#lib/types.ts';
	import Modal from '#lib/components/shared/Modal.svelte';

	let {
		personId,
		name,
		items,
		index,
		onclose,
		onnavigate
	}: {
		personId: number;
		name: string;
		items: PersonImage[];
		index: number;
		onclose: () => void;
		onnavigate: (i: number) => void;
	} = $props();

	let closeBtn = $state<HTMLButtonElement | null>(null);
	let imgEl = $state<HTMLImageElement | null>(null);
	let loaded = $state(false);

	const current = $derived(items[index]);

	onMount(() => {
		closeBtn?.focus();
	});

	// A fresh image swaps in on navigate; reset the loaded flag so the stable-size
	// placeholder shows again until it decodes (avoids a layout jump). A cached image
	// can finish loading before the onload listener attaches (no 'load' event fires
	// in that case), so also poll img.complete once the DOM has updated.
	$effect(() => {
		void current;
		loaded = false;
		tick().then(() => {
			if (imgEl?.complete) loaded = true;
		});
	});

	function src(img: PersonImage): string {
		return api.personGalleryImageURL(personId, img.id, { version: img.version });
	}
</script>

<Modal titleId="viewer-title" {onclose} size="media" align="center">
	<!-- basis-[85vh] + min-h-0 stands in for the former panel's h-full: the viewer fills
	     Modal's max-h-[85vh] panel and shrinks to fit it (border included). -->
	<div
		class="relative flex min-h-0 basis-[85vh] flex-col items-center justify-center p-4"
	>
		<h2 id="viewer-title" class="sr-only">{name} — gallery image {index + 1}</h2>

		<button
			bind:this={closeBtn}
			onclick={onclose}
			aria-label="Close"
			class="absolute right-3 top-3 rounded-theme px-2 py-0.5 text-muted hover:text-ink"
		>
			✕
		</button>

		<div class="flex min-h-0 flex-1 items-center justify-center">
			<!-- aria-disabled (not disabled) at the ends, mirroring the row's move-left/right
			     convention (PersonGallery.svelte) — the button no-ops out of bounds but stays
			     a focusable, unsurprising tab stop. -->
			<button
				onclick={() => index > 0 && onnavigate(index - 1)}
				aria-disabled={index === 0}
				aria-label="Previous image"
				class="mr-2 shrink-0 rounded-theme border border-rule bg-surface px-2 py-3 text-ink hover:border-accent hover:text-accent {index ===
				0
					? 'opacity-40'
					: ''}"
			>
				‹
			</button>

			<div class="flex min-h-0 flex-1 items-center justify-center bg-surface-2">
				{#if current}
					<img
						bind:this={imgEl}
						src={src(current)}
						alt={`${name} — gallery image ${index + 1}`}
						loading="eager"
						decoding="async"
						onload={() => (loaded = true)}
						class="max-h-[65vh] max-w-full object-contain transition-opacity {loaded
							? 'opacity-100'
							: 'opacity-0'}"
					/>
				{/if}
			</div>

			<button
				onclick={() => index < items.length - 1 && onnavigate(index + 1)}
				aria-disabled={index === items.length - 1}
				aria-label="Next image"
				class="ml-2 shrink-0 rounded-theme border border-rule bg-surface px-2 py-3 text-ink hover:border-accent hover:text-accent {index ===
				items.length - 1
					? 'opacity-40'
					: ''}"
			>
				›
			</button>
		</div>

		<p class="mt-2 text-sm text-muted" aria-live="polite">{index + 1} / {items.length}</p>
	</div>
</Modal>
