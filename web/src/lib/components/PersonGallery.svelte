<script lang="ts">
	// Person-page extra-image gallery (F25, ADR-038). A single horizontally-scrollable
	// row of uniform-height, uncropped thumbnails. Read-only for viewers. Owner-aware: an
	// "Add image" tile (multi-select upload, role=extra), and per-item controls revealed
	// on hover/focus — set-as-{headshot|banner|poster} (via the crop editor), delete, and
	// keyboard move-left/right reorder. The 20-extra cap disables Add with a themed warn
	// message (the server also enforces it). Tokens only; QA all three skins.
	import { api } from '$lib/api';
	import { theme } from '$lib/theme.svelte';
	import { toMessage } from '$lib/format';
	import type { PersonImage, PersonImageRole } from '$lib/types';
	import CropEditor from './CropEditor.svelte';

	const MAX_EXTRAS = 20;
	const CORE_ROLES: Exclude<PersonImageRole, 'extra'>[] = ['headshot', 'banner', 'poster'];
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
	// The gallery item being promoted (opens the crop editor) + its target role.
	let cropImage = $state<PersonImage | null>(null);
	let cropRole = $state<Exclude<PersonImageRole, 'extra'>>('headshot');

	const full = $derived(items.length >= MAX_EXTRAS);

	function thumbSrc(img: PersonImage): string {
		return api.personGalleryImageURL(personId, img.id, { version: img.version, skin: theme.current });
	}

	async function onPick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const files = input.files ? Array.from(input.files) : [];
		input.value = ''; // allow re-picking the same file(s) after an error
		if (!files.length || uploading) return;
		uploading = true;
		error = '';
		let added = 0;
		try {
			// Upload sequentially; a server-side cap rejection (409) stops the batch.
			for (const file of files) {
				await api.uploadPersonImage(personId, file, 'extra');
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

	function promote(img: PersonImage, role: Exclude<PersonImageRole, 'extra'>) {
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
	<h2 class="text-xs uppercase tracking-wide text-muted">Gallery</h2>

	{#if !items.length && !owner}
		<p class="text-sm text-muted">No gallery images.</p>
	{:else}
		<ul class="flex gap-3 overflow-x-auto pb-2">
			{#each items as img, i (img.id)}
				<li class="group relative shrink-0">
					<!-- eager, NOT lazy: a w-auto image collapses to a zero-area box before it
					     loads, and browsers won't fire native lazy-loading for a zero-area element,
					     so the gallery would never load. The gallery is a small bounded set (≤20). -->
					<img
						src={thumbSrc(img)}
						alt={`${name} — gallery image ${i + 1}`}
						loading="eager"
						decoding="async"
						class="h-40 w-auto rounded-theme border border-rule bg-surface-2"
					/>
					{#if owner}
						<!-- Controls reveal on hover (mouse) or focus-within (keyboard). -->
						<div
							class="absolute inset-0 flex flex-col justify-between rounded-theme bg-bg/55 p-1.5 opacity-0 transition group-hover:opacity-100 focus-within:opacity-100"
						>
							<div class="flex items-center gap-1">
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
							<div class="flex items-center justify-between gap-1">
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
						onclick={() => fileInput?.click()}
						disabled={full || uploading}
						aria-label="Add images"
						title="Add images"
						class="flex h-40 w-40 items-center justify-center rounded-theme border border-rule bg-surface-2 text-3xl text-muted hover:text-accent disabled:opacity-50"
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
		<p class="rounded-theme border border-warn px-3 py-2 text-sm text-warn">
			Gallery is full (20 max).
		</p>
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
