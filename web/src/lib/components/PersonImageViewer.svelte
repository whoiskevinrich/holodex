<script lang="ts">
	// Full-page image viewer modal (HOLODEX-174), opened by clicking/Entering any
	// gallery thumbnail (inline row or grid modal). Darker backdrop than the grid
	// modal so it reads "on top" when stacked. Prev/next with no wraparound, a
	// position counter, and a fit-to-modal image that never upscales past native
	// resolution. Escape/arrow keys are handled by the parent (PersonGallery), not
	// this component — see PersonGalleryModal.svelte for why. Tokens only; QA 3 skins.
	import { onMount, tick } from 'svelte';
	import { api } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import type { PersonImage } from '$lib/types';

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

	let dialogEl = $state<HTMLElement | null>(null);
	let closeBtn = $state<HTMLButtonElement | null>(null);
	let imgEl = $state<HTMLImageElement | null>(null);
	let loaded = $state(false);
	let trigger: HTMLElement | null = null;

	const current = $derived(items[index]);

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null; // the thumbnail that opened this
		closeBtn?.focus();
		return () => trigger?.focus?.();
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

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('button')].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
		);
		if (f.length === 0) return;
		const first = f[0];
		const last = f[f.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	function src(img: PersonImage): string {
		return api.personGalleryImageURL(personId, img.id, {
			version: img.version,
			skin: theme.current
		});
	}
</script>

<div
	class="fixed inset-0 z-[60] flex items-center justify-center bg-bg/85 px-4 py-[6vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="viewer-pop relative flex h-full max-h-[85vh] w-full max-w-4xl flex-col items-center justify-center rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="viewer-title"
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
</div>

<style>
	@media (prefers-reduced-motion: no-preference) {
		.viewer-pop {
			animation: viewer-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes viewer-rise {
		from {
			opacity: 0;
			transform: scale(0.98);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
</style>
