<script lang="ts">
	// The name field's curation line under a Person/Studio/Film heading (F60 RD10,
	// HOLODEX-378). The heading renders the *resolved* name; this line renders only when
	// that differs from the canonical column — "In files as <canonical>" for everyone,
	// plus the name field's SourceBadge (the "Display as" verb) for the owner — so the
	// at-rest header is unchanged and visitor/owner views stay identical (HOLODEX-268).
	// With no decision standing the owner gets a quiet "Display as…" link in the same
	// slot, which opens the badge's chip row in place.
	//
	// Two verbs, two existing affordances (handoff §4c): the badge here changes a
	// per-field decision (DB only); the docked pencil on the heading is "Rename in files"
	// and prefills canonical (NameEditControl `editValue`). No new buttons — but once a
	// decision stands the record spelling on this line is itself the owner's trigger for
	// that same rename form (`onRename` → NameEditControl.open()): the pencil is
	// hover-revealed beside a heading that no longer shows the record spelling, and QA 4.3's
	// follow-up found no visible way to change what the files say.
	//
	// `field` is the resolved `name` row. The resolver drops it when a decided provider
	// has no stored spelling; the page then renders canonical and this line renders
	// nothing — a re-enrich restores the row. Tokens only; QA 3 skins.
	import { tick, untrack } from 'svelte';
	import type { DecisionSource, ResolvedField } from '$lib/types';
	import { expandedField } from '$lib/expandedField.svelte';
	import { isProviderSource, providerOf } from '$lib/f36';
	import SourceBadge from '../curation/SourceBadge.svelte';

	let {
		field,
		canonical,
		isOwner,
		decide,
		onRename,
		renameLabel = 'Rename',
		prefix = 'In files as'
	}: {
		field: ResolvedField | undefined;
		canonical: string;
		isOwner: boolean;
		decide: (source: DecisionSource, manualValue?: string) => Promise<void>;
		// Opens the heading's rename form (the page passes `() => nameControl.open()`);
		// with it, the record spelling on the line is a button for the owner.
		onRename?: () => void;
		// Accessible name for that button, e.g. "Rename this person".
		renameLabel?: string;
		// "In files as" for the kinds whose record spelling comes from file tags
		// (person, studio); a film's title is owner-asserted, never read from a file, so
		// it passes "On record as".
		prefix?: string;
	} = $props();

	const display = $derived(field?.values[0] ?? '');
	const differs = $derived(display !== '' && display !== canonical);
	const open = $derived(expandedField.isOpen('name'));

	// The link names the first provider spelling that differs from the record — "Display
	// as Ana Keßler (tmdb)…" — so a reader with no context can see another spelling is on
	// offer at all. QA 4.3's first pass (2026-09-15) reached for the provider's Refresh
	// button instead: at rest nothing else on the page says a second spelling exists.
	// With no such candidate the link stays the bare "Display as…" (Custom is still there).
	const offer = $derived(
		(field?.candidates ?? []).find(
			(c) => isProviderSource(c.source) && c.value.trim() !== '' && c.value !== canonical
		)
	);
	const linkLabel = $derived(
		offer ? `Display as ${offer.value} (${offer.provider || providerOf(offer.source)})…` : 'Display as…'
	);

	let lineEl = $state<HTMLDivElement | null>(null);
	let linkEl = $state<HTMLButtonElement | null>(null);

	// The link and the badge occupy the same slot, so opening replaces the focused link
	// with the badge (and closing does the reverse) — hand focus across the swap so a
	// keyboard owner is never dropped on <body>. Only when nothing differs: once a
	// decision stands the badge is always mounted and SourceBadge's own close() refocuses it.
	let wasOpen = false;
	$effect(() => {
		const now = open;
		const swap = untrack(() => !differs);
		if (swap && now !== wasOpen) {
			tick().then(() => {
				if (now) lineEl?.querySelector<HTMLElement>('[data-source-badge="name"] button')?.focus();
				else linkEl?.focus();
			});
		}
		wasOpen = now;
	});
</script>

{#if field && (differs || (isOwner && open))}
	<div bind:this={lineEl} class="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted">
		{#if differs}
			<span>
				{prefix}
				{#if isOwner && onRename}
					<button
						type="button"
						aria-label={`${renameLabel} — in files as ${canonical}`}
						onclick={onRename}
						class="wrap-anywhere font-mono text-ink underline decoration-dotted underline-offset-2 hover:text-accent focus-visible:text-accent"
					>
						{canonical}
					</button>
				{:else}
					<span class="wrap-anywhere font-mono text-ink">{canonical}</span>
				{/if}
			</span>
		{/if}
		{#if isOwner}
			<SourceBadge {field} baselineKey="record" showValue={false} {decide} />
			{#if open}
				<p class="w-full">
					Changes how the name is shown here and in search. Files, aliases, and writeback keep
					the record spelling.
				</p>
			{/if}
		{/if}
	</div>
{:else if field && isOwner}
	<button
		bind:this={linkEl}
		type="button"
		class="btn-quiet mt-1 text-xs"
		onclick={() => expandedField.expand('name')}
	>
		{linkLabel}
	</button>
{/if}
