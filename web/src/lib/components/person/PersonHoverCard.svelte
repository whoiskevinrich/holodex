<script lang="ts">
	// The person hover card (F68, HOLODEX-431, design handoff § The card): mounted
	// only by PersonLinkChip, which owns open/close and placement. Header block
	// (headshot + name + meta + aliases) is the profile link for everyone — the
	// owner's edit path; the F65.8 CompletenessRing beside the name is the card's
	// one action and sits as a SIBLING of that link (a button may not nest in an
	// anchor); the link row is Videos · Films · provider badges. Absent facts leave
	// no trace (no "—"), and `card` null is the loading state: name line only.
	import type { PersonCard } from '$lib/types';
	import { videoCount, sortExternalLinks } from '$lib/format';
	import PersonImageFrame from './PersonImageFrame.svelte';
	import NationalityFlags from './NationalityFlags.svelte';
	import CompletenessRing from '$lib/components/completeness/CompletenessRing.svelte';
	import ProviderLinkBadge from '$lib/components/enrichment/ProviderLinkBadge.svelte';
	import { invalidatePersonCard } from './personCard.svelte';

	let {
		id,
		name,
		card,
		cardId,
		onrefreshed
	}: {
		id: number;
		/** The chip's own name — drawn while `card` is still loading. */
		name: string;
		card: PersonCard | null;
		cardId: string;
		/** The ring refreshed the person: the chip re-fetches and replaces `card`. */
		onrefreshed?: () => void;
	} = $props();

	const shownName = $derived(card?.display_name ?? card?.name ?? name);
	const meta = $derived.by(() => {
		if (!card) return [];
		const parts: string[] = [];
		if (card.age !== undefined) parts.push(String(card.age));
		else if (card.age_at_death !== undefined) parts.push(`†${card.age_at_death}`);
		parts.push(videoCount(card.video_count));
		if (card.film_count > 0) parts.push(`${card.film_count} film${card.film_count === 1 ? '' : 's'}`);
		return parts;
	});
	const aliases = $derived(card?.aliases?.slice(0, 3) ?? []);
	const links = $derived(sortExternalLinks(card?.external_links ?? []));
</script>

<!-- The typography resets matter: a trigger can live inside a heading (RelatedShelf's
     uppercase tracking-wide h2) and the card must not inherit its case, tracking or weight. -->
<div
	id={cardId}
	role="group"
	aria-label={shownName}
	class="person-hover-card w-72 max-w-[calc(100vw-2rem)] rounded-theme border border-rule bg-surface p-3 text-left text-sm font-normal normal-case tracking-normal text-ink shadow-lg"
>
	<div class="flex items-start gap-3">
		<!-- Named group: the tile that hosts this card is itself a `.group`, and a bare
		     `group-hover:` here would colour the name whenever the tile is hovered. -->
		<a href={`/people/${id}`} class="group/card flex min-w-0 flex-1 gap-3">
			{#if card}
				<PersonImageFrame
					personId={id}
					role="headshot"
					name={shownName}
					version={card.headshot_version}
					frameClass="portrait-frame--1x1 w-12"
					eager
				/>
			{:else}
				<!-- Loading: the empty frame holds the slot so nothing shifts when the image
				     lands; requesting the headshot before its version is known would fetch it twice. -->
				<span class="portrait-frame portrait-frame--1x1 w-12 shrink-0" aria-hidden="true"></span>
			{/if}
			<span class="min-w-0 flex-1">
				<span class="flex items-center gap-1.5">
					<span class="skin-title truncate font-display text-sm font-semibold group-hover/card:text-accent">
						{shownName}
					</span>
					{#if card?.nationality?.length}
						<NationalityFlags values={card.nationality} />
					{/if}
				</span>
				{#if meta.length}
					<span class="block text-xs text-muted">{meta.join(' · ')}</span>
				{/if}
				{#if aliases.length}
					<span class="block truncate text-xs italic text-muted">also credited as {aliases.join(', ')}</span>
				{/if}
			</span>
		</a>
		{#if card?.completeness}
			<!-- Owner-only by payload (F65.5); a sibling of the header link (F65.8). -->
			<span class="shrink-0 pt-0.5">
				<CompletenessRing
					required={card.completeness.required}
					extras={card.completeness.extras}
					size="row"
					entity={{ kind: 'person', id }}
					onrefreshed={() => {
						invalidatePersonCard(id);
						onrefreshed?.();
					}}
				/>
			</span>
		{/if}
	</div>
	{#if card}
		<div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-rule pt-2 text-xs">
			<a href={`/people/${id}#videos`} class="text-accent hover:text-ink">Videos</a>
			{#if card.film_count > 0}
				<a href={`/people/${id}#films`} class="text-accent hover:text-ink">Films</a>
			{/if}
			{#each links as link (link.provider)}
				<ProviderLinkBadge {link} entityName={shownName} />
			{/each}
		</div>
	{/if}
</div>
