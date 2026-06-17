<script lang="ts">
	// Minimal promote-with-crop editor (F25.15, P1). Previews a gallery image inside
	// the target core-role aspect frame with a zoom slider + drag-to-position, then
	// saves by calling promotePersonImage. The backend re-normalizes the stored copy;
	// this editor is currently a *visual preview* of the framing — the exact pixel
	// crop is not yet sent to the server.
	// TODO(F25): send crop coordinates so the server stores the exact framed copy
	// (today promote re-normalizes the whole source image; the zoom/pan is preview-only).
	import { api } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import { toMessage } from '$lib/format';
	import type { PersonImage, PersonImageRole } from '$lib/types';

	let {
		personId,
		image,
		role,
		onclose,
		onpromoted
	}: {
		personId: number;
		image: PersonImage;
		role: Exclude<PersonImageRole, 'extra'>;
		onclose: () => void;
		onpromoted: () => void;
	} = $props();

	const frameClass: Record<typeof role, string> = {
		headshot: 'crop-frame crop-frame--1x1',
		banner: 'crop-frame crop-frame--banner',
		poster: 'crop-frame crop-frame--2x3'
	};
	const roleLabel: Record<typeof role, string> = {
		headshot: 'headshot',
		banner: 'banner',
		poster: 'poster'
	};

	const src = $derived(
		api.personGalleryImageURL(personId, image.id, { version: image.version, skin: theme.current })
	);

	let zoom = $state(1);
	let offsetX = $state(0);
	let offsetY = $state(0);
	let dragging = $state(false);
	let startX = 0;
	let startY = 0;
	let busy = $state(false);
	let error = $state('');
	let dialogEl = $state<HTMLElement | null>(null);
	let trigger: HTMLElement | null = null;

	const transform = $derived(
		`translate(${offsetX}px, ${offsetY}px) scale(${zoom})`
	);

	function down(e: PointerEvent) {
		dragging = true;
		startX = e.clientX - offsetX;
		startY = e.clientY - offsetY;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function move(e: PointerEvent) {
		if (!dragging) return;
		offsetX = e.clientX - startX;
		offsetY = e.clientY - startY;
	}
	function up() {
		dragging = false;
	}

	async function save() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await api.promotePersonImage(personId, image.id, role);
			onpromoted();
			onclose();
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}

	// Focus management: trap Tab inside the modal, return focus to the trigger.
	function onMount(node: HTMLElement) {
		trigger = document.activeElement as HTMLElement | null;
		node.querySelector<HTMLElement>('button, input')?.focus();
		return { destroy: () => trigger?.focus?.() };
	}
	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const f = [...dialogEl.querySelectorAll<HTMLElement>('input, button')].filter(
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
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[8vh]"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget && !busy) onclose();
	}}
>
	<div
		bind:this={dialogEl}
		use:onMount
		onkeydown={trapTab}
		tabindex="-1"
		class="flex w-full max-w-md flex-col gap-3 rounded-theme border border-rule bg-surface p-4 shadow-xl"
		role="dialog"
		aria-modal="true"
		aria-labelledby="crop-title"
	>
		<div class="flex items-start justify-between gap-3">
			<h2 id="crop-title" class="skin-title text-lg font-semibold text-ink">
				Set as {roleLabel[role]}
			</h2>
			<button
				onclick={onclose}
				aria-label="Close"
				class="rounded-theme px-2 py-0.5 text-muted hover:text-ink">✕</button
			>
		</div>

		<p class="text-xs text-muted">Position and zoom, then save a cropped copy. The gallery original stays.</p>

		<!-- Preview well in the target ratio; the full original is shown (object-fit:contain)
		     and is pannable/zoomable inside it. Uses .crop-frame (not .portrait-frame) so no
		     skin scanline/grain flourish sits over the image being framed. -->
		<div class="{frameClass[role]} relative w-full">
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<img
				{src}
				alt={`Cropping ${roleLabel[role]}`}
				draggable="false"
				style={`transform: ${transform}; touch-action: none; cursor: ${dragging ? 'grabbing' : 'grab'};`}
				onpointerdown={down}
				onpointermove={move}
				onpointerup={up}
				onpointercancel={up}
			/>
			<!-- Rule-of-thirds guide: fixed to the frame (does NOT move with the image), so
			     the owner can align the subject on the thirds while panning/zooming under it.
			     Lines use the ink token (light in every skin) at low opacity; non-interactive. -->
			<div class="pointer-events-none absolute inset-0" aria-hidden="true">
				<div class="absolute inset-y-0 left-1/3 w-px bg-ink/40"></div>
				<div class="absolute inset-y-0 left-2/3 w-px bg-ink/40"></div>
				<div class="absolute inset-x-0 top-1/3 h-px bg-ink/40"></div>
				<div class="absolute inset-x-0 top-2/3 h-px bg-ink/40"></div>
			</div>
		</div>

		<label class="flex items-center gap-2 text-xs text-muted">
			Zoom
			<input
				type="range"
				min="1"
				max="3"
				step="0.05"
				bind:value={zoom}
				aria-label="Zoom"
				class="flex-1 accent-accent"
			/>
		</label>

		{#if error}
			<p class="text-sm text-warn">{error}</p>
		{/if}

		<div class="flex items-center justify-end gap-2">
			<button
				onclick={onclose}
				disabled={busy}
				class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
			>
				Cancel
			</button>
			<button
				onclick={save}
				disabled={busy}
				class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
			>
				{busy ? 'Saving…' : `Save as ${roleLabel[role]}`}
			</button>
		</div>
	</div>
</div>

<svelte:window onkeydown={(e) => e.key === 'Escape' && !busy && onclose()} />
