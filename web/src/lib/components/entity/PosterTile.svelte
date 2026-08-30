<script lang="ts">
	// Shared poster-tile chip (HOLODEX-296): the display shell common to the Media
	// detail page's Films chip and PeopleGrid's People chip — curation-chip shape,
	// hover-reveal remove badge, truncated label. Image markup differs enough between
	// callers (PersonPoster component vs. an inline monogram plate) that it stays a
	// caller-supplied snippet rather than being folded in here; same for the optional
	// badge line (Films' Full film/#N tag, which People has no equivalent of). The
	// "+Add" affordance tiles (Films' dashed CTA box, People's PersonPicker popover)
	// are a different mechanism per entity and intentionally NOT unified here — mirrors
	// TagLinkChip.svelte, which owns only the display chip, not the add control.
	import type { Snippet } from 'svelte';

	let {
		href,
		label,
		image,
		badge,
		busy = false,
		onRemove,
		class: className = ''
	}: {
		href: string;
		label: string;
		image: Snippet;
		badge?: Snippet;
		busy?: boolean;
		// Remove control renders only when supplied — the caller's own gate (owner,
		// editable, …) decides, so this component needs no ownership boolean of its
		// own. Same convention as TagLinkChip.
		onRemove?: () => void;
		class?: string;
	} = $props();
</script>

<li class="curation-chip group relative {className}">
	<a {href} class="block space-y-1.5 text-ink" title={label}>
		{@render image()}
		<span class="line-clamp-2 text-xs text-muted group-hover:text-accent">{label}</span>
		{@render badge?.()}
	</a>
	{#if onRemove}
		<!-- Hover-reveal remove badge (HOLODEX-272), a sibling of <a> rather than nested
		     inside it (a nested interactive control inside an anchor is invalid). -->
		<button
			type="button"
			onclick={onRemove}
			disabled={busy}
			aria-label={`Remove ${label}`}
			class="curation-actions absolute right-1.5 top-1.5 flex h-6 w-6 items-center justify-center rounded-full border border-rule bg-surface-2/90 text-sm text-muted hover:border-accent hover:text-accent focus-visible:border-accent focus-visible:text-accent disabled:cursor-default"
		>
			{busy ? '…' : '×'}
		</button>
	{/if}
</li>
