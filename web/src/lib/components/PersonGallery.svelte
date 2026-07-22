<script lang="ts">
	// Person-page extra-image gallery (F25, ADR-038). A single horizontally-scrollable
	// row of uniform-height, uncropped thumbnails. Read-only for viewers. Owner-aware: an
	// "Add image" tile (multi-select upload, role=extra), and per-item controls revealed
	// on hover/focus — set-as-{headshot|banner|poster} (via the crop editor), delete, and
	// keyboard move-left/right reorder. The gallery cap (PERSON_GALLERY_MAX, default 20)
	// disables the Add tile with an informational note and offers an explicit "Add anyway"
	// over-cap upload (the server enforces both). Tokens only; QA all three skins.
	import { api } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import { activity } from '$lib/activity.svelte';
	import { toMessage } from '$lib/format';
	import { CORE_ROLES, type CoreRole, type PersonImage } from '$lib/types';
	import CropEditor from './CropEditor.svelte';
	import PersonGalleryModal from './PersonGalleryModal.svelte';
	import PersonImageViewer from './PersonImageViewer.svelte';

	const ROLE_LABEL: Record<string, string> = {
		headshot: 'headshot',
		banner: 'banner',
		poster: 'poster'
	};

	let {
		personId,
		name,
		items,
		owner,
		onchanged
	}: {
		personId: number;
		name: string;
		items: PersonImage[];
		owner: boolean;
		// Bubble image-set changes up so the page can refresh core slots/avatars too.
		onchanged: () => void;
	} = $props();

	let uploading = $state(false);
	let error = $state('');
	let fileInput = $state<HTMLInputElement | null>(null);
	// Whether the pending file-pick should bypass the gallery cap (owner "Add anyway").
	let pendingOverCap = $state(false);
	// The gallery item being promoted (opens the crop editor) + its target role.
	let cropImage = $state<PersonImage | null>(null);
	let cropRole = $state<CoreRole>('headshot');

	// Full-page grid modal + image viewer (HOLODEX-174). viewerIndex is non-null
	// whether the viewer was opened from the row or from the grid modal; galleryOpen
	// tracks the grid independently so closing the viewer returns to the grid (if it
	// was open) or straight to the page (if it wasn't) — "two Escapes" behavior.
	let galleryOpen = $state(false);
	let viewerIndex = $state<number | null>(null);

	const max = $derived(activity.galleryMax);
	const full = $derived(items.length >= max);

	function openViewer(i: number) {
		viewerIndex = i;
	}

	// A single centralized keydown handler (rather than one per modal) so Escape and
	// the arrow keys route to whichever layer is topmost: viewer over grid over page.
	// If each modal owned its own window-level Escape listener, one keypress would
	// fire both when stacked, closing viewer and grid at once.
	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (viewerIndex !== null) viewerIndex = null;
			else if (galleryOpen) galleryOpen = false;
			return;
		}
		if (viewerIndex === null) return;
		if (e.key === 'ArrowLeft' && viewerIndex > 0) {
			e.preventDefault();
			viewerIndex -= 1;
		} else if (e.key === 'ArrowRight' && viewerIndex < items.length - 1) {
			e.preventDefault();
			viewerIndex += 1;
		}
	}

	function thumbSrc(img: PersonImage): string {
		return api.personGalleryImageURL(personId, img.id, { version: img.version, skin: theme.current });
	}

	// Open the file picker; overCap=true marks the batch as an explicit over-cap add.
	function openPicker(overCap: boolean) {
		pendingOverCap = overCap;
		fileInput?.click();
	}

	async function onPick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const files = input.files ? Array.from(input.files) : [];
		input.value = ''; // allow re-picking the same file(s) after an error
		const overCap = pendingOverCap;
		pendingOverCap = false;
		if (!files.length || uploading) return;
		uploading = true;
		error = '';
		let added = 0;
		try {
			// Upload sequentially; a server-side cap rejection (409) stops the batch
			// (unless this is an explicit over-cap add).
			for (const file of files) {
				await api.uploadPersonImage(personId, file, 'extra', overCap);
				added++;
			}
		} catch (err) {
			// Make a partial batch explicit so the user knows what landed vs. was dropped
			// (e.g. 3 of 5 added, then the gallery cap stopped the rest).
			error =
				added > 0
					? `Added ${added} of ${files.length}. ${files.length - added} not added: ${toMessage(err)}`
					: toMessage(err);
		} finally {
			uploading = false;
			if (added) onchanged(); // refresh whatever made it in before any failure
		}
	}

	async function remove(img: PersonImage) {
		error = '';
		try {
			await api.deletePersonImage(personId, img.id);
			onchanged();
		} catch (err) {
			error = toMessage(err);
		}
	}

	function promote(img: PersonImage, role: CoreRole) {
		cropRole = role;
		cropImage = img;
	}

	// Keyboard reorder: move an item one step and persist the new order. Avoids a
	// drag-only interaction (a11y) — see handoff "keyboard alternative".
	async function move(index: number, delta: number) {
		const next = index + delta;
		if (next < 0 || next >= items.length) return;
		const order = items.map((i) => i.id);
		[order[index], order[next]] = [order[next], order[index]];
		error = '';
		try {
			await api.reorderPersonImages(personId, order);
			onchanged();
		} catch (err) {
			error = toMessage(err);
		}
	}
</script>

<section class="space-y-3">
	<div class="flex items-center justify-between gap-3">
		<h2 class="text-xs uppercase tracking-wide text-muted">Gallery</h2>
		{#if items.length}
			<button
				onclick={() => (galleryOpen = true)}
				class="flex items-center gap-1 text-xs text-muted hover:text-accent"
			>
				Gallery ({items.length}) <span aria-hidden="true">›</span>
			</button>
		{/if}
	</div>

	{#if !items.length && !owner}
		<p class="text-sm text-muted">No gallery images.</p>
	{:else}
		<ul class="flex gap-3 overflow-x-auto pb-2">
			{#each items as img, i (img.id)}
				<li class="group relative shrink-0">
					<!-- eager, NOT lazy: a w-auto image collapses to a zero-area box before it
					     loads, and browsers won't fire native lazy-loading for a zero-area element,
					     so the gallery would never load. The gallery is a small bounded set (≤20). -->
					<button onclick={() => openViewer(i)} aria-label={`${name} — gallery image ${i + 1}`} class="block">
						<img
							src={thumbSrc(img)}
							alt={`${name} — gallery image ${i + 1}`}
							loading="eager"
							decoding="async"
							class="h-40 w-auto rounded-theme border border-rule bg-surface-2"
						/>
					</button>
					{#if owner}
						<!-- Controls reveal on hover (mouse) or focus-within (keyboard). The overlay
						     itself is pointer-events-none — it sits above the <img> even when invisible,
						     so without that a click on the image area would land here instead of the
						     button underneath. Only its buttons (pointer-events-auto) are clickable;
						     everywhere else, clicks fall through to the row's own image button. -->
						<div
							class="pointer-events-none absolute inset-0 flex flex-col justify-between rounded-theme bg-bg/55 p-1.5 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100"
						>
							<div class="pointer-events-auto flex items-center gap-1">
								{#each CORE_ROLES as role (role)}
									<button
										onclick={() => promote(img, role)}
										aria-label={`Set as ${ROLE_LABEL[role]}`}
										title={`Set as ${ROLE_LABEL[role]}`}
										class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-ink hover:border-accent hover:text-accent"
									>
										{ROLE_LABEL[role].charAt(0).toUpperCase()}
									</button>
								{/each}
							</div>
							<div class="pointer-events-auto flex items-center justify-between gap-1">
								<div class="flex gap-1">
									<!-- aria-disabled (not disabled) at the ends: move() no-ops out of bounds, and
									     keeping the button focusable means a keyboard reorder to an end doesn't drop
									     focus (which would collapse this hover/focus overlay). -->
									<button
										onclick={() => move(i, -1)}
										aria-disabled={i === 0}
										aria-label="Move image left"
										title="Move left"
										class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-ink hover:border-accent hover:text-accent {i ===
										0
											? 'opacity-40'
											: ''}"
									>
										←
									</button>
									<button
										onclick={() => move(i, 1)}
										aria-disabled={i === items.length - 1}
										aria-label="Move image right"
										title="Move right"
										class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-ink hover:border-accent hover:text-accent {i ===
										items.length - 1
											? 'opacity-40'
											: ''}"
									>
										→
									</button>
								</div>
								<button
									onclick={() => remove(img)}
									aria-label="Delete image"
									title="Delete"
									class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-muted hover:border-warn hover:text-warn"
								>
									✕
								</button>
							</div>
						</div>
					{/if}
				</li>
			{/each}

			{#if owner}
				<li class="shrink-0">
					<button
						onclick={() => openPicker(false)}
						disabled={full || uploading}
						aria-label="Add images"
						title="Add images"
						class="flex h-40 w-40 items-center justify-center rounded-theme border border-rule bg-surface-2 text-3xl text-muted hover:text-accent disabled:border-transparent"
					>
						{uploading ? '…' : '+'}
					</button>
					<input
						bind:this={fileInput}
						onchange={onPick}
						type="file"
						accept="image/*"
						multiple
						class="hidden"
						aria-hidden="true"
						tabindex="-1"
					/>
				</li>
			{/if}
		</ul>
	{/if}

	{#if owner && full}
		<!-- Informational (not an error): the gallery is at the cap, but the owner can
		     still add deliberately. Neutral tokens, never text-warn — a full gallery is a
		     status, not a failure of the action just taken. -->
		<div
			class="flex flex-wrap items-center gap-2 rounded-theme border border-rule bg-surface-2 px-3 py-2 text-sm text-muted"
		>
			<span>Gallery full — {max} of {max}.</span>
			<button
				onclick={() => openPicker(true)}
				disabled={uploading}
				class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-ink hover:border-accent hover:text-accent disabled:opacity-50"
			>
				Add anyway
			</button>
		</div>
	{/if}

	{#if error}
		<p class="text-sm text-warn">{error}</p>
	{/if}
</section>

{#if cropImage}
	<CropEditor
		{personId}
		image={cropImage}
		role={cropRole}
		onclose={() => (cropImage = null)}
		onpromoted={onchanged}
	/>
{/if}

{#if galleryOpen}
	<PersonGalleryModal
		{personId}
		{name}
		{items}
		onclose={() => (galleryOpen = false)}
		onselect={openViewer}
	/>
{/if}

{#if viewerIndex !== null}
	<PersonImageViewer
		{personId}
		{name}
		{items}
		index={viewerIndex}
		onclose={() => (viewerIndex = null)}
		onnavigate={(i) => (viewerIndex = i)}
	/>
{/if}

<svelte:window onkeydown={onKey} />
