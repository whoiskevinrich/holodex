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
	//
	// `variant="frame"` (HOLODEX-307) drops the label/Replace/Remove row for hero placement
	// (e.g. the Film detail header) — just the image frame plus small owner overlay buttons
	// (pencil = replace, × = remove) in the frame's corners, instead of a dedicated section.
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
		onchanged,
		variant = 'row',
		frameClass: frameClassProp
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
		variant?: 'row' | 'frame';
		frameClass?: string;
	} = $props();

	const isPoster = $derived(role === 'poster');
	const isFrame = $derived(variant === 'frame');
	const frameClass = $derived(frameClassProp ?? (isPoster ? 'h-24 w-16' : 'h-16 w-16'));
	const monogramClass = $derived(isFrame ? 'text-4xl' : 'text-sm');
	// The poster role's empty state has no placeholder of its own (see the file-level
	// comment) except in the frame variant, where a hero placement needs the monogram
	// fallback just as much as every other role does.
	const showMonogram = $derived(!isPoster || isFrame);
	const dashedEmptyPoster = $derived(isPoster && !url && !isFrame);

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

<div class={isFrame ? 'inline-block' : 'flex items-center gap-4 rounded-theme border border-rule bg-surface p-3'}>
	<span
		class="relative flex shrink-0 items-center justify-center overflow-hidden rounded-theme {dashedEmptyPoster
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
		{:else if showMonogram}
			<span class="font-display font-semibold text-logo-plate-ink {monogramClass}" aria-hidden="true">
				{monogram(entityName)}
			</span>
		{/if}
		{#if uploading && url}
			<span class="absolute inset-0 animate-pulse bg-surface-2/60" aria-hidden="true"></span>
		{/if}

		{#if isFrame && isOwner && url}
			<!-- Hero overlay (HOLODEX-307) — same "small button in the frame's corner" shape
			     as the Person hero's editBtn, since this variant replaces a dedicated section. -->
			<button
				onclick={openPicker}
				disabled={uploading}
				aria-label={`Replace ${label.toLowerCase()}`}
				title={`Replace ${label.toLowerCase()}`}
				class="absolute right-1 top-1 z-10 flex h-6 w-6 items-center justify-center rounded-theme bg-bg/80 text-ink shadow-sm backdrop-blur-sm hover:text-accent disabled:opacity-60"
			>
				<svg
					class="h-3 w-3"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					aria-hidden="true"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
					/>
				</svg>
			</button>
			{#if confirmingRemove}
				<button
					onclick={remove}
					class="absolute bottom-1 right-1 z-10 rounded-theme bg-warn px-1.5 py-0.5 text-[10px] font-semibold text-warn-ink shadow-sm hover:bg-warn/90"
				>
					Remove
				</button>
				<button
					onclick={() => (confirmingRemove = false)}
					aria-label="Cancel remove"
					class="absolute bottom-1 left-1 z-10 rounded-theme bg-bg/80 px-1.5 py-0.5 text-[10px] text-ink shadow-sm backdrop-blur-sm hover:text-accent"
				>
					Cancel
				</button>
			{:else}
				<button
					onclick={() => (confirmingRemove = true)}
					aria-label={`Remove ${label.toLowerCase()}`}
					title={`Remove ${label.toLowerCase()}`}
					class="absolute bottom-1 right-1 z-10 flex h-6 w-6 items-center justify-center rounded-theme bg-bg/80 text-warn shadow-sm backdrop-blur-sm hover:bg-warn/10"
				>
					×
				</button>
			{/if}
		{/if}
	</span>

	{#if isFrame}
		{#if error}
			<p class="mt-1 text-xs text-warn">{error}</p>
		{/if}
	{:else}
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
			{/if}
		</div>
	{/if}

	{#if isOwner}
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
