<script lang="ts" generics="TRole extends string">
	// Entity image role control (HOLODEX-286): upload / replace / remove for one of an
	// entity's single-slot image roles. Generalized from the Studio (F51, ADR-079) and
	// Film (F56/HOLODEX-280, ADR-086) versions, which were byte-for-byte identical
	// apart from the entity id/name prop names and which api.ts calls to make — those
	// now come in as `upload`/`remove` props instead. No gallery, no promote, no viewer
	// modal — unlike Person's image system, which keeps its own dedicated components
	// (PersonGallery et al.) since it has a capped gallery, dedup, and promote that
	// this simple "one row per role" shape doesn't. Visitors see the frame read-only;
	// owners get Replace/Remove. The `"poster"` role has no prior placeholder, so its
	// empty state is the upload affordance itself (owner-only) or a plain dashed box
	// (visitor); every other role falls back to the entity's monogram when empty.
	import { toMessage, monogram } from '$lib/format';

	let {
		entityId,
		entityName,
		role,
		label,
		url,
		isOwner,
		upload,
		remove: removeImage,
		onchanged
	}: {
		entityId: number;
		entityName: string;
		role: TRole;
		label: string;
		url?: string;
		isOwner: boolean;
		upload: (entityId: number, file: File, role: TRole) => Promise<unknown>;
		remove: (entityId: number, role: TRole) => Promise<unknown>;
		onchanged: () => void;
	} = $props();

	const isPoster = $derived(role === 'poster');
	const frameClass = $derived(isPoster ? 'h-24 w-16' : 'h-16 w-16');

	let uploading = $state(false);
	let confirmingRemove = $state(false);
	let error = $state('');
	let fileInput = $state<HTMLInputElement | null>(null);

	function openPicker() {
		fileInput?.click();
	}

	async function onPick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = ''; // allow re-picking the same file after an error
		if (!file || uploading) return;
		uploading = true;
		error = '';
		try {
			await upload(entityId, file, role);
			onchanged();
		} catch (err) {
			error = toMessage(err);
		} finally {
			uploading = false;
		}
	}

	async function remove() {
		error = '';
		try {
			await removeImage(entityId, role);
			confirmingRemove = false;
			onchanged();
		} catch (err) {
			error = toMessage(err);
		}
	}
</script>

<div class="flex items-center gap-4 rounded-theme border border-rule bg-surface p-3">
	<span
		class="relative flex shrink-0 items-center justify-center overflow-hidden rounded-theme {isPoster &&
		!url
			? 'border border-dashed border-rule'
			: 'bg-logo-plate'} {frameClass}"
	>
		{#if url}
			<img
				src={url}
				alt={`${entityName} ${label.toLowerCase()}`}
				class="h-full w-full object-contain p-1 {uploading ? 'opacity-60' : ''}"
			/>
		{:else if isPoster && isOwner}
			<button
				onclick={openPicker}
				disabled={uploading}
				aria-label={`Upload ${label.toLowerCase()}`}
				class="flex h-full w-full items-center justify-center text-2xl text-muted hover:text-accent"
			>
				{uploading ? '…' : '+'}
			</button>
		{:else if !isPoster}
			<span class="font-display text-sm font-semibold text-logo-plate-ink" aria-hidden="true">
				{monogram(entityName)}
			</span>
		{/if}
		{#if uploading && url}
			<span class="absolute inset-0 animate-pulse bg-surface-2/60" aria-hidden="true"></span>
		{/if}
	</span>

	<div class="min-w-0 flex-1 space-y-1">
		<p class="text-sm font-medium text-ink">{label}</p>
		{#if error}
			<p class="text-xs text-warn">{error}</p>
		{/if}
		{#if isOwner}
			<div class="flex items-center gap-2">
				{#if !isPoster || url}
					<button
						onclick={openPicker}
						disabled={uploading}
						class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-ink hover:border-accent hover:text-accent disabled:opacity-50"
					>
						Replace
					</button>
				{/if}
				{#if url}
					{#if confirmingRemove}
						<span class="text-xs text-muted">Remove?</span>
						<button
							onclick={remove}
							class="rounded-theme border border-warn px-2 py-0.5 text-xs text-warn hover:bg-warn/10"
						>
							Remove
						</button>
						<button
							onclick={() => (confirmingRemove = false)}
							class="text-xs text-muted hover:text-ink"
						>
							Cancel
						</button>
					{:else}
						<button
							onclick={() => (confirmingRemove = true)}
							class="rounded-theme border border-rule bg-surface px-2 py-0.5 text-xs text-muted hover:border-warn hover:text-warn"
						>
							Remove
						</button>
					{/if}
				{/if}
			</div>
			<input
				bind:this={fileInput}
				onchange={onPick}
				type="file"
				accept="image/*"
				class="hidden"
				aria-hidden="true"
				tabindex="-1"
			/>
		{/if}
	</div>
</div>
