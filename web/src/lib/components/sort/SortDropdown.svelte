<script lang="ts" generics="T extends string = SortOrder">
	import type { SortOrder } from '$lib/types';
	import { MEDIA_SORTS } from '$lib/filters';
	import type { SortOption } from '$lib/listState';

	// owner gates the ownerOnly entries (the Completeness sorts) — the server 401s a
	// non-owner request using them, so they must not even render as options; for the
	// owner they sit in a trailing "Owner" group. extra prepends page-specific entries
	// (the playlist page's `manual`, F69) so a page never forks the dropdown.
	// `options` is the page's list (default MEDIA_SORTS; listState.ts holds the entity
	// lists). `compact` is the F73 list-toolbar form: no visible caption, 32px tall, able
	// to shrink. `onchange` is where a page records the preference (ADR-114 D3: only the
	// change handler writes storage, never an effect).
	let {
		sort = $bindable(),
		owner = false,
		extra = [],
		options = MEDIA_SORTS as unknown as readonly SortOption<T>[],
		compact = false,
		onchange
	}: {
		sort: T;
		owner?: boolean;
		extra?: { value: T; label: string }[];
		options?: readonly SortOption<T>[];
		compact?: boolean;
		onchange?: (value: T) => void;
	} = $props();

	const PUBLIC = $derived([...extra, ...options.filter((o) => !o.ownerOnly)]);
	const OWNER = $derived(owner ? options.filter((o) => o.ownerOnly) : []);
</script>

<div class={compact ? 'min-w-0' : ''}>
	<label class={compact ? 'sr-only' : 'mb-1 block text-xs text-muted'} for="sort">Sort</label>
	<select
		id="sort"
		bind:value={sort}
		onchange={(e) => onchange?.(e.currentTarget.value as T)}
		class="max-w-full rounded-theme border border-rule bg-surface text-sm text-ink outline-none focus:border-accent {compact
			? 'h-8 min-w-0 px-3 py-0'
			: 'px-3 py-2'}"
	>
		{#each PUBLIC as o (o.value)}
			<option value={o.value}>{o.label}</option>
		{/each}
		{#if OWNER.length}
			<optgroup label="Owner">
				{#each OWNER as o (o.value)}
					<option value={o.value}>{o.label}</option>
				{/each}
			</optgroup>
		{/if}
	</select>
</div>
