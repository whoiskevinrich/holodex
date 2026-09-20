<script lang="ts">
	// Shared 2:3 poster chip (HOLODEX-296): the `<li>` the Media detail page's Films
	// row and `PeopleGrid`'s People row both draw — fixed w-20 so the two rows line up
	// beside each other (HOLODEX-328), a linked poster + two-line caption, and the
	// hover-reveal remove badge (HOLODEX-272) as a sibling of the <a>, never nested in
	// it (an interactive control inside an anchor is invalid). Owner vs. read-only is
	// decided by whether the caller passes `onRemove` (the `TagLinkChip` convention).
	// `poster` draws only the image box; the caption comes from `name`. `children` is
	// the tile's extra overlay slot — Films puts its scene pill there, which is also why
	// `removeSide` exists: that pill owns the top-right corner, so Films docks remove
	// top-left while People keeps the default right.
	import type { Snippet } from 'svelte';

	let {
		href,
		name,
		poster,
		onRemove,
		busy = false,
		removeSide = 'right',
		children
	}: {
		href: string;
		name: string;
		poster: Snippet;
		onRemove?: () => void;
		busy?: boolean;
		removeSide?: 'left' | 'right';
		children?: Snippet;
	} = $props();
</script>

<li class="curation-chip group relative w-20 shrink-0">
	<a {href} class="block space-y-1.5 text-ink" title={name}>
		{@render poster()}
		<span class="line-clamp-2 text-xs text-muted group-hover:text-accent">{name}</span>
	</a>
	{@render children?.()}
	{#if onRemove}
		<button
			type="button"
			onclick={onRemove}
			disabled={busy}
			aria-label={`Remove ${name}`}
			class="curation-actions absolute top-1.5 z-[2] flex h-6 w-6 items-center justify-center rounded-full border border-rule bg-surface-2/90 text-sm text-muted hover:border-accent hover:text-accent focus-visible:border-accent focus-visible:text-accent disabled:cursor-default {removeSide === 'left'
				? 'left-1.5'
				: 'right-1.5'}"
		>
			{busy ? '…' : '×'}
		</button>
	{/if}
</li>
