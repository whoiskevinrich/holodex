<script lang="ts">
	// Person-page extra-image gallery (F24, ADR-037). Read-only for viewers: a grid of
	// 1:1 thumbnails. Owner-aware: an "Add image" tile (uploads role=extra), per-item
	// delete + set-as-{headshot|banner|poster} (via the crop editor) + keyboard
	// move-up/down reorder. The 20-extra cap disables Add with a themed warn message
	// (the server also enforces it). Tokens only; QA all three skins.
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
		const file = input.files?.[0];
		input.value = ''; // allow re-picking the same file after an error
		if (!file || uploading) return;
		uploading = true;
		error = '';
		try {
			await api.uploadPersonImage(personId, file, 'extra');
			onchanged();
		} catch (err) {
			error = toMessage(err);
		} finally {
			uploading = false;
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
		<ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
			{#each items as img, i (img.id)}
				<li class="space-y-1.5">
					<div class="portrait-frame portrait-frame--1x1 w-full">
						<img src={thumbSrc(img)} alt={`${name} — gallery image ${i + 1}`} loading="lazy" decoding="async" />
					</div>
					{#if owner}
						<div class="flex flex-wrap items-center gap-1">
							{#each CORE_ROLES as role (role)}
								<button
									onclick={() => promote(img, role)}
									aria-label={`Set as ${ROLE_LABEL[role]}`}
									title={`Set as ${ROLE_LABEL[role]}`}
									class="rounded-theme border border-rule px-1.5 py-0.5 text-xs text-ink hover:border-accent"
								>
									{ROLE_LABEL[role].charAt(0).toUpperCase()}
								</button>
							{/each}
							<button
								onclick={() => move(i, -1)}
								disabled={i === 0}
								aria-label="Move image up"
								title="Move up"
								class="rounded-theme border border-rule px-1.5 py-0.5 text-xs text-ink hover:border-accent disabled:opacity-40"
							>
								↑
							</button>
							<button
								onclick={() => move(i, 1)}
								disabled={i === items.length - 1}
								aria-label="Move image down"
								title="Move down"
								class="rounded-theme border border-rule px-1.5 py-0.5 text-xs text-ink hover:border-accent disabled:opacity-40"
							>
								↓
							</button>
							<button
								onclick={() => remove(img)}
								aria-label="Delete image"
								title="Delete"
								class="rounded-theme border border-rule px-1.5 py-0.5 text-xs text-muted hover:border-warn hover:text-warn"
							>
								✕
							</button>
						</div>
					{/if}
				</li>
			{/each}

			{#if owner}
				<li>
					<button
						onclick={() => fileInput?.click()}
						disabled={full || uploading}
						aria-label="Add image"
						class="portrait-frame portrait-frame--1x1 flex w-full items-center justify-center text-3xl text-muted hover:text-accent disabled:opacity-50"
					>
						{uploading ? '…' : '+'}
					</button>
					<input
						bind:this={fileInput}
						onchange={onPick}
						type="file"
						accept="image/*"
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
