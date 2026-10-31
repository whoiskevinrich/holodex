<script lang="ts" module>
	// The accessible name names the lock when the owner set this image, so enrichment keeps
	// it (F33/HOLODEX-43); an enrichment-sourced slot keeps the plain label.
	export function coreEditLabel(role: string, locked: boolean): string {
		return locked ? `Replace ${role} (yours — kept on re-enrich)` : `Replace ${role}`;
	}
</script>

<script lang="ts">
	// Uniform owner "Edit" overlay for a core image slot (banner/headshot/poster) — one
	// shape, one label, one scrim across all three, positioned per call site. `compact`
	// swaps the text pill for an icon-only square — the headshot identity badge (64-80px)
	// is too small for the full "Edit" pill without the button covering a big chunk of the
	// image itself. A locked slot adds a muted lock before the label (or beside the pencil
	// when compact, keeping it 24px tall). `onremove`, passed only for a filled slot, adds a
	// Remove control beside Edit in the same shape (HOLODEX-610). Visitors get nothing.
	let {
		role,
		position,
		compact = false,
		isOwner,
		locked = false,
		busy = false,
		onclick,
		onremove
	}: {
		role: string;
		position: string;
		compact?: boolean;
		isOwner: boolean;
		locked?: boolean;
		busy?: boolean;
		onclick: () => void;
		onremove?: () => void;
	} = $props();

	const label = $derived(coreEditLabel(role, locked));
	const chrome =
		'flex items-center justify-center rounded-theme bg-bg/80 text-ink shadow-sm backdrop-blur-sm disabled:opacity-60';
</script>

{#if isOwner}
	<div class="absolute z-10 {position} flex gap-1">
		<button
			{onclick}
			disabled={busy}
			aria-label={label}
			title={label}
			class="{chrome} hover:text-accent {compact
				? locked
					? 'h-6 gap-0.5 px-1'
					: 'h-6 w-6'
				: 'gap-1 px-2.5 py-1.5 text-xs font-semibold'}"
		>
			<!-- Withdrawn while busy: the button's disabled:opacity-60 would drop the muted lock
			     below contrast (frontend-theming rule). -->
			{#if locked && !busy}
				<svg
					class="h-3 w-3 text-muted"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					aria-hidden="true"
					data-lock
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
					/>
				</svg>
			{/if}
			{#if compact}
				<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
					/>
				</svg>
			{:else}
				{busy ? '…' : 'Edit'}
			{/if}
		</button>
		{#if onremove}
			<button
				onclick={onremove}
				disabled={busy}
				aria-label={`Remove ${role}`}
				title={`Remove ${role}`}
				data-remove
				class="{chrome} hover:text-warn {compact ? 'h-6 w-6' : 'px-2.5 py-1.5 text-xs font-semibold'}"
			>
				{#if compact}
					<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
					</svg>
				{:else}
					Remove
				{/if}
			</button>
		{/if}
	</div>
{/if}
