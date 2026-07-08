<script lang="ts">
	// Full-page gallery grid modal (HOLODEX-174): a read-only browse surface over a
	// person's full gallery, opened from PersonGallery's "Gallery (N)" trigger.
	// Structural twin of ConfirmDialog (focus trap, backdrop/X close, focus return) —
	// see ConfirmDialog.svelte for the shared dialog idiom. No owner editing here
	// (promote/move/delete stay in the inline row) — this is browse-only. Escape is
	// handled by the parent (PersonGallery), not this component, so it can route a
	// single Escape press to the topmost of this modal and a stacked image viewer
	// rather than closing both at once. Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import type { PersonImage } from '$lib/types';

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
	let trigger: HTMLElement | null = null;

	onMount(() => {
		trigger = document.activeElement as HTMLElement | null; // the "Gallery" trigger
		// The header's own Close button is the first <button> in DOM order — scope to
		// the grid so initial focus lands on the first grid item, per spec.
		dialogEl?.querySelector<HTMLElement>('ul button')?.focus();
		return () => trigger?.focus?.();
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

	function thumbSrc(img: PersonImage): string {
		return api.personGalleryImageURL(personId, img.id, {
			version: img.version,
			skin: theme.current
		});
	}
</script>

<div
	class="fixed inset-0 z-50 flex items-center justify-center bg-bg/70 px-4 py-[6vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		onkeydown={trapTab}
		tabindex="-1"
		class="gallery-modal-pop flex max-h-[85vh] w-full max-w-5xl flex-col rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="gallery-modal-title"
	>
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
</div>

<style>
	@media (prefers-reduced-motion: no-preference) {
		.gallery-modal-pop {
			animation: gallery-modal-rise 0.15s cubic-bezier(0.2, 0.7, 0.2, 1) both;
		}
	}
	@keyframes gallery-modal-rise {
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
